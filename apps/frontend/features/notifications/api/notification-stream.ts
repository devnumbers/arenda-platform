import { parseStreamFrame, type StreamFrame } from './stream-frame';

/**
 * Живой слой уведомлений на фронте (#747): подключение к SSE-стриму
 * GET /notifications/stream (#742, ADR 0058) без React — чистая логика
 * соединения, реакт-обвязка в notification-stream-provider. Компонент
 * держит EventSource на каждую вкладку — бэк рассчитан на это (лимит хаба
 * 8 соединений на пользователя, вкладки/устройства).
 *
 * Переподключение двоякое: сетевые обрывы браузер ретраит сам (retry-hint
 * бэка 3с, readyState CONNECTING) — не мешаем; fail-соединение
 * (readyState CLOSED — 401, не-SSE ответ, вытеснение хабом) браузером не
 * ретраится, поэтому здесь ручной перезапуск с экспоненциальным бэкоффом
 * 1с→30с, сброс на успешном open. На каждом открытии провайдер
 * перечитывает живое через react-query (replay-курсора в v1 нет).
 */

/** Срез EventSource, который использует соединение (тесты подсовывают фейк). */
export type EventSourceLike = {
  readonly readyState: number;
  close(): void;
  addEventListener(type: string, listener: FakeListener): void;
  removeEventListener(type: string, listener: FakeListener): void;
};

export type FakeListener = (event: { data?: unknown }) => void;

export type NotificationStreamHandlers = {
  /** Стрим открыт — время перечитать живое (react-query-инвалидация). */
  readonly onOpen?: () => void;
  /** Разобранный кадр стрима; мусорные кадры сюда не доходят. */
  readonly onFrame?: (frame: StreamFrame) => void;
};

/** readyState спецификации EventSource: авто-reconnect браузера идёт в
 * CONNECTING, fail-соединение — CLOSED. */
const READY_STATE_CLOSED = 2;

const RETRY_DELAYS_MS = [1000, 2000, 4000, 8000, 16000, 30000] as const;

/** Имена кадров стрима (ADR 0058): стабильные грубые имена — добавление
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
  const { createSource, handlers } = deps;
  const scheduleRetry = deps.scheduleRetry ?? defaultScheduleRetry;

  let disposed = false;
  let source: EventSourceLike | undefined;
  let retryIndex = 0;
  let retryTimer: (() => void) | undefined;

  function clearRetryTimer(): void {
    retryTimer?.();
    retryTimer = undefined;
  }

  function open(): void {
    if (disposed) return;
    const current = createSource();
    source = current;

    const onOpen = (): void => {
      retryIndex = 0;
      handlers.onOpen?.();
    };

    const onFrame = (name: string) => (event: { data?: unknown }): void => {
      if (disposed || typeof event.data !== 'string') return;
      const frame = parseStreamFrame(name, event.data);
      if (frame !== null) handlers.onFrame?.(frame);
    };

    const onError = (): void => {
      if (disposed || current !== source) return;
      if (current.readyState !== READY_STATE_CLOSED) return; // браузер ретраит сам
      // Fail-соединение: ручной перезапуск с бэкоффом (один таймер на раз —
      // повторные error при живом таймере игнорируются).
      if (retryTimer !== undefined) return;
      // Индекс cap'ится длиной массива, «?? 30000» — для noUncheckedIndexedAccess.
      const delay = RETRY_DELAYS_MS[Math.min(retryIndex, RETRY_DELAYS_MS.length - 1)] ?? 30000;
      retryIndex += 1;
      retryTimer = scheduleRetry(() => {
        retryTimer = undefined;
        open();
      }, delay);
    };

    current.addEventListener('open', onOpen);
    current.addEventListener('error', onError);
    for (const name of STREAM_EVENT_NAMES) {
      current.addEventListener(name, onFrame(name));
    }
  }

  open();

  return () => {
    disposed = true;
    clearRetryTimer();
    source?.close();
    source = undefined;
  };
}

function defaultScheduleRetry(fn: () => void, delayMs: number): () => void {
  const timer = setTimeout(fn, delayMs);
  return () => clearTimeout(timer);
}
