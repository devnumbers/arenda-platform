/**
 * Работа с датами 'YYYY-MM-DD' без таймзонных сюрпризов (как у платежей:
 * якорь — UTC; «сегодня» и просрочка на сервере, клиент не держит часов).
 */

import type { IsoDate } from '../model/types';

const MS_PER_DAY = 86_400_000;

export function fromIso(iso: IsoDate): Date {
  return new Date(`${iso}T00:00:00Z`);
}

export function addDays(iso: IsoDate, days: number): IsoDate {
  const next = new Date(fromIso(iso).getTime() + days * MS_PER_DAY);
  return next.toISOString().slice(0, 10);
}
