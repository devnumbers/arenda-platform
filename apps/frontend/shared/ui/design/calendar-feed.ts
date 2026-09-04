/**
 * Чистая математика бесконечной календарной ленты (пикер даты): месяцы
 * нумеруются абсолютным индексом year*12+month0, лента идёт от стартового
 * месяца вперёд без конца — назад не нужна, задним числом даты не
 * выбираются. ISO-даты 'YYYY-MM-DD' без таймзонных сюрпризов — якорь UTC,
 * как у платёжных dates-модулей; IsoDate повторён локально: shared не
 * импортирует entities (FSD), тип структурно тот же string.
 */

/** Дата 'YYYY-MM-DD' (структурный дубль IsoDate в entities). */
export type IsoDate = string;

/** Блок месяца ленты; month0 — 0..11, как у Date. */
export type CalendarMonthRef = {
  readonly year: number;
  readonly month0: number;
};

export function calendarMonthOf(iso: IsoDate): CalendarMonthRef {
  const date = new Date(`${iso}T00:00:00Z`);
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

/** Старт ленты пикера: самый ранний из «сегодня» и текущего значения —
 * якорь даты в прошлом (правка #502) остаётся reachable. */
export function calendarFeedStart(today: IsoDate, value: IsoDate | null): CalendarMonthRef {
  const todayMonth = calendarMonthOf(today);
  if (value !== null) {
    const valueMonth = calendarMonthOf(value);
    if (calendarMonthIndex(valueMonth) < calendarMonthIndex(todayMonth)) {
      return valueMonth;
    }
  }
  return todayMonth;
}

/** Месяц колеса с учётом нижней границы: в минимальном году месяцы раньше
 * min.month0 не существуют (прошлое закрыто), в остальных — любой. */
export function clampMonthToMin(min: CalendarMonthRef, year: number, month0: number): number {
  return year === min.year ? Math.max(month0, min.month0) : month0;
}

/** Сборка ISO из год/месяц/день (UTC). */
export function isoDateOf(year: number, month0: number, day: number): IsoDate {
  return new Date(Date.UTC(year, month0, day)).toISOString().slice(0, 10);
}
