import type { Locator, Page } from '@playwright/test';
import {
  expect,
  openCabinetWithSeededSession,
  test,
  type SeededUser,
} from './fixtures';

// Drag секций хаба «Платежи» — нативный тач + drag-прокси мыши/пера
// (#1349; без снапа — решение владельца 19.10 «магнитность убрать, пусть
// просто скроллится»; канон механики — DESIGN.md §8 «Drag горизонтальных
// лент»). Проверяется контракт, важный е2е: мышь тащит ленту (scrollLeft),
// отпускание из покоя НЕ магнитит ленту к краю, отпускание с velocity —
// инерционный доезд по проекции остановки в пределах диапазона, у краёв —
// резинка (поле последней карточки / трансформ) и возврат в границы,
// bleed ≤560 держит карточки на линии 24, клик живёт до порога 8px,
// тач-тап нативный, свайп пальцем скроллит нативно (touch-action pan-x),
// плитки fluid: 2 на мобилке, 3 на планшете/ПК (решение владельца 18.10).

/** Лента «Платежей объектов»: 3 объекта + «Показать все» = 4 карточки
 * (fluid ≥165px + gap 8) ≈ 685px — переполняет и колонку 512, и мобильный
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

/** Резинка за правым краем: поле-расширение ПОСЛЕДНЕЙ КАРТОЧКИ (w-full
 * контент-див прячет своё поле за торчащими детьми), px (пустой инлайн —
 * резинки нет). */
