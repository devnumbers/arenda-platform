import type { Page } from '@playwright/test';
import { formatPhoneDisplay } from '@/shared/lib/phone';
import {
  captureScreen,
  countCodesSentTo,
  extractCodeSentTo,
  openCabinetWithSeededSession,
  test,
  expect,
} from './fixtures';

// The phone-change flow on /profile/account/phone (#1205): the real run
// below reads the code from the backend log like email-change.spec — all
// identity letters travel the fake email sender, and the phone-change code
// carries its own per-case subject «Код для смены телефона в Рентли»
// (#1204), which is what this spec pins live. The flow swaps the seeded
// phone and swaps it back in one test (the suite shares one seeded user;
// the phone is the login, so the original number must survive the run).
// The resend tile (#733, mockup 1869-68137) is fully route-mocked and
// driven with page.clock — «00:59» right after the send, unlocked only
// after a clock jump, field cleared and timer restarted by a successful
// resend. The entry/step layout of the screen itself is covered by
// profile-tree.spec.
const NEW_PHONE_DIGITS = '9261112233';
const PHONE_CHANGE_SUBJECT = 'Код для смены телефона в Рентли';

const PHONE_FIELD = { name: 'Новый телефон' };
const CONTINUE_BUTTON = { name: 'Продолжить' };
const CODE_FIELD = { name: 'Код', exact: true };
const RESEND_BUTTON = { name: 'Отправить новый код' };

/** Форматированный номер из цифр сида — дисплей считается тем же
 * formatPhoneDisplay, что рисует экран, а не второй записью маски. */
function displayOf(phoneDigits: string): string {
  return formatPhoneDisplay(`+7${phoneDigits}`);
}

/** Fills the active code field and submits it with «Продолжить». */
async function submitCode(page: Page, code: string): Promise<void> {
  await page.getByRole('textbox', CODE_FIELD).fill(code);
  await page.getByRole('button', CONTINUE_BUTTON).click();
}

test.use({ viewport: { width: 390, height: 844 } });

test('смена телефона: письмо «смена телефона» на текущую почту, 401 inline, успех и возврат', async ({ page, seededUser }) => {
  const newPhoneDisplay = displayOf(NEW_PHONE_DIGITS);
  const seededPhoneDisplay = displayOf(seededUser.phoneDigits);

  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/profile/account');

  // --- Leg 1: +79150000001 → +79261112233 --------------------------------
  // Entry row on the account screen («Телефон», mockup 1789-99037) — the
  // same entry the email spec takes, so «Хорошо» lands back on the account.
  await page.getByRole('link', { name: seededPhoneDisplay }).click();
  await expect(page.getByRole('heading', { name: 'Введите новый номер телефона' })).toBeVisible();

  const codesBefore = await countCodesSentTo(seededUser, seededUser.email, PHONE_CHANGE_SUBJECT);
  await page.getByRole('textbox', PHONE_FIELD).fill(NEW_PHONE_DIGITS);
  await page.getByRole('button', CONTINUE_BUTTON).click();

  await expect(page.getByRole('heading', { name: 'Код отправлен на почту' })).toBeVisible();
  // The letter lands on the CURRENT email with the phone-change subject
  // (#1204): the subject-filtered extraction is the live check of that —
  // a login letter to the same address would never satisfy it.
  await captureScreen(page, test.info(), '01-phone-code');
  const code = await extractCodeSentTo(seededUser, seededUser.email, codesBefore, PHONE_CHANGE_SUBJECT);

  // One wrong attempt: the 401 «Неверный код» surfaces INLINE in the code
  // field (mockup 2343-51004), not as a toast; typing replaces it. A single
  // failure is far below the 15-failure attempt-window block.
  const wrongCode = code === '000000' ? '111111' : '000000';
  await submitCode(page, wrongCode);
  await expect(page.getByText('Неверный код', { exact: true })).toBeVisible();
  await expect(page.getByText('Не удалось изменить номер телефона')).toHaveCount(0);
  await captureScreen(page, test.info(), '02-phone-wrong-code-inline');

  await page.getByRole('textbox', CODE_FIELD).fill(code);
  await expect(page.getByText('Неверный код', { exact: true })).toBeHidden();
  await page.getByRole('button', CONTINUE_BUTTON).click();
  await expect(
    page.getByRole('heading', { name: `Новый номер телефона ${newPhoneDisplay}` }),
  ).toBeVisible();
  await captureScreen(page, test.info(), '03-phone-success');

  await page.getByRole('button', { name: 'Хорошо' }).click();
  await expect(page.getByText(newPhoneDisplay)).toBeVisible();

  // --- Leg 2: revert +79261112233 → +79150000001 -------------------------
  await page.getByRole('link', { name: newPhoneDisplay }).click();
  await expect(page.getByRole('heading', { name: 'Введите новый номер телефона' })).toBeVisible();

  const revertCodesBefore = await countCodesSentTo(seededUser, seededUser.email, PHONE_CHANGE_SUBJECT);
  await page.getByRole('textbox', PHONE_FIELD).fill(seededUser.phoneDigits);
  await page.getByRole('button', CONTINUE_BUTTON).click();

  await expect(page.getByRole('heading', { name: 'Код отправлен на почту' })).toBeVisible();
  const revertCode = await extractCodeSentTo(seededUser, seededUser.email, revertCodesBefore, PHONE_CHANGE_SUBJECT);
  await submitCode(page, revertCode);
  await expect(
    page.getByRole('heading', { name: `Новый номер телефона ${seededPhoneDisplay}` }),
  ).toBeVisible();

  await page.getByRole('button', { name: 'Хорошо' }).click();
  await expect(page.getByText(seededPhoneDisplay)).toBeVisible();
});

test('resend-плитка шага кода телефона: таймер, разблокировка, очистка поля', async ({ page, seededUser }) => {
  let sendCalls = 0;
  await page.route('**/api/me/phone/send-code', (route) => {
    sendCalls += 1;
    return route.fulfill({ status: 204 });
  });
  await page.clock.install();
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/profile/account/phone');

  await expect(page.getByRole('heading', { name: 'Введите новый номер телефона' })).toBeVisible();
  await page.getByRole('textbox', PHONE_FIELD).fill('9261112233');
  await page.getByRole('button', CONTINUE_BUTTON).click();

  await expect(page.getByRole('heading', { name: 'Код отправлен на почту' })).toBeVisible();
  await captureScreen(page, test.info(), '04-phone-code-tile');
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
  expect(sendCalls).toBe(2);
});
