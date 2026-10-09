import type { Page } from '@playwright/test';
import {
  captureScreen,
  countCodesSentTo,
  extractCodeSentTo,
  openCabinetWithSeededSession,
  test,
  expect,
  type SeededUser,
} from './fixtures';

// The email-change flow (map #723, ticket #722) on /profile/account/email:
// the entry row on the account screen, the double confirmation (a code to
// the current address — verified server-side by verify-current, which issues
// the grant; then the new address bound to the grant receives its own code),
// and the success sheet. The codes travel through the fake email sender into
// the backend log, same channel as the login spec. The address is swapped
// and swapped back in one test: the suite shares one seeded user, and the
// other specs expect its original email.
//
// Budget note: each leg of the swap sends one code to the address being
// changed TO — at that moment a "new" address for the user — so a run
// consumes 2 of the backend's new-address sends. The budget is 5/hour per
// user (burst 3, RATE_LIMIT_EMAIL_CHANGE_SEND_PER_HOUR) held in the backend
// process memory. That is safe only because the e2e harness starts a fresh
// backend per run; a long-lived backend process would 429 the next runs.
// The resend-tile test below is fully route-mocked (send-code,
// verify-current, request-new-email-code, resend-code) and spends none of
// that budget.
//
// Throttle note for the live test: the leg-2 «Назад» block re-sends a
// step-1 code through the real backend, whose minSendInterval throttles
// re-issuance for the (user, address, step) triple to one per minute — the
// same minute the tile's caption counts down. The test waits out the tile
// unlock instead of poking the backend early. Every address's (address,
// step) pairs inside one run stay ≥60s apart on their own, so no other
// send needs waiting.
const NEW_EMAIL = 'e2e-email-change@example.com';

const CODE_FIELD = { name: 'Код', exact: true };
const CONTINUE_BUTTON = { name: 'Продолжить' };
const RESEND_BUTTON = { name: 'Отправить новый код' };

/**
 * Enters the flow from the account screen and waits for the auto-sent code
 * to appear in the backend log (the entry step is «Подтвердите текущую
 * почту» for the address the code went to). The baseline count is taken
 * before the click, so a stale send to the same address (the login spec,
 * the previous leg) is never mistaken for this one.
 */
async function openEmailChangeFlow(
  page: Page,
  user: SeededUser,
  currentEmail: string,
): Promise<string> {
  const codesBefore = await countCodesSentTo(user, currentEmail);
  await page.getByRole('link', { name: currentEmail }).click();
  await expect(page.getByRole('heading', { name: 'Подтвердите текущую почту' })).toBeVisible();
  await expect(page.getByText(`на вашу почту ${currentEmail}`, { exact: false })).toBeVisible();
  return extractCodeSentTo(user, currentEmail, codesBefore);
}

/** Fills the active code field and submits it with «Продолжить». */
async function submitCode(page: Page, code: string): Promise<void> {
  await page.getByRole('textbox', CODE_FIELD).fill(code);
  await page.getByRole('button', CONTINUE_BUTTON).click();
}

test.use({ viewport: { width: 390, height: 844 } });