const rubberOf = (rail: Locator): Promise<number> =>
  rail.evaluate((element) => {
    const last = (element.firstElementChild as HTMLElement | null)
      ?.lastElementChild as HTMLElement | null;
    const margin = last?.style.marginRight;
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

/** Достижимый максимум scrollLeft (дробный, по rect последней карточки —
 * зеркало baseMaxScrollLeft прокси). */
const maxScrollOf = (rail: Locator): Promise<number> =>
  rail.evaluate((element) => {
    const content = element.firstElementChild as HTMLElement;
    const last = [...content.children].at(-1) as HTMLElement | undefined;
    const extent =
      last !== undefined
        ? last.getBoundingClientRect().right -
          content.getBoundingClientRect().left
        : content.scrollWidth;
    const paddingLeft =
      Number.parseFloat(getComputedStyle(element).paddingLeft) || 0;
    return (
      Math.max(paddingLeft + extent, element.clientWidth) -
      element.clientWidth
    );
  });

test.describe('drag секций хаба — десктоп-мышь (нативный скролл + прокси)', () => {
  test.use({ viewport: { width: 1280, height: 900 } });

  test('отпускание из покоя не магнитит ленту к краю (без снапа, 19.10)', async ({
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
    // Пауза гасит скорость: отпускание из покоя — лента ОСТАЁТСЯ у точки
    // отпускания. Магнит к началу (регресс 19.10: Chrome реснапил ленту
    // к запомненной цели при переподключении снапа) — запрет.
    await page.waitForTimeout(150);
    const atRelease = await scrollLeftOf(rail);
    await page.mouse.up();

    await waitForSettle(page);
    const settled = await scrollLeftOf(rail);
    expect(
      Math.abs(settled - atRelease),
      `отпустили в ${atRelease}, осела в ${settled} — магнит к краю`,
    ).toBeLessThanOrEqual(4);
    expect(settled).toBeGreaterThan(50);
  });

  test('отпускание с velocity — инерционный доезд по проекции', async ({
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

    // Резкий короткий флик: отпускание в движении — инерция увозит ленту
    // дальше точки отпускания (проекция остановки d=0.998) и осаживает
    // её в пределах диапазона, без привязки к карточным линиям.
    await page.mouse.move(x, y);
    await page.mouse.down();
    await page.mouse.move(x - 60, y, { steps: 2 });
    const atRelease = await scrollLeftOf(rail);
    await page.mouse.up();

    await waitForSettle(page);
    const settled = await scrollLeftOf(rail);
    expect(
      settled,
      `отпустили в ${atRelease}, инерция не увезла (${settled})`,
    ).toBeGreaterThan(atRelease + 20);
    const max = await maxScrollOf(rail);
    expect(settled).toBeLessThanOrEqual(max + 1.5);
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

  test('drag не выделяет текст карточек — выделение гасится на активном жесте (#1436)', async ({
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

    // Выделение текста синтетической мышью не воспроизводится вовсе
    // (drag-select в Chromium ведётся реальным вводом) — проверяется
    // сам механизм подавления: на активном жесте на ленте стоит
    // user-select: none (жест-стиль, гасит drag-select реального ввода,
    // apple-design §2 — прямая манипуляция без побочных эффектов),
    // после жеста стиль снят — честное выделение текста мышью живёт.
    await page.mouse.move(x, y);
    await page.mouse.down();
    await page.mouse.move(x - 2, y + 16, { steps: 4 });
    await page.mouse.move(x - 120, y, { steps: 8 });
    await expect
      .poll(() => rail.evaluate((element) => element.style.userSelect))
      .toBe('none');
    await page.mouse.up();
    await waitForSettle(page);
    await expect
      .poll(() => rail.evaluate((element) => element.style.userSelect))
      .toBe('');
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
    const max = await maxScrollOf(rail);
    expect(
      Math.abs((await scrollLeftOf(rail)) - max),
      'после отпускания лента вернулась в правую границу',
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

  test('лента в край вьюпорта, карточки — на визуальной линии 24', async ({
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
    // В покое (scroll 0) первая карточка стоит на визуальной линии 24.
    expect(geometry.firstCardX).toBeCloseTo(24, 0);
  });

  test('drag мышью на узком вьюпорте: отпускание из покоя остаётся у точки (bleed)', async ({
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
    // Пауза гасит скорость: отпускание из покоя — лента остаётся у точки.
    await page.waitForTimeout(150);
    const atRelease = await scrollLeftOf(rail);
    await page.mouse.up();
    await waitForSettle(page);

    const settled = await scrollLeftOf(rail);
    expect(
      Math.abs(settled - atRelease),
      `отпустили в ${atRelease}, осела в ${settled}`,
    ).toBeLessThanOrEqual(4);
    // Bleed жив: осевшая карточка в пределах видимости (не улетела за
    // край вьюпорта) — на линии 24 или правее, свободно.
    const visibleCardX = await rail.evaluate((element) => {
      const content = element.firstElementChild as HTMLElement;
      const lefts = [...content.children].map(
        (card) => card.getBoundingClientRect().left,
      );
      const visible = lefts.filter((left) => left > -1);
      return visible.length > 0 ? Math.min(...visible) : null;
    });
    expect(visibleCardX, 'после жеста карточка видна').toBeGreaterThanOrEqual(-1);
  });

  test('полный скролл — последняя карточка на стандартном отступе 24 (19.10)', async ({
    page,
    seededUser,
  }) => {
    await openHub(page, seededUser);
    const rail = page.getByTestId(RAIL);
    await rail.scrollIntoViewIfNeeded();

    /** Зазор от правого края последней карточки до края вьюпорта, px. */
    const endGapOf = () =>
      rail.evaluate((element) => {
        const content = element.firstElementChild as HTMLElement;
        const last = [...content.children].at(-1) as HTMLElement | undefined;
        if (last === undefined) {
          throw new Error('в ленте нет карточек');
        }
        return (
          element.getBoundingClientRect().right -
          last.getBoundingClientRect().right
        );
      });

    // Нативный путь (тач): доезд в конец диапазона браузера — последняя
    // карточка встаёт на стандартный отступ страницы 24px (px-6), а не
    // вплотную к краю экрана (решение владельца 19.10; хвост — канон
    // SnapRail, телефонный ярус ≤560).
    await rail.evaluate((element) => {
      element.scrollLeft = element.scrollWidth;
    });
    await page.waitForTimeout(150);
    expect(await endGapOf(), 'нативный доезд: хвостовой отступ 24').toBeCloseTo(24, 0);

    // Прокси-путь (мышь): бросок за край из конца — возврат резинки
    // садится на тот же максимум, хвост не съедается клампом прокси.
    const box = await boxOrThrow(rail, 'лента');
    const x = box.x + box.width / 2;
    const y = box.y + box.height / 2;
    await page.mouse.move(x, y);
    await page.mouse.down();
    await page.mouse.move(x - 300, y, { steps: 5 });
    await page.mouse.up();
    await waitForSettle(page);
    expect(await endGapOf(), 'доезд прокси: хвостовой отступ 24').toBeCloseTo(24, 0);
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

  test('свайп пальцем скроллит ленту нативно (touch-action pan-x)', async ({
    page,
    seededUser,
  }) => {
    // Регрессия 18.10: touch-action pan-y (по образцу JS-карусели, где
    // горизонталь ведёт свой JS) глушит НАТИВНЫЙ горизонтальный скролл —
    // на телефоне лента не двигалась вовсе (прокси тач не берёт). Контракт:
    // pan-x разрешает браузеру вести горизонтальную ось — свайп пальцем
    // скроллит нативно и оседает в snap-точке.
    await openHub(page, seededUser);
    const rail = page.getByTestId(RAIL);
    await rail.scrollIntoViewIfNeeded();
    await page.waitForTimeout(300);
    await page.mouse.move(195, 300);
    await page.mouse.wheel(0, 200);
    await page.waitForTimeout(300);
    const box = await boxOrThrow(rail, 'лента');
    const y = box.y + box.height / 2;
    const startX = box.x + box.width / 2;

    // Реальный тач-свайп: touchscreen API Playwright умеет только тап,
    // жест собирается через CDP Input.dispatchTouchEvent.
    const session = await page.context().newCDPSession(page);
    await session.send('Input.dispatchTouchEvent', {
      type: 'touchStart',
      touchPoints: [{ x: startX, y }],
    });
    for (const dx of [20, 45, 75, 110, 150, 200]) {
      await session.send('Input.dispatchTouchEvent', {
        type: 'touchMove',
        touchPoints: [{ x: startX - dx, y }],
      });
      await page.waitForTimeout(16);
    }
    await session.send('Input.dispatchTouchEvent', {
      type: 'touchEnd',
      touchPoints: [],
    });

    await expect
      .poll(() => scrollLeftOf(rail), { timeout: 5_000 })
      .toBeGreaterThan(100);
    // Proximity-канон (решение владельца 18.10): после нативного флика
    // браузер волен остановить ленту У САМОЙ снап-точки (не строго на ней)
    // — ассерт строгости снапа тут не применяется, его держит drag-прокси
    // мыши. Достаточно того, что лента нативно уехала и успокоилась.
    await waitForSettle(page);
  });
});

test.describe('fluid-ширины плиток (решение владельца 18.10: 2 на мобилке, 3 на планшете/ПК)', () => {
  const cardBoxes = (
    rail: Locator,
  ): Promise<ReadonlyArray<{ width: number; left: number; right: number }>> =>
    rail.evaluate((element) =>
      [...(element.firstElementChild as HTMLElement).children].map((card) => {
        const box = card.getBoundingClientRect();
        return { width: box.width, left: box.left, right: box.right };
      }),
    );
  const railRightOf = (rail: Locator): Promise<number> =>
    rail.evaluate((element) => element.getBoundingClientRect().right);

  const fourOrThrow = (
    cards: ReadonlyArray<{ width: number; left: number; right: number }>,
  ): readonly [
    { width: number; left: number; right: number },
    { width: number; left: number; right: number },
    { width: number; left: number; right: number },
    { width: number; left: number; right: number },
  ] => {
    if (cards.length < 4) {
      throw new Error(`в ленте ${cards.length} карточек, ожидалось 4`);
    }
    return cards as readonly [
      { width: number; left: number; right: number },
      { width: number; left: number; right: number },
      { width: number; left: number; right: number },
      { width: number; left: number; right: number },
    ];
  };

  test.describe('мобилка 390', () => {
    test.use({ viewport: { width: 390, height: 844 } });

    test('карточки (100%−8)/2 = 167px, в ряду две полных', async ({
      page,
      seededUser,
    }) => {
      await openHub(page, seededUser);
      const rail = page.getByTestId(RAIL);
      await rail.scrollIntoViewIfNeeded();
      const [first, second, third] = fourOrThrow(await cardBoxes(rail));
      // (390 − 2×24 паддинга bleed − 8 gap) / 2
      expect(first.width).toBeCloseTo(167, 0);
      expect(second.width).toBeCloseTo(167, 0);
      // Вторая карточка вровень с контентным краем (24+167+8+167 = 366),
      // третья выглядывает хвостом (peek), не обрезана посередине.
      expect(second.right).toBeCloseTo(390 - 24, 0);
      expect(third.left).toBeGreaterThan(366);
      expect(third.left).toBeLessThan(390);
    });
  });

  test.describe('планшет-ярус колонки 768 (561–1023)', () => {
    test.use({ viewport: { width: 768, height: 1024 } });

    test('карточки (100%−16)/3 ≈ 165.3px, в ряду три полных без обрезков', async ({
      page,
      seededUser,
    }) => {
      await openHub(page, seededUser);
      const rail = page.getByTestId(RAIL);
      await rail.scrollIntoViewIfNeeded();
      const [first, , third, fourth] = fourOrThrow(await cardBoxes(rail));
      const railRight = await railRightOf(rail);
      // (512 − 2×8 gap) / 3 — колонка 560 минус боковые 24 обёртки хаба.
      expect(first.width).toBeCloseTo(165.33, 0);
      expect(third.width).toBeCloseTo(165.33, 0);
      // Третья карточка вровень с правым краем колонки, четвёртая —
      // целиком за клипом (никаких 6px обрезков четвёртой).
      expect(third.right).toBeLessThanOrEqual(railRight + 1.5);
      expect(fourth.left).toBeGreaterThanOrEqual(railRight + 4);
    });
  });
});

test.describe('prefers-reduced-motion — без инерционной проекции', () => {
  test.use({ viewport: { width: 1280, height: 900 } });

  test('бросок остаётся у точки отпускания (без проекции и магнита)', async ({
    page,
    seededUser,
  }) => {
    // Reduced-motion отключает инерционную проекцию: лента остаётся у
    // пальца; никакого магнита к карточным линиям или краю (19.10).
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
    await page.waitForTimeout(150);
    const atRelease = await scrollLeftOf(rail);
    await page.mouse.up();

    await waitForSettle(page);
    const settled = await scrollLeftOf(rail);
    expect(
      Math.abs(settled - atRelease),
      `отпустили в ${atRelease}, осела в ${settled}`,
    ).toBeLessThanOrEqual(4);
    expect(settled).toBeGreaterThan(0);
  });
});
