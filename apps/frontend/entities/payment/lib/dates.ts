/**
 * Работа с датами 'YYYY-MM-DD' без таймзонных сюрпризов — порт dates.ts
 * прототипа (якорь — UTC; тик и «сегодня» на сервере, клиент не держит часов).
 * Форматирование дат живёт в `date-format.ts`.
 */

import type { IsoDate } from '../model/types';

const MS_PER_DAY = 86_400_000;

export function fromIso(iso: IsoDate): Date {
  return new Date(`${iso}T00:00:00Z`);
}

export function cmp(a: IsoDate, b: IsoDate): -1 | 0 | 1 {
  if (a < b) return -1;
  return a > b ? 1 : 0;
}

export function addDays(iso: IsoDate, days: number): IsoDate {
  const next = new Date(fromIso(iso).getTime() + days * MS_PER_DAY);
  return next.toISOString().slice(0, 10);
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
