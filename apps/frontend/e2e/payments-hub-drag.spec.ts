import type { Locator, Page } from '@playwright/test';
import {
  expect,
  openCabinetWithSeededSession,
  test,
  type SeededUser,
} from './fixtures';

// Drag секций хаба «Платежи» «как у Apple» (тикет #1349): мышь тащит
// ленту (pointer-drag прокси поверх нативного скролла), отпускание —
// инерционный доезд к snap-точке (inertia framer-motion), тач не трогается.
// Bleed ≤560: лента уходит в край вьюпорта, карточки и snap-точки — на
// линии 24. Клик-vs-drag: смещение до порога — клик открывает карточку,
// после порога — прокрутка и click отпускания гасится.

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

const scrollLeftOf = (rail: Locator): Promise<number> =>
  rail.evaluate((element) => (element as HTMLElement).scrollLeft);

/** Inline scroll-snap-type ленты: 'none' на время жеста/доезда (off-switch),
 * '' в покое — по нему ждём завершения доезда (onComplete хука). */
const snapTypeOf = (rail: Locator): Promise<string> =>
  rail.evaluate((element) => (element as HTMLElement).style.scrollSnapType);

/** Эффективные snap-точки ленты: левый край ребёнка относительно нуля
 * скролла минус scroll-padding (bleed держит линию 24), кламп в достижимый
 * диапазон scrollLeft — хвост ленты снапится к концу, как у нативного
 * скролла (та же математика, что collectSnapPoints хука — это контракт). */
const snapOffsetsOf = (rail: Locator): Promise<number[]> =>
  rail.evaluate((element) => {
    const railRect = element.getBoundingClientRect();
    const zeroX = railRect.left - element.scrollLeft;
    const maxScrollLeft = element.scrollWidth - element.clientWidth;
    const padding = Number.parseFloat(getComputedStyle(element).scrollPaddingLeft) || 0;
    return [...element.children].map((child) => {
      const point = child.getBoundingClientRect().left - zeroX - padding;
      return Math.min(Math.max(point, 0), maxScrollLeft);
    });
  });

const waitForRideEnd = async (page: Page): Promise<void> => {
  await page.waitForFunction(
    (testId) => {
      const element = document.querySelector(`[data-testid="${testId}"]`);
      return element !== null && (element as HTMLElement).style.scrollSnapType === '';
    },
    RAIL,
    { timeout: 5_000 },
  );
};

/** Остановилась ли лента ровно в ближайшей snap-точке (±1px). */
async function expectSettledAtSnap(rail: Locator): Promise<void> {
  const settled = await scrollLeftOf(rail);
  const snapOffsets = await snapOffsetsOf(rail);
  const nearest = snapOffsets.reduce((best, offset) =>
    Math.abs(offset - settled) < Math.abs(best - settled) ? offset : best,
  );
  expect(
    Math.abs(settled - nearest),
    `лента остановилась в ${settled}, ближайшая snap-точка ${nearest}`,
  ).toBeLessThanOrEqual(1);
}

test.describe('drag секций хаба — десктоп-мышь', () => {
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
    // Порог 8px не взят — скролл ещё на месте, drag не начался.
    await page.mouse.move(x - 5, y, { steps: 2 });
    await expect.poll(() => scrollLeftOf(rail)).toBe(0);
    await page.mouse.move(x - 90, y, { steps: 4 });
    // Жест идёт: scrollLeft следует за мышью, snap выключен off-switch'ем.
    await expect.poll(() => scrollLeftOf(rail)).toBeGreaterThan(50);
    await expect.poll(() => snapTypeOf(rail)).toBe('none');
    await page.mouse.move(x - 150, y, { steps: 4 });
    await page.mouse.up();

    await waitForRideEnd(page);
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
    const card = rail.locator('> div').first();
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
    const card = rail.locator('> div').first();
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

  test('после drag-доезда карточки кликабельны (снап восстановлен)', async ({
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
    await waitForRideEnd(page);

    // «Показать все» — последняя карточка ленты; первый после drag клик
    // должен пройти (гасится только click отпускания самого жеста).
    await rail.getByText('Показать все').click();
    await expect(page).toHaveURL(/\/payments\/objects$/);
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
      scrollPaddingLeft: getComputedStyle(element).scrollPaddingLeft,
      firstCardX: element.children[0]?.getBoundingClientRect().left ?? null,
    }));
    expect(geometry.scrollPaddingLeft).toBe('24px');
    // В покое (scrollLeft 0) первая карточка стоит на визуальной линии 24.
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
    await page.mouse.up();
    await waitForRideEnd(page);

    // С bleed snap-точки стоят на линии 24 (scroll-padding-inline) —
    // остановка в ближайшей из них проверяет и сам доезд, и паддинг.
    await expectSettledAtSnap(rail);
  });
});

test.describe('тач — нативное поведение, прокси не заходит в код', () => {
  test.use({ viewport: { width: 390, height: 844 }, hasTouch: true });

  test('тап по карточке открывает объект (drag-прокси не съедает тач)', async ({
    page,
    seededUser,
  }) => {
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

    // Тап в центр первой карточки: pointerType touch мимо гвара — клик жив.
    await page.touchscreen.tap(box.x + box.width / 2, box.y + box.height / 2);
    await expect(page).toHaveURL(/\/properties\/[0-9a-f-]{36}\/payments$/);
  });
});

test.describe('prefers-reduced-motion — жест жив, доезд мгновенный', () => {
  test.use({ viewport: { width: 1280, height: 900 } });

  test('отпускание ставит ленту в snap-точку без доезда', async ({
    page,
    seededUser,
  }) => {
    // Хук читает matchMedia реактивно (переподписка на change) —
    // эмуляция доходит и до флага reducedMotionRef.
    await page.emulateMedia({ reducedMotion: 'reduce' });
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

    // Доезд схлопнут в мгновенную установку: сразу после отпускания snap
    // уже восстановлен (анимированный путь держит off-switch до конца).
    await expect.poll(() => snapTypeOf(rail)).toBe('');
    await expectSettledAtSnap(rail);
    expect(await scrollLeftOf(rail)).toBeGreaterThan(0);
  });
});
