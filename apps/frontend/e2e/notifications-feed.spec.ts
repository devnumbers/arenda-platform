import {
  captureScreen,
  execE2eSql,
  expect,
  openCabinetWithSeededSession,
  screenHeader,
  test,
} from './fixtures';

// Экран ленты уведомлений по макетам 3178/3183 (#1170–#1172, карта #1162):
// кебаб-меню из трёх пунктов с мутацией «Прочитать все» (#1171) и пустые
// состояния обоих видов (#1172). Данные сеанс сеет и убирает сам — сид
// уведомлений не содержит, лента стартует пустой.

// Owner сеанса (seed.sql, фиксированный UUID) — все вставки сцены его.
const OWNER_ID = '11111111-1111-4111-8111-111111111111';

const insertNotification = (id: string, title: string, hoursAgo: number): string => `
INSERT INTO notifications (id, user_id, category, event_type, title, body, context_label, payload, dedup_key, created_at)
VALUES ('${id}', '${OWNER_ID}', 'payments_operations', 'payment_due', '${title}',
        'Оплатите платёж «Аренда» 45 000 Р. Срок оплаты: 5 октября', 'Квартира на Ленина',
        '{}'::jsonb, 'e2e:notif:${id}', now() - interval '${hoursAgo} hours')
ON CONFLICT DO NOTHING`;

test.describe('лента уведомлений — меню и пустые состояния', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('кебаб-меню по макету 3183: «Прочитать все» гасит точки и счётчик', async ({
    page,
    seededUser,
  }, testInfo) => {
    await execE2eSql('DELETE FROM notifications;');
    await execE2eSql(
      insertNotification('b1171000-0000-4000-8000-000000000001', 'Оплатите платёж', 1) +
        '; ' +
        insertNotification('b1171000-0000-4000-8000-000000000002', 'Оплатите платёж', 2),
    );
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/notifications');

    // Кебаб открывает меню из трёх пунктов по макету 3183-86541. Инстансов
    // три (компактные слоты TopNav + строка HubTitle) — компактные накрыты
    // слотами шапки, кликаем экземпляр контентной строки.
    await page.getByRole('button', { name: 'Действия с уведомлениями' }).last().click();
    const menu = page.getByRole('menu');
    await expect(menu.getByRole('menuitem', { name: 'Прочитать все' })).toBeVisible();
    await expect(menu.getByRole('menuitem', { name: 'Настроить уведомления' })).toBeVisible();
    await expect(menu.getByRole('menuitem', { name: 'Удалить все уведомления' })).toBeVisible();
    await captureScreen(page, testInfo, 'notifications-menu');

    // «Прочитать все» — мутация без подтверждения: попап успеха; под ним
    // страница aria-hidden — сначала закрыть, потом ассерты ленты.
    await menu.getByRole('menuitem', { name: 'Прочитать все' }).click();
    await expect(page.getByText('Все уведомления прочитаны')).toBeVisible();
    await page.getByRole('button', { name: 'Закрыть' }).click();
    await expect(page.getByRole('dialog')).toHaveCount(0);
    // Строки погасли (прочитаны), счётчик чипа слетел.
    await expect(page.getByRole('button', { name: 'Непрочитанные', exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Непрочитанные 2' })).toHaveCount(0);
  });

  test('пустая лента «Все»: «Нет уведомлений», шестерёнка в хедере, чипов нет', async ({
    page,
    seededUser,
  }, testInfo) => {
    await execE2eSql('DELETE FROM notifications;');
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/notifications');

    // Макет 3178-86086: колокол + «Нет уведомлений» (текст по макету),
    // служебные чипы спрятаны вместе со списком, кебаб заменён
    // шестерёнкой настроек в хаб-хедере.
    await expect(page.getByText('Нет уведомлений')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Непрочитанные' })).toHaveCount(0);
    await expect(
      page.getByRole('button', { name: 'Настроить уведомления' }).first(),
    ).toBeVisible();
    await expect(
      page.getByRole('button', { name: 'Действия с уведомлениями' }),
    ).toHaveCount(0);
    await captureScreen(page, testInfo, 'notifications-empty-all');
  });

  test('пустой фильтр «Непрочитанные»: «Все уведомления прочитаны», чипы остаются', async ({
    page,
    seededUser,
  }, testInfo) => {
    await execE2eSql('DELETE FROM notifications;');
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/notifications?unread=1');

    // Макет 3187-88176: колокол + текст прочитанности, чипы на месте
    // (фильтр можно выключить), кебаб — не шестерёнка (удалять нечего
    // только в «Все»; в фильтре меню живёт по макету 2333-159051).
    await expect(page.getByText('Все уведомления прочитаны')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Все', exact: true })).toBeVisible();
    await expect(
      page.getByRole('button', { name: /Непрочитанные/ }).first(),
    ).toBeVisible();
    await expect(
      page.getByRole('button', { name: 'Действия с уведомлениями' }).first(),
    ).toBeVisible();
    await captureScreen(page, testInfo, 'notifications-empty-unread');
  });

  test('шапка ленты — хаб с крыльями и на мобайле (макет 3178)', async ({
    page,
    seededUser,
  }) => {
    await execE2eSql('DELETE FROM notifications;');
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/notifications');
    const header = screenHeader(page);

    await expect(header.getByRole('link', { name: 'Объекты' })).toBeVisible();
    await expect(
      page.getByRole('heading', { level: 1, name: 'Уведомления', exact: true }),
    ).toBeVisible();
    await expect(page.getByRole('button', { name: 'Все', exact: true })).toHaveCount(0);
  });
});
