import type { Locator, Page } from '@playwright/test';
import {
  expect,
  openCabinetWithSeededSession,
  test,
  type SeededUser,
} from './fixtures';

// Drag секций хаба «Платежи» — Embla v9 dragFree: 'snap' (тикет #1349,
// решение владельца 13.10; канон механики — DESIGN.md §8 «Drag
// горизонтальных snap-лент»). Проверяется контракт, важный е2е: мышь
// тащит ленту, отпускание — доезд к ближайшей snap-точке, у краёв —
// сопротивление и возврат в границы, bleed ≤560 держит карточки и
// снап-линию на 24, клик живёт до dragThreshold 10px.

/** Лента «Платежей объектов»: 3 объекта + «Показать все» = 4 карточки
 * (168.5px + gap 8) ≈ 698px — переполняет и колонку 512, и мобильный
 * вьюпорт 390, т.е. скроллится на обоих ярусах. */
const RAIL = 'payments-rail-objects';

async function openHub(page: Page, seededUser: SeededUser): Promise<void> {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/payments');
}

/** Бокс элемента для жеста: boundingBox с честной ошибкой вместо
 * null-гвардов на месте вызова (канон favorites-edit.spec.ts). */
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

/** Смещение контейнера Embla (translate3d по X): отрицательное = прокрутили.
 * Контейнер — первый ребёнок вьюпорта, transform пишет сам Embla. */
