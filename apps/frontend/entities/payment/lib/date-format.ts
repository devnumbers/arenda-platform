/**
 * Формат дат для строк платежей (резолюция #452): «11 августа» — день и
 * склонённый месяц; в контексте даты вне текущего года год добавляется:
 * «13 мая» этого года, «13 мая, 2027» другого. «Сегодня» приходит параметром —
 * календарное «сегодня» интерфейса считает сервер по TZ собственника (ADR 0048),
 * зона смотрящего на тексты не влияет. Сюда же — текст срока просрочки
 * «N дней» из секции «Просроченные».
 */

import type { IsoDate } from '../model/types';
import { fromIso } from './dates';
const dayMonthFormatter = new Intl.DateTimeFormat('ru-RU', {
  day: 'numeric',
  month: 'long',
  timeZone: 'UTC',
});

/** День и склонённый месяц без года: «11 августа». */
export function formatDayMonth(iso: IsoDate): string {
  return dayMonthFormatter.format(fromIso(iso));
}

/** Тот же формат с годом вне текущего: «13 мая» / «13 мая, 2027». */
export function formatDayMonthWithYear(iso: IsoDate, today: IsoDate): string {
  const base = formatDayMonth(iso);
  if (iso.slice(0, 4) === today.slice(0, 4)) {
    return base;
  }
  return `${base}, ${iso.slice(0, 4)}`;
}

/** Русское склонение «день/дня/дней»: 1 день, 3 дня, 5 дней, 21 день. */
export function formatOverdueDays(days: number): string {
  const mod100 = Math.abs(days) % 100;
  const mod10 = mod100 % 10;
  if (mod100 >= 11 && mod100 <= 14) {
    return `${days} дней`;
  }
  if (mod10 === 1) {
    return `${days} день`;
  }
  if (mod10 >= 2 && mod10 <= 4) {
    return `${days} дня`;
  }
  return `${days} дней`;
}
