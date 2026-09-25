import { describe, expect, it } from 'vitest';
import type { EventStreamVisibility } from '@/shared/api/sse-client';
import { connectRealtimeStream } from './realtime-stream';
import type { RealtimeFrame } from './realtime-frame';

/** Минимальный фейк EventSource (как в тестах общего ядра sse-client):
 * слушатели по именам, ручной контроль readyState и диспетчеризации. */
class FakeEventSource {
  readyState = 0; // CONNECTING
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
    this.readyState = 2; // CLOSED
  }

  emit(type: string, data?: string): void {
    for (const listener of [...(this.listeners.get(type) ?? [])]) {
      listener({ data });
    }
  }
}

function frameData(payload: unknown): string {
  return JSON.stringify({ v: 1, occurredAt: '2026-09-25T10:00:00Z', payload });
}

function at<T>(items: ReadonlyArray<T>, index: number): T {
  const item = items[index];
  if (item === undefined) throw new Error(`нет элемента #${index}`);
  return item;
}

describe('connectRealtimeStream — обвязка стрима поверх общего ядра', () => {
  it('слушает единственное имя entity.changed плюс open/error', () => {
    const sources: FakeEventSource[] = [];
    connectRealtimeStream({
      createSource: () => {
        const source = new FakeEventSource();
        sources.push(source);
        return source;
      },
      handlers: {},
    });
    expect([...at(sources, 0).listeners.keys()].sort()).toEqual(
      ['entity.changed', 'error', 'open'].sort(),
    );
  });

  it('кадр entity.changed разбирается и диспатчится в onFrame', () => {
    const sources: FakeEventSource[] = [];
    const frames: RealtimeFrame[] = [];
    connectRealtimeStream({
      createSource: () => {
        const source = new FakeEventSource();
        sources.push(source);
        return source;
      },
      handlers: { onFrame: (frame) => frames.push(frame) },
    });
    at(sources, 0).emit('entity.changed', frameData({ propertyId: 'p1', entity: 'access' }));
    expect(frames).toStrictEqual([{ entity: 'access', propertyId: 'p1' }]);
  });

  it('мусорный кадр в onFrame не доходит', () => {
    const sources: FakeEventSource[] = [];
    const frames: RealtimeFrame[] = [];
    connectRealtimeStream({
      createSource: () => {
        const source = new FakeEventSource();
        sources.push(source);
        return source;
      },
      handlers: { onFrame: (frame) => frames.push(frame) },
    });
    at(sources, 0).emit('entity.changed', 'не json');
    expect(frames).toHaveLength(0);
  });

  it('гвард видимости передаётся ядру: скрытие закрывает источник', () => {
    const sources: FakeEventSource[] = [];
    const listeners = new Set<() => void>();
    let visible = true;
    const visibility: EventStreamVisibility = {
      isVisible: () => visible,
      onChange: (listener) => {
        listeners.add(listener);
        return () => listeners.delete(listener);
      },
    };
    const dispose = connectRealtimeStream({
      createSource: () => {
        const source = new FakeEventSource();
        sources.push(source);
        return source;
      },
      handlers: {},
      visibility,
    });
    expect(sources).toHaveLength(1);
    visible = false; // вкладку скрыли
    for (const listener of [...listeners]) listener();
    expect(at(sources, 0).closed).toBe(true);
    dispose();
  });
});
