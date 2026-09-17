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
// the current address, then the new address — one grant in between), and
// the success sheet. The codes travel through the fake email sender into
// the backend log, same channel as the login spec. The address is swapped
// and swapped back in one test: the suite shares one seeded user, and the
// other specs expect its original email.
//
// Budget note: this spec sends codes to NEW addresses 4 times per run
// (both legs × 2), and the backend budget on such sends is 5/hour per
// user held in the backend process memory (RATE_LIMIT_EMAIL_CHANGE_SEND_
// PER_HOUR). That is safe only because the e2e harness starts a fresh
// backend per run; a long-lived backend process would 429 the fifth send.
const NEW_EMAIL = 'e2e-email-change@example.com';

const CODE_FIELD = { name: 'Код', exact: true };
const CONTINUE_BUTTON = { name: 'Продолжить' };

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
  await expect(page.getByText(`на почту ${currentEmail}`, { exact: false })).toBeVisible();
  return extractCodeSentTo(user, currentEmail, codesBefore);
}

/** Fills the active code field and submits it with «Продолжить». */
async function submitCode(page: Page, code: string): Promise<void> {
  await page.getByRole('textbox', CODE_FIELD).fill(code);
  await page.getByRole('button', CONTINUE_BUTTON).click();
}

test.use({ viewport: { width: 390, height: 844 } });

test('смена почты двойным кодом и возврат адреса назад', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/profile/account');
  await expect(page.getByRole('link', { name: seededUser.email })).toBeVisible();
  await captureScreen(page, test.info(), '01-account-entry');

  // --- Leg 1: e2e@example.com → NEW_EMAIL --------------------------------
  const currentCode = await openEmailChangeFlow(page, seededUser, seededUser.email);
  await captureScreen(page, test.info(), '02-code-current');

  await submitCode(page, currentCode);
  await expect(page.getByRole('heading', { name: 'Введите новую почту' })).toBeVisible();

  // Same-as-current is a client-side error: inline message, «Продолжить»
  // stays disabled (#720 Q4).
  await page.getByRole('textbox', { name: 'Электронная почта' }).fill(seededUser.email);
  await expect(page.getByText('Новый адрес совпадает с текущим')).toBeVisible();
  await expect(page.getByRole('button', CONTINUE_BUTTON)).toBeDisabled();

  await page.getByRole('textbox', { name: 'Электронная почта' }).fill(NEW_EMAIL);
  await captureScreen(page, test.info(), '03-new-email');
  await page.getByRole('button', CONTINUE_BUTTON).click();

  await expect(page.getByRole('heading', { name: 'Подтвердите новую почту' })).toBeVisible();
  await expect(page.getByText(`на почту ${NEW_EMAIL}`, { exact: false })).toBeVisible();

  // One wrong attempt first: the backend detail surfaces as a toast
  // («Неверный код»); a single failure is far below the 15-failure
  // attempt-window block.
  const newCode = await extractCodeSentTo(seededUser, NEW_EMAIL);
  const wrongCode = newCode === '000000' ? '111111' : '000000';
  await submitCode(page, wrongCode);
  await expect(page.getByText('Неверный код', { exact: true })).toBeVisible();
  await captureScreen(page, test.info(), '04-code-new-wrong');

  await submitCode(page, newCode);
  await expect(
    page.getByRole('heading', { name: `Электронная почта изменена на ${NEW_EMAIL}` }),
  ).toBeVisible();
  await captureScreen(page, test.info(), '05-success');

  await page.getByRole('button', { name: 'Хорошо' }).click();
  await expect(page.getByRole('link', { name: NEW_EMAIL })).toBeVisible();

  // --- Leg 2: revert NEW_EMAIL → e2e@example.com -------------------------
  const revertCode = await openEmailChangeFlow(page, seededUser, NEW_EMAIL);
  await submitCode(page, revertCode);
  await expect(page.getByRole('heading', { name: 'Введите новую почту' })).toBeVisible();

  const revertCodesBefore = await countCodesSentTo(seededUser, seededUser.email);
  await page.getByRole('textbox', { name: 'Электронная почта' }).fill(seededUser.email);
  await page.getByRole('button', CONTINUE_BUTTON).click();

  await expect(page.getByRole('heading', { name: 'Подтвердите новую почту' })).toBeVisible();
  const revertNewCode = await extractCodeSentTo(seededUser, seededUser.email, revertCodesBefore);
  await submitCode(page, revertNewCode);
  await expect(
    page.getByRole('heading', { name: `Электронная почта изменена на ${seededUser.email}` }),
  ).toBeVisible();
  await captureScreen(page, test.info(), '06-reverted');

  await page.getByRole('button', { name: 'Хорошо' }).click();
  await expect(page.getByRole('link', { name: seededUser.email })).toBeVisible();
});
