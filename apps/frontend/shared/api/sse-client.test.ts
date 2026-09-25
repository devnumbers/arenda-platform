import { describe, expect, it } from 'vitest';
import {
  connectEventStream,
  type EventSourceLike,
  type EventStreamVisibility,
} from './sse-client';

/** Минимальный фейк EventSource: слушатели по именам, ручной контроль
 * readyState и диспетчеризации. Числа readyState — из спецификации
 * EventSource: CONNECTING=0, OPEN=1, CLOSED=2. */
class FakeEventSource implements EventSourceLike {
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
class FakeScheduler {
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
class FakeVisibility implements EventStreamVisibility {
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
function at<T>(items: ReadonlyArray<T>, index: number): T {
  const item = items[index];
  if (item === undefined) throw new Error(`нет элемента #${index}`);
  return item;
}

const TWO_NAMES = ['stream.event-a', 'stream.event-b'] as const;

function createFixture(overrides?: {
  visibility?: FakeVisibility;
}) {
  const sources: FakeEventSource[] = [];
  const scheduler = new FakeScheduler();
  const events: Array<{ name: string; data: string }> = [];
  let openCount = 0;
  const dispose = connectEventStream({
    createSource: () => {
      const source = new FakeEventSource();
      sources.push(source);
      return source;
    },
    eventNames: TWO_NAMES,
    handlers: {
      onOpen: () => {
        openCount += 1;
      },
      onEvent: (name, data) => {
        events.push({ name, data });
      },
    },
    scheduleRetry: (fn, delayMs) => scheduler.schedule(fn, delayMs),
    ...(overrides?.visibility ? { visibility: overrides.visibility } : {}),
  });
  return { sources, scheduler, events, getOpenCount: () => openCount, dispose };
}

describe('connectEventStream — подключение', () => {
  it('открывает источник сразу и слушает имена стрима плюс open/error', () => {
    const { sources } = createFixture();
    expect(sources).toHaveLength(1);
    expect([...at(sources, 0).listeners.keys()].sort()).toEqual(
      ['error', 'open', 'stream.event-a', 'stream.event-b'].sort(),
    );
  });

  it('open-событие бьёт в onOpen', () => {
    const { sources, getOpenCount } = createFixture();
    at(sources, 0).readyState = FakeEventSource.OPEN;
    at(sources, 0).emit('open');
    expect(getOpenCount()).toBe(1);
  });

  it('события диспатчатся в onEvent с именем и data-строкой', () => {
    const { sources, events } = createFixture();
    at(sources, 0).emit('stream.event-a', '{"x":1}');
    at(sources, 0).emit('stream.event-b', '{}');
    expect(events).toEqual([
      { name: 'stream.event-a', data: '{"x":1}' },
      { name: 'stream.event-b', data: '{}' },
    ]);
  });

  it('не-строковые data в onEvent не доходят', () => {
    const { sources, events } = createFixture();
    at(sources, 0).listeners.get('stream.event-a')?.forEach((listener) => listener({}));
    expect(events).toHaveLength(0);
  });
});

describe('connectEventStream — reconnect', () => {
  it('fail-соединение (readyState CLOSED — браузер сам не переподключится) перезапускается через 1с', () => {
    const { sources, scheduler } = createFixture();
    at(sources, 0).readyState = FakeEventSource.CLOSED;
    at(sources, 0).emit('error');
    expect(sources).toHaveLength(1); // новый источник — по таймеру, не сразу
    at(scheduler.entries, 0).fn();
    expect(sources).toHaveLength(2);
    expect(at(scheduler.entries, 0).delayMs).toBe(1000);
  });

  it('авто-reconnect браузера (readyState CONNECTING) не дублируется ручным', () => {
    const { sources, scheduler } = createFixture();
    at(sources, 0).readyState = FakeEventSource.CONNECTING;
    at(sources, 0).emit('error');
    expect(sources).toHaveLength(1);
    expect(scheduler.entries).toHaveLength(0);
  });

  it('бэкофф растёт 1с → 2с, успешный open его сбрасывает', () => {
    const { sources, scheduler } = createFixture();
    at(sources, 0).readyState = FakeEventSource.CLOSED;
    at(sources, 0).emit('error');
    at(sources, 0).emit('error'); // второй error при живом таймере не ставит второй таймер
    expect(scheduler.entries).toHaveLength(1);

    at(scheduler.entries, 0).fn();
    expect(sources).toHaveLength(2);
    at(sources, 1).readyState = FakeEventSource.CLOSED;
    at(sources, 1).emit('error');
    expect(at(scheduler.entries, 1).delayMs).toBe(2000);

    at(scheduler.entries, 1).fn();
    at(sources, 2).readyState = FakeEventSource.OPEN;
    at(sources, 2).emit('open');
    at(sources, 2).readyState = FakeEventSource.CLOSED;
    at(sources, 2).emit('error');
    expect(at(scheduler.entries, 2).delayMs).toBe(1000);
  });

  it('бэкофф капится на 30с', () => {
    const { sources, scheduler } = createFixture();
    let last = at(sources, 0);
    for (let i = 0; i < 8; i += 1) {
      last.readyState = FakeEventSource.CLOSED;
      last.emit('error');
      scheduler.runAll();
      const created = at(sources, sources.length - 1);
      expect(created).not.toBe(last);
      last = created;
    }
    const delays = scheduler.entries.map((entry) => entry.delayMs);
    expect(delays.slice(0, 5)).toEqual([1000, 2000, 4000, 8000, 16000]);
    expect(delays.slice(5)).toEqual([30000, 30000, 30000]);
  });
});

describe('connectEventStream — видимость вкладки', () => {
  it('скрытая вкладка источник не открывает — открытие ждёт показа', () => {
    const visibility = new FakeVisibility();
    visibility.hide();
    const { sources } = createFixture({ visibility });
    expect(sources).toHaveLength(0);
    visibility.show();
    expect(sources).toHaveLength(1);
  });

  it('скрытие закрывает источник и глушит висящий таймер переподключения', () => {
    const visibility = new FakeVisibility();
    const { sources, scheduler } = createFixture({ visibility });
    at(sources, 0).readyState = FakeEventSource.CLOSED;
    at(sources, 0).emit('error');
    expect(scheduler.entries).toHaveLength(1);

    visibility.hide();
    expect(at(sources, 0).closed).toBe(true);
    scheduler.runAll();
    expect(sources).toHaveLength(1); // таймер погашен — источник не пересоздался
    expect(scheduler.entries.every((entry) => entry.cancelled)).toBe(true);
  });

  it('возврат видимости открывает свежий источник со сброшенным бэкоффом', () => {
    const visibility = new FakeVisibility();
    const { sources, scheduler } = createFixture({ visibility });
    at(sources, 0).readyState = FakeEventSource.CLOSED;
    at(sources, 0).emit('error'); // бэкофф 1с запланирован

    visibility.hide();
    visibility.show();

    expect(sources).toHaveLength(2);
    at(sources, 1).readyState = FakeEventSource.CLOSED;
    at(sources, 1).emit('error');
    // Бэкофф начался заново с 1с, а не продолжился с накопленной.
    expect(at(scheduler.entries, scheduler.entries.length - 1).delayMs).toBe(1000);
  });

  it('повторное скрытие при закрытом источнике безопасно', () => {
    const visibility = new FakeVisibility();
    const { sources, dispose } = createFixture({ visibility });
    visibility.hide();
    visibility.hide();
    expect(at(sources, 0).closed).toBe(true);
    expect(() => visibility.show()).not.toThrow();
    dispose();
  });
});

describe('connectEventStream — dispose', () => {
  it('закрывает источник, глушит таймер и слушатели', () => {
    const source = new FakeEventSource();
    const scheduler = new FakeScheduler();
    const events: Array<{ name: string; data: string }> = [];
    const dispose = connectEventStream({
      createSource: () => source,
      eventNames: TWO_NAMES,
      handlers: { onEvent: (name, data) => events.push({ name, data }) },
      scheduleRetry: (fn, delayMs) => scheduler.schedule(fn, delayMs),
    });
    dispose();
    expect(source.closed).toBe(true);
    source.emit('stream.event-a', '{"x":1}');
    expect(events).toHaveLength(0);
    source.readyState = FakeEventSource.CLOSED;
    source.emit('error');
    scheduler.runAll();
    expect(scheduler.entries.every((entry) => entry.cancelled)).toBe(true);
  });

  it('повторный dispose безопасен', () => {
    const { dispose } = createFixture();
    expect(() => {
      dispose();
      dispose();
    }).not.toThrow();
  });

  it('отписывается от видимости', () => {
    const visibility = new FakeVisibility();
    const { dispose } = createFixture({ visibility });
    expect(visibility.unsubscribed).toBe(0);
    dispose();
    expect(visibility.unsubscribed).toBe(1);
    // После dispose смена видимости соединение не трогает.
    visibility.hide();
    visibility.show();
    expect(visibility.unsubscribed).toBe(1);
  });
});
