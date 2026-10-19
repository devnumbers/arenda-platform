/**
 * Единая математика ISO-дат 'YYYY-MM-DD' для всего фронта (итерация
 * унификации 2026-09-04; до этого — дословные дубликаты в entities/payment
 * и entities/task и приватные парсеры в фичах). Якорь — UTC: тик и
 * «сегодня» интерфейса считает сервер по TZ собственника (ADR 0048), зона
 * смотрящего на календарную математику не сказывается. IsoDate объявлен
 * здесь; сущностные типы структурно совместимы (string).
 */

export type IsoDate = string;

const MS_PER_DAY = 86_400_000;

export function fromIso(iso: IsoDate): Date {
  return new Date(`${iso}T00:00:00Z`);
}

/** Обратное преобразование: Date → 'YYYY-MM-DD' (UTC). */
export function dateToIso(date: Date): IsoDate {
  return date.toISOString().slice(0, 10);
}

/** Тот же переход по локальным часам браузера: календарный день того
 * пояса, где смотрит (#624 — группировка истории подписочных платежей по
 * датам createdAt-таймстампов, сервер для них календарный день не
 * считает). Расхождение с TZ собственника ограничено краевыми часами
 * суток — как у клиентского «сегодня» проекции платежей (#453). */
export function dateToIsoLocal(date: Date): IsoDate {
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${date.getFullYear()}-${month}-${day}`;
}

/** Лексикографическое сравнение ISO-строк: −1 / 0 / 1. */
export function cmp(a: IsoDate, b: IsoDate): -1 | 0 | 1 {
  if (a < b) return -1;
  return a > b ? 1 : 0;
}

export function addDays(iso: IsoDate, days: number): IsoDate {
  return dateToIso(new Date(fromIso(iso).getTime() + days * MS_PER_DAY));
}

/** Длина периода включительно, в днях: с 3 по 14 августа — 12 дней. */
export function inclusiveDays(from: IsoDate, to: IsoDate): number {
  return Math.round((fromIso(to).getTime() - fromIso(from).getTime()) / MS_PER_DAY) + 1;
}

/** Просрочка в днях: сколько дней прошло от срока до «сегодня». */
export function daysOverdue(dueIso: IsoDate, today: IsoDate): number {
  return Math.round((fromIso(today).getTime() - fromIso(dueIso).getTime()) / MS_PER_DAY);
}

/** Дни до даты, зеркало серверного daysUntil (ADR 0053 §2): сколько
 * календарных дней осталось от `from` до `to`, отрицательное — дата
 * прошла. Симметрично daysOverdue. */
export function daysUntil(from: IsoDate, to: IsoDate): number {
  return Math.round((fromIso(to).getTime() - fromIso(from).getTime()) / MS_PER_DAY);
}

export function lastDayOfMonth(year: number, monthIndex0: number): number {
  return new Date(Date.UTC(year, monthIndex0 + 1, 0)).getUTCDate();
}

/**
 * Дата в заданном месяце с прижатием дня к последнему дню месяца (решение
 * №1/№10 карты прототипа): 31-е → 28/29/30-е. monthIndex0 может выходить за
 * 0..11 — Date.UTC нормализует.
 */
export function dateInMonth(year: number, monthIndex0: number, day: number): IsoDate {
  const clampedDay = Math.min(day, lastDayOfMonth(year, monthIndex0));
  return new Date(Date.UTC(year, monthIndex0, clampedDay)).toISOString().slice(0, 10);
}

/** День месяца из date-строки (1..31), без заводимого Date. */
export function isoDayOfMonth(iso: IsoDate): number {
  return Number(iso.slice(8));
}

/** Номер месяца из date-строки (1..12). */
export function isoMonthNumber(iso: IsoDate): number {
  return Number(iso.slice(5, 7));
}

/** Год из date-строки, без заводимого Date. */
export function isoYear(iso: IsoDate): number {
  return Number(iso.slice(0, 4));
}

/** Сборка ISO из год/месяц/день (UTC). */
export function isoDateOf(year: number, month0: number, day: number): IsoDate {
  return new Date(Date.UTC(year, month0, day)).toISOString().slice(0, 10);
}

/** Полных календарных месяцев от `from` до `to` (прошло времени, «бессрочная
 * аренда» #531): 10 мая → 7 сентября — 3, по 10-е число — уже 4. Граница
 * месяца прижимается к длине месяца (31 января + месяц = 28 февраля — к
 * 28-му полный месяц); интервал короче месяца и обратный — ноль. */
export function fullMonthsBetween(from: IsoDate, to: IsoDate): number {
  const months
    = (isoYear(to) - isoYear(from)) * 12 + (isoMonthNumber(to) - isoMonthNumber(from));
  const boundaryDay = Math.min(
    isoDayOfMonth(from),
    lastDayOfMonth(isoYear(to), isoMonthNumber(to) - 1),
  );
  return Math.max(0, months - (isoDayOfMonth(to) < boundaryDay ? 1 : 0));
}

/** Блок месяца ленты; month0 — 0..11, как у Date. */
export type CalendarMonthRef = {
  readonly year: number;
  readonly month0: number;
};

export function calendarMonthOf(iso: IsoDate): CalendarMonthRef {
  const date = fromIso(iso);
  return { year: date.getUTCFullYear(), month0: date.getUTCMonth() };
}

/** Абсолютный номер месяца — единая шкала для стартов, прыжков и окон. */
export function calendarMonthIndex(month: CalendarMonthRef): number {
  return month.year * 12 + month.month0;
}

export function calendarMonthOfIndex(index: number): CalendarMonthRef {
  return { year: Math.floor(index / 12), month0: index % 12 };
}

/** Лента месяцев от стартового вперёд: count подряд через границу года. */
export function listCalendarMonths(
  start: CalendarMonthRef,
  count: number,
): ReadonlyArray<CalendarMonthRef> {
  const startIndex = calendarMonthIndex(start);
  return Array.from({ length: count }, (_, offset) => calendarMonthOfIndex(startIndex + offset));
}

/** Старт ленты пикера: самый ранний из «сегодня», текущего значения и пола
 * прошлого — якорь даты в прошлом (правка #502) и открытое allowPast-прошлое
 * (мастер завершения #805) остаются reachable. */
export function calendarFeedStart(
  today: IsoDate, value: IsoDate | null, pastFloor?: CalendarMonthRef,
): CalendarMonthRef {
  let start = calendarMonthOf(today);
  if (value !== null) {
    const valueMonth = calendarMonthOf(value);
    if (calendarMonthIndex(valueMonth) < calendarMonthIndex(start)) {
      start = valueMonth;
    }
  }
  if (pastFloor !== undefined && calendarMonthIndex(pastFloor) < calendarMonthIndex(start)) {
    start = pastFloor;
  }
  return start;
}

/** Пол прошлого пикера дат: до какой черты прошедшие дни закрыты. По
 * умолчанию — сегодня (задним числом даты не создаются, #498); пикер с
 * allowPast (дата завершения бывает задним числом, ADR 0053 §3 — #805)
 * опускает черту до minDate; allowPast без minDate — режима «задним
 * числом без пола» (#1393): черты нет (null), прошедшие дни открыты
 * без ограничения. */
export function calendarPastFloor(
  today: IsoDate, minDate: IsoDate | undefined, allowPast: boolean,
): IsoDate | null {
  return allowPast ? (minDate ?? null) : today;
}

/** Погашен ли день в пикере дат: до пола прошлого, если он есть (null —
 * режима «без пола» #1393: прошлое открыто без ограничения; значение-якорь
 * остаётся тапабельным и под полом — правка #502), раньше minDate, позже
 * maxDate. */
export function calendarDayDisabled(args: {
  readonly iso: IsoDate;
  readonly today: IsoDate;
  readonly value: IsoDate | null;
  readonly minDate: IsoDate | undefined;
  readonly maxDate: IsoDate | undefined;
  readonly allowPast: boolean;
}): boolean {
  const floor = calendarPastFloor(args.today, args.minDate, args.allowPast);
  return (
    (floor !== null && cmp(args.iso, floor) < 0 && args.iso !== args.value)
    || (args.minDate !== undefined && cmp(args.iso, args.minDate) < 0)
    || (args.maxDate !== undefined && cmp(args.iso, args.maxDate) > 0)
  );
}

/** Месяц колеса с учётом нижней границы: в минимальном году месяцы раньше
 * min.month0 не существуют (прошлое закрыто), в остальных — любой. */
export function clampMonthToMin(min: CalendarMonthRef, year: number, month0: number): number {
  return year === min.year ? Math.max(month0, min.month0) : month0;
}

/** Месяц колеса с учётом верхней границы: в максимальном году месяцы позже
 * max.month0 не существуют (будущее закрыто), в остальных — любой. */
export function clampMonthToMax(max: CalendarMonthRef, year: number, month0: number): number {
  return year === max.year ? Math.min(month0, max.month0) : month0;
}

/** Платформенный пояс по умолчанию — тот же, что ставит бекенд пустой
 * колонке users.timezone (ADR 0048, миграция 000088). */
export const DEFAULT_TIMEZONE = 'Europe/Moscow';

/** «Сегодня» календаря в заданном IANA-поясе (ADR 0048): клиентский дефолт
 * шага даты визарда операции (#1394). Авторитет «сегодня» — сервер
 * (мутация считает по поясу владельца объекта и отклоняет будущее 400-й) —
 * здесь только предвыбор и закрытие будущих дней. Без пояса — платформенный
 * дефолт; невалидный пояс не роняет рендер — локальная дата устройства
 * (dateToIsoLocal), расхождение ограничено краевыми часами суток. */
export function isoTodayInTimezone(
  timezone: string | null | undefined,
  now: Date = new Date(),
): IsoDate {
  const zone = timezone ?? DEFAULT_TIMEZONE;
  try {
    const parts = new Intl.DateTimeFormat('en-CA', {
      timeZone: zone,
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    }).formatToParts(now);
    const value = (type: string): string =>
      parts.find((part) => part.type === type)?.value ?? '';
    return `${value('year')}-${value('month')}-${value('day')}`;
  } catch {
    return dateToIsoLocal(now);
  }
}

/** Диапазон дат с включительными границами (пикер периода операций). */
export type IsoRange = {
  readonly from: IsoDate;
  readonly to: IsoDate;
};

/** Черновик выбора диапазона: граница без конца — диапазон не завершён. */
export type IsoRangeDraft = {
  readonly start: IsoDate;
  readonly end: IsoDate | null;
};

/** Тап по дню в календаре диапазона: пустой черновик (null — пикер открыт
 * без применённого периода, #670) первый тап делает границей; второй
 * завершает диапазон в любую сторону — раньше или позже первой границы
 * (решение владельца 2026-09-05: 5→1 = период 1–5); тап по тому же числу —
 * период одного дня; по завершённому — перезапуск с новой границы. */
export function pickIsoRange(draft: IsoRangeDraft | null, day: IsoDate): IsoRangeDraft {
  if (draft === null || draft.end !== null) {
    return { start: day, end: null };
  }
  if (day < draft.start) {
    return { start: day, end: draft.start };
  }
  return { start: draft.start, end: day };
}

/** Завершённый диапазон черновика: без конца — один день старта. */
export function settleIsoRange(draft: IsoRangeDraft): IsoRange {
  return { from: draft.start, to: draft.end ?? draft.start };
}

/** Стартовый запас ленты пикера периода назад (план 051, зеркально
 * CALENDAR_FEED_BACKWARD_SEED одиночного пикера): свежий выбор открывается
 * с запасом контекста выше порога приписки, дорисовка не живёт на первом
 * пикселе прокрутки назад. */
export const RANGE_FEED_BACKWARD_SEED = 11;

/** Окно ленты пикера диапазона (план 054 — низ жёсткий, верх бесконечен):
 * низ — «сегодня» плюс два приглушённых (жёсткий край), верх — месяц
 * начала диапазона минус месяц, но не глубже стартового запаса до
 * текущего (свежий выбор открывается с контекстом, глубокий — у своего
 * начала); пустой выбор (null — дефолт «весь период», #670) — тот же
 * запас назад без преселекта. Глубже окно растёт prepend'ом (позицию
 * держит библиотека по ключам), будущее — жёсткий край. */
export function rangeFeedWindow(
  value: IsoRange | null,
  today: IsoDate,
): {
  first: CalendarMonthRef;
  count: number;
  startEdge: CalendarMonthRef | null;
  forwardEdge: CalendarMonthRef;
} {
  const currentIdx = calendarMonthIndex(calendarMonthOf(today));
  const fromIdx =
    value !== null
      ? calendarMonthIndex(calendarMonthOf(value.from)) - 1
      : currentIdx - RANGE_FEED_BACKWARD_SEED;
  const firstIdx = Math.min(fromIdx, currentIdx - RANGE_FEED_BACKWARD_SEED);
  const forwardIdx = currentIdx + CALENDAR_FEED_EDGE_CONTEXT;
  return {
    first: calendarMonthOfIndex(firstIdx),
    count: forwardIdx - firstIdx + 1,
    startEdge: null,
    forwardEdge: calendarMonthOfIndex(forwardIdx),
  };
}

/** Порция дорисовки ленты (план 054): prepend истории сверху и append
 * будущего снизу. */
export const CALENDAR_FEED_CHUNK = 6;

/** Стартовый запас ленты назад (план 051): сид сажает ленту на столько
 * месяцев раньше семантического старта calendarFeedStart, чтобы при
 * открытии scrollTop был заметно выше порога приписки (3000px ≈ 8
 * месяцев): прежние 6 месяцев ≈ 2160px клали позицию открытия прямо в
 * зону приписки — дорисовка стартовала с первого пикселя прокрутки назад
 * и шла нон-стоп. Запас 12 ≈ 4300px держит первый кадр вне зоны, а
 * антистенную защиту несут порог + гейт, не размер коммита. */
export const CALENDAR_FEED_BACKWARD_SEED = 12;

/** Месяцы приглушённого контекста за жёстким краем ленты (план 054,
 * слова владельца «если есть ограничение — даётся всего +2 месяца»):
 * у нижнего края — за maxDate/«сегодня», у верхнего — за полом прошлого.
 * Симметрично обе стороны. */
export const CALENDAR_FEED_EDGE_CONTEXT = 2;

/** Стартовый запас ленты будущего (открытый низ растёт append'ом). */
export const CALENDAR_FEED_MONTHS = 12;

/** Окно ленты одиночного пикера (план 054 — Т-Банк: низ будущее, верх
 * прошлое; вьюпорт якорится к нижнему краю). Жёсткий край получает ровно
 * CALENDAR_FEED_EDGE_CONTEXT приглушённых месяцев контекста: низ — за
 * maxDate, верх — за полом прошлого (null — сторона бесконечна и растёт
 * порциями при прокрутке: prepend истории сверху / append будущего
 * снизу). Открытая сторона будущего стартует с запаса «сегодня» + 2;
 * открытая сторона прошлого — с запаса CALENDAR_FEED_BACKWARD_SEED до
 * якоря (значение правки #502 или «сегодня»). */
export function calendarFeedWindow(args: {
  readonly anchorMonth: CalendarMonthRef;
  readonly todayMonth: CalendarMonthRef;
  readonly pastFloorMonth?: CalendarMonthRef;
  readonly maxDateMonth?: CalendarMonthRef;
}): {
  readonly first: CalendarMonthRef;
  readonly count: number;
  readonly startEdge: CalendarMonthRef | null;
  readonly forwardEdge: CalendarMonthRef | null;
} {
  const forwardEdge = args.maxDateMonth
    ? calendarMonthOfIndex(calendarMonthIndex(args.maxDateMonth) + CALENDAR_FEED_EDGE_CONTEXT)
    : null;
  const startEdge = args.pastFloorMonth
    ? calendarMonthOfIndex(calendarMonthIndex(args.pastFloorMonth) - CALENDAR_FEED_EDGE_CONTEXT)
    : null;
  const bottomIdx = forwardEdge
    ? calendarMonthIndex(forwardEdge)
    : calendarMonthIndex(args.todayMonth) + CALENDAR_FEED_EDGE_CONTEXT;
  const firstIdx = startEdge
    ? calendarMonthIndex(startEdge)
    : calendarMonthIndex(args.anchorMonth) - CALENDAR_FEED_BACKWARD_SEED;
  return {
    first: calendarMonthOfIndex(firstIdx),
    count: bottomIdx - firstIdx + 1,
    startEdge,
    forwardEdge,
  };
}

/** Порог дорисовки будущего: до нижнего края осталось ~5 экранов — запас
 * на флик (план 042, feel-check владельца 16.10). */
export const CALENDAR_FEED_APPEND_THRESHOLD_PX = 1500;

/** Порог дорисовки истории (prepend сверху): до верхнего края осталось
 * меньше ~8 экранов (планы 042/051; позицию при prepend'е держит
 * библиотека — план 054). */
export const CALENDAR_FEED_PREPEND_THRESHOLD_PX = 3000;

/** Допуск «на стене» для жестов: резинка iOS даёт отрицательный scrollTop
 * на отскоке, scrollHeight бывает дробным — стены сравниваются с запасом
 * 1px. */
export const CALENDAR_FEED_WALL_EPS_PX = 1;

/** Порция дорисовки, о решении которой автомат уже сказал, а коммит окна
 * ещё не прошёл (гейт «в полёте»). */
export type CalendarFeedInflight = 'none' | 'prepend' | 'append';

/** Решение автомата дорисовки. */
export type CalendarFeedGrowth = 'none' | 'prepend' | 'append';

/** Откуда автомат увидел событие: scroll-событие (направление = знак
 * дельты), касание (направления не имеет) или колесо (знак deltaY). */
export type CalendarFeedTrigger = 'scroll' | 'touch' | 'wheel';

/** Автомат дорисовки ленты календаря (план 055) — чистое решение; компонент
 * кормит им все триггеры. Гибрид «направление + стена», починка репро
 * владельца 19.10 «апрель 2025»:
 *
 * — **scroll** (направление дельты обязательно): вверх — prepend в зоне
 * порога (3000px), вниз — append в зоне порога (1500px). Направление
 * отсекает паразитный рост на коротких лентах, где зоны перекрываются
 * (регресс e2e визарда платежа «прошлое закрыто и лента не растёт назад»:
 * скролл вверх от жёсткого верха проваливался в append короткого
 * открытого низа). Цепочка внутри одного флика жива: у инерции направление
 * постоянно, коммит окна открывает гейт «в полёте», следующее событие
 * решает по свежей геометрии.
 *
 * — **touchstart / wheel** (хозяева стены): у упора scroll-событий не
 * бывает вовсе — инерция телефона, обогнав дорисовку, умирает об стену,
 * и только жест может доращить ленту (репро «стена апрель 2025»; ПК чисто:
 * колесо не перескакивает буфер одним жестом). Жест действует только НА
 * стене (±1px) и только своей стороны: касание и wheel вверх открывают
 * верхнюю, wheel вниз — нижнюю; чужую сторону у противоположной стены жест
 * не трогает — инертность жёсткого края (контракт «прошлое закрыто»)
 * важнее упреждающего роста.
 *
 * Гейт «в полёте» вместо гистерезиса 054 «одна порция на приближение»:
 * флаг, сбрасываемый только выходом из зоны, навсегда запирал цепочку
 * после смерти инерции в зоне. Каскад «2324 год» (репро 18.10) невозможен
 * по построению: каждая порция выносит позицию из зоны (~6 месяцев выше
 * порогов), append позицию не трогает вовсе (followOnAppend: false,
 * scrollEndThreshold: -1). Жёсткие края (пол прошлого, maxDate) не растут:
 * их стороны приходят закрытыми (startEdgeOpen/forwardEdgeOpen = false). */
export function calendarFeedGrowth(args: {
  readonly scrollTop: number;
  readonly clientHeight: number;
  readonly scrollHeight: number;
  readonly startEdgeOpen: boolean;
  readonly forwardEdgeOpen: boolean;
  readonly inflight: CalendarFeedInflight;
  readonly trigger: CalendarFeedTrigger;
  readonly direction: 'up' | 'down' | null;
}): CalendarFeedGrowth {
  if (args.inflight !== 'none') {
    return 'none';
  }
  if (args.trigger === 'scroll') {
    if (
      args.direction === 'up'
      && args.startEdgeOpen
      && args.scrollTop <= CALENDAR_FEED_PREPEND_THRESHOLD_PX
    ) {
      return 'prepend';
    }
    if (
      args.direction === 'down'
      && args.forwardEdgeOpen
      && args.scrollTop + args.clientHeight
        >= args.scrollHeight - CALENDAR_FEED_APPEND_THRESHOLD_PX
    ) {
      return 'append';
    }
    return 'none';
  }
  const atTop = args.scrollTop <= CALENDAR_FEED_WALL_EPS_PX;
  const atEnd =
    args.scrollTop + args.clientHeight >= args.scrollHeight - CALENDAR_FEED_WALL_EPS_PX;
  if (args.direction !== 'down' && atTop && args.startEdgeOpen) {
    return 'prepend';
  }
  if (args.direction !== 'up' && atEnd && args.forwardEdgeOpen) {
    return 'append';
  }
  return 'none';
}

/** Непрерывные отрезки подложки диапазона внутри одной недели календаря:
 * серые «пилюли» от границы до границы — ряды целиком в диапазоне и
 * частичные ряды краёв. Индексы 0-based включительно по колонкам недели. */
export function booleanRunSegments(
  flags: ReadonlyArray<boolean>,
): ReadonlyArray<readonly [number, number]> {
  const segments: Array<readonly [number, number]> = [];
  let runStart: number | null = null;
  for (const [index, flag] of flags.entries()) {
    if (flag && runStart === null) {
      runStart = index;
    }
    if ((!flag || index === flags.length - 1) && runStart !== null) {
      const runEnd = flag && index === flags.length - 1 ? index : index - 1;
      segments.push([runStart, runEnd]);
      runStart = null;
    }
  }
  return segments;
}
