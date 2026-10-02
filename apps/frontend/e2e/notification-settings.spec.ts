import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  test,
} from './fixtures';

// Матрица состояний пуш-колонки (спека #1028 §1, слайс 3 #1039): колонка
// видна всегда, тумблеры всегда кликабельны (дизейблов нет), обратная
// связь «включить нельзя» — единый красный текст-слот под мастером, только
// по клику, без тостов и без in-app шита (снесён).
//
// В headless chromium разрешение НЕ грантовано, а системный промпт
// автоматически отклоняется (безответный auto-dismiss = denied): клик
// мастера при `default` проходит системную развилку и возвращается
// `denied` — экран обязан показать красный слот с инструкцией
// разблокировки и остаться честно выключенным. Тихую подписку кликом
// (granted) e2e не гоняет: VAPID-ключ в e2e-стек не задан (config.go —
// ключи опциональны), путь закрывается живым walkthrough приёмки #1040.

test.describe('настройки уведомлений — матрица состояний и красный слот (спека #1028, #1039)', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('без подписки всё выключено; клик мастера при default даёт красный слот, шита нет', async ({
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
    await expect(master).toBeEnabled();
    await expect(master).toHaveAttribute('aria-checked', 'false');

    // Тумблеры категорий кликабельны и не затемнены (сверка #827 —
    // 2333-180696, теперь для всей матрицы без дизейблов).
    const rentalPush = page.getByRole('switch', { name: 'Пуш-уведомления — Аренда' });
    await expect(rentalPush).toBeEnabled();
    await expect(rentalPush).toHaveAttribute('aria-checked', 'false');
    await expect(rentalPush.locator('xpath=..')).toHaveCSS('opacity', '1');
    await captureScreen(page, testInfo, 'notification-settings-no-subscription');

    // Клик мастера: системный промпт в headless отклоняется → denied.
    // In-app шит «Разрешите пуши» больше не существует, вместо тоста —
    // красный текст-слот под мастером; тумблер не двигается.
    await master.click();
    await expect(page.getByRole('dialog')).toHaveCount(0);
    // role=alert ловится и Next-овым route-announcer'ом (div), а name по
    // роли у alert не вычисляется — слот ищем по тегу с ролью.
    const slot = page.locator('p[role="alert"]');
    await expect(slot).toBeVisible();
    await expect(slot).toHaveText(
      'Разрешение на уведомления заблокировано в настройках браузера. Разрешите уведомления для этого сайта (значок замка в адресной строке → „Уведомления“ → „Разрешить“) и нажмите тумблер ещё раз.',
    );
    await expect(master).toHaveAttribute('aria-checked', 'false');
    await expect(rentalPush).toHaveAttribute('aria-checked', 'false');
    await captureScreen(page, testInfo, 'notification-settings-denied-slot');

    // Слот — только по клику: после перезагрузки его нет.
    await page.reload();
    await expect(master).toBeVisible();
    await expect(page.locator('p[role="alert"]')).toHaveCount(0);
  });
});