test('смена почты двойным кодом и возврат адреса назад', async ({ page, seededUser }) => {
  // The leg-2 «Назад» block waits out the 1-minute server resend throttle.
  test.setTimeout(240_000);

  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/profile/account');
  await expect(page.getByRole('link', { name: seededUser.email })).toBeVisible();
  await captureScreen(page, test.info(), '01-account-entry');

  // --- Leg 1: e2e@example.com → NEW_EMAIL --------------------------------
  const currentCode = await openEmailChangeFlow(page, seededUser, seededUser.email);
  await captureScreen(page, test.info(), '02-code-current');

  // Step 1 is verified server-side right away (#1206): a wrong code on
  // «Подтвердите текущую почту» surfaces INLINE in the field (mockup
  // 2343-51004), not as a toast — the same canonical behavior the new-code
  // step shows below. One failure is far below the 15-failure window.
  const wrongCurrentCode = currentCode === '000000' ? '111111' : '000000';
  await submitCode(page, wrongCurrentCode);
  await expect(page.getByText('Неверный код', { exact: true })).toBeVisible();
  await expect(page.getByText('Не удалось изменить электронную почту')).toHaveCount(0);
  await captureScreen(page, test.info(), '03-step1-wrong-inline');

  await submitCode(page, currentCode);
  await expect(page.getByRole('heading', { name: 'Введите новую почту' })).toBeVisible();

  // Same-as-current is a client-side error: inline message, «Продолжить»
  // stays disabled (#720 Q4).
  await page.getByRole('textbox', { name: 'Электронная почта' }).fill(seededUser.email);
  await expect(page.getByText('Новый адрес совпадает с текущим')).toBeVisible();
  await expect(page.getByRole('button', CONTINUE_BUTTON)).toBeDisabled();

  await page.getByRole('textbox', { name: 'Электронная почта' }).fill(NEW_EMAIL);
  await captureScreen(page, test.info(), '04-new-email');
  await page.getByRole('button', CONTINUE_BUTTON).click();

  await expect(page.getByRole('heading', { name: 'Подтвердите новую почту' })).toBeVisible();
  await expect(page.getByText(`на вашу почту ${NEW_EMAIL}`, { exact: false })).toBeVisible();

  // The resend tile waits out the 60s server throttling (#733, mockup
  // 1869-68137): disabled with the countdown caption while the timer runs.
  const resendButton = page.getByRole('button', RESEND_BUTTON);
  await expect(resendButton).toBeDisabled();
  await expect(page.getByText(/Запросить новый код можно через 00:5/)).toBeVisible();

  // One wrong attempt: the 401 «Неверный код» surfaces INLINE in the code
  // field (mockup 2343-51004), not as a toast; typing replaces it. A single
  // failure is far below the 15-failure attempt-window block.
  const newCode = await extractCodeSentTo(seededUser, NEW_EMAIL);
  const wrongCode = newCode === '000000' ? '111111' : '000000';
  await submitCode(page, wrongCode);
  await expect(page.getByText('Неверный код', { exact: true })).toBeVisible();
  await expect(page.getByText('Не удалось изменить электронную почту')).toHaveCount(0);
  await captureScreen(page, test.info(), '05-code-new-wrong');

  await page.getByRole('textbox', CODE_FIELD).fill(newCode);
  await expect(page.getByText('Неверный код', { exact: true })).toBeHidden();
  await page.getByRole('button', CONTINUE_BUTTON).click();
  await expect(
    page.getByRole('heading', { name: `Новая электронная почта ${NEW_EMAIL}` }),
  ).toBeVisible();
  await captureScreen(page, test.info(), '06-success');

  await page.getByRole('button', { name: 'Хорошо' }).click();
  await expect(page.getByRole('link', { name: NEW_EMAIL })).toBeVisible();

  // --- Leg 2: revert NEW_EMAIL → e2e@example.com -------------------------
  const revertCode = await openEmailChangeFlow(page, seededUser, NEW_EMAIL);
  await submitCode(page, revertCode);
  await expect(page.getByRole('heading', { name: 'Введите новую почту' })).toBeVisible();

  // The leg-2 «Назад» contract (#1202, #1206): stepping back from the
  // address step leaves the step-1 code burned by verify-current, so the
  // same code now fails server-side — and the fresh one comes from the
  // step-1 resend tile; re-verifying re-issues the grant.
  await page.getByRole('textbox', { name: 'Электронная почта' }).fill(seededUser.email);
  await page.getByRole('button', { name: 'Назад' }).click();
  await expect(page.getByRole('heading', { name: 'Подтвердите текущую почту' })).toBeVisible();

  // The draft code is still in the field; submitting it hits the burned
  // code: inline «Неверный код» again, the flow stays on step 1.
  await page.getByRole('button', CONTINUE_BUTTON).click();
  await expect(page.getByText('Неверный код', { exact: true })).toBeVisible();
  await expect(page.getByText('Не удалось изменить электронную почту')).toHaveCount(0);
  await captureScreen(page, test.info(), '07-back-burned-inline');

  // The step-1 tile is still inside the 1-minute server throttle of the
  // auto-sent code: wait out its unlock, then take the fresh code.
  const revertResendButton = page.getByRole('button', RESEND_BUTTON);
  await expect(revertResendButton).toBeEnabled({ timeout: 75_000 });
  const revertCodesOnNewEmail = await countCodesSentTo(seededUser, NEW_EMAIL);
  await revertResendButton.click();
  const revertResendCode = await extractCodeSentTo(seededUser, NEW_EMAIL, revertCodesOnNewEmail);
  // Real time between the click and the assert — any second of «00:5x».
  await expect(page.getByText(/Запросить новый код можно через 00:5[789]/)).toBeVisible();
  await expect(page.getByRole('textbox', CODE_FIELD)).toHaveValue('');
  await captureScreen(page, test.info(), '08-back-resend-reissued');

  // The re-issued code re-opens the address step (a fresh grant): the
  // drafted revert address survived the step back — «Продолжить» goes on
  // without re-typing.
  await submitCode(page, revertResendCode);
  await expect(page.getByRole('heading', { name: 'Введите новую почту' })).toBeVisible();
  await expect(page.getByRole('textbox', { name: 'Электронная почта' })).toHaveValue(seededUser.email);
  const revertCodesBefore = await countCodesSentTo(seededUser, seededUser.email);
  await page.getByRole('button', CONTINUE_BUTTON).click();

  await expect(page.getByRole('heading', { name: 'Подтвердите новую почту' })).toBeVisible();
  const revertNewCode = await extractCodeSentTo(seededUser, seededUser.email, revertCodesBefore);
  await submitCode(page, revertNewCode);
  await expect(
    page.getByRole('heading', { name: `Новая электронная почта ${seededUser.email}` }),
  ).toBeVisible();
  await captureScreen(page, test.info(), '09-reverted');

  await page.getByRole('button', { name: 'Хорошо' }).click();
  await expect(page.getByRole('link', { name: seededUser.email })).toBeVisible();
});

