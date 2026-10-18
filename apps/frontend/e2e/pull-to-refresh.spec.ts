import { type Page } from '@playwright/test';
import { expect, openCabinetWithSeededSession, test } from './fixtures';

// Платформенный гейт кастомного pull-to-refresh (ротация ADR 0031,
// решение владельца 12.10.2026): жест живёт только на iOS standalone, где
// WebKit отключает нативный PTR. На Android standalone (WebAPK) системный
// reload-жест браузера должен остаться нетронутым — кастомный компонент не
// биндит слушатели и не вешает overscroll-гвард. Опыт-эталон — PWA T-Banka
// (research/pwa-native-pull-to-refresh.md).
//
// Гейт цели касания (решение владельца 19.10.2026): жест принадлежит
// только документной поверхности — тяга не стартует из внутреннего
// скроллера (лента календаря, визард), из-под открытой модальной
// поверхности (лок фона) и не кормится зигзагом скролла или
// горизонтальным жестом (направление решается после гистерезиса ~10px,
// разворот вверх честно кончает жест).
//
// Обе ветки эмулируются: standalone — моком matchMedia (канон auth.spec.ts,
// headless Chromium не умеет display-mode: standalone), платформа —
// подменой userAgent (test.use уровня describe: внутри тела теста фикстуры
// уже созданы). Жест диспетчеризуется синтетическими TouchEvent-ами на
// window — слушатели компонента слушают именно window; наблюдаемый след —
// класс ptr-suppress-overscroll на <html>, который активный жест вешает на
// touchstart и снимает по завершении refresh, и инлайн-стили спиннера,
// которые компонент пишет покадрово во время тяги.

const IPHONE_SAFARI_UA =
  'Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1';
const ANDROID_CHROME_UA =
  'Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36';

/** Мок matchMedia: матчится только standalone-запрос (канон auth.spec.ts). */
async function emulateStandalonePwa(page: Page): Promise<void> {
  await page.addInitScript(() => {
    const native = window.matchMedia.bind(window);
    window.matchMedia = (query: string): MediaQueryList => {
      if (query === '(display-mode: standalone)') {
        const mql = native(query);
        return {
          matches: true,
          media: mql.media,
          onchange: null,
          addEventListener: () => {},
          removeEventListener: () => {},
          addListener: () => {},
          removeListener: () => {},
          dispatchEvent: () => false,
        } as MediaQueryList;
      }
      return native(query);
    };
  });
}

type GestureStep = 'start' | 'pull' | 'end';
type TouchEventType = 'touchstart' | 'touchmove' | 'touchend';
/** Цель документной поверхности: реальный WebKit даёт жесту элемент старта
 *  касания (не body) — в кабинете это контент страницы, первый заголовок. */
const CONTENT_TARGET = 'h1, h2, h3';

/** Тело ленты календаря в открытом пикере периода: внутренний скроллер
 *  (overflow-y-auto overscroll-contain), канон CalendarRangePicker. */
const CALENDAR_FEED_TARGET = '[role="dialog"] .overflow-y-auto';

/** Низкоуровневый шаг синтетического жеста: диспетчеризует TouchEvent на
 *  window с явной целью (селектор) и координатами. Слушатели компонента
 *  читают цель из event.touches[0] — элемент старта касания, как в реальном
 *  WebKit. */
async function dispatchTouch(
  page: Page,
  type: TouchEventType,
  point: { readonly x: number; readonly y: number },
  targetSelector: string = CONTENT_TARGET,
  opts: { readonly scrollToTopFirst?: boolean } = {},
): Promise<void> {
  await page.evaluate(
    ({ eventType, at, selector, scrollToTopFirst }) => {
      // Жест стартует только у потолка страницы (scrollY <= 0): гарантируем
      // верх перед первым шагом, иначе проскролленный вход даёт ложный красный.
      if (scrollToTopFirst) window.scrollTo(0, 0);
      const target = document.querySelector(selector);
      if (target === null) {
        throw new Error(`нет цели жеста: ${selector}`);
      }
      const touchAt = (clientX: number, clientY: number): Touch =>
        new Touch({ identifier: 1, target, clientX, clientY });
      const touches = eventType === 'touchend' ? [] : [touchAt(at.x, at.y)];
      window.dispatchEvent(
        new TouchEvent(eventType, {
          touches,
          changedTouches: [touchAt(at.x, at.y)],
          bubbles: true,
          cancelable: true,
        }),
      );
    },
    {
      eventType: type,
      at: point,
      selector: targetSelector,
      scrollToTopFirst: opts.scrollToTopFirst ?? false,
    },
  );
}

