'use client';

import { useEffect, useRef, type JSX } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import type { NotificationCategory } from '@/entities/notification';
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

/** Предикат тост-гейта (#790, решение #737): true — тост категории
 * показывается. Решение о тосте принимается в момент кадра. */
export type ToastGate = (category: NotificationCategory) => boolean;

export function handleStreamFrame(
  frame: StreamFrame,
  queryClient: StreamQueryClient,
  shouldToast?: ToastGate,
): void {
  switch (frame.kind) {
    case 'created':
      // Кадр счётчика идёт следом, но лента должна обновиться сразу.
      void queryClient.invalidateQueries({ queryKey: notificationKeys.all });
      // Тост — живое отображение пуш-канала: категория, заглушенная
      // push-настройкой устройства, тоста не получает (#790); лента
      // инвалидируется независимо от гейта — пишется всегда (ADR 0058).
      // Гейта нет — показываются все: тот же дефолт «?? true», что и у
      // живого гейта провайдера ниже.
      const allowed = shouldToast?.(frame.category) ?? true;
      if (allowed) {
        notifyNotificationCreated(frame);
      }
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
function streamHandlers(
  queryClient: StreamQueryClient,
  toastGate: ToastGate,
): NotificationStreamHandlers {
  return {
    onOpen: () => {
      void queryClient.invalidateQueries({ queryKey: notificationKeys.all });
    },
    onFrame: (frame) => {
      handleStreamFrame(frame, queryClient, toastGate);
    },
  };
}

/**
 * Живой слой уведомлений (#747): одно SSE-соединение на вкладку
 * (EventSource на /api/notifications/stream, cookie-сессия), кадры —
 * инвалидации react-query и тосты о новых уведомлениях. Тосты проходят
 * через необязательный тост-гейт (#790): без него показываются все —
 * прежнее поведение. Бэк рассчитан на соединение на вкладку (лимит хаба 8
 * на пользователя). Рвется при unmount; 401 до старта стрима —
 * fail-соединение, ручной бэкофф соединения повторяет попытки, живой слой
 * не мешает работе приложения.
 */
export function NotificationStreamProvider({
  shouldToast,
}: {
  readonly shouldToast?: ToastGate;
} = {}): JSX.Element | null {
  const queryClient = useQueryClient();
  // Гейт читается в момент кадра и обновляется эффектом: настройки
  // устройства прилетают асинхронно (проба подписки + GET), а соединение
  // при их смене не переоткрывается.
  const shouldToastRef = useRef<ToastGate | undefined>(undefined);
  useEffect(() => {
    shouldToastRef.current = shouldToast;
  }, [shouldToast]);

  useEffect(() => {
    const dispose = connectNotificationStream({
      createSource: () => new EventSource('/api/notifications/stream'),
      handlers: streamHandlers(
        queryClient,
        (category) => shouldToastRef.current?.(category) ?? true,
      ),
    });
    return dispose;
  }, [queryClient]);

  return null;
}
