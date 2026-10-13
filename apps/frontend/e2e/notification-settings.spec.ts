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

test.describe('настройки уведомлений — email-колонка без мутационного дизейбла (#1381)', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  // Канон #1028 распространён на email-колонку (тикет #1381): тумблер не
  // дизейблится на время мутации — отклик оптимистичный (значение двигается
  // сразу), при ошибке хук откатывает значение и показывает тост. PUT
  // удерживается в полёте, чтобы окно isPending было детерминированным:
  // старый код вешал disabled:opacity-50 на ВСЕ четыре тумблера колонки.
  test('во время мутации все тумблеры колонки активны и не затемнены', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/profile/notifications');

    const emailSwitch = (category: string) =>
      page.getByRole('switch', { name: `Электронная почта — ${category}` });
    const categories = ['Аренда', 'Платежи и операции', 'Задачи', 'Совместный доступ'];

    // Сид без строки настроек = всё включено.
    await expect(emailSwitch('Аренда')).toHaveAttribute('aria-checked', 'true');

    let releasePut: (() => void) | undefined;
    const putReleased = new Promise<void>((resolve) => {
      releasePut = resolve;
    });
    await page.route('**/api/notification-preferences', async (route) => {
      if (route.request().method() !== 'PUT') {
        await route.continue();
        return;
      }
      await putReleased;
      await route.continue();
    });

    await emailSwitch('Аренда').click();

    // Оптимистичный отклик: значение двигается до ответа сервера.
    await expect(emailSwitch('Аренда')).toHaveAttribute('aria-checked', 'false');

    // Всё окно мутации ни один тумблер колонки не заглушен и не затемнён.
    for (const category of categories) {
      await expect(emailSwitch(category)).toBeEnabled();
      await expect(emailSwitch(category).locator('xpath=..')).toHaveCSS('opacity', '1');
    }
    await captureScreen(page, testInfo, 'notification-settings-mutation-no-disable');

    releasePut?.();
    await expect(emailSwitch('Аренда')).toHaveAttribute('aria-checked', 'false');
    await page.unroute('**/api/notification-preferences');
  });
});
