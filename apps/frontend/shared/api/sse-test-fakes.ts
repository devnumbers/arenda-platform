/**
 * Общие тестовые фейки SSE-подсистемы (ядро sse-client и стримы поверх
 * него — уведомления, realtime) вместо копии в каждом тестовом файле.
 * Модуль только для тестов: в продукте не импортируется.
 */

import type { EventSourceLike, EventStreamVisibility } from './sse-client';

/** Минимальный фейк EventSource: слушатели по именам, ручной контроль
 * readyState и диспетчеризации. Числа readyState — из спецификации
 * EventSource: CONNECTING=0, OPEN=1, CLOSED=2. */
export class FakeEventSource implements EventSourceLike {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSED = 2;

  readyState = FakeEventSource.CONNECTING;
  closed = false;
  readonly listeners = new Map<string, Set<(event: { data?: unknown }) => void>>();

  addEventListener(type: string, listener: (event: { data?: unknown }) => void): void {
    let set = this.listeners.get(type);
    if (!set) {
      set = new Set();
      this.listeners.set(type, set);
    }
    set.add(listener);
  }

  removeEventListener(type: string, listener: (event: { data?: unknown }) => void): void {
    this.listeners.get(type)?.delete(listener);
  }

  close(): void {
    this.closed = true;
    this.readyState = FakeEventSource.CLOSED;
  }

  emit(type: string, data?: string): void {
    for (const listener of [...(this.listeners.get(type) ?? [])]) {
      listener({ data });
    }
  }
}

/** Ручной планировщик: копит (fn, delay), тест дергает сам. */
export class FakeScheduler {
  readonly entries: Array<{ fn: () => void; delayMs: number; cancelled: boolean }> = [];

  schedule(fn: () => void, delayMs: number): () => void {
    const entry = { fn, delayMs, cancelled: false };
    this.entries.push(entry);
    return () => {
      entry.cancelled = true;
    };
  }

  runAll(): void {
    for (const entry of [...this.entries]) {
      if (!entry.cancelled) entry.fn();
    }
  }
}

/** Фейк видимости: тест переключает вкладку вручную. */
export class FakeVisibility implements EventStreamVisibility {
  visible = true;
  private readonly listeners = new Set<() => void>();
  private unsubscribeCount = 0;

  isVisible(): boolean {
    return this.visible;
  }

  onChange(listener: () => void): () => void {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
      this.unsubscribeCount += 1;
    };
  }

  hide(): void {
    this.visible = false;
    for (const listener of [...this.listeners]) listener();
  }

  show(): void {
    this.visible = true;
    for (const listener of [...this.listeners]) listener();
  }

  get unsubscribed(): number {
    return this.unsubscribeCount;
  }
}

/** Доступ по индексу без non-null assertions (eslint-канон): в тестах
 * отсутствие элемента — ошибка сценария, не ветка логики. */
export function at<T>(items: ReadonlyArray<T>, index: number): T {
  const item = items[index];
  if (item === undefined) throw new Error(`нет элемента #${index}`);
  return item;
}
