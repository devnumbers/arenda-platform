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