/** Шаг канонического жеста тяги: start — верх страницы + касание (y=120);
 *  pull — тяга на 300px (после сопротивления 0.5 — 150px визуального pull,
 *  порог armed 70px пройден с запасом); end — отпускание. Возвращает
 *  состояние overscroll-гварда на <html> — синхронный след активного
 *  компонента. */
async function dispatchGestureStep(
  page: Page,
  step: GestureStep,
  targetSelector: string = CONTENT_TARGET,
): Promise<boolean> {
  const type: TouchEventType =
    step === 'start' ? 'touchstart' : step === 'pull' ? 'touchmove' : 'touchend';
  await dispatchTouch(
    page,
    type,
    { x: 40, y: step === 'start' ? 120 : 420 },
    targetSelector,
    { scrollToTopFirst: step === 'start' },
  );
  return guardActive(page);
}

/** Состояние overscroll-гварда на <html>. */
async function guardActive(page: Page): Promise<boolean> {
  return page.evaluate(() =>
    document.documentElement.classList.contains('ptr-suppress-overscroll'),
  );
}

/** Насколько проявлен спиннер PTR (0..1): компонент пишет проявление
 *  покадрово во время живой тяги; «0» — легитимный сброс отменённого
 *  жеста. CSS-модуль хранит локальное имя в собранном классе, поэтому
 *  [class*="spinner"] устойчив к хэшу. */
async function spinnerRevealValue(page: Page): Promise<number> {
  return page.evaluate(() => {
    const spinner = document.querySelector<HTMLElement>('span[class*="spinner"]');
    if (spinner === null) {
      throw new Error('спиннер PTR не найден');
    }
    return Number.parseFloat(spinner.style.opacity || '0');
  });
}

test.describe('Android standalone: системный PTR не подавлен', () => {
  test.use({ serviceWorkers: 'block', userAgent: ANDROID_CHROME_UA });

  test('гварда и жеста нет на всех шагах тяги', async ({ page, seededUser }) => {
    await emulateStandalonePwa(page);
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/properties');
    await expect(page.getByRole('heading', { name: 'Объекты' })).toBeVisible();

    // Полный жест на коде без гейта оставил бы гвард на всех трёх шагах;
    // с гейтом компонент не слушает window вовсе — гварда нет ни во время
    // тяги, ни после отпускания: системный reload-жест Chrome свободен.
    expect(await dispatchGestureStep(page, 'start')).toBe(false);
    expect(await dispatchGestureStep(page, 'pull')).toBe(false);
    expect(await dispatchGestureStep(page, 'end')).toBe(false);
  });
});

