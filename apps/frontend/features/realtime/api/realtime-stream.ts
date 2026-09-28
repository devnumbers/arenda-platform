import {
  connectEventStream,
  type EventSourceLike,
  type EventStreamVisibility,
} from '@/shared/api/sse-client';
import { parseRealtimeFrame, type RealtimeFrame } from './realtime-frame';

/**
 * Подключение к SSE-стриму GET /realtime/stream (карта #714, тикет #717;
 * ADR 0062) без React — чистая логика соединения, реакт-обвязка в
 * realtime-stream-provider. State-машина переподключения (ручной перезапуск
 * fail-соединения с бэкоффом 1с→30с, сетевые обрывы ретраит браузер) —
 * общее ядро sse-client. Гвард видимости (бережёт бюджет хаба — 8
 * соединений на пользователя на два стрима): скрытая вкладка источник
 * закрывает, показ открывает заново; кадры без реплея, перечитывание
 * живого привязано к открытию стрима — возврат видимости догоняет пропущенное
 * сам.
 */

export type RealtimeStreamHandlers = {
  /** Стрим открыт — время перечитать живое (react-query-инвалидация). */
  readonly onOpen?: () => void;
  /** Разобранный кадр entity.changed; мусорные кадры сюда не доходят. */
  readonly onFrame?: (frame: RealtimeFrame) => void;
};

/** Единственное имя события стрима (ADR 0062 §2), словарь — в payload. */
const STREAM_EVENT_NAMES = ['entity.changed'] as const;

/**
 * Открывает стрим и возвращает disposer. Создание источника — в колбэке:
 * модуль не привязан к браузерному EventSource и тестируем в node.
 */
export function connectRealtimeStream(deps: {
  readonly createSource: () => EventSourceLike;
  readonly handlers: RealtimeStreamHandlers;
  /** Планировщик ручного перезапуска; по умолчанию setTimeout — тесты
   * подменяют ручным. Возвращает cancel. */
  readonly scheduleRetry?: (fn: () => void, delayMs: number) => () => void;
  /** Гвард видимости вкладки; без него соединение живёт всегда. */
  readonly visibility?: EventStreamVisibility;
}): () => void {
  return connectEventStream({
    createSource: deps.createSource,
    eventNames: STREAM_EVENT_NAMES,
    scheduleRetry: deps.scheduleRetry,
    visibility: deps.visibility,
    handlers: {
      onOpen: deps.handlers.onOpen,
      onEvent: (_name, data) => {
        const frame = parseRealtimeFrame(data);
        if (frame !== null) deps.handlers.onFrame?.(frame);
      },
    },
  });
}
