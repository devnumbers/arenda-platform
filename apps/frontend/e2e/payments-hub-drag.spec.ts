import type { Locator, Page } from '@playwright/test';
import {
  expect,
  openCabinetWithSeededSession,
  test,
  type SeededUser,
} from './fixtures';

// Drag секций хаба «Платежи» — нативный тач + drag-прокси мыши/пера
// (тикет #1349, решение владельца 14.10 — возврат с Embla; канон механики —
// DESIGN.md §8 «Drag горизонтальных snap-лент»). Проверяется контракт,
// важный е2е: мышь тащит ленту (scrollLeft), отпускание — доезд к ближайшей
// snap-точке проекции остановки, у краёв — резинка (трансформ контента) и
// возврат в границы, bleed ≤560 держит карточки и снап-линию на 24
// (scroll-padding), клик живёт до порога 8px, тач-тап нативный.

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

/** Прокрутка ленты: scrollLeft нативного скролла (положительное = вправо). */
const scrollLeftOf = (rail: Locator): Promise<number> =>
  rail.evaluate((element) => element.scrollLeft);

/** Резинка за правым краем: поле-расширение контента, px (пустой инлайн —
 * резинки нет). */
const rubberOf = (rail: Locator): Promise<number> =>
  rail.evaluate((element) => {
    const margin = (element.firstElementChild as HTMLElement | null)?.style
      .marginRight;
    return margin === undefined || margin === ''
      ? 0
      : Number.parseFloat(margin);
  });

/** Лента встала: scrollLeft и резинка стабильны на двух выборках подряд
 * (спринг доезда заканчивается полным покоем обеих линий). */
const waitForSettle = async (page: Page): Promise<void> => {
  let previousScroll: number | null = null;
  let previousRubber: number | null = null;
  await expect
    .poll(
      async () => {
        const rail = page.getByTestId(RAIL);
        const scroll = await scrollLeftOf(rail);
        const rubber = await rubberOf(rail);
        const stable =
          previousScroll !== null &&
          Math.abs(scroll - previousScroll) < 0.01 &&
          previousRubber !== null &&
          Math.abs(rubber - previousRubber) < 0.01;
        previousScroll = scroll;
        previousRubber = rubber;
        return stable;
      },
      { timeout: 10_000 },
    )
    .toBe(true);
};

/** Список snap-целей scrollLeft и текущий scrollLeft: зеркало механики
 * прокси (collectSnapPoints — это контракт): снап карточки = её offsetLeft
 * минус offsetLeft контейнера, краевой снап клампится в достижимый
 * диапазон [0, scrollWidth − clientWidth] — конец ленты без пустот, как у
 * нативного скролла. Замер на offsetLeft — трансформ-резинка лэйаут не
 * двигает, точки верны в любой момент. */
const snapStateOf = (
  rail: Locator,
): Promise<{ scroll: number; snaps: number[] }> =>
  rail.evaluate((element) => {
    const content = element.firstElementChild as HTMLElement;
    const contentLeft = content.offsetLeft;
    const maxScrollLeft = element.scrollWidth - element.clientWidth;
    const snaps = [...content.children].map(
      (card) =>
        Math.min(
          Math.max((card as HTMLElement).offsetLeft - contentLeft, 0),
          maxScrollLeft,
        ),
    );
    return { scroll: element.scrollLeft, snaps };
  });

/** Остановилась ли лента в ближайшей snap-точке (±1.5px: offsetLeft
 * округляется до целого). */
async function expectSettledAtSnap(rail: Locator): Promise<void> {
  const { scroll, snaps } = await snapStateOf(rail);
  const nearest = snaps.reduce((best, snap) =>
    Math.abs(snap - scroll) < Math.abs(best - scroll) ? snap : best,
  );
  expect(
    Math.abs(scroll - nearest),
    `лента встала в ${scroll}, ближайшая snap-точка ${nearest} (все: ${JSON.stringify(snaps)})`,
  ).toBeLessThanOrEqual(1.5);
}

