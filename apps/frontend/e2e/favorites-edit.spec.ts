import type { CDPSession, Page } from '@playwright/test';
import {
  captureScreen,
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

async function restoreSeedFavorites(): Promise<void> {
  // Возврат к сиду: в избранном только аренда, без позиции.
  await execE2eSql(
    `UPDATE payments SET is_favorite = FALSE, favorite_order = NULL
     WHERE id IN ('${ELECTRICITY_ID}', '${INTERCOM_ID}');`,
  );
  await execE2eSql(
    `UPDATE payments SET is_favorite = TRUE, favorite_order = NULL
     WHERE id = '${RENT_ID}';`,
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

/** Серверная правда сохранения: favorite_order в БД — плотные 1..N
 * ровно в ожидаемом порядке (positions строит PUT /favorites/order). */
async function expectServerOrder(expectedIds: string[]): Promise<void> {
  const dbOrder = await execE2eSql(
    `SELECT id FROM payments
     WHERE id IN ('${FAVORITE_IDS.join("','")}') AND is_favorite
     ORDER BY favorite_order;`,
  );
  expect(dbOrder.split('\n')).toEqual(expectedIds);
}

test.describe('правка избранного: перетаскивание за ручку', () => {
  test.beforeEach(async () => {
    await seedFavorites();
  });

  test.afterAll(async () => {
    await restoreSeedFavorites();
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

    // keydown ручки не всплывает в строку: захват не переключает выделение.
    await expect(
      page
        .getByTestId(`favorites-edit-row-${firstId}`)
        .locator('[aria-pressed="true"]'),
    ).toHaveCount(0);
    await expect(page.getByTestId('favorites-trash')).toHaveCount(0);
    await expect(page.getByText(/^Выбрано/)).toHaveCount(0);

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

    // keydown ручки не всплывает в строку: захват не переключает выделение.
    await expect(
      page
        .getByTestId(`favorites-edit-row-${firstId}`)
        .locator('[aria-pressed="true"]'),
    ).toHaveCount(0);
    await expect(page.getByTestId('favorites-trash')).toHaveCount(0);
    await expect(page.getByText(/^Выбрано/)).toHaveCount(0);

    await page.getByTestId('favorites-save').click();
    await expect(page.getByTestId('favorites-save')).toBeDisabled();

    // Серверная правда: взятый первым платеж теперь на позиции 2.
    await expectServerOrder(afterMove);

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
    // тап по строке (не ручке) порядок не меняет: он теперь переключает
    // выделение (#814) и ничего не перетаскивает.
    await page
      .getByTestId(`favorites-edit-row-${firstId}`)
      .getByText('Арендная плата')
      .click();
    expect(await editRowIds(page)).toEqual(ids);
    await expect(
      page
        .getByTestId(`favorites-edit-row-${firstId}`)
        .locator('[aria-pressed="true"]'),
    ).toHaveCount(1);

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

    await expectServerOrder(afterDrag);
  });

  test('скриншоты для Figma-сверки: просмотр и правка, mobile 375 и desktop 1440', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(FAVORITES_URL);

    for (const [label, viewport] of [
      ['mobile', { width: 375, height: 667 }],
      ['desktop', { width: 1440, height: 900 }],
    ] as const) {
      await page.setViewportSize(viewport);

      // Просмотр: звёзды строк — маркер загрузки, скелетон не снимаем.
      await expect(
        page.locator('[data-testid^="favorites-row-star-"]'),
      ).toHaveCount(3);
      await captureScreen(page, test.info(), `favorites-view-${label}`);

      // Правка: вход через карандаш, выход из неё для следующего вьюпорта.
      await page.getByTestId('favorites-edit-button').click();
      await expect(page.getByTestId('favorites-save')).toBeVisible();
      await captureScreen(page, test.info(), `favorites-edit-${label}`);
      await page.getByLabel('Выйти из правки').click();
    }
  });
});

// Выделение и удаление (#814): клик строки на десктопе выделяет,
// long-press на таче выделяет и включает режим выбора (тапы дальше
// переключают), заголовок → «Выбрано N», trash в навбаре удаляет через
// ConfirmDialog с попапом успеха. Серверная правда — is_favorite и
// favorite_order.
test.describe('правка избранного: выделение и удаление', () => {
  test.beforeEach(async () => {
    await seedFavorites();
  });

  test.afterAll(async () => {
    await restoreSeedFavorites();
  });

  /** Строка правки по id. */
  function editRow(page: Page, id: string): ReturnType<Page['getByTestId']> {
    return page.getByTestId(`favorites-edit-row-${id}`);
  }

  /** Выделена ли строка: aria-pressed переключателя на строке. */
  async function expectSelected(
    page: Page,
    id: string,
    selected: boolean,
  ): Promise<void> {
    await expect(
      editRow(page, id).locator(`[aria-pressed="${selected}"]`),
    ).toHaveCount(1);
  }

  /** Тач-жест настоящим touch-пайпом браузера (CDP): приходят
   * pointer-события с pointerType touch и клик после подъёма пальца.
   * holdMs = 0 — тап, 700 — long-press дольше 500ms-таймера зажатия. */
  type RowBox = { x: number; y: number; width: number; height: number };

  async function touchGesture(
    page: Page,
    cdp: CDPSession,
    box: RowBox,
    holdMs: number,
  ): Promise<void> {
    const point = { x: box.x + box.width / 2, y: box.y + box.height / 2 };
    await cdp.send('Input.dispatchTouchEvent', {
      type: 'touchStart',
      touchPoints: [point],
    });
    if (holdMs > 0) {
      await page.waitForTimeout(holdMs);
    }
    await cdp.send('Input.dispatchTouchEvent', {
      type: 'touchEnd',
      touchPoints: [],
    });
  }

  async function touchTap(
    page: Page,
    cdp: CDPSession,
    box: RowBox,
  ): Promise<void> {
    await touchGesture(page, cdp, box, 0);
  }

  async function touchLongPress(
    page: Page,
    cdp: CDPSession,
    box: RowBox,
  ): Promise<void> {
    await touchGesture(page, cdp, box, 700);
  }

  async function rowBox(
    page: Page,
    id: string,
  ): Promise<RowBox> {
    const box = await editRow(page, id).boundingBox();
    if (box === null) {
      throw new Error(`строка ${id} не нашлась для жеста`);
    }
    return box;
  }

  test('десктоп: клик выделяет и снимает, trash с ConfirmDialog удаляет — серверная правда', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(FAVORITES_URL);

    await page.getByTestId('favorites-edit-button').click();
    await expect(page.getByTestId('favorites-save')).toBeVisible();

    const ids = await editRowIds(page);
    expect(ids).toHaveLength(3);

    // Без выделения trash в навбаре нет.
    await expect(page.getByTestId('favorites-trash')).toHaveCount(0);

    // Клик строки выделяет: заголовок «Выбрано N», trash появляется.
    await editRow(page, ids[0] as string).click();
    await expectSelected(page, ids[0] as string, true);
    await expect(page.getByText('Выбрано 1 платёж')).toBeVisible();
    await expect(page.getByTestId('favorites-trash')).toBeVisible();

    // Клик второй — счётчик растёт; повторный клик первой снимает
    // выделение; снова клик — возвращает (переключение).
    await editRow(page, ids[1] as string).click();
    await expect(page.getByText('Выбрано 2 платежа')).toBeVisible();
    await editRow(page, ids[0] as string).click();
    await expectSelected(page, ids[0] as string, false);
    await expect(page.getByText('Выбрано 1 платёж')).toBeVisible();
    await editRow(page, ids[0] as string).click();
    await expect(page.getByText('Выбрано 2 платежа')).toBeVisible();

    // Клик ручки выделение не переключает (клик не всплывает в строку).
    await page.getByTestId(`favorites-grip-${ids[0]}`).click();
    await expect(page.getByText('Выбрано 2 платежа')).toBeVisible();

    // Trash → канон ConfirmDialog → «Убрать».
    await page.getByTestId('favorites-trash').click();
    await expect(
      page.getByText('Убрать выбранные платежи из избранного?'),
    ).toBeVisible();
    await page.getByRole('button', { name: 'Убрать', exact: true }).click();

    // Попап успеха после выходной анимации диалога.
    await expect(
      page.getByText('Платежи больше не в избранном').first(),
    ).toBeVisible({ timeout: 5000 });
    await page.getByLabel('Закрыть').click();

    // Правка продолжается: выделение сброшено, заголовок обычный,
    // trash скрыт, «Сохранить» погашена (порядок не менялся).
    await expect(page.getByTestId('favorites-trash')).toHaveCount(0);
    await expect(page.getByText('Избранные платежи')).toBeVisible();
    const remaining = ids.filter((id) => id !== ids[0] && id !== ids[1]);
    await expectSelected(page, remaining[0] as string, false);
    await expect(page.locator('[data-testid^="favorites-edit-row-"]')).toHaveCount(
      1,
    );
    await expect(page.getByTestId('favorites-save')).toBeDisabled();

    // Серверная правда: убранные is_favorite=false, оставшийся в порядке.
    await expectServerOrder([remaining[0] as string]);
  });

  test('тач: до зажатия тап не выделяет; зажатие выделяет и включает выбор, тапы переключают', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(FAVORITES_URL);

    await page.getByTestId('favorites-edit-button').click();
    await expect(page.getByTestId('favorites-save')).toBeVisible();

    const ids = await editRowIds(page);
    expect(ids).toHaveLength(3);
    const cdp = await page.context().newCDPSession(page);

    // Тап по строке ДО зажатия ничего не делает (вход в выбор — только
    // long-press, решения владельца #814).
    await touchTap(page, cdp, await rowBox(page, ids[0] as string));
    await expectSelected(page, ids[0] as string, false);
    await expect(page.getByTestId('favorites-trash')).toHaveCount(0);

    // Зажатие на ручке dnd выделение не включает — жест не из плашки.
    const grip = page.getByTestId(`favorites-grip-${ids[0]}`);
    const gripBox = await grip.boundingBox();
    if (gripBox === null) {
      throw new Error('ручек не нашлась для жеста');
    }
    await touchLongPress(page, cdp, gripBox);
    await expectSelected(page, ids[0] as string, false);
    expect(await editRowIds(page)).toEqual(ids);

    // Зажатие плашки выделяет её и включает режим выбора.
    await touchLongPress(page, cdp, await rowBox(page, ids[0] as string));
    await expectSelected(page, ids[0] as string, true);
    await expect(page.getByText('Выбрано 1 платёж')).toBeVisible();
    await expect(page.getByTestId('favorites-trash')).toBeVisible();

    // После зажатия одиночные тапы переключают: второй — выделен, тем же
    // тапом — снят; и зажатая строка от повторного зажатия не слетает.
    await touchTap(page, cdp, await rowBox(page, ids[1] as string));
    await expect(page.getByText('Выбрано 2 платежа')).toBeVisible();
    await touchTap(page, cdp, await rowBox(page, ids[1] as string));
    await expect(page.getByText('Выбрано 1 платёж')).toBeVisible();
    await touchLongPress(page, cdp, await rowBox(page, ids[0] as string));
    await expectSelected(page, ids[0] as string, true);
    await expect(page.getByText('Выбрано 1 платёж')).toBeVisible();

    // Снятие выделения тапом — заголовок и trash уходят.
    await touchTap(page, cdp, await rowBox(page, ids[0] as string));
    await expect(page.getByTestId('favorites-trash')).toHaveCount(0);
    await expect(page.getByText('Избранные платежи')).toBeVisible();
  });

  test('drag за ручку при активном выделении не переключает выделение — клик-хвост жеста съеден', async ({
    page,
    seededUser,
  }) => {
    // Живая приёмка #814: отпускание drag'а над дотащенной строкой
    // отдавало click контейнеру строки и мусорно переключало выделение.
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(FAVORITES_URL);

    await page.getByTestId('favorites-edit-button').click();
    await expect(page.getByTestId('favorites-save')).toBeVisible();

    const ids = await editRowIds(page);
    expect(ids).toHaveLength(3);
    const [firstId, secondId] = ids as [string, string];

    const gripBox = await page.getByTestId(`favorites-grip-${firstId}`).boundingBox();
    const secondBox = await page.getByTestId(`favorites-edit-row-${secondId}`).boundingBox();
    if (gripBox === null || secondBox === null) {
      throw new Error('ручек или строка не нашлись для жеста');
    }

    // Drag первой строки на вторую: строка уезжает под курсор, pointerup
    // ложится внутрь перетаскиваемой плашки — выделение не переключает.
    await page.mouse.move(gripBox.x + gripBox.width / 2, gripBox.y + gripBox.height / 2);
    await page.mouse.down();
    await page.mouse.move(secondBox.x + secondBox.width / 2, secondBox.y + secondBox.height / 2, {
      steps: 10,
    });
    await page.mouse.up();

    const afterDrag = await editRowIds(page);
    expect(afterDrag).not.toEqual(ids);
    await expectSelected(page, firstId, false);
    await expectSelected(page, secondId, false);
    await expect(page.getByTestId('favorites-trash')).toHaveCount(0);

    // Тап по строке сразу после drag'а выделяет как обычно (флаг жеста
    // не залип).
    await page.getByTestId(`favorites-edit-row-${firstId}`).click();
    await expectSelected(page, firstId, true);
    await expect(page.getByText('Выбрано 1 платёж')).toBeVisible();
  });

  test('удаление всего избранного: пустое состояние и выход из правки', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(FAVORITES_URL);

    await page.getByTestId('favorites-edit-button').click();
    const ids = await editRowIds(page);
    expect(ids).toHaveLength(3);

    for (const id of ids) {
      await editRow(page, id).click();
    }
    await expect(page.getByText('Выбрано 3 платежа')).toBeVisible();
    await page.getByTestId('favorites-trash').click();
    await page.getByRole('button', { name: 'Убрать', exact: true }).click();

    await expect(
      page.getByText('Платежи больше не в избранном').first(),
    ).toBeVisible({ timeout: 5000 });
    await page.getByLabel('Закрыть').click();

    // Избранного не осталось: пустое состояние, правки больше нет
    // (и карандаша нет — править нечего).
    await expect(page.getByTestId('favorites-empty')).toBeVisible();
    await expect(page.getByTestId('favorites-edit-button')).toHaveCount(0);
    await expect(page.getByTestId('favorites-save')).toHaveCount(0);

    const favoriteRows = await execE2eSql(
      `SELECT id FROM payments WHERE id IN ('${FAVORITE_IDS.join("','")}') AND is_favorite;`,
    );
    expect(favoriteRows).toBe('');
  });
});
