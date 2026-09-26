/** Обработка разобранного кадра стрима (#747). Кадры best-effort — система
 * записи лента, поэтому состояние выражается через react-query: created
 * инвалидирует общий корень (активные запросы перечитывают, неактивные
 * помечаются устаревшими), unread-count ставит кэш счётчика напрямую —
 * кадр от бэка совпадает с его же состоянием, перечет не нужен.
 *
 * Слои — как у realtime-близнеца (features/realtime/api/realtime-handlers.ts):
 * чистая кадровая логика живёт в api рядом с соединением
 * (notification-stream.ts), ui-провайдер тощий. Единственное отличие — тост
 * о новом уведомлении: это DOM-эффект (react-toastify), он приходит
 * инъекцией колбэка из провайдера — api-слой про DOM не знает (#883). */

import type { NotificationCategory } from '@/entities/notification';
import { notificationKeys } from '@/shared/api/query-keys';
import type { NotificationStreamHandlers } from './notification-stream';
import type { NotificationCreatedFrame, StreamFrame } from './stream-frame';

/** Срез react-query-клиента, который использует обработчик кадров; полный
 * QueryClient совместим структурно, тесты подсовывают фейк. */
export type StreamQueryClient = {
  invalidateQueries(filter: { queryKey: readonly unknown[] }): Promise<unknown>;
  setQueryData(key: readonly unknown[], data: unknown): unknown;
};

/** Предикат тост-гейта (#790, решение #737): true — тост категории
 * показывается. Решение о тосте принимается в момент кадра. */
export type ToastGate = (category: NotificationCategory) => boolean;

/** Показ тоста о новом уведомлении — DOM-эффект, инъекция из ui-провайдера
 * (там это notifyNotificationCreated); тесты подсовывают лежачую запись. */
export type ShowNotificationToast = (frame: NotificationCreatedFrame) => void;

export function handleStreamFrame(
  frame: StreamFrame,
  queryClient: StreamQueryClient,
  showToast: ShowNotificationToast,
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
      // живого гейта провайдера.
      const allowed = shouldToast?.(frame.category) ?? true;
      if (allowed) {
        showToast(frame);
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
export function notificationHandlers(deps: {
  readonly queryClient: StreamQueryClient;
  readonly showToast: ShowNotificationToast;
  readonly toastGate: ToastGate;
}): NotificationStreamHandlers {
  return {
    onOpen: () => {
      void deps.queryClient.invalidateQueries({ queryKey: notificationKeys.all });
    },
    onFrame: (frame) => {
      handleStreamFrame(frame, deps.queryClient, deps.showToast, deps.toastGate);
    },
  };
}
