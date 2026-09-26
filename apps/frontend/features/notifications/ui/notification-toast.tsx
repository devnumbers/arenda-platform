'use client';

import { createElement, type JSX } from 'react';
import Link from 'next/link';
import { toast } from 'react-toastify/unstyled';
import { Cancel } from '@/shared/assets/icons';
import { formatTime } from '@/shared/lib/date-format';
import { ROUTES } from '@/shared/config/routes';
import { NotificationCategoryIcon } from '@/entities/notification';
import type { NotificationCreatedFrame } from '@/features/notifications/api/stream-frame';
import styles from '@/shared/ui/toast/ToastProvider.module.css';

/** Автозакрытие тоста уведомления: читается дольше канонных 4с
 * подтверждений; пауза при наведении канона остаётся. */
const NOTIFICATION_TOAST_AUTOCLOSE_MS = 6000;

/**
 * Тост о новом уведомлении (макет 2343:57307, тикеты #747/#778): круг иконки
 * категории 44 без точки непрочитанного и без тарифного бейджа (в макете
 * тоста Badge=None), строка контекста + время (зазор 8), заголовок 16/18
 * одна строка с ellipsis, описание 14/16 максимум в две строки (решение
 * владельца 19.09 — подпись-заглушка макета «в три строки» устарела),
 * X 44×44 с отступами 8 сверху и 8 справа, глиф 24 по центру зоны.
 * Решение владельца 21.09 — «сделать 1 в 1 как на фигме» (#778): ширина
 * карточки 402 на десктопе (макет: поля 24 во фрейме 450) и экран − 48 на
 * мобиле (поля 24) — живёт в варианте `.notification`
 * в ToastProvider.module.css. Тап по карточке ведёт на страницу
 * уведомления («тап → страница уведомления», #747); deeplink url из кадра
 * остаётся канону пушей (sw.js). X — сестринский элемент ссылки, не
 * вложенный (валидная интерактивная вложенность).
 */
export function NotificationToast({
  frame,
  closeToast,
}: {
  readonly frame: NotificationCreatedFrame;
  readonly closeToast: () => void;
}): JSX.Element {
  return (
    <div className="relative w-full">
      <Link
        href={ROUTES.notification(frame.id)}
        onClick={closeToast}
        className="flex w-full items-start gap-4 rounded-[24px] p-4 pr-16 text-left outline-none focus-visible:ring-2 focus-visible:ring-primary"
      >
        <NotificationCategoryIcon category={frame.category} unread={false} badge={false} />
        <span className="flex min-w-0 flex-1 flex-col gap-2">
          <span className="flex flex-col gap-1.5">
            {(frame.contextLabel !== null || frame.occurredAt !== '') && (
              <span className="flex items-baseline gap-2">
                {frame.contextLabel !== null && (
                  <span className="min-w-0 flex-1 truncate text-sm leading-4 text-content">
                    {frame.contextLabel}
                  </span>
                )}
                {frame.occurredAt !== '' && (
                  <span className="shrink-0 text-sm leading-4 text-content">
                    {formatTime(frame.occurredAt)}
                  </span>
                )}
              </span>
            )}
            <span className="truncate text-base font-medium leading-[18px] text-content">{frame.title}</span>
          </span>
          {frame.body !== '' && (
            <span className="line-clamp-2 text-sm leading-4 text-content-secondary">{frame.body}</span>
          )}
        </span>
      </Link>
      <button
        type="button"
        aria-label="Закрыть"
        onClick={(event) => {
          event.preventDefault();
          event.stopPropagation();
          closeToast();
        }}
        className="absolute right-2 top-2 flex h-11 w-11 items-center justify-center rounded-pill text-content-secondary outline-none transition-colors hover:text-content focus-visible:ring-2 focus-visible:ring-primary"
      >
        <Cancel width={24} height={24} aria-hidden />
      </button>
    </div>
  );
}

/** Показывает тост о новом уведомлении в канон-контейнере. Классы передаются
 * ОБА: опция className тоста в react-toastify v11 ЗАМЕНЯЕТ toastClassName
 * контейнера (поймано приёмкой #747 — карточка без .toast теряла
 * pointer-events: auto и не пропускала тапы), а bare-вариант
 * `.toast.notification` перекрашивает хром тоста под макет (радиус 24,
 * тень 0 8px 24px) — хром на тосте, а не на карточке, чтобы свёрнутый
 * стек показывал краешки задних карточек (нативный peek); X — своя,
 * closeButton канона выключен. */
export function notifyNotificationCreated(frame: NotificationCreatedFrame): void {
  toast(({ closeToast }) => createElement(NotificationToast, { frame, closeToast }), {
    toastId: `notification-${frame.id}`,
    className: `${styles.toast} ${styles.notification}`,
    closeButton: false,
    autoClose: NOTIFICATION_TOAST_AUTOCLOSE_MS,
  });
}
