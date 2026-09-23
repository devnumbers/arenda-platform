import type { Page } from '@playwright/test';
import {
  execE2eSql,
  expect,
  openCabinetWithSeededSession,
  test,
} from './fixtures';

// Режим правки «Избранных платежей» — плавное перетаскивание (#813, карта
// #811): плашка следует за ручкой (framer-motion Reorder, жест только на
// ручке), клавиатурный порядок (пробел берёт, стрелки перемещают, Escape
// отпускает, анонсы в live-регионе), «Сохранить» — PUT
// /payments/favorites/order с проверкой серверной правды (favorite_order).

const FAVORITES_URL = '/payments/favorites';

// Три платежа сида на квартире: аренда (в сиде уже в избранном),
// электричество и домофон — их звезды другие спеки не трогают
// (payment-detail переключает только страхование …552).
const RENT_ID = '55555555-5555-4555-8555-555555555551';
const ELECTRICITY_ID = '55555555-5555-4555-8555-555555555553';
const INTERCOM_ID = '55555555-5555-4555-8555-555555555554';
const FAVORITE_IDS = [RENT_ID, ELECTRICITY_ID, INTERCOM_ID];

async function seedFavorites(): Promise<void> {
  // Плотный сброс ровно троих: is_favorite без позиции = конец списка;
  // стартовый порядок — порядок ленты, читаем его из DOM, а не угадываем.
  await execE2eSql(
    `UPDATE payments SET is_favorite = FALSE, favorite_order = NULL
     WHERE id IN ('${FAVORITE_IDS.join("','")}');`,
  );
  await execE2eSql(
    `UPDATE payments SET is_favorite = TRUE, favorite_order = NULL
     WHERE id IN ('${FAVORITE_IDS.join("','")}');`,
  );
}

/** id строк режима правки в текущем DOM-порядке. */
async function editRowIds(page: Page): Promise<string[]> {
  const rows = page.locator('[data-testid^="favorites-edit-row-"]');
  const count = await rows.count();
  const ids: string[] = [];
  for (let i = 0; i < count; i += 1) {
    const testid = await rows.nth(i).getAttribute('data-testid');
    ids.push((testid ?? '').replace('favorites-edit-row-', ''));
  }
  return ids;
}

test.describe('правка избранного: перетаскивание за ручку', () => {
  test.beforeEach(async () => {
    await seedFavorites();
  });

  test.afterAll(async () => {
    // Возврат к сиду: в избранном только аренда, без позиции.
    await execE2eSql(
      `UPDATE payments SET is_favorite = FALSE, favorite_order = NULL
       WHERE id IN ('${ELECTRICITY_ID}', '${INTERCOM_ID}');`,
    );
    await execE2eSql(
      `UPDATE payments SET is_favorite = TRUE, favorite_order = NULL
       WHERE id = '${RENT_ID}';`,
    );
  });

  test('клавиатурный порядок: пробел берёт, стрелка перемещает, Escape отпускает, сохранение пишет сервер', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(FAVORITES_URL);

    await page.getByTestId('favorites-edit-button').click();
    await expect(page.getByTestId('favorites-save')).toBeVisible();

    const ids = await editRowIds(page);
    expect(ids).toHaveLength(3);
    const [firstId] = ids;

    // Без диффа «Сохранить» погашена.
    await expect(page.getByTestId('favorites-save')).toBeDisabled();

    const grip = page.getByTestId(`favorites-grip-${firstId}`);
    await grip.focus();
    await page.keyboard.press('Space');

    // Взятие анонсится позицией и подсказкой клавиш.
    await expect(page.getByTestId('favorites-dnd-live')).toContainText(
      'Стрелки вверх и вниз — переместить',
    );

    await page.keyboard.press('ArrowDown');
    const afterMove = await editRowIds(page);
    expect(afterMove[1]).toBe(firstId);

    // Перемещение анонсится новой позицией (2 из 3).
    await expect(page.getByTestId('favorites-dnd-live')).toContainText(
      'позиция 2 из 3',
    );

    // Дифф появился — «Сохранить» живая.
    await expect(page.getByTestId('favorites-save')).toBeEnabled();

    await page.keyboard.press('Escape');
    await expect(page.getByTestId('favorites-dnd-live')).toContainText(
      'Сохранить',
    );

    await page.getByTestId('favorites-save').click();
    await expect(page.getByTestId('favorites-save')).toBeDisabled();

    // Серверная правда: взятый первым платеж теперь на позиции 2.
    const dbOrder = await execE2eSql(
      `SELECT id || ':' || favorite_order FROM payments
       WHERE id IN ('${FAVORITE_IDS.join("','")}') AND is_favorite
       ORDER BY favorite_order;`,
    );
    expect(dbOrder.split('\n')).toEqual([
      `${afterMove[0]}:1`,
      `${afterMove[1]}:2`,
      `${afterMove[2]}:3`,
    ]);

    // Порядок переживает перезагрузку — прочитан с сервера.
    await page.reload();
    const viewRows = page.locator('[data-testid^="favorites-row-star-"]');
    await expect(viewRows).toHaveCount(3);
    const reloadedIds: string[] = [];
    for (let i = 0; i < 3; i += 1) {
      reloadedIds.push(
        (await viewRows.nth(i).getAttribute('data-testid'))?.replace(
          'favorites-row-star-',
          '',
        ) ?? '',
      );
    }
    expect(reloadedIds).toEqual(afterMove);
  });

  test('drag мышью за ручку: плашка меняет место, сохранение пишет сервер', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(FAVORITES_URL);

    await page.getByTestId('favorites-edit-button').click();
    await expect(page.getByTestId('favorites-save')).toBeVisible();

    const ids = await editRowIds(page);
    expect(ids).toHaveLength(3);
    const [firstId] = ids;

    // Жест стартует на ручке; строка вне ручки драг не начинает —
    // проверяем, что тап по строке (не ручке) порядок не меняет.
    await page
      .getByTestId(`favorites-edit-row-${firstId}`)
      .getByText('Арендная плата')
      .click();
    expect(await editRowIds(page)).toEqual(ids);

    const grip = page.getByTestId(`favorites-grip-${firstId}`);
    const gripBox = await grip.boundingBox();
    const secondBox = await page
      .getByTestId(`favorites-edit-row-${ids[1]}`)
      .boundingBox();
    if (gripBox === null || secondBox === null) {
      throw new Error('ручек или строка не нашлись для жеста');
    }

    await page.mouse.move(
      gripBox.x + gripBox.width / 2,
      gripBox.y + gripBox.height / 2,
    );
    await page.mouse.down();
    // Решительный жест вниз — центр второй строки; velocity-based
    // перестановка Reorder срабатывает на движении, а не на стоянке.
    await page.mouse.move(
      secondBox.x + secondBox.width / 2,
      secondBox.y + secondBox.height / 2,
      { steps: 10 },
    );
    await page.mouse.up();

    const afterDrag = await editRowIds(page);
    expect(afterDrag[1]).toBe(firstId);
    expect(afterDrag).not.toEqual(ids);

    await page.getByTestId('favorites-save').click();
    await expect(page.getByTestId('favorites-save')).toBeDisabled();

    const dbOrder = await execE2eSql(
      `SELECT id || ':' || favorite_order FROM payments
       WHERE id IN ('${FAVORITE_IDS.join("','")}') AND is_favorite
       ORDER BY favorite_order;`,
    );
    expect(dbOrder.split('\n')).toEqual([
      `${afterDrag[0]}:1`,
      `${afterDrag[1]}:2`,
      `${afterDrag[2]}:3`,
    ]);
  });
});