test.describe('iOS standalone: кастомный жест жив', () => {
  test.use({ serviceWorkers: 'block', userAgent: IPHONE_SAFARI_UA });

  test('гвард в тяге, держится на armed-отпускании, снимается после refresh', async ({
    page,
    seededUser,
  }) => {
    await emulateStandalonePwa(page);
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/properties');
    await expect(page.getByRole('heading', { name: 'Объекты' })).toBeVisible();

    // Тяга выше порога: touchstart вешает гвард, armed-отпускание держит
    // его, пока идёт refresh (invalidateQueries), затем снимает.
    expect(await dispatchGestureStep(page, 'start')).toBe(true);
    expect(await dispatchGestureStep(page, 'pull')).toBe(true);
    expect(await dispatchGestureStep(page, 'end')).toBe(true);
    await expect
      .poll(() =>
        page.evaluate(() =>
          document.documentElement.classList.contains('ptr-suppress-overscroll'),
        ),
        { timeout: 10_000 },
      )
      .toBe(false);
  });

  test('тяга из открытого пикера периода — не наш жест', async ({
    page,
    seededUser,
  }) => {
    await emulateStandalonePwa(page);
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/operations');
    const periodChip = page.getByRole('button', { name: 'Период', exact: true });
    await expect(periodChip).toBeVisible();
    await periodChip.click();
    await expect(page.getByRole('dialog', { name: 'Выберите период' })).toBeVisible();

    // Лента календаря — внутренний скроллер под локом фона: репро
    // 19.10 («вниз скроллишь — PTR срабатывает»). Драг через порог не
    // должен ни вешать гвард, ни тянуть контент: лента тянет себя.
    expect(await dispatchGestureStep(page, 'start', CALENDAR_FEED_TARGET)).toBe(false);
    expect(await dispatchGestureStep(page, 'pull', CALENDAR_FEED_TARGET)).toBe(false);
    expect(await dispatchGestureStep(page, 'end', CALENDAR_FEED_TARGET)).toBe(false);
  });

  test('зигзаг скролла одним касанием не кормит тягу', async ({
    page,
    seededUser,
  }) => {
    await emulateStandalonePwa(page);
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/properties');
    await expect(page.getByRole('heading', { name: 'Объекты' })).toBeVisible();

    // Репро 19.10 на документных страницах: вниз прочитать — тем же
    // касанием назад наверх. Разворот вверх честно кончает жест: обратная
    // тяга до порога не доезжает, трансформ и спиннер не пишутся.
    expect(await dispatchGestureStep(page, 'start')).toBe(true);
    await dispatchTouch(page, 'touchmove', { x: 40, y: 20 });
    await dispatchTouch(page, 'touchmove', { x: 40, y: 420 });
    expect(await spinnerRevealValue(page)).toBe(0);
    expect(await guardActive(page)).toBe(false);
    await dispatchTouch(page, 'touchend', { x: 40, y: 420 });
    expect(await guardActive(page)).toBe(false);
  });

  test('горизонтальный жест не тянет контент', async ({ page, seededUser }) => {
    await emulateStandalonePwa(page);
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/properties');
    await expect(page.getByRole('heading', { name: 'Объекты' })).toBeVisible();

    // Доминанта горизонтали (свайп чипов/карусели с дрейфом вниз) —
    // мёртвый жест: направление решается после ~10px, до вертикали дело
    // не доходит.
    expect(await dispatchGestureStep(page, 'start')).toBe(true);
    await dispatchTouch(page, 'touchmove', { x: 340, y: 140 });
    expect(await spinnerRevealValue(page)).toBe(0);
    expect(await guardActive(page)).toBe(false);
    await dispatchTouch(page, 'touchend', { x: 340, y: 140 });
  });

  test('тап после прерванной тяги не стреляет refresh', async ({
    page,
    seededUser,
  }) => {
    await emulateStandalonePwa(page);
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/properties');
    await expect(page.getByRole('heading', { name: 'Объекты' })).toBeVisible();

    // Тяга до armed и отмена, затем тап без движения: новый жест не
    // наследует прошлую тягу — stale pullRef в flickTriggersRefresh не
    // превращает тап в refresh.
    expect(await dispatchGestureStep(page, 'start')).toBe(true);
    expect(await dispatchGestureStep(page, 'pull')).toBe(true);
    await dispatchTouch(page, 'touchmove', { x: 40, y: 20 });
    await dispatchTouch(page, 'touchend', { x: 40, y: 20 });
    expect(await guardActive(page)).toBe(false);
    expect(await dispatchGestureStep(page, 'start')).toBe(true);
    await dispatchTouch(page, 'touchend', { x: 40, y: 120 });
    expect(await spinnerRevealValue(page)).toBe(0);
    expect(await guardActive(page)).toBe(false);
  });
});
