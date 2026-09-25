import {
  connectEventStream,
  type EventSourceLike,
} from '@/shared/api/sse-client';
import { parseStreamFrame, type StreamFrame } from './stream-frame';

/**
 * Живой слой уведомлений на фронте (#747): подключение к SSE-стриму
 * GET /notifications/stream (#742, ADR 0060) без React — чистая логика
 * соединения, реакт-обвязка в notification-stream-provider. Компонент
 * держит EventSource на каждую вкладку — бэк рассчитан на это (лимит хаба
 * 8 соединений на пользователя, вкладки/устройства). Соединение живёт,
 * пока жива вкладка, независимо от видимости — уведомления нужны и в
 * фоне; state-машина переподключения — общее ядро sse-client.
 *
 * Переподключение двоякое: сетевые обрывы браузер ретраит сам (retry-hint
 * бэка 3с, readyState CONNECTING) — не мешаем; fail-соединение
 * (readyState CLOSED — 401, не-SSE ответ, вытеснение хабом) браузером не
 * ретраится, поэтому здесь ручной перезапуск с экспоненциальным бэкоффом
 * 1с→30с, сброс на успешном open. На каждом открытии провайдер
 * перечитывает живое через react-query (replay-курсора в v1 нет).
 */

export type { EventSourceLike };

export type NotificationStreamHandlers = {
  /** Стрим открыт — время перечитать живое (react-query-инвалидация). */
  readonly onOpen?: () => void;
  /** Разобранный кадр стрима; мусорные кадры сюда не доходят. */
  readonly onFrame?: (frame: StreamFrame) => void;
};

/** Имена кадров стрима (ADR 0060): стабильные грубые имена — добавление
 * новых имён назад-совместимо, старый клиент их просто не слушает. */
const STREAM_EVENT_NAMES = ['connected', 'notification.created', 'notification.unread_count'] as const;

/**
 * Открывает стрим и возвращает disposer. Создание источника — в колбэке:
 * модуль не привязан к браузерному EventSource и тестируем в node.
 */
export function connectNotificationStream(deps: {
  readonly createSource: () => EventSourceLike;
  readonly handlers: NotificationStreamHandlers;
  /** Планировщик ручного перезапуска; по умолчанию setTimeout — тесты
   * подменяют ручным. Возвращает cancel. */
  readonly scheduleRetry?: (fn: () => void, delayMs: number) => () => void;
}): () => void {
  return connectEventStream({
    createSource: deps.createSource,
    eventNames: STREAM_EVENT_NAMES,
    scheduleRetry: deps.scheduleRetry,
    handlers: {
      onOpen: deps.handlers.onOpen,
      onEvent: (name, data) => {
        const frame = parseStreamFrame(name, data);
        if (frame !== null) deps.handlers.onFrame?.(frame);
      },
    },
  });
}
