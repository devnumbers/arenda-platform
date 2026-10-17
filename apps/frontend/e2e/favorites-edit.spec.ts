import type { CDPSession, Locator, Page } from '@playwright/test';
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

/** Пара ярусов Figma-сверки: mobile 375 и desktop 1440 (контракт
 * testing-strategy.md:84). */
const SPEC_VIEWPORTS = [
  { label: 'mobile', width: 375, height: 667 },
  { label: 'desktop', width: 1440, height: 900 },
] as const;

/** Подзапрос id владельца сида — звезда личная, строки keyed по пользователю
 * (личный cutover #1308); тот же паттерн, что OWNER_SQL в rental-спеках. */
const OWNER_SQL = `(SELECT id FROM users WHERE email = '${process.env.E2E_USER_EMAIL}')`;

// Три платежа сида на квартире: аренда (в сиде уже в избранном),
// электричество и домофон — их звезды другие спеки не трогают
// (payment-detail переключает только страхование …552).
const RENT_ID = '55555555-5555-4555-8555-555555555551';
const ELECTRICITY_ID = '55555555-5555-4555-8555-555555555553';
const INTERCOM_ID = '55555555-5555-4555-8555-555555555554';
const FAVORITE_IDS = [RENT_ID, ELECTRICITY_ID, INTERCOM_ID];

async function seedFavorites(): Promise<void> {
  // Плотный сброс ровно троих личных строк владельца: строка без позиции =
  // конец списка; стартовый порядок — порядок ленты, читаем его из DOM,
  // а не угадываем.
  await execE2eSql(
    `DELETE FROM user_payment_favorites
     WHERE user_id = ${OWNER_SQL}
       AND payment_id IN ('${FAVORITE_IDS.join("','")}');`,
  );
  await execE2eSql(
    `INSERT INTO user_payment_favorites (user_id, payment_id)
     SELECT ${OWNER_SQL}, id FROM payments
     WHERE id IN ('${FAVORITE_IDS.join("','")}');`,
  );
}

