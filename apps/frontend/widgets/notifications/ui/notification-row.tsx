'use client';

import type { JSX } from 'react';
import { isNotificationUnread, NotificationCategoryIcon } from '@/entities/notification';
import type { Notification } from '@/entities/notification';
import { formatTime } from '@/shared/lib/date-format';

/**
 * Строка ленты уведомлений (#744, Figma 2329:149013 Notification Component):
 * 3D-иконка категории 44 с точкой непрочитанного, строка контекста
 * (объект / «Тариф Про» / «Системные уведомления») и время справа 14/16,
 * жирный заголовок 16/18, серое описание 14/16 (до трёх строк — дальше
 * только на странице уведомления, #745). Прочитанные приглушены
 * (opacity-60, макет 2329-148575); время показывается только в группах
 * «Сегодня»/«Вчера» (решение владельца #744) — showTime. Строка — кнопка:
 * тап открывает страницу уведомления (#745), сама прочтение не мутирует.
 */
export function NotificationRow({
  notification,
  showTime,
  onOpen,
}: {
  readonly notification: Notification;
  readonly showTime: boolean;
  readonly onOpen: (notification: Notification) => void;
}): JSX.Element {
  const unread = isNotificationUnread(notification);
  return (
    <button
      type="button"
      onClick={() => onOpen(notification)}
      className={`flex w-full cursor-pointer items-start gap-4 py-4 pl-6 pr-4 text-left outline-none transition-opacity hover:opacity-80 focus-visible:ring-4 focus-visible:ring-primary active:opacity-80 ${
        unread ? '' : 'opacity-60'
      }`}
    >
      <NotificationCategoryIcon
        category={notification.category}
        eventType={notification.eventType}
        unread={unread}
      />
      <div className="flex min-w-0 flex-1 flex-col gap-2">
        <div className="flex flex-col gap-1.5">
          <div className="flex items-start gap-2 whitespace-nowrap text-sm leading-4 text-content">
            <span className="min-w-0 flex-1 overflow-hidden text-ellipsis">
              {notification.contextLabel}
            </span>
            {showTime && <span className="shrink-0 text-right">{formatTime(notification.createdAt)}</span>}
          </div>
          <p className="w-full overflow-hidden text-balance text-base font-medium leading-[18px] text-content">
            {notification.title}
          </p>
        </div>
        <p className="w-full overflow-hidden text-balance text-sm leading-4 text-content-secondary line-clamp-3">
          {notification.body}
        </p>
      </div>
    </button>
  );
}
