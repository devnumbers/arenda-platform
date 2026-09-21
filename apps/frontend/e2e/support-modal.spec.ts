import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  test,
} from './fixtures';

// Единая поверхность поддержки — модалка «Связаться с нами» (карта #761,
// тикет #766): страница /support снесена, пилюля десктопа, шестая ячейка
// шита «Еще», кнопка экрана «Тариф» и пилюли «Написать в поддержку» шагов
// входа открывают одну и ту же модалку (Telegram-заглушка t.me/swirnowwwivan,
// почта hello@rentlee.ru с копированием в буфер).

test('модалка открывается с пилюли «Написать в поддержку» шага входа', async ({ page }, testInfo) => {
  await page.goto('/login');
  await page.getByRole('button', { name: 'Написать в поддержку' }).click();

  const dialog = page.getByRole('dialog', { name: 'Связаться с нами' });
  await expect(dialog).toBeVisible();
  await expect(page.getByText('Напишите нам в Телеграм или на почту')).toBeVisible();

  // Telegram — внешняя ссылка в новой вкладке; адрес — заглушка из
  // shared/config/support.ts (владелец заменит).
  const telegram = dialog.getByRole('link', { name: 'Написать в Телеграм' });
  await expect(telegram).toHaveAttribute('href', 'https://t.me/swirnowwwivan');
  await expect(telegram).toHaveAttribute('target', '_blank');

  // Почта — копирование в буфер: иконка/подпись сменяются на «Скопировано»
  // на 2 с (канон карточки контакта); на вьюпорте ≥768 есть крестик.
  const copy = dialog.getByRole('button', { name: /Скопировать/ });
  await expect(copy).toContainText('hello@rentlee.ru');
  await copy.click();
  await expect(dialog.getByRole('button', { name: 'Скопировано' })).toBeVisible();

  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();

  await captureScreen(page, testInfo, 'support-modal-login');
});

test('пилюля «Поддержка» кабинета открывает модалку', async ({ page, seededUser }, testInfo) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/properties');

  await page.getByRole('button', { name: 'Поддержка' }).click();
  const dialog = page.getByRole('dialog', { name: 'Связаться с нами' });
  await expect(dialog).toBeVisible();

  await captureScreen(page, testInfo, 'support-modal-pill');
});

test.describe('мобайл 390', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('шестая ячейка шита «Еще» открывает модалку', async ({ page, seededUser }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/properties');

    await page.getByRole('button', { name: 'Еще' }).click();
    await page.getByRole('button', { name: 'Поддержка' }).click();

    // Шит закрылся, модалка открылась поверх страницы.
    const dialog = page.getByRole('dialog', { name: 'Связаться с нами' });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole('button', { name: /Скопировать/ })).toBeVisible();

    await captureScreen(page, testInfo, 'support-modal-more-sheet');
  });
});

test('страницы /support больше нет — обычный 404', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);

  const response = await page.goto('/support');
  expect(response?.status()).toBe(404);
});
