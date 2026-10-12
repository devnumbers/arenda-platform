import { type Page } from '@playwright/test';
import { expect, openCabinetWithSeededSession, test } from './fixtures';

// Платформенный гейт кастомного pull-to-refresh (ротация ADR 0031,
// решение владельца 12.10.2026): жест живёт только на iOS standalone, где
// WebKit отключает нативный PTR. На Android standalone (WebAPK) системный
// reload-жест браузера должен остаться нетронутым — кастомный компонент не
// биндит слушатели и не вешает overscroll-гвард. Опыт-эталон — PWA T-Banka
// (research/pwa-native-pull-to-refresh.md).
//
// Обе ветки эмулируются: standalone — моком matchMedia (канон auth.spec.ts,
// headless Chromium не умеет display-mode: standalone), платформа —
// подменой userAgent (test.use уровня describe: внутри тела теста фикстуры
// уже созданы). Жест диспетчеризуется синтетическими TouchEvent-ами на
// window — слушатели компонента слушают именно window; наблюдаемый след —
// класс ptr-suppress-overscroll на <html>, который активный жест вешает на
// touchstart и снимает по завершении refresh.

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

/** Диспетчеризует шаг синтетического жеста и возвращает текущее состояние
 *  overscroll-гварда на <html> — синхронный след активного компонента. */
async function dispatchGestureStep(page: Page, step: GestureStep): Promise<boolean> {
  return page.evaluate((gestureStep: GestureStep) => {
    // Жест стартует только у потолка страницы (scrollY <= 0): гарантируем
    // верх перед первым шагом, иначе проскролленный вход даёт ложный красный.
    if (gestureStep === 'start') window.scrollTo(0, 0);
    const touchAt = (clientY: number): Touch =>
      new Touch({ identifier: 1, target: document.body, clientX: 40, clientY });
    // Тяга на 300px: после сопротивления 0.5 — 150px визуального pull,
    // порог armed 70px пройден с запасом.
    const clientY = gestureStep === 'start' ? 120 : 420;
    const touches = gestureStep === 'end' ? [] : [touchAt(clientY)];
    const type = gestureStep === 'start' ? 'touchstart' : gestureStep === 'pull' ? 'touchmove' : 'touchend';
    window.dispatchEvent(
      new TouchEvent(type, {
        touches,
        changedTouches: [touchAt(clientY)],
        bubbles: true,
        cancelable: true,
      }),
    );
    return document.documentElement.classList.contains('ptr-suppress-overscroll');
  }, step);
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
});
