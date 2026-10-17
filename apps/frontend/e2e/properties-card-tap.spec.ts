import {
  expect,
  openCabinetWithSeededSession,
  test,
} from './fixtures';

// Клик по плашке объекта в списке (карта #586): вся карточка —
// оверлей-ссылка на деталь, включая фото и шеврон. Оверлей лежал под
// позиционированными аватаром и шевроном (те — позже по дереву, z-index
// auto) — тапы по кругу фото и стрелке в деталь не вели (баг по слову
// владельца 17.10.2026). Проверка тапом в точку: клик по координате внутри
// круга фото (паддинг 24 + радиус 32 → центр 56,56 — центр аватара на
// обоих раскладах) и в центр шеврона (24 + 12 от правого/верхнего края) —
// до фикса перехватчик съедает тап (перехода нет), после — ведёт
// оверлей-ссылка.

test.describe('плашка объекта — клик по фото и шеврону', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  // Пара ярусов, как в favorites-edit: мобайл (фото над текстом) и
  // десктоп (фото слева вровень) — оба расклада держит один оверлей.
  const SPEC_VIEWPORTS = [
    { label: 'mobile', width: 390, height: 844 },
    { label: 'desktop', width: 1440, height: 900 },
  ] as const;

  test('тап по фото открывает деталь, как тап по всей плашке', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    for (const viewport of SPEC_VIEWPORTS) {
      await page.setViewportSize(viewport);
      await page.goto('/properties');
      const list = page.getByTestId('properties-list');
      await expect(list).toBeVisible();

      const firstCard = list.locator('article').first();
      await expect(firstCard).toBeVisible();

      await firstCard.click({ position: { x: 56, y: 56 } });
      await expect(page).toHaveURL(/\/properties\/[0-9a-f-]{36}$/);
    }
  });

  test('тап по шеврону открывает деталь', async ({ page, seededUser }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/properties');
    const list = page.getByTestId('properties-list');
    await expect(list).toBeVisible();

    const firstCard = list.locator('article').first();
    await expect(firstCard).toBeVisible();
    const box = await firstCard.boundingBox();
    if (box === null) {
      throw new Error('first card has no bounding box');
    }

    await firstCard.click({ position: { x: box.width - 36, y: 36 } });
    await expect(page).toHaveURL(/\/properties\/[0-9a-f-]{36}$/);
  });
});
