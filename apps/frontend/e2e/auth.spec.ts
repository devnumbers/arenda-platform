import { expect, extractLoginCode, loginViaUi, test } from './fixtures';

// Smoke of the real login flow (ticket #456): the seeded owner enters the
// phone, the code arrives in the backend log (fake email sender), and the
// cabinet opens on the properties list. Validates the whole chain — UI
// form, /api proxy, auth endpoints, session cookie, middleware redirect —
// before payment screens start relying on this infrastructure.
test('вход по коду из письма ведёт в кабинет', async ({ page, seededUser }) => {
  await loginViaUi(page, seededUser);
});

// The redesigned code step (#765): a wrong code shows the mockup's inline
// «Неверный код» in the field (no toast), the field's × resets value and
// error, and the correct code then logs in through the same autosubmit.
test('неверный код — инлайн в поле, крестик сбрасывает', async ({ page, seededUser }) => {
  await page.goto('/login');
  await page.getByRole('textbox', { name: 'Телефон' }).fill(seededUser.phoneDigits);
  await page.getByRole('button', { name: 'Войти' }).click();
  await expect(page.getByRole('heading', { name: 'Введите код' })).toBeVisible();

  await page.getByRole('textbox', { name: 'Код' }).fill('000000');
  await expect(page.getByText('Неверный код')).toBeVisible();

  await page.getByRole('button', { name: 'Очистить поле' }).click();
  await expect(page.getByText('Неверный код')).toBeHidden();
  await expect(page.getByRole('textbox', { name: 'Код' })).toHaveValue('');

  const code = await extractLoginCode(seededUser);
  await page.getByRole('textbox', { name: 'Код' }).fill(code);
  await page.waitForURL('**/properties');
});

// The ← in the top bar returns to the previous step: the seeded user logged
// in by phone only (no email in the draft), so back means the phone step.
test('стрелка назад ведёт на предыдущий шаг', async ({ page, seededUser }) => {
  await page.goto('/login');
  await page.getByRole('textbox', { name: 'Телефон' }).fill(seededUser.phoneDigits);
  await page.getByRole('button', { name: 'Войти' }).click();
  await expect(page.getByRole('heading', { name: 'Введите код' })).toBeVisible();

  // Resend-канон #733: the tile is disabled with the countdown caption while
  // the 60 s cooldown from retryAfter runs. The mockup has no length counter
  // under the field (walking back a native-maxLength regression, #765).
  await expect(page.getByRole('button', { name: 'Отправить новый код' })).toBeDisabled();
  await expect(page.getByText(/Запросить новый код можно через/)).toBeVisible();
  await expect(page.getByText(/\d\/6/)).toHaveCount(0);

  await page.getByRole('button', { name: 'Назад' }).click();
  await expect(page.getByRole('heading', { name: 'Введите номер телефона' })).toBeVisible();
});
