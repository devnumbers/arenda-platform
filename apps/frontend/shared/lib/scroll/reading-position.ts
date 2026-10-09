/**
 * Позиция чтения страниц-лент (карта #1324, D8): страницы истории
 * открываются «с конца» (якорь ленты — widgets/history/lib/feed-scroll.ts)
 * и восстанавливают позицию чтения при возврате «Назад» (паттерн
 * мессенджеров, D8.2). На остальных страницах — канон «всегда верх»
 * (ScrollToTop), здесь живут только история-роуты.
 *
 * Механика — своя, поверх history.scrollRestoration = 'manual'
 * (ScrollToTop): браузерная реставрация с ручным режимом не работает, а
 * Next-овская переметалась бы на якорь ленты. Позиции живут в памяти
 * модуля (ключ — href записи истории, его знает сам экран): реставрация
 * имеет смысл только внутри живой SPA-сессии — перезагрузка страницы
 * открывает ленту заново, с конца, как свежий вход в мессенджер.
 * popstate на историю-роут помечает цель как «ждёт реставрации», лента
 * при первом позиционировании вместо якоря тянет сохранённый scrollTop.
 * Пометка одноразовая и валидна только для своей цели — стейл-флаг с
 * чужого захода не реставрируется (takeReadingPosition сверяет href).
 */

/** Позиции чтения по href записи истории (SPA-сессия; web storage не
 * используется — решение #331). */
const readingPositions = new Map<string, number>();

/** Роуты, чьё позиционирование ведёт сама лента (якорь/реставрация):
 * реестр «якорёных» роутов D8.1 — ScrollToTop их не скроллит к верху. */
export function isAnchoredRoute(pathname: string): boolean {
  return pathname === '/history' || pathname.startsWith('/history/');
}

/** Сохранить scrollTop под ключом href. Ключ и позицию передаёт экран —
 * из своего рендера и синхронного очиста: window.location в момент
 * сохранения доверять нельзя (Next пушит новый URL и скроллит верх
 * синхронно с навигацией — пассивный очист уже видел бы чужой pathname и
 * нулевой скролл). */
export function saveReadingPosition(href: string, scrollY: number): void {
  if (typeof window === 'undefined') {
    return;
  }
  readingPositions.set(href, scrollY);
}

/** Пометить цель popstate ждущим реставрации. Ключ — та же относительная
 * форма, что у экрана (pathname + search): полный location.href не
 * годится, экран сохраняет позиции по рендер-href без происхождения. */
let pendingHref: string | null = null;

export function notePendingReadingPosition(): void {
  if (typeof window === 'undefined') {
    return;
  }
  pendingHref = `${window.location.pathname}${window.location.search}`;
}

/** Снять пометку реставрации и отдать сохранённый scrollTop цели — или
 * null, если пометки нет / цель не совпала (стейл-флаг чужого захода) /
 * позиции для цели не сохранилось. Пометка расходуется одноразово; href —
 * ключ экрана из его рендера (лента смонтирована под URL popstate-
 * перехода). */
export function takeReadingPosition(href: string): number | null {
  if (typeof window === 'undefined' || pendingHref === null) {
    return null;
  }
  const target = pendingHref;
  pendingHref = null;
  if (target !== href) {
    return null;
  }
  return readingPositions.get(href) ?? null;
}