test.describe('drag секций хаба — десктоп-мышь (нативный скролл + прокси)', () => {
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

    await expect.poll(() => scrollLeftOf(rail)).toBe(0);

    await page.mouse.move(x, y);
    await page.mouse.down();
    // Смещение до порога (8px) — ещё клик, но лента уже следует 1:1.
    await page.mouse.move(x - 90, y, { steps: 4 });
    // Жест идёт: лента следует за мышью (scrollLeft уехал вправо).
    await expect.poll(() => scrollLeftOf(rail)).toBeGreaterThan(50);
    await page.mouse.move(x - 150, y, { steps: 4 });
    // Пауза гасит скорость броска: отпускание из покоя — доезд к снапу,
    // ближайшему к точке отпускания (детерминированный mid-снап).
    await page.waitForTimeout(150);
    await page.mouse.up();

    await waitForSettle(page);
    await expectSettledAtSnap(rail);
    expect(await scrollLeftOf(rail)).toBeGreaterThan(0);
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
    await expect.poll(() => scrollLeftOf(rail)).toBeGreaterThan(50);
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

  test('протяжка за правый край: резинка вязнет, отпускание возвращает в границы', async ({
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
    const endScroll = await scrollLeftOf(rail);

    // Тянем дальше влево (за правый край контента): scrollLeft стоит на
    // границе, резинка играет трансформом контента — 60px жеста дают
    // меньше хода (сопротивление растёт с дистанцией).
    await page.mouse.move(x, y);
    await page.mouse.down();
    await page.mouse.move(x - 60, y, { steps: 10 });
    const stretched = await rubberOf(rail);
    await page.mouse.up();
    await waitForSettle(page);

    expect(
      stretched,
      `резинка должна вязнуть: 60px жеста дали поле ${stretched}`,
    ).toBeGreaterThan(10);
    expect(stretched).toBeLessThan(55);
    expect(await rubberOf(rail), 'после отпускания резинка вернулась в 0').toBe(0);
    expect(
      Math.abs((await scrollLeftOf(rail)) - endScroll),
      'после отпускания лента вернулась в конечный снап',
    ).toBeLessThanOrEqual(1.5);

    // Конец ленты без пустот: последняя карточка вровень с краем вьюпорта.
    const endOverflow = await rail.evaluate((element) => {
      const content = element.firstElementChild as HTMLElement;
      const last = [...content.children].at(-1);
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
      scrollPaddingLeft: getComputedStyle(element).scrollPaddingLeft,
      firstCardX: element.firstElementChild?.getBoundingClientRect().left ?? null,
    }));
    expect(geometry.paddingLeft).toBe('24px');
    // scroll-padding — линия снапа нативного скролла и drag-прокси.
    expect(geometry.scrollPaddingLeft).toBe('24px');
    // В покое (scroll 0) первая карточка стоит на визуальной линии 24.
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

    // Снап-линия = scroll-padding 24; список снапов включает краевой
    // (конец ленты без пустот).
    await expectSettledAtSnap(rail);
    // Осевшая карточка стоит ровно на линии 24 (bleed-паддинг = снап-линия).
    const visibleCardX = await rail.evaluate((element) => {
      const content = element.firstElementChild as HTMLElement;
      const lefts = [...content.children].map(
        (card) => card.getBoundingClientRect().left,
      );
      const visible = lefts.filter((left) => left > -1);
      return visible.length > 0 ? Math.min(...visible) : null;
    });
    expect(visibleCardX, 'осевшая карточка на линии 24 (не на краю)').toBeGreaterThanOrEqual(22);
    expect(visibleCardX, 'осевшая карточка на линии 24').toBeLessThanOrEqual(25.5);
  });
});

test.describe('тач — нативный скролл, тап жив', () => {
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

    // Тап в центр первой карточки: тач в прокси не заходит (гвард
    // pointerType) — нативный скролл не гасит клик.
    await page.touchscreen.tap(box.x + box.width / 2, box.y + box.height / 2);
    await expect(page).toHaveURL(/\/properties\/[0-9a-f-]{36}\/payments$/);
  });
});

test.describe('prefers-reduced-motion — докат к ближайшему снапу', () => {
  test.use({ viewport: { width: 1280, height: 900 } });

  test('бросок с скоростью оседает в соседнем снапе, а не в проекции остановки', async ({
    page,
    seededUser,
  }) => {
    // Reduced-motion отключает инерционную проекцию: отпускание с velocity
    // даёт докат к СОСЕДНЕМУ снапу (ближайшему к точке отпускания);
    // регрессия к проекции уехала бы к снапу у проекции остановки
    // (здесь — краевому, дальше на ~10px) — ассерт ниже это различает.
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
    const { scroll, snaps } = await snapStateOf(rail);
    const adjacentSnap = snaps.at(1);
    if (adjacentSnap === undefined) {
      throw new Error('в ленте меньше двух snap-точек');
    }
    expect(
      Math.abs(scroll - adjacentSnap),
      `бросок осел в ${scroll}, соседний снап ${adjacentSnap} (все: ${JSON.stringify(snaps)})`,
    ).toBeLessThanOrEqual(1.5);
    expect(scroll).toBeGreaterThan(0);
  });
});
