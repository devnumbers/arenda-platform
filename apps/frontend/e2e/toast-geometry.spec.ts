import type { Page } from '@playwright/test';
import { execE2eSql, expect, openCabinetWithSeededSession, test } from './fixtures';

// Поля тостов от краёв экрана — 24 на всех ярусах (решение владельца
// 19.10.2026): нативные дефолты react-toastify (16) и мобильные поля 12
// снесены; верх на телефонах (нативный ≤480 перебивает отступ на
// env(safe-area-inset-top), в браузере он 0) больше не прижат к краю.
// Триггер — сценарный тост «Прочитать все» (канон notifications-спек);
// контейнер общий с тостом-уведомлением, геометрия одна на все виды.

// Owner сеанса (seed.sql, фиксированный UUID) — все вставки сцены его.
const OWNER_ID = '11111111-1111-4111-8111-111111111111';

const insertNotification = (id: string, title: string, hoursAgo: number): string => `
INSERT INTO notifications (id, user_id, category, event_type, title, body, context_label, payload, dedup_key, created_at)
VALUES ('${id}', '${OWNER_ID}', 'payments_operations', 'payment_due', '${title}',
        'Оплатите платёж «Аренда» 45 000 Р. Срок оплаты: 5 октября', 'Квартира на Ленина',
        '{}'::jsonb, 'e2e:toast-geometry:${id}', now() - interval '${hoursAgo} hours')
ON CONFLICT DO NOTHING`;

const MOBILE = { width: 390, height: 844 };
const TABLET = { width: 768, height: 1024 };
const PC = { width: 1440, height: 900 };

async function showScenarioToast(page: Page, seededUser: Parameters<typeof openCabinetWithSeededSession>[1]) {
  await execE2eSql('DELETE FROM notifications;');
  await execE2eSql(insertNotification('b1172000-0000-4000-8000-000000000001', 'Оплатите платёж', 1));
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/notifications');
  await page.getByRole('button', { name: 'Действия с уведомлениями' }).last().click();
  await page.getByRole('menuitem', { name: 'Прочитать все' }).click();
  const toast = page.locator('.Toastify__toast', { hasText: 'Все уведомления прочитаны' });
  await expect(toast).toBeVisible();
  // Входная анимация 350ms (--dl-duration-move, §8) едет transform'ом —
  // boundingBox во время полёта врёт; меряем после успокоения.
  await page.waitForTimeout(450);
  return toast;
}

test.describe('поля тостов — 24 от краёв на всех ярусах', () => {
  test('мобайл: 24 от верха и боков', async ({ page, seededUser }) => {
    await page.setViewportSize(MOBILE);
    const toast = await showScenarioToast(page, seededUser);
    const box = await toast.boundingBox();
    if (!box) throw new Error('тост без геометрии');

    expect(Math.abs(box.x - 24)).toBeLessThanOrEqual(1);
    expect(Math.abs(MOBILE.width - (box.x + box.width) - 24)).toBeLessThanOrEqual(1);
    expect(Math.abs(box.y - 24)).toBeLessThanOrEqual(1);

    // Канон notifications-спек: закрываем кнопкой, чтобы не перехватывал клики.
    await toast.getByRole('button', { name: 'Закрыть' }).click();
  });

  // Планшет и ПК — одна ветка top-right (позиция флипает на 768 по
  // matchMedia контейнера), та же пара max(24px, env)-переменных.
  for (const tier of [
    { name: 'планшет', viewport: TABLET },
    { name: 'ПК', viewport: PC },
  ]) {
    test(`${tier.name}: 24 от верха и правого края`, async ({ page, seededUser }) => {
      await page.setViewportSize(tier.viewport);
      const toast = await showScenarioToast(page, seededUser);
      const box = await toast.boundingBox();
      if (!box) throw new Error('тост без геометрии');

      expect(Math.abs(box.y - 24)).toBeLessThanOrEqual(1);
      expect(
        Math.abs(tier.viewport.width - (box.x + box.width) - 24),
      ).toBeLessThanOrEqual(1);

      await toast.getByRole('button', { name: 'Закрыть' }).click();
    });
  }
});
