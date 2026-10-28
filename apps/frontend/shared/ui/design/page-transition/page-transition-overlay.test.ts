import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import {
  COVER_DELAY_MS,
  COVER_DURATION_MS,
  CROSSFADE_DURATION_MS,
  NAVIGATION_FALLBACK_MS,
  REVEAL_DURATION_MS,
} from './page-transition-machine';

/**
 * Страж дрейфа переходов между страницами (прецедент
 * chrome-surface-layering): (1) тайминги JS-машины совпадают с токенами
 * канона DESIGN.md §8 — кроссфейд VT --pt-crossfade-duration, покрытие
 * --pt-cover-duration, раскрытие --pt-reveal-duration; (2) лист собран
 * на утилитах этих же токенов; (3) оверлей смонтирован в ScreenLayout
 * под Suspense (useSearchParams при статическом рендере требует
 * границы); (4) перехват кликов — capture-фаза на document, страницы
 * /ui-walkthrough смотрят живой переход; (5) лист выше любого хрома;
 * (6) eslint-страж держит навигацию слоёв на pushPage/replacePage;
 * (7) основной движок — кроссфейд View Transitions API: раннер в
 * оверлее, CSS-правила пары old/new в globals.css на токене.
 */

const overlaySource = readFileSync(
  fileURLToPath(new URL('./page-transition-overlay.tsx', import.meta.url)),
  'utf8',
);

const tokensCss = readFileSync(
  fileURLToPath(new URL('../../../styles/tokens.css', import.meta.url)),
  'utf8',
);

const globalsCss = readFileSync(
  fileURLToPath(new URL('../../../../app/globals.css', import.meta.url)),
  'utf8',
);

const screenLayoutSource = readFileSync(
  fileURLToPath(new URL('../../../../widgets/screen-layout/ui/screen-layout.tsx', import.meta.url)),
  'utf8',
);

const eslintConfig = readFileSync(
  fileURLToPath(new URL('../../../../eslint.config.mjs', import.meta.url)),
  'utf8',
);

function tokenMs(name: string): number {
  const match = tokensCss.match(new RegExp(`${name}:\\s*(\\d+)ms`));
  if (match === null) {
    throw new Error(`токен ${name} не найден в tokens.css`);
  }
  return Number(match[1]);
}

describe('Переходы между страницами: стражи дрейфа', () => {
  it('тайминги машины — значения собственных токенов переходов', () => {
    expect(CROSSFADE_DURATION_MS).toBe(tokenMs('--pt-crossfade-duration'));
    expect(COVER_DURATION_MS).toBe(tokenMs('--pt-cover-duration'));
    expect(REVEAL_DURATION_MS).toBe(tokenMs('--pt-reveal-duration'));
  });

  it('reduced-motion укорачивает переходы ниже обычного темпа', () => {
    expect(tokenMs('--pt-crossfade-duration')).toBeGreaterThan(100);
    expect(tokenMs('--pt-cover-duration')).toBeGreaterThan(100);
    expect(tokenMs('--pt-reveal-duration')).toBeGreaterThan(100);
  });

  it('VT-кроссфейд: раннер в оверлее, CSS-правила пары old/new на токене', () => {
    expect(overlaySource).toContain('document.startViewTransition');
    expect(globalsCss).toContain('::view-transition-old(root),');
    expect(globalsCss).toContain('::view-transition-new(root)');
    expect(globalsCss).toContain('animation-duration: var(--pt-crossfade-duration)');
    expect(globalsCss).toContain('animation-timing-function: var(--dl-ease)');
  });

  it('промисы перехода погашены: ready/finished ловятся, бросок startViewTransition не роняет навигацию', () => {
    // По доке View Transitions: ready ренджектится на каждом skipTransition
    // (watchdog, замена, bfcache) — без обработчика это необработанный
    // AbortError в консоли; сам startViewTransition бросает на неактивном
    // документе — навигация тогда едет напрямую, без анимации.
    expect(overlaySource).toContain('transition.ready.catch');
    expect(overlaySource).toContain('transition.finished.catch');
    expect(overlaySource).toMatch(/try \{[\s\S]*document\.startViewTransition[\s\S]*\} catch \{/);
  });

  it('лист собран на утилитах тех же токенов, reduced-motion короче', () => {
    expect(overlaySource).toContain('duration-(--pt-reveal-duration)');
    expect(overlaySource).toContain('data-[phase=covering]:duration-(--pt-cover-duration)');
    expect(overlaySource).toContain('ease-apple');
    expect(overlaySource).toContain('motion-reduce:duration-100');
  });

  it('пауза покрытия больше покрытия — программный переход стартует по белому листу', () => {
    expect(COVER_DELAY_MS).toBeGreaterThan(COVER_DURATION_MS);
    expect(NAVIGATION_FALLBACK_MS).toBeGreaterThan(COVER_DELAY_MS + COVER_DURATION_MS);
  });

  it('оверлей смонтирован в ScreenLayout под Suspense', () => {
    expect(screenLayoutSource).toContain('<PageTransitionOverlay />');
    expect(screenLayoutSource).toMatch(/<Suspense fallback=\{null\}>\s*<PageTransitionOverlay \/>/);
  });

  it('клики перехватываются в capture-фазе на document, bfcache сбрасывает лист', () => {
    expect(overlaySource).toContain("document.addEventListener('click', handleClick, { capture: true })");
    expect(overlaySource).toContain("window.addEventListener('pageshow'");
    expect(overlaySource).toContain('machine.reset()');
  });

  it('лист выше любого хрома и поверх скримов (z-index максимума)', () => {
    expect(overlaySource).toContain('z-[2147483000]');
  });

  it('skip-метка исключает якорь из перехода', () => {
    expect(overlaySource).toContain('data-page-transition-skip');
  });

  it('eslint-страж держит навигацию слоёв на pushPage/replacePage', () => {
    expect(eslintConfig).toContain('object: "router"');
    expect(eslintConfig).toMatch(/no-restricted-properties[\s\S]*property: "push"[\s\S]*page-transition curtain/);
    expect(eslintConfig).toMatch(/no-restricted-properties[\s\S]*property: "replace"[\s\S]*useUrlParams/);
  });
});