const translateXOf = (rail: Locator): Promise<number> =>
  rail.evaluate((element) => {
    const container = element.firstElementChild as HTMLElement | null;
    const [, px] = /translate3d\((-?[\d.]+)px/.exec(container?.style.transform ?? '') ?? [];
    return px === undefined ? 0 : Number.parseFloat(px);
  });

/** Контейнер встал: две последовательные выборки translate совпали
 * (attraction-симуляция Embla заканчивается полным покоем transform). */
const waitForSettle = async (page: Page): Promise<void> => {
  let previous: number | null = null;
  await expect
    .poll(
      async () => {
        const current = await translateXOf(page.getByTestId(RAIL));
        const stable = previous !== null && Math.abs(current - previous) < 0.01;
        previous = current;
        return stable;
      },
      { timeout: 10_000 },
    )
    .toBe(true);
};

/** Список snap-целей translate и текущий translate: зеркало математики
 * Embla (ScrollSnaps/ScrollLimit/ScrollContain — это контракт): снап
 * карточки = минус её offset от контейнера, краевой снап клампится в
 * диапазон [viewSize − contentSize, 0] — конец ленты без пустот, как у
 * нативного скролла. Контейнер с transform — offsetParent для карточек,
 * поэтому offset-ы меряются с временно снятым transform (приём NodeHandler
 * Embla: getRects измеряет при transform: none). */
const snapStateOf = (
  rail: Locator,
): Promise<{ translate: number; snaps: number[] }> =>
  rail.evaluate((element) => {
    const container = element.firstElementChild as HTMLElement;
    const previousTransform = container.style.transform;
    const [, px] = /translate3d\((-?[\d.]+)px/.exec(previousTransform) ?? [];
    const translate = px === undefined ? 0 : Number.parseFloat(px);
    container.style.transform = 'none';
    const containerLeft = container.offsetLeft;
    const restOffsets = [...container.children].map(
      (card) => (card as HTMLElement).offsetLeft - containerLeft,
    );
    container.style.transform = previousTransform;
    const viewSize = container.offsetWidth;
    const last = [...container.children].at(-1) as HTMLElement | undefined;
    const lastOffset = restOffsets.at(-1);
    const contentSize =
      lastOffset === undefined || last === undefined
        ? 0
        : lastOffset + last.offsetWidth;
    const min = Math.min(0, viewSize - contentSize);
    return {
      translate,
      snaps: restOffsets.map((offset) => Math.min(Math.max(-offset, min), 0)),
    };
  });

/** Остановилась ли лента в ближайшей snap-точке (±1.5px: offsetLeft
 * округляется до целого). */
async function expectSettledAtSnap(rail: Locator): Promise<void> {
  const { translate, snaps } = await snapStateOf(rail);
  const nearest = snaps.reduce((best, snap) =>
    Math.abs(snap - translate) < Math.abs(best - translate) ? snap : best,
  );
  expect(
    Math.abs(translate - nearest),
    `лента встала в ${translate}, ближайшая snap-точка ${nearest} (все: ${JSON.stringify(snaps)})`,
  ).toBeLessThanOrEqual(1.5);
}

test.describe('drag секций хаба — десктоп-мышь (Embla)', () => {
  test.use({ viewport: { width: 1280, height: 900 } });

  test('мышь тащит ленту, отпускание — доезд к ближайшей snap-точке', async ({
    page,
    seededUser,
  }) => {
    await openHub(page, seededUser);
    const rail = page.getByTestId(RAIL);
    await rail.scrollIntoViewIfNeeded();
    const box = await boxOrThrow(rail, 'лента');
    const x = box.x + box.width / 2;
    const y = box.y + box.height / 2;

    await expect.poll(() => translateXOf(rail)).toBe(0);

    await page.mouse.move(x, y);
    await page.mouse.down();
    // Смещение до dragThreshold (10px) — ещё клик, но лента уже следует
    // 1:1; жёсткого порога старта у Embla нет, контракт — только клики.
    await page.mouse.move(x - 90, y, { steps: 4 });
    // Жест идёт: контейнер следует за мышью (translate уехал влево).
    await expect.poll(() => translateXOf(rail)).toBeLessThan(-50);
    await page.mouse.move(x - 150, y, { steps: 4 });
    // Пауза гасит скорость броска: отпускание из покоя — доезд к снапу,
    // ближайшему к точке отпускания (детерминированный mid-снап).
    await page.waitForTimeout(150);
    await page.mouse.up();

    await waitForSettle(page);
    await expectSettledAtSnap(rail);
    expect(await translateXOf(rail)).toBeLessThan(0);
  });

  test('drag до порога не съедает клик по карточке', async ({
    page,
    seededUser,
  }) => {
    await openHub(page, seededUser);
    const rail = page.getByTestId(RAIL);
    await rail.scrollIntoViewIfNeeded();
    const card = rail.locator('> div > div').first();
    const box = await boxOrThrow(card, 'карточка');
    const x = box.x + box.width / 2;
    const y = box.y + box.height / 2;

    await page.mouse.move(x, y);
    await page.mouse.down();
    await page.mouse.move(x - 4, y, { steps: 2 });
    await page.mouse.up();

    // Первая карточка ленты — первый объект сида по алфавиту («Гараж на
    // Садовой», 44444444); ассерт не привязан к конкретному объекту.
    await expect(page).toHaveURL(/\/properties\/[0-9a-f-]{36}\/payments$/);
  });

  test('реальный drag гасит click — карточка не открывается', async ({
    page,
    seededUser,
  }) => {
    await openHub(page, seededUser);
    const rail = page.getByTestId(RAIL);
    await rail.scrollIntoViewIfNeeded();
    const card = rail.locator('> div > div').first();
    const box = await boxOrThrow(card, 'карточка');
    const x = box.x + box.width / 2;
    const y = box.y + box.height / 2;

    await page.mouse.move(x, y);
    await page.mouse.down();
    await page.mouse.move(x - 120, y, { steps: 8 });
    await page.mouse.up();

    expect(page.url()).toMatch(/\/payments$/);
    // И лента реально прокрутилась — жест был скроллом, а не кликом.
    await expect.poll(() => translateXOf(rail)).toBeLessThan(-50);
  });

  test('после drag-доезда карточки кликабельны (гасится только click жеста)', async ({
    page,
    seededUser,
  }) => {
    await openHub(page, seededUser);
    const rail = page.getByTestId(RAIL);
    await rail.scrollIntoViewIfNeeded();
    const box = await boxOrThrow(rail, 'лента');
    const x = box.x + box.width / 2;
    const y = box.y + box.height / 2;

    await page.mouse.move(x, y);
    await page.mouse.down();
    await page.mouse.move(x - 150, y, { steps: 8 });
    await page.mouse.up();
    await waitForSettle(page);

    // «Показать все» — последняя карточка ленты; первый после drag клик
    // должен пройти (гасится только click отпускания самого жеста).
    await rail.getByText('Показать все').click();
    await expect(page).toHaveURL(/\/payments\/objects$/);
  });

  test('протяжка за правый край: сопротивление, отпускание возвращает в границы', async ({
    page,
    seededUser,
  }) => {
    await openHub(page, seededUser);
    const rail = page.getByTestId(RAIL);
    await rail.scrollIntoViewIfNeeded();
    const box = await boxOrThrow(rail, 'лента');
    const x = box.x + box.width / 2;
    const y = box.y + box.height / 2;

    // Сильный бросок влево — лента доезжает до конца диапазона.
    await page.mouse.move(x, y);
    await page.mouse.down();
    await page.mouse.move(x - 400, y, { steps: 5 });
    await page.mouse.up();
    await waitForSettle(page);
    const atEnd = await translateXOf(rail);

    // Тянем дальше влево (за правый край контента): движение вязнет —
    // смещение меньше жеста (сопротивление резинки); после отпускания
    // лента возвращается ровно в конечный снап — блоки не остаются за
    // границей и не висят в пустоте.
    await page.mouse.move(x, y);
    await page.mouse.down();
    await page.mouse.move(x - 60, y, { steps: 10 });
    const stretched = await translateXOf(rail);
    await page.mouse.up();
    await waitForSettle(page);

    expect(
      stretched,
      `резинка не даёт тянуть за край как по маслу: было ${atEnd}, стало ${stretched}`,
    ).toBeGreaterThan(atEnd - 55);
    expect(
      Math.abs((await translateXOf(rail)) - atEnd),
      'после отпускания лента вернулась в конечный снап',
    ).toBeLessThanOrEqual(1.5);

    // Конец ленты без пустот: последняя карточка вровень с краем вьюпорта.
    const endOverflow = await rail.evaluate((element) => {
      const container = element.firstElementChild as HTMLElement;
      const last = [...container.children].at(-1);
      if (last === undefined) {
        throw new Error('в ленте нет карточек');
      }
      return (
        last.getBoundingClientRect().right - element.getBoundingClientRect().right
      );
    });
    expect(Math.abs(endOverflow), 'конец ленты не вровень с краем').toBeLessThanOrEqual(2);
  });
});

test.describe('bleed лент на мобайл-ярусе (≤560)', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('лента в край вьюпорта, карточки и snap — на линии 24', async ({
    page,
    seededUser,
  }) => {
    await openHub(page, seededUser);
    const rail = page.getByTestId(RAIL);
    await rail.scrollIntoViewIfNeeded();

    const box = await boxOrThrow(rail, 'лента');
    expect(box.x, 'лента прижата к левому краю вьюпорта').toBe(0);
    expect(box.width, 'лента на всю ширину вьюпорта (bleed)').toBe(390);

    const geometry = await rail.evaluate((element) => ({
      paddingLeft: getComputedStyle(element).paddingLeft,
      firstCardX: element.firstElementChild?.getBoundingClientRect().left ?? null,
    }));
    expect(geometry.paddingLeft).toBe('24px');
    // В покое (translate 0) первая карточка стоит на визуальной линии 24.
    expect(geometry.firstCardX).toBeCloseTo(24, 0);
  });

  test('drag мышью на узком вьюпорте доезжает до снапа с bleed-паддингом', async ({
    page,
    seededUser,
  }) => {
    await openHub(page, seededUser);
    const rail = page.getByTestId(RAIL);
    await rail.scrollIntoViewIfNeeded();
    const box = await boxOrThrow(rail, 'лента');
    const x = box.x + box.width / 2;
    const y = box.y + box.height / 2;

    await page.mouse.move(x, y);
    await page.mouse.down();
    await page.mouse.move(x - 140, y, { steps: 8 });
    // Пауза гасит скорость: доезд к снапу у точки отпускания.
    await page.waitForTimeout(150);
    await page.mouse.up();
    await waitForSettle(page);

    // Контейнер сидит внутри bleed-паддинга, так что снап «край контейнера»
    // = линия 24; список снапов включает краевой (конец ленты без пустот).
    await expectSettledAtSnap(rail);
    // Осевшая карточка стоит ровно на линии 24 (bleed-паддинг = снап-линия).
    const visibleCardX = await rail.evaluate((element) => {
      const container = element.firstElementChild as HTMLElement;
      const lefts = [...container.children].map(
        (card) => card.getBoundingClientRect().left,
      );
      const visible = lefts.filter((left) => left > -1);
      return visible.length > 0 ? Math.min(...visible) : null;
    });
    expect(visibleCardX, 'осевшая карточка на линии 24 (не на краю)').toBeGreaterThanOrEqual(22);
    expect(visibleCardX, 'осевшая карточка на линии 24').toBeLessThanOrEqual(25.5);
  });
});

test.describe('тач — единый движок Embla, тап жив', () => {
  test.use({ viewport: { width: 390, height: 844 }, hasTouch: true });

  test('тап по карточке открывает объект', async ({ page, seededUser }) => {
    await openHub(page, seededUser);
    const rail = page.getByTestId(RAIL);
    await rail.scrollIntoViewIfNeeded();
    await page.waitForTimeout(300);
    // Лента после scrollIntoViewIfNeeded садится на нижний край — под
    // TabBar; колесо поднимает страницу, чтобы тап пришёл в карточку.
    await page.mouse.move(195, 300);
    await page.mouse.wheel(0, 200);
    await page.waitForTimeout(300);
    const box = await boxOrThrow(rail, 'лента');

    // Тап в центр первой карточки: движение нулевое — Embla не гасит клик.
    await page.touchscreen.tap(box.x + box.width / 2, box.y + box.height / 2);
    await expect(page).toHaveURL(/\/properties\/[0-9a-f-]{36}\/payments$/);
  });
});

test.describe('prefers-reduced-motion — без инерционного доезда', () => {
  test.use({ viewport: { width: 1280, height: 900 } });

  test('бросок с скоростью оседает в соседнем снапе, а не в проекции остановки', async ({
    page,
    seededUser,
  }) => {
    // dragFree: false при reduced-motion: отпускание с velocity даёт
    // докат к СОСЕДНЕМУ снапу; регрессия в dragFree: 'snap' уехала бы к
    // снапу у проекции остановки (здесь — краевому, дальше на ~10px) —
    // ассерт ниже это различает.
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await openHub(page, seededUser);
    const rail = page.getByTestId(RAIL);
    await rail.scrollIntoViewIfNeeded();
    const box = await boxOrThrow(rail, 'лента');
    const x = box.x + box.width / 2;
    const y = box.y + box.height / 2;

    await page.mouse.move(x, y);
    await page.mouse.down();
    await page.mouse.move(x - 150, y, { steps: 2 });
    await page.mouse.up();

    await waitForSettle(page);
    const { translate, snaps } = await snapStateOf(rail);
    const adjacentSnap = snaps.at(1);
    if (adjacentSnap === undefined) {
      throw new Error('в ленте меньше двух snap-точек');
    }
    expect(
      Math.abs(translate - adjacentSnap),
      `бросок осел в ${translate}, соседний снап ${adjacentSnap} (все: ${JSON.stringify(snaps)})`,
    ).toBeLessThanOrEqual(1.5);
    expect(translate).toBeLessThan(0);
  });
});