// The resend tile of the «Подтвердите новую почту» step (#733) on virtual
// time: every identity call is route-mocked, so the backend budget is
// untouched, and page.clock freezes Date.now — the countdown deterministically
// shows «00:59» right after the send and unlocks only after a clock jump.
// Real backend semantics of resend-code live in the backend suite (#732).
test('resend-плитка шага новой почты: таймер, разблокировка, очистка поля', async ({ page, seededUser }) => {
  let resendCalls = 0;
  await page.route('**/api/me/email/send-code', (route) => route.fulfill({ status: 204 }));
  await page.route('**/api/me/email/verify-current', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ grant: 'e2e-grant-token' }),
    }));
  await page.route('**/api/me/email/request-new-email-code', (route) =>
    route.fulfill({ status: 204 }));
  await page.route('**/api/me/email/resend-code', (route) => {
    resendCalls += 1;
    return route.fulfill({ status: 204 });
  });
  await page.clock.install();
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/profile/account/email');

  await expect(page.getByRole('heading', { name: 'Подтвердите текущую почту' })).toBeVisible();
  await submitCode(page, '000000');
  await page.getByRole('textbox', { name: 'Электронная почта' }).fill(NEW_EMAIL);
  await page.getByRole('button', CONTINUE_BUTTON).click();

  await expect(page.getByRole('heading', { name: 'Подтвердите новую почту' })).toBeVisible();
  const resendButton = page.getByRole('button', RESEND_BUTTON);
  await expect(resendButton).toBeDisabled();
  await expect(page.getByText('Запросить новый код можно через 00:59')).toBeVisible();

  await page.clock.fastForward(62_000);
  await expect(resendButton).toBeEnabled();
  await expect(page.getByText(/Запросить новый код можно через/)).toHaveCount(0);

  await resendButton.click();
  await expect(resendButton).toBeDisabled();
  await expect(page.getByText('Запросить новый код можно через 00:59')).toBeVisible();
  await expect(page.getByRole('textbox', CODE_FIELD)).toHaveValue('');
  expect(resendCalls).toBe(1);
});

