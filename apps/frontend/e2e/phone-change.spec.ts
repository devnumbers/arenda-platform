import {
  captureScreen,
  openCabinetWithSeededSession,
  test,
  expect,
} from './fixtures';

// The phone-change flow on /profile/account/phone. The code travels through
// the fake SMS sender (no email-log channel), so unlike email-change.spec
// this spec runs the flow fully route-mocked and on virtual time: the
// send-code call is intercepted, and the resend tile (#733, mockup
// 1869-68137) is driven with page.clock — «00:59» right after the send,
// unlocked only after a clock jump, field cleared and timer restarted by a
// successful resend. The entry/step layout of the screen itself is covered
// by profile-tree.spec.
const PHONE_FIELD = { name: 'Новый телефон' };
const CONTINUE_BUTTON = { name: 'Продолжить' };
const CODE_FIELD = { name: 'Код', exact: true };
const RESEND_BUTTON = { name: 'Отправить новый код' };

test.use({ viewport: { width: 390, height: 844 } });

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
  await captureScreen(page, test.info(), '01-phone-code-tile');
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
