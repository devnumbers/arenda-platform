/**
 * Решение прокрутки живой ленты «История действий» (карта #704 → #718):
 * выдача растёт с двух сторон двустороннего keyset — prepend СТАРЫХ при
 * прокрутке вверх (fetchNextPage) и append СВЕЖИХ снизу (live-prepend по
 * кадрам realtime, тикет #718) — и полностью сдвигается перечитыванием окна
 * после разрыва (onOpen-инвалидация, ADR 0062 §5). Для каждой из трёх
 * причин роста — своё поведение:
 *
 * - prepend старых держит позицию читателя компенсацией дельты высоты
 *   (решение владельца 23.09, #709; iOS overflow-anchor не умеем);
 * - append свежих открывает новые строки, только если читатель был на дне
 *   (в пределах FOLLOW_BOTTOM_PX от нижнего края ДО роста) — «как в
 *   мессенджере»; читатель выше остаётся на месте, внизу появляется
 *   аккуратный индикатор «Есть новые» (тикет #718, стиль плашек дней);
 * - перечитывание окна (разрыв соединения, редкий burst сверх потолка
 *   доprepend'ов) сдвигает ОБА края выдачи — реанкерует на дно: содержимое
 *   под читателем уже сменилось, честнее показать самые свежие записи;
 *   когда вырос только нижний край (live-prepend), читателя не трогаем.
 *
 * Рост может прийти из двух источников в ОДНОМ React-коммите: useLayoutEffect
 * видит уже смерженные края и по одним только границам не отличит «перечитали
 * окно» от «fetchNextPage дописал старые сверху, пока live-prepend влил
 * свежие снизу». Об источнике говорит opts.liveMerged (метка времени
 * последнего live-влития против времени снимка краёв — см. экран ленты):
 * гонка трактуется как prepend свежих — читателя старых строк не реанкерим,
 * решаем follow/indicate по расстоянию до старого дна.
 */

/** Расстояние от текущей прокрутки до нижнего края документа (px). */
export function distanceToBottom(): number {
  return document.documentElement.scrollHeight - (window.scrollY + window.innerHeight);
}

/** Границы загруженной выдачи ленты: размер и крайние записи. Записи
 * иммутабельны (keyset по (created_at, id), #708), поэтому id краёв —
 * надёжные метки того, с какой стороны выросла выдача. */
export type FeedEdges = {
  readonly count: number;
  readonly firstId: string | null;
  readonly lastId: string | null;
};

/** Состояние прокрутки ДО изменения выдачи; distanceToOldBottom — расстояние
 * до нижнего края в СТАРОЙ системе координат (новые строки ещё не выросли). */
export type FeedScrollPrev = {
  readonly edges: FeedEdges | null;
  readonly height: number;
  readonly distanceToOldBottom: number;
};

export type FeedScrollAction =
  | { readonly kind: 'anchor' }
  | { readonly kind: 'compensate'; readonly delta: number }
  | { readonly kind: 'follow'; readonly delta: number }
  | { readonly kind: 'indicate' }
  | { readonly kind: 'none' };

/** Порог «читатель на дне»: расстояние до нижнего края (px), внутри которого
 * append свежих считается следующим за взглядом — две с лишним строки. */
export const FOLLOW_BOTTOM_PX = 120;

export function decideFeedScroll(
  prev: FeedScrollPrev,
  next: FeedEdges,
  nextHeight: number,
  opts: { readonly liveMerged?: boolean } = {},
): FeedScrollAction {
  // Пустая выдача — пустые состояния («Действий не было», «Ничего не
  // найдено») не сообщения: никакого скролла и якоря (решение 24.09).
  if (next.count === 0) {
    return { kind: 'none' };
  }
  const previous = prev.edges;
  // Первая загрузка и лента, выросшая из пустой, — окно на самых свежих.
  if (previous === null || previous.count === 0) {
    return { kind: 'anchor' };
  }
  // Сужение выдачи (поиск #710, фильтры #711) ведёт себя как первая
  // загрузка: окно на самых свежих найденных.
  if (next.count < previous.count) {
    return { kind: 'anchor' };
  }
  if (next.count > previous.count) {
    const delta = nextHeight - prev.height;
    if (next.firstId !== previous.firstId && next.lastId !== previous.lastId) {
      if (opts.liveMerged) {
        // Гонка источников роста в одном коммите (fetchNextPage дописал
        // старые сверху, live-prepend влил свежие снизу): для читателя это
        // prepend свежих — семантика живого догона, реанкерить его нельзя.
        // Осознанная цена: live-влитие в одном коммите с onOpen-перечитыванием
        // окна тоже попадает сюда и остаётся follow/indicate вместо реанкера —
        // читателя доставляет до свежих клик по индикатору «Есть новые».
        return prev.distanceToOldBottom < FOLLOW_BOTTOM_PX
          ? { kind: 'follow', delta }
          : { kind: 'indicate' };
      }
      // Перечитанное окно (сдвинулись оба края): реанкер на дно.
      return { kind: 'anchor' };
    }
    if (next.lastId !== previous.lastId) {
      // Свежие снизу: читатель был на дне — открыть новые; выше —
      // индикатор, позицию не трогаем.
      return prev.distanceToOldBottom < FOLLOW_BOTTOM_PX
        ? { kind: 'follow', delta }
        : { kind: 'indicate' };
    }
    // Старые сверху: держим позицию компенсацией дельты высоты.
    return { kind: 'compensate', delta };
  }
  // Размер не изменился, а нижняя граница новая — окно сдвинулось целиком
  // (перечитывание после разрыва): реанкер на самые свежие.
  if (next.lastId !== previous.lastId) {
    return { kind: 'anchor' };
  }
  return { kind: 'none' };
}
