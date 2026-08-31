import type { IsoDate } from '@/entities/payment';

/**
 * Календарные конвертеры шагов дат визарда (#464): CalendarMonth работает
 * с Date, домен-порт и черновик — со строками 'YYYY-MM-DD'. Обе стороны
 * UTC-наивные: компоненты календаря рендерятся в UTC, зоны интерфейса
 * не знает ни один слой (ADR 0048).
 */

export function isoToUtcDate(iso: IsoDate): Date {
  return new Date(`${iso}T00:00:00Z`);
}

export function utcDateToIso(date: Date): IsoDate {
  return date.toISOString().slice(0, 10);
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
