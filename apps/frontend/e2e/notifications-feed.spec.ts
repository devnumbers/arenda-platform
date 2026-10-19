import {
  captureScreen,
  execE2eSql,
  expect,
  openCabinetWithSeededSession,
  screenHeader,
  test,
} from './fixtures';
import { paymentCategoryBySlug } from '@/features/payment-categories';

/** toHaveCSS сравнивает вычисленный цвет строкой rgb(...), каталог хранит
 * hex — конвертация для ассерта. */
function hexToRgbCss(hex: string): string {
  const value = hex.replace('#', '');
  const num = parseInt(value, 16);
  return `rgb(${(num >> 16) & 255}, ${(num >> 8) & 255}, ${num & 255})`;
}

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

    // «Прочитать все» — мутация без подтверждения: тост успеха (тост
    // вместо успех-попапа, решение владельца 19.10.2026); закрываем
    // кнопкой, чтобы не перехватывал клики (канон contacts-спек).
    await menu.getByRole('menuitem', { name: 'Прочитать все' }).click();
    const toast = page.locator('.Toastify__toast', { hasText: 'Все уведомления прочитаны' });
    await expect(toast).toBeVisible();
    await toast.getByRole('button', { name: 'Закрыть' }).click();
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

test.describe('лента — кругляш платёжных событий (#1348)', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  // Регрессионный замок решения владельца 14.10 (#1348): снимок категории
  // платежа рисуется векторной иконкой каталога на цветной подложке даже
  // когда в payload есть фото объекта (иконка главнее фото); строка без
  // снимка (старая лента) каталожного кругляша не получает.
  test('снимок категории — иконка каталога вместо фото; строка без снимка — без каталожного кругляша', async ({
    page,
    seededUser,
  }, testInfo) => {
    await execE2eSql('DELETE FROM notifications;');
    const withSnapshot = `
INSERT INTO notifications (id, user_id, category, event_type, title, body, context_label, payload, dedup_key, created_at)
VALUES ('b1348000-0000-4000-8000-000000000001', '${OWNER_ID}', 'payments_operations', 'payment_due',
        'Платёж со снимком категории',
        'Платёж «Водоснабжение» по объекту «Квартира на Ленина»: 2 500 ₽. Срок оплаты: 5 октября',
        'Квартира на Ленина',
        '{"property": {"id": "22222222-2222-4222-8222-222222222222", "name": "Квартира на Ленина", "photo": "/api/properties/22222222-2222-4222-8222-222222222222/photo"}, "payment_id": "33333333-3333-4333-8333-333333333333", "payment_category": {"source": "default", "slug": "water-supply"}}'::jsonb,
        'e2e:notif:1348-1', now() - interval '1 hours')
ON CONFLICT DO NOTHING`;
    const withoutSnapshot = `
INSERT INTO notifications (id, user_id, category, event_type, title, body, context_label, payload, dedup_key, created_at)
VALUES ('b1348000-0000-4000-8000-000000000002', '${OWNER_ID}', 'payments_operations', 'payment_due',
        'Платёж без снимка категории',
        'Платёж «Аренда» по объекту «Квартира на Ленина»: 45 000 ₽. Срок оплаты: 5 октября',
        'Квартира на Ленина',
        '{"property": {"id": "22222222-2222-4222-8222-222222222222", "name": "Квартира на Ленина", "photo": "/api/properties/22222222-2222-4222-8222-222222222222/photo"}, "payment_id": "33333333-3333-4333-8333-333333333333"}'::jsonb,
        'e2e:notif:1348-2', now() - interval '2 hours')
ON CONFLICT DO NOTHING`;
    await execE2eSql(withSnapshot + '; ' + withoutSnapshot);
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/notifications');

    const snapshotRow = page.getByRole('button', { name: /Платёж со снимком категории/ });
    await expect(snapshotRow).toBeVisible();
    // Кругляш снимка — инлайн-подложка цвета каталога; фото объекта не
    // рисуется вовсе (не onError-откат, а приоритет решения владельца).
    const entry = paymentCategoryBySlug('water-supply');
    if (entry === undefined) throw new Error('water-supply отсутствует в каталоге');
    const catalogCircle = snapshotRow.locator('div[style*="background-color"]');
    await expect(catalogCircle).toHaveCount(1);
    await expect(catalogCircle).toHaveCSS('background-color', hexToRgbCss(entry.color));
    await expect(snapshotRow.locator('img')).toHaveCount(0);
    await captureScreen(page, testInfo, 'notifications-payment-category-icon');

    // Старая строка без снимка: каталожного кругляша нет — деградация в
    // фото/стандарт, инлайн-подложки не появляется.
    const legacyRow = page.getByRole('button', { name: /Платёж без снимка категории/ });
    await expect(legacyRow).toBeVisible();
    await expect(legacyRow.locator('div[style*="background-color"]')).toHaveCount(0);
  });
});
