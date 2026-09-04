/**
 * Форматирование UTC-дневных дат для строк списков (резолюция #452 и
 * унификация 2026-09-04 — до этого дословные дубликаты в entities/payment
 * и entities/task): «11 августа» — день и склонённый месяц; вне текущего
 * года год добавляется: «13 мая, 2027». «Сегодня» приходит параметром —
 * календарное «сегодня» интерфейса считает сервер по TZ собственника
 * (ADR 0048). Полноширинный формат «12.08.2026» легаси-профиля живёт в
 * format-date.ts (локальное время, другой контракт).
 */

import type { IsoDate } from './calendar';
import { fromIso, isoYear } from './calendar';
import { pluralize } from './pluralize';

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
  if (isoYear(iso) === isoYear(today)) {
    return base;
  }
  return `${base}, ${isoYear(iso)}`;
}

/** Русское склонение «день/дня/дней»: 1 день, 3 дня, 5 дней, 21 день. */
export function formatOverdueDays(days: number): string {
  return `${days} ${pluralize(Math.abs(days), 'день', 'дня', 'дней')}`;
}
