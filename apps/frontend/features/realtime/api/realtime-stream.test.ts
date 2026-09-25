import { describe, expect, it } from 'vitest';
import { at, FakeEventSource, FakeVisibility } from '@/shared/api/sse-test-fakes';
import { connectRealtimeStream } from './realtime-stream';
import type { RealtimeFrame } from './realtime-frame';

function frameData(payload: unknown): string {
  return JSON.stringify({ v: 1, occurredAt: '2026-09-25T10:00:00Z', payload });
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
    const visibility = new FakeVisibility();
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
    visibility.hide(); // вкладку скрыли
    expect(at(sources, 0).closed).toBe(true);
    dispose();
  });
});
