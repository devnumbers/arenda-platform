import { describe, expect, it } from 'vitest';
import { at, FakeEventSource, FakeScheduler } from '@/shared/api/sse-test-fakes';
import { connectNotificationStream } from './notification-stream';
import type { StreamFrame } from './stream-frame';

function frameData(payload: unknown): string {
  return JSON.stringify({ v: 1, occurredAt: '2026-09-19T10:40:00Z', payload });
}

function createFixture() {
  const sources: FakeEventSource[] = [];
  const scheduler = new FakeScheduler();
  const frames: StreamFrame[] = [];
  let openCount = 0;
  const dispose = connectNotificationStream({
    createSource: () => {
      const source = new FakeEventSource();
      sources.push(source);
      return source;
    },
    handlers: {
      onOpen: () => {
        openCount += 1;
      },
      onFrame: (frame) => {
        frames.push(frame);
      },
    },
    scheduleRetry: (fn, delayMs) => scheduler.schedule(fn, delayMs),
  });
  return { sources, scheduler, frames, getOpenCount: () => openCount, dispose };
}

describe('connectNotificationStream — подключение', () => {
  it('открывает источник сразу и слушает три имени стрима', () => {
    const { sources } = createFixture();
    expect(sources).toHaveLength(1);
    expect([...at(sources, 0).listeners.keys()].sort()).toEqual(
      ['connected', 'error', 'notification.created', 'notification.unread_count', 'open'].sort(),
    );
  });

  it('open-событие бьёт в onOpen', () => {
    const { sources, getOpenCount } = createFixture();
    at(sources, 0).readyState = FakeEventSource.OPEN;
    at(sources, 0).emit('open');
    expect(getOpenCount()).toBe(1);
  });

  it('кадры диспатчатся в onFrame разобранными', () => {
    const { sources, frames } = createFixture();
    at(sources, 0).emit('notification.created', frameData({ id: 'n1', title: 'Привет', category: 'tasks', body: '' }));
    at(sources, 0).emit('notification.unread_count', frameData({ count: 3 }));
    expect(frames).toEqual([
      expect.objectContaining({ kind: 'created', id: 'n1' }),
      expect.objectContaining({ kind: 'unread-count', count: 3 }),
    ]);
  });

  it('мусорный кадр молча игнорируется', () => {
    const { sources, frames } = createFixture();
    at(sources, 0).emit('notification.created', 'не json');
    expect(frames).toHaveLength(0);
  });
});

describe('connectNotificationStream — reconnect', () => {
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

describe('connectNotificationStream — dispose', () => {
  it('закрывает источник, глушит таймер и слушатели', () => {
    const source = new FakeEventSource();
    const scheduler = new FakeScheduler();
    const frames: StreamFrame[] = [];
    const dispose = connectNotificationStream({
      createSource: () => source,
      handlers: { onFrame: (frame) => frames.push(frame) },
      scheduleRetry: (fn, delayMs) => scheduler.schedule(fn, delayMs),
    });
    dispose();
    expect(source.closed).toBe(true);
    source.emit('notification.created', frameData({ id: 'x', title: 't' }));
    expect(frames).toHaveLength(0);
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
});