// The client side of the «Назад» contract (#1202, #1206): the step-1 resend
// tile issues a fresh code, the re-verification's grant — not the one from
// before the step back — rides on request-new-email-code with the drafted,
// not re-typed address. Route-mocked with virtual time like the tile test
// above: the server-side truth of burned codes and grant re-issuance lives
// in the backend suite (#1203) and in the live swap test's leg 2.
test('Назад с шага адреса: resend шага 1, повторная проверка — новый грант в request-new-email-code', async ({ page, seededUser }) => {
  let verifyCalls = 0;
  const requestBodies: Array<{ grant: string; newEmail: string }> = [];
  await page.route('**/api/me/email/send-code', (route) => route.fulfill({ status: 204 }));
  await page.route('**/api/me/email/verify-current', (route) => {
    verifyCalls += 1;
    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ grant: `grant-${verifyCalls}` }),
    });
  });
  await page.route('**/api/me/email/request-new-email-code', (route) => {
    requestBodies.push(route.request().postDataJSON() as { grant: string; newEmail: string });
    return route.fulfill({ status: 204 });
  });
  await page.route('**/api/me/email/resend-code', (route) => route.fulfill({ status: 204 }));
  await page.clock.install();
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/profile/account/email');

  await expect(page.getByRole('heading', { name: 'Подтвердите текущую почту' })).toBeVisible();
  await submitCode(page, '000000');
  await page.getByRole('textbox', { name: 'Электронная почта' }).fill(NEW_EMAIL);
  await page.getByRole('button', CONTINUE_BUTTON).click();
  await expect(page.getByRole('heading', { name: 'Подтвердите новую почту' })).toBeVisible();
  expect(requestBodies).toEqual([{ grant: 'grant-1', newEmail: NEW_EMAIL }]);

  // Two steps back: «Подтвердите новую почту» → the address step (the
  // drafted address survives) → the burned step 1.
  const backButton = page.getByRole('button', { name: 'Назад' });
  await backButton.click();
  await expect(page.getByRole('heading', { name: 'Введите новую почту' })).toBeVisible();
  await expect(page.getByRole('textbox', { name: 'Электронная почта' })).toHaveValue(NEW_EMAIL);
  await backButton.click();
  await expect(page.getByRole('heading', { name: 'Подтвердите текущую почту' })).toBeVisible();

  // The step-1 tile is the way back in: virtual-time jump past its
  // countdown, resend clears the field and restarts the timer.
  const step1Resend = page.getByRole('button', RESEND_BUTTON);
  await page.clock.fastForward(62_000);
  await expect(step1Resend).toBeEnabled();
  await step1Resend.click();
  await expect(step1Resend).toBeDisabled();
  await expect(page.getByText('Запросить новый код можно через 00:59')).toBeVisible();
  await expect(page.getByRole('textbox', CODE_FIELD)).toHaveValue('');

  // The re-verified code issues grant-2, which — not grant-1 — rides with
  // the same drafted address on request-new-email-code.
  await submitCode(page, '000000');
  await expect(page.getByRole('heading', { name: 'Введите новую почту' })).toBeVisible();
  await expect(page.getByRole('textbox', { name: 'Электронная почта' })).toHaveValue(NEW_EMAIL);
  await page.getByRole('button', CONTINUE_BUTTON).click();
  await expect(page.getByRole('heading', { name: 'Подтвердите новую почту' })).toBeVisible();

  expect(requestBodies).toEqual([
    { grant: 'grant-1', newEmail: NEW_EMAIL },
    { grant: 'grant-2', newEmail: NEW_EMAIL },
  ]);
  expect(verifyCalls).toBe(2);
});
