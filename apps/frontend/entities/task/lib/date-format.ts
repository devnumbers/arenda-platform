/**
 * Формат дат и подписей строк задач (#499, Figma 1535-75894): срок
 * просрочки «N дней назад», отметка выполнения «Выполнена 12 августа»,
 * заголовки секций-дат «27 августа» (вне текущего года — с годом).
 * «Сегодня» приходит параметром — календарное «сегодня» интерфейса
 * считает сервер по TZ собственника (ADR 0048).
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

/** Заголовок датированной секции: «13 мая» своего года, «13 мая, 2027» чужого. */
export function formatSectionDate(iso: IsoDate, today: IsoDate): string {
  const base = formatDayMonth(iso);
  if (iso.slice(0, 4) === today.slice(0, 4)) {
    return base;
  }
  return `${base}, ${iso.slice(0, 4)}`;
}

/** Подпись выполненной задачи: «Выполнена 12 августа» (год — чужой). */
export function formatCompletedLabel(completedIso: IsoDate, today: IsoDate): string {
  const base = formatDayMonth(completedIso);
  if (completedIso.slice(0, 4) === today.slice(0, 4)) {
    return `Выполнена ${base}`;
  }
  return `Выполнена ${base}, ${completedIso.slice(0, 4)}`;
}

/** Русское склонение «день/дня/дней»: 1 день, 3 дня, 5 дней, 21 день. */
function formatDays(days: number): string {
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

/** Красная подпись просрочки: «N дней назад». Просрочка внутри текущих
 * суток (время дня уже прошло, дата ещё сегодня) даёт 0 дней — подписью
 * остаётся само время, решает экран. */
export function daysOverdue(dueIso: IsoDate, today: IsoDate): number {
  return Math.round((fromIso(today).getTime() - fromIso(dueIso).getTime()) / 86_400_000);
}

export function formatOverdueAgo(dueIso: IsoDate, today: IsoDate): string {
  return `${formatDays(daysOverdue(dueIso, today))} назад`;
}
