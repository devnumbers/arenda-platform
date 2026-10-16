/**
 * Группировка ленты уведомлений по дням (#744, макет 2329-148700):
 * «Сегодня»/«Вчера» — клиентский день смотрящего, дальше — «10 августа»
 * (год вне текущего — «30 декабря, 2025»); канон дат shared/lib и
 * канон группировки #624 (подряд идущие записи одного дня — одна группа,
 * лента приходит newest-first). У «Сегодня»/«Вчера» в строках показывается
 * время, у групп старше — скрыто (решение владельца #744, на странице
 * уведомления время остаётся).
 */

import type { Notification } from '@/entities/notification';
import { dateToIsoLocal, addDays, type IsoDate } from '@/shared/lib/calendar';
import { formatDayMonthWithYear } from '@/shared/lib/date-format';

export type NotificationFeedGroup = {
  /** Локальный календарный день группы ('YYYY-MM-DD') — ключ секции. */
  readonly day: IsoDate;
  readonly label: string;
  /** Рисовать ли HH:mm в строках группы (только «Сегодня»/«Вчера»). */
  readonly showTime: boolean;
  readonly notifications: readonly Notification[];
};

export function groupNotificationsByDay(
  notifications: ReadonlyArray<Notification>,
  today: IsoDate,
): ReadonlyArray<NotificationFeedGroup> {
  const yesterday = addDays(today, -1);
  const groups: { day: IsoDate; label: string; showTime: boolean; notifications: Notification[] }[] =
    [];
  for (const notification of notifications) {
    const day = dateToIsoLocal(new Date(notification.createdAt));
    const current = groups.at(-1);
    if (current !== undefined && current.day === day) {
      current.notifications.push(notification);
      continue;
    }
    const label =
      day === today ? 'Сегодня' : day === yesterday ? 'Вчера' : formatDayMonthWithYear(day, today);
    groups.push({
      day,
      label,
      showTime: day === today || day === yesterday,
      notifications: [notification],
    });
  }
  return groups;
}
