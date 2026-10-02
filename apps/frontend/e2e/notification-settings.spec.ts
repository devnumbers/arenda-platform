import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  test,
} from './fixtures';

// Спека #1028 §0 (слайс 2 #1038): дефолт для всех — выключено. Мастер —
// факт живой подписки браузера: строка есть = включён, строки нет =
// выключен, у свежего браузера он честно ВЫКЛ. Тумблеры категорий остаются
// кликабельными и не затемнены (сверка #827 — 2333-180696).
//
// Клик мастера при не-выданном разрешении идёт через флоу разрешения
// (шит), не двигая тумблер молча; в headless chromium разрешение НЕ
// грантовано, поэтому здесь проверяется именно эта развилка. Тихую
// подписку кликом (granted) e2e не гоняет: VAPID-ключ в e2e-стек не
// задан (config.go — ключи опциональны), путь закрывается живым
// walkthrough слайса 3 (#1039).

test.describe('настройки уведомлений — мастер по факту подписки (спека #1028, #1038)', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('без подписки мастер выключен; клик при default открывает флоу разрешения и не врёт', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/profile/notifications');

    const master = page.getByRole('switch', {
      name: 'Получать пуш-уведомления на этом устройстве',
    });
    // Свежий браузер без подписки: строка нет = выключено, вранья «включён» нет.
    await expect(master).toBeVisible();
    await expect(master).toHaveAttribute('aria-checked', 'false');

    const rentalPush = page.getByRole('switch', { name: 'Пуш-уведомления — Аренда' });
    await expect(rentalPush).toBeEnabled();
    // строка в полную непрозрачность — затемнения нет
    await expect(rentalPush.locator('xpath=..')).toHaveCSS('opacity', '1');
    await captureScreen(page, testInfo, 'notification-settings-no-subscription');

    // Разрешение default: включение — только через системное окно, шит его
    // держит; отказ возвращает экран в то же честное «выключено».
    await master.click();
    const sheet = page.getByRole('dialog', { name: 'Разрешите пуши' });
    await expect(sheet).toBeVisible();
    await sheet.getByRole('button', { name: 'Не разрешать' }).click();
    await expect(sheet).toBeHidden();
    await expect(master).toHaveAttribute('aria-checked', 'false');
    await captureScreen(page, testInfo, 'notification-settings-declined');
  });
});
