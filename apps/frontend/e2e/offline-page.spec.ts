import { captureScreen, expect, openCabinetWithSeededSession, test } from './fixtures';

// Офлайн-экран по макету (карта #779, тикет #783): public/offline.html
// отдаётся из кэша SW при выключенной сети, поэтому страница самодостаточна
// (лого инлайном, стили/JS инлайном). Контакты поддержки — канон #766:
// Telegram-ссылка + почта с тап-копированием и подсказкой «Скопировано».

test('офлайн-страница: заголовок, кнопки, поддержка с копированием', async ({ page }, testInfo) => {
  await page.goto('/offline.html');

  await expect(page.getByRole('heading', { name: 'Нет соединения' })).toBeVisible();
  await expect(page.getByText('Проверьте подключение к интернету и попробуйте снова')).toBeVisible();

  // Блок поддержки скрыт до тапа «Обратиться в поддержку».
  const telegram = page.getByRole('link', { name: 'Написать в Телеграм' });
  await expect(telegram).toBeHidden();

  await page.getByRole('button', { name: 'Обратиться в поддержку' }).click();
  await expect(telegram).toBeVisible();
  // Адрес — заглушка из shared/config/support.ts (владелец заменит).
  await expect(telegram).toHaveAttribute('href', 'https://t.me/swirnowwwivan');
  await expect(telegram).toHaveAttribute('target', '_blank');

  // Почта — копирование в буфер: иконка/подпись сменяются на «Скопировано»
  // на 2 с (канон карточки контакта), затем возвращаются.
  await page.context().grantPermissions(['clipboard-read', 'clipboard-write']);
  const copy = page.getByRole('button', { name: /Скопировать/ });
  await expect(copy).toContainText('hello@rentlee.ru');
  await copy.click();
  await expect(page.getByRole('button', { name: 'Скопировано' })).toBeVisible();
  await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe('hello@rentlee.ru');
  await expect(page.getByRole('button', { name: /Скопировать/ })).toBeVisible({ timeout: 5000 });

  // «Обновить страницу» перезагружает документ: маркер текущего окна после
  // перезагрузки исчезает (новый документ его не ставил).
  await page.evaluate(() => {
    (window as { offlineE2eMarker?: boolean }).offlineE2eMarker = true;
  });
  await page.getByRole('button', { name: 'Обновить страницу' }).click();
  await expect.poll(() => page.evaluate(() => (window as { offlineE2eMarker?: boolean }).offlineE2eMarker)).toBeFalsy();

  await captureScreen(page, testInfo, 'offline-page');
});

test.describe('мобайл 390', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('офлайн-страница на мобайле: раскрытая поддержка', async ({ page }, testInfo) => {
    await page.goto('/offline.html');

    await expect(page.getByRole('heading', { name: 'Нет соединения' })).toBeVisible();
    await page.getByRole('button', { name: 'Обратиться в поддержку' }).click();
    await expect(page.getByRole('link', { name: 'Написать в Телеграм' })).toBeVisible();

    await captureScreen(page, testInfo, 'offline-page-mobile');
  });
});

test('SW отдаёт офлайн-страницу при потере сети', async ({ page, context, seededUser }) => {
  test.setTimeout(60_000);
  // SW регистрируется внутри ScreenLayout (кабинетные экраны), поэтому
  // нужен сеанс: /login вне кабинетного layout регистрации не делает.
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/properties');
  // ServiceWorkerRegister делает clients.claim — контроллер появляется
  // на текущей странице после install.
  await page.waitForFunction(() => navigator.serviceWorker.controller !== null, undefined, {
    timeout: 30_000,
  });

  await context.setOffline(true);
  await page.goto('/properties');
  await expect(page.getByRole('heading', { name: 'Нет соединения' })).toBeVisible();

  await context.setOffline(false);
});
