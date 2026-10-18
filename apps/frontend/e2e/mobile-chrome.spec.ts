import type { Locator, Page } from '@playwright/test';
import {
  expect,
  openCabinetWithSeededSession,
  test,
} from './fixtures';

// Мобильный смоук хрома (карта #1416, тикеты #1421/#1441, проект
// mobile-chromium): нижняя навигация работает от настоящих touch-событий
// (hasTouch проекта Pixel 7), не от мыши. Локаторы — по a11y-ролям;
// тап — координатами элемента через page.touchscreen (click шлёт
// mouse-события и тач не проверяет).

/** Тап по центру элемента настоящим тачем. */
async function tapCenter(page: Page, locator: Locator): Promise<void> {
  const box = await locator.boundingBox();
  if (box === null) {
    throw new Error('цель тача не найдена');
  }
  await page.touchscreen.tap(box.x + box.width / 2, box.y + box.height / 2);
}

test.describe('Мобильный хром', () => {
  test('нижняя навигация от тача: таб «Уведомления» и шит «Еще»', async ({ page, seededUser }) => {
    // Смоук живёт только в мобильном проекте: touchscreen.tap требует
    // hasTouch, которого нет у десктопного chromium — без гарда дефолтный
    // прогон (--project=chromium) краснел бы на спеке.
    test.skip(
      test.info().project.name !== 'mobile-chromium',
      'тач-смоук — только в проекте mobile-chromium (E2E_PROJECT=mobile-chromium)',
    );

    await openCabinetWithSeededSession(page, seededUser);
    // Фикстура только ставит куку сессии — навигация на хаб объектов здесь
    // (конвенция спек: frame-shell, contacts-book).
    await page.goto('/properties');

    const nav = page.locator('nav[aria-label="Нижняя навигация"]');
    await expect(nav).toBeVisible();

    await tapCenter(page, nav.getByRole('link', { name: 'Уведомления' }));
    await expect(page).toHaveURL(/\/notifications/);

    await tapCenter(page, nav.getByRole('button', { name: 'Еще' }));
    await expect(page.getByRole('dialog', { name: 'Еще' })).toBeVisible();

    // Пока шит открыт, vaul (Radix DialogContentModal → hideOthers)
    // aria-скрывает весь бар, хотя тот кликабелен и видим — «Еще»-тумблер
    // матчится только с includeHidden (находка — к владельцу, вне скоупа).
    await tapCenter(page, nav.getByRole('button', { name: 'Еще', includeHidden: true }));
    await expect(page.getByRole('dialog', { name: 'Еще' })).toBeHidden();
  });
});
