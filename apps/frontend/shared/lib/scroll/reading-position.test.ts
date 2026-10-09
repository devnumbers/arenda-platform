import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  isAnchoredRoute,
  notePendingReadingPosition,
  saveReadingPosition,
  takeReadingPosition,
} from './reading-position';

/** Фейковое окно: location по канону модульных фейков (node-среда, DOM
 * нет). href у location и pathname — независимые поля: тест управляет ими
 * напрямую. Позиции — память модуля; scrollY экран передаёт параметром. */
function stubWindow({
  href,
  pathname,
  search = '',
}: {
  readonly href: string;
  readonly pathname: string;
  readonly search?: string;
}): void {
  vi.stubGlobal('window', { location: { href, pathname, search } });
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('isAnchoredRoute — реестр якорёных роутов D8.1', () => {
  it('все четыре страницы истории — якорёные, включая вложенные', () => {
    expect(isAnchoredRoute('/history')).toBe(true);
    expect(isAnchoredRoute('/history/participants/123')).toBe(true);
    expect(isAnchoredRoute('/history/properties/456')).toBe(true);
    expect(isAnchoredRoute('/history/participants/123/properties/456')).toBe(true);
  });

  it('чужие роуты и похожие префиксы — нет', () => {
    expect(isAnchoredRoute('/history-x')).toBe(false);
    expect(isAnchoredRoute('/operations')).toBe(false);
    expect(isAnchoredRoute('/')).toBe(false);
  });
});

describe('позиция чтения (D8.2)', () => {
  it('сохраняет scrollTop под переданным href и отдаёт по пометке popstate', () => {
    stubWindow({ href: 'http://x/history', pathname: '/history' });
    saveReadingPosition('/history', 3400);

    // Возврат «Назад» на тот же href: popstate помечает цель — реставрация.
    notePendingReadingPosition();
    expect(takeReadingPosition('/history')).toBe(3400);
    // Пометка одноразовая.
    expect(takeReadingPosition('/history')).toBeNull();
  });

  it('ключ — href из рендера экрана: запрос с фильтрами — свой слот', () => {
    stubWindow({ href: 'http://x/history?actions=payment', pathname: '/history', search: '?actions=payment' });
    saveReadingPosition('/history', 3400);
    saveReadingPosition('/history?actions=payment', 120);
    notePendingReadingPosition();
    expect(takeReadingPosition('/history?actions=payment')).toBe(120);
  });

  it('без сохранённой позиции пометка отдаёт null (первый заход)', () => {
    stubWindow({ href: 'http://x/history?actions=payment', pathname: '/history', search: '?actions=payment' });
    notePendingReadingPosition();
    expect(takeReadingPosition('/history')).toBeNull();
  });

  it('пометка чужого захода не реставрируется: цель popstate ≠ href экрана', () => {
    stubWindow({ href: 'http://x/history', pathname: '/history' });
    saveReadingPosition('/history', 100);
    notePendingReadingPosition(); // цель — /history

    // Стейл-флаг: consuming-экран другой записи истории — mismatch
    // расходует пометку, реставрации не будет и на своей цели.
    stubWindow({ href: 'http://x/history?actions=payment', pathname: '/history', search: '?actions=payment' });
    expect(takeReadingPosition('/history?actions=payment')).toBeNull();
    stubWindow({ href: 'http://x/history', pathname: '/history' });
    expect(takeReadingPosition('/history')).toBeNull();
  });

  it('без пометки popstate позиция не применяется (открытие по ссылке — якорь на дно)', () => {
    stubWindow({ href: 'http://x/history', pathname: '/history' });
    saveReadingPosition('/history', 3400);
    expect(takeReadingPosition('/history')).toBeNull();
  });

  it('вне браузера (SSR-оценка модуля) — no-op без падений', () => {
    vi.stubGlobal('window', undefined);
    expect(() => saveReadingPosition('/history', 10)).not.toThrow();
    expect(() => notePendingReadingPosition()).not.toThrow();
    expect(takeReadingPosition('/history')).toBeNull();
  });
});
