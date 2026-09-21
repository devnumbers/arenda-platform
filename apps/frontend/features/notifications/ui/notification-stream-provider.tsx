'use client';

import { useEffect, type JSX } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { notificationKeys } from '@/shared/api/query-keys';
import {
  connectNotificationStream,
  type NotificationStreamHandlers,
} from '@/features/notifications/api/notification-stream';
import type { StreamFrame } from '@/features/notifications/api/stream-frame';
import { notifyNotificationCreated } from './notification-toast';

/** Обработка разобранного кадра стрима (#747). Кадры best-effort — система
 * записи лента, поэтому состояние выражается через react-query: created
 * инвалидирует общий корень (активные запросы перечитывают, неактивные
 * помечаются устаревшими), unread-count ставит кэш счётчика напрямую —
 * кадр от бэка совпадает с его же состоянием, перечет не нужен. */
/** Срез react-query-клиента, который использует обработчик кадров; полный
 * QueryClient совместим структурно, тесты подсовывают фейк. */
export type StreamQueryClient = {
  invalidateQueries(filter: { queryKey: readonly unknown[] }): Promise<unknown>;
  setQueryData(key: readonly unknown[], data: unknown): unknown;
};

export function handleStreamFrame(frame: StreamFrame, queryClient: StreamQueryClient): void {
  switch (frame.kind) {
    case 'created':
      // Кадр счётчика идёт следом, но лента должна обновиться сразу.
      void queryClient.invalidateQueries({ queryKey: notificationKeys.all });
      notifyNotificationCreated(frame);
      break;
    case 'unread-count':
      // Бейдж обновляется мгновенно из кадра, без рефеча. Прочтения с
      // других устройств стрим не несёт (кадр шлёт только пайплайн
      // публикации, #742) — их закрывает refetch-on-focus счётчика.
      queryClient.setQueryData(notificationKeys.unreadCount(), frame.count);
      break;
    case 'connected':
      break;
  }
}

/** Открытые-хендлеры стрима: на каждом открытии (включая переподключение —
 * replay-курсора в v1 нет, ADR 0060) клиент перечитывает живое через
 * react-query. */
function streamHandlers(queryClient: StreamQueryClient): NotificationStreamHandlers {
  return {
    onOpen: () => {
      void queryClient.invalidateQueries({ queryKey: notificationKeys.all });
    },
    onFrame: (frame) => {
      handleStreamFrame(frame, queryClient);
    },
  };
}

/**
 * Живой слой уведомлений (#747): одно SSE-соединение на вкладку
 * (EventSource на /api/notifications/stream, cookie-сессия), кадры —
 * инвалидации react-query и тосты о новых уведомлениях. Бэк рассчитан на
 * соединение на вкладку (лимит хаба 8 на пользователя). Рвется при
 * unmount; 401 до старта стрима — fail-соединение, ручной бэкофф
 * соединения повторяет попытки, живой слой не мешает работе приложения.
 */
export function NotificationStreamProvider(): JSX.Element | null {
  const queryClient = useQueryClient();

  useEffect(() => {
    const dispose = connectNotificationStream({
      createSource: () => new EventSource('/api/notifications/stream'),
      handlers: streamHandlers(queryClient),
    });
    return dispose;
  }, [queryClient]);

  return null;
}
