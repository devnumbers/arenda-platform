import { captureScreen, expect, openCabinetWithSeededSession, test } from './fixtures';

// Офлайн-экран по макету (карта #779, тикет #783): public/offline.html
// отдаётся из кэша SW при выключенной сети, поэтому страница самодостаточна
// (лого инлайном, стили/JS инлайном). «Обратиться в поддержку» — прямая
// ссылка на Telegram из shared/config/support.ts (упрощение владельца
// 21.09: «на тг ссылку просто», без блока копирования).

test('офлайн-страница: заголовок, кнопки, поддержка', async ({ page }, testInfo) => {
  await page.goto('/offline.html');

  await expect(page.getByRole('heading', { name: 'Нет соединения' })).toBeVisible();
  await expect(page.getByText('Проверьте подключение к интернету и попробуйте снова')).toBeVisible();

  // «Обратиться в поддержку» — внешняя ссылка на Telegram; адрес — заглушка
  // из shared/config/support.ts (владелец заменит).
  const telegram = page.getByRole('link', { name: 'Обратиться в поддержку' });
  await expect(telegram).toHaveAttribute('href', 'https://t.me/swirnowwwivan');
  await expect(telegram).toHaveAttribute('target', '_blank');

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

  test('офлайн-страница на мобайле', async ({ page }, testInfo) => {
    await page.goto('/offline.html');

    await expect(page.getByRole('heading', { name: 'Нет соединения' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Обратиться в поддержку' })).toBeVisible();

    // Оптический центр (решение владельца 21.09): блок приподнят на −5vh,
    // его центр выше центра вьюпорта — «мёртвое» центрирование оставляло
    // треть экрана пустоты сверху.
    const centerOffset = await page.evaluate(() => {
      const panel = document.querySelector('.panel') as HTMLElement;
      const rect = panel.getBoundingClientRect();
      return rect.top + rect.height / 2 - window.innerHeight / 2;
    });
    expect(centerOffset).toBeLessThan(0);

    await captureScreen(page, testInfo, 'offline-page-mobile');
  });
});

test('SW отдаёт офлайн-страницу при потере сети', async ({ page, context, seededUser }) => {
  test.setTimeout(60_000);
  // SW регистрируется внутри ScreenLayout (кабинетные экраны), поэтому
  // нужен сеанс: /login вне кабинетного layout регистрации не делает.
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/properties');
  // SW сам делает clients.claim в activate (public/sw.js) — контроллер
  // появляется на уже открытой странице без перезагрузки;
  // ServiceWorkerRegister только register('/sw.js').
  await page.waitForFunction(() => navigator.serviceWorker.controller !== null, undefined, {
    timeout: 30_000,
  });

  await context.setOffline(true);
  await page.goto('/properties');
  await expect(page.getByRole('heading', { name: 'Нет соединения' })).toBeVisible();

  await context.setOffline(false);
});
