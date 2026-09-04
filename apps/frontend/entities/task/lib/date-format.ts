/**
 * Доменные подписи строк задач (#499, Figma 1535-75894): отметка выполнения
 * «Выполнена 12 августа» и красная просрочка «N дней назад». Базовые
 * форматы дат живут в shared/lib/date-format, календарная разница — в
 * shared/lib/calendar (унификация 2026-09-04). «Сегодня» приходит
 * параметром — календарное «сегодня» интерфейса считает сервер по TZ
 * собственника (ADR 0048).
 */

import type { IsoDate } from '../model/types';
import { daysOverdue } from '@/shared/lib/calendar';
import { formatDayMonthWithYear } from '@/shared/lib/date-format';
import { pluralize } from '@/shared/lib/pluralize';

/** Подпись выполненной задачи: «Выполнена 12 августа» (год — чужой). */
export function formatCompletedLabel(completedIso: IsoDate, today: IsoDate): string {
  return `Выполнена ${formatDayMonthWithYear(completedIso, today)}`;
}

/** Красная подпись просрочки: «N дней назад». Просрочка внутри текущих
 * суток (время дня уже прошло, дата ещё сегодня) даёт 0 дней — подписью
 * остаётся само время, решает экран. */
export function formatOverdueAgo(dueIso: IsoDate, today: IsoDate): string {
  const days = daysOverdue(dueIso, today);
  return `${days} ${pluralize(Math.abs(days), 'день', 'дня', 'дней')} назад`;
}
