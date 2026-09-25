/**
 * Общее ядро браузерного SSE-соединения (карты #734, #714): одна
 * state-машина EventSource на все стримы платформы — открытие, ручной
 * перезапуск с экспоненциальным бэкоффом на fail-соединении, опциональная
 * пауза по видимости вкладки. Конкретные стримы (слои api фич) дают имена
 * событий и разбор кадров; серверная сторона — общий прокси sse-proxy.ts
 * (общий catch-all режет долгоживущие SSE по 30с, ADR 0060/0062).
 *
 * Переподключение двоякое: сетевые обрывы браузер ретраит сам (retry-hint
 * бэка, readyState CONNECTING) — не мешаем; fail-соединение (readyState
 * CLOSED — 401, не-SSE ответ, вытеснение хабом) браузером не ретраится,
 * поэтому здесь ручной перезапуск с бэкоффом 1с→30с, сброс на успешном
 * open. Гвард видимости: без его депсов соединение живёт всегда
 * (уведомления #734); с ним скрытая вкладка источник закрывает и ждёт
 * показа (realtime #714 — бережёт бюджет хаба, 8 соединений на пользователя
 * на два стрима; кадры без реплея, показ вкладки открывает соединение
 * заново, а открытие у потребителя — перечитывание живого).
 */

/** Срез EventSource, который использует соединение (тесты подсовывают фейк). */
export type EventSourceLike = {
  readonly readyState: number;
  close(): void;
  addEventListener(type: string, listener: EventStreamListener): void;
  removeEventListener(type: string, listener: EventStreamListener): void;
};

/** Слушатель события EventSource; ядро читает только data-строку. */
export type EventStreamListener = (event: { data?: unknown }) => void;

export type EventStreamHandlers = {
  /** Стрим открыт — время перечитать живое (react-query-инвалидация). */
  readonly onOpen?: () => void;
  /** Событие стрима: имя и data-строка; не-строковые data сюда не доходят.
   * Разбор конверта — забота потребителя. */
  readonly onEvent?: (eventName: string, data: string) => void;
};

/** Источник видимости вкладки для гварда; браузерная реализация —
 * document.visibilityState + visibilitychange (в провайдере realtime). */
export type EventStreamVisibility = {
  isVisible(): boolean;
  /** Подписка на смену видимости; возвращает отписку. */
  onChange(listener: () => void): () => void;
};

/** readyState спецификации EventSource: авто-reconnect браузера идёт в
 * CONNECTING, fail-соединение — CLOSED. */
const READY_STATE_CLOSED = 2;

const RETRY_DELAYS_MS = [1000, 2000, 4000, 8000, 16000, 30000] as const;

/**
 * Открывает стрим и возвращает disposer. Создание источника — в колбэке:
 * модуль не привязан к браузерному EventSource и тестируем в node. С
 * гвардом видимости скрытая вкладка источник не открывает (и закрывает
 * живой при скрытии), показ открывает заново со сброшенным бэкоффом.
 */
export function connectEventStream(deps: {
  readonly createSource: () => EventSourceLike;
  readonly eventNames: ReadonlyArray<string>;
  readonly handlers: EventStreamHandlers;
  /** Планировщик ручного перезапуска; по умолчанию setTimeout — тесты
   * подменяют ручным. Возвращает cancel. */
  readonly scheduleRetry?: (fn: () => void, delayMs: number) => () => void;
  readonly visibility?: EventStreamVisibility;
}): () => void {
  const { createSource, eventNames, handlers } = deps;
  const scheduleRetry = deps.scheduleRetry ?? defaultScheduleRetry;
  const visibility = deps.visibility;

  let disposed = false;
  let source: EventSourceLike | undefined;
  let retryIndex = 0;
  let retryTimer: (() => void) | undefined;
  let unsubscribeVisibility: (() => void) | undefined;

  function clearRetryTimer(): void {
    retryTimer?.();
    retryTimer = undefined;
  }

  /** Гасит текущее соединение: источник закрыт, висящий ручной
   * переподключение-таймер погашен. */
  function teardown(): void {
    clearRetryTimer();
    source?.close();
    source = undefined;
  }

  function open(): void {
    if (disposed) return;
    const current = createSource();
    source = current;

    const onOpen = (): void => {
      retryIndex = 0;
      handlers.onOpen?.();
    };

    const onEvent = (name: string) => (event: { data?: unknown }): void => {
      if (disposed || typeof event.data !== 'string') return;
      handlers.onEvent?.(name, event.data);
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
    for (const name of eventNames) {
      current.addEventListener(name, onEvent(name));
    }
  }

  function onVisibilityChange(): void {
    if (disposed || visibility === undefined) return;
    if (visibility.isVisible()) {
      if (source === undefined) {
        // Показ вкладки — новая сессия: бэкофф заново (накопленное до
        // скрытия к скрытию не относится).
        retryIndex = 0;
        open();
      }
      return;
    }
    if (source !== undefined) teardown();
  }

  if (visibility !== undefined) {
    unsubscribeVisibility = visibility.onChange(onVisibilityChange);
  }

  if (visibility === undefined || visibility.isVisible()) {
    open();
  }

  return () => {
    disposed = true;
    unsubscribeVisibility?.();
    unsubscribeVisibility = undefined;
    teardown();
  };
}

function defaultScheduleRetry(fn: () => void, delayMs: number): () => void {
  const timer = setTimeout(fn, delayMs);
  return () => clearTimeout(timer);
}