async function restoreSeedFavorites(): Promise<void> {
  // Возврат к сиду: в избранном владельца только аренда, без позиции.
  await execE2eSql(
    `DELETE FROM user_payment_favorites
     WHERE user_id = ${OWNER_SQL}
       AND payment_id IN ('${FAVORITE_IDS.join("','")}');`,
  );
  await execE2eSql(
    `INSERT INTO user_payment_favorites (user_id, payment_id)
     VALUES (${OWNER_SQL}, '${RENT_ID}');`,
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

/** Серверная правда сохранения: favorite_order в личных строках владельца —
 * плотные 1..N ровно в ожидаемом порядке (positions строит PUT
 * /favorites/order; NULL-позиции сортируются в конец — NULLS LAST). */
async function expectServerOrder(expectedIds: string[]): Promise<void> {
  const dbOrder = await execE2eSql(
    `SELECT payment_id FROM user_payment_favorites
     WHERE user_id = ${OWNER_SQL}
       AND payment_id IN ('${FAVORITE_IDS.join("','")}')
     ORDER BY favorite_order;`,
  );
  expect(dbOrder.split('\n')).toEqual(expectedIds);
}

/** Бокс элемента для жеста: boundingBox с честной ошибкой вместо
 * null-гвардов на месте вызова. */
async function boxOrThrow(
  locator: Locator,
  what: string,
): Promise<{ x: number; y: number; width: number; height: number }> {
  const box = await locator.boundingBox();
  if (box === null) {
    throw new Error(`${what} не нашлись для жеста`);
  }
  return box;
}

/** Мышью тащит строку за ручку dnd к целевой строке: центры боксов,
 * решительный жест с шагами — velocity-based перестановка Reorder
 * срабатывает на движении, а не на стоянке. */
async function dragGripToRow(
  page: Page,
  gripTestId: string,
  targetRowTestId: string,
): Promise<void> {
  const gripBox = await boxOrThrow(
    page.getByTestId(gripTestId),
    'ручек или строка',
  );
  const rowBox = await boxOrThrow(
    page.getByTestId(targetRowTestId),
    'ручек или строка',
  );
  await page.mouse.move(
    gripBox.x + gripBox.width / 2,
    gripBox.y + gripBox.height / 2,
  );
  await page.mouse.down();
  await page.mouse.move(
    rowBox.x + rowBox.width / 2,
    rowBox.y + rowBox.height / 2,
    { steps: 10 },
  );
  await page.mouse.up();
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

    // Авто-повтор удержания (keydown c repeat) не отпускает ручку:
    // повторное событие — не второе нажатие, лифт и анонсы не флапают.
    await grip.dispatchEvent('keydown', {
      key: ' ',
      repeat: true,
      bubbles: true,
    });
    await expect(page.getByTestId('favorites-dnd-live')).toContainText(
      'Стрелки вверх и вниз — переместить',
    );
    await expect(page.getByTestId('favorites-dnd-live')).not.toContainText(
      'Изменения порядка применятся',
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

    await dragGripToRow(
      page,
      `favorites-grip-${firstId}`,
      `favorites-edit-row-${ids[1]}`,
    );

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

    for (const viewport of SPEC_VIEWPORTS) {
      await page.setViewportSize(viewport);
      const { label } = viewport;

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

// Анатомия строки правки — макеты 954-52219 (mobile) и 954-51409 (desktop),
// решение владельца 17.10 (отменяет «ручка в паддинге, без сдвига» #821:
// глиф прижимался к краю экрана в 6px). По инспектору макета: корень строки
// 12, трейлинг-слот ручки 44×44 с полем 12 до края (глиф Move 24 по центру
// — 22 от края), сумма с датой отъезжает на ширину слота — правый край блока
// в 68 от края строки; иконка категории на 24 от левого края (12 корня +
// 12 середины). Все замеры — от бокса строки, на обоих ярусах; там же пин
// «просмотр не тронут» (954-52460): контент просмотра на 36 от краёв.
test.describe('правка избранного: анатомия строки по макету', () => {
  test.beforeEach(async () => {
    await seedFavorites();
  });

  test.afterAll(async () => {
    await restoreSeedFavorites();
  });

  test('слот ручки 44 с полем 12, глиф 22 от края, сумма на 68, иконка на 24', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(FAVORITES_URL);

    for (const viewport of SPEC_VIEWPORTS) {
      await page.setViewportSize(viewport);

      // Пин «просмотр не тронут» (954-52460, решение владельца 17.10):
      // в просмотре сумма/дата и иконка сидят на 36 от краёв строки
      // (px-6 корня + px-3 середины) — правку чиним, просмотр не трогаем.
      // Строка просмотра — первый div-предок звезды (середины канона —
      // span'ы).
      const viewRow = page
        .locator('[data-testid^="favorites-row-star-"]')
        .first()
        .locator('xpath=ancestor::div[1]');
      const viewRowBox = await boxOrThrow(viewRow, 'строка просмотра');
      const viewAmountBox = await boxOrThrow(
        viewRow.locator('span.flex.shrink-0.flex-col'),
        'блок суммы просмотра',
      );
      const viewBox = await boxOrThrow(
        viewRow.locator('span.h-11.w-11').first(),
        'иконка категории просмотра',
      );
      expect(
        viewRowBox.x + viewRowBox.width - (viewAmountBox.x + viewAmountBox.width),
      ).toBeCloseTo(36, 1);
      expect(viewBox.x - viewRowBox.x).toBeCloseTo(36, 1);

      await page.getByTestId('favorites-edit-button').click();
      await expect(page.getByTestId('favorites-save')).toBeVisible();

      const [firstId] = await editRowIds(page);
      const row = page.getByTestId(`favorites-edit-row-${firstId}`);
      const rowBox = await boxOrThrow(row, 'строка');
      const grip = page.getByTestId(`favorites-grip-${firstId}`);
      const gripBox = await boxOrThrow(grip, 'ручек');
      const glyphBox = await boxOrThrow(grip.locator('svg'), 'глиф ручки');
      // Классы суммы и иконки — сам контракт анатомии, который тест
      // проверяет; локаторы замкнуты на строку.
      const amountBox = await boxOrThrow(
        row.locator('span.flex.shrink-0.flex-col'),
        'блок суммы',
      );
      const iconBox = await boxOrThrow(
        row.locator('span.h-11.w-11').first(),
        'иконка категории',
      );

      const fromRight = (box: { x: number; width: number }): number =>
        rowBox.x + rowBox.width - (box.x + box.width);
      const fromLeft = (box: { x: number }): number => box.x - rowBox.x;

      // Хит-зона ручки — слот 44×44 по анатомии макета.
      expect(gripBox.width).toBe(44);
      expect(gripBox.height).toBe(44);
      expect(fromRight(gripBox)).toBeCloseTo(12, 1);
      expect(fromRight(glyphBox)).toBeCloseTo(22, 1);
      expect(fromRight(amountBox)).toBeCloseTo(68, 1);
      expect(fromLeft(iconBox)).toBeCloseTo(24, 1);

      // Выход из правки для следующего яруса.
      await page.getByLabel('Выйти из правки').click();
      await expect(page.getByTestId('favorites-save')).toHaveCount(0);
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
    const gripBox = await boxOrThrow(
      page.getByTestId(`favorites-grip-${ids[0]}`),
      'ручек',
    );
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

  test('drag за ручку не выделяет строку — клик-хвост жеста съеден', async ({
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

    // Drag первой строки на вторую: строка уезжает под курсор, pointerup
    // ложится внутрь перетаскиваемой плашки — выделение не переключает.
    await dragGripToRow(page, `favorites-grip-${firstId}`, `favorites-edit-row-${secondId}`);

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
      `SELECT payment_id FROM user_payment_favorites
       WHERE user_id = ${OWNER_SQL}
         AND payment_id IN ('${FAVORITE_IDS.join("','")}');`,
    );
    expect(favoriteRows).toBe('');
  });
});
