export type TransitionClickFacts = {
  readonly defaultPrevented: boolean;
  readonly button: number;
  /** meta/ctrl/shift/alt зажаты — клик уходит браузеру (новая вкладка и пр.). */
  readonly modified: boolean;
};

export type TransitionAnchorFacts = {
  /** Атрибут href как есть (может быть относительным); null — href нет. */
  readonly rawHref: string | null;
  readonly download: boolean;
  /** target отсутствует или _self. */
  readonly selfTarget: boolean;
  /** Якорь внутри поддерева [data-page-transition-skip]. */
  readonly withinSkip: boolean;
};

/**
 * Классификация клика для шторки переходов (порт alterix-new): возвращает
 * цель перехода `path?query#hash` или null, если клик мимо шторки — тогда
 * браузер/Next обрабатывают его как обычно. Тонкая DOM-обёртка
 * (extractTransitionAnchor) собирает факты якоря, эта функция — чистая
 * семантика решения.
 */
export function resolveTransitionHref(
  click: TransitionClickFacts,
  anchor: TransitionAnchorFacts | null,
  currentHref: string,
): string | null {
  if (click.defaultPrevented || click.button !== 0 || click.modified) {
    return null;
  }

  if (!anchor || anchor.withinSkip) {
    return null;
  }

  const rawHref = anchor.rawHref?.trim();

  if (!rawHref || rawHref.startsWith('#') || anchor.download || !anchor.selfTarget) {
    return null;
  }

  let url: URL;

  try {
    url = new URL(rawHref, currentHref);
  } catch {
    return null;
  }

  if (url.protocol !== 'http:' && url.protocol !== 'https:') {
    return null;
  }

  const current = new URL(currentHref);

  if (url.origin !== current.origin) {
    return null;
  }

  const samePathAndSearch = url.pathname === current.pathname && url.search === current.search;

  if (url.href === current.href || (samePathAndSearch && url.hash)) {
    return null;
  }

  return `${url.pathname}${url.search}${url.hash}`;
}

/**
 * Факты якоря из живого события (thin DOM-слой; в node-тестах DOM нет —
 * семантика решения проверяется на resolveTransitionHref).
 */
export function extractTransitionAnchor(event: MouseEvent): TransitionAnchorFacts | null {
  if (!(event.target instanceof Element)) {
    return null;
  }

  const anchor = event.target.closest<HTMLAnchorElement>('a[href]');

  if (!anchor) {
    return null;
  }

  return {
    rawHref: anchor.getAttribute('href'),
    download: anchor.hasAttribute('download'),
    selfTarget: !anchor.target || anchor.target.toLowerCase() === '_self',
    withinSkip: anchor.closest('[data-page-transition-skip]') !== null,
  };
}

/** Модификаторы клика, уводящие его у шторки. */
export function isModifiedClick(event: MouseEvent): boolean {
  return event.metaKey || event.ctrlKey || event.shiftKey || event.altKey;
}
