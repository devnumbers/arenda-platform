import { beforeEach, describe, expect, it, vi } from 'vitest';
import { handleStreamFrame, type StreamQueryClient } from './notification-stream-provider';
import { notifyNotificationCreated } from './notification-toast';
import type { StreamFrame } from '../api/stream-frame';

vi.mock('./notification-toast', () => ({ notifyNotificationCreated: vi.fn() }));

const notify = vi.mocked(notifyNotificationCreated);

/** Фейк QueryClient структурного среза обработчика: фиксирует инвалидации и
 * запись кэша счётчика — без vi.fn, сигнатуры совпадают с Pick<QueryClient…>. */
function fakeQueryClient() {
  const invalidated: ReadonlyArray<unknown>[] = [];
  const written: Array<{ key: ReadonlyArray<unknown>; data: unknown }> = [];
  const client: StreamQueryClient = {
    invalidateQueries: ({ queryKey }: { queryKey: ReadonlyArray<unknown> }): Promise<unknown> => {
      invalidated.push(queryKey);
      return Promise.resolve();
    },
    setQueryData: (key: ReadonlyArray<unknown>, data: unknown): unknown => {
      written.push({ key, data });
      return undefined;
    },
  };
  return { ...client, invalidated, written };
}

const CREATED: StreamFrame = {
  kind: 'created',
  id: 'n1',
  category: 'tasks',
  contextLabel: null,
  title: 'Напоминание о задаче',
  body: 'Задача «Позвонить» — сегодня',
  url: null,
  occurredAt: '2026-09-19T10:40:00Z',
};

beforeEach(() => {
  notify.mockClear();
});

describe('handleStreamFrame — кадровая логика провайдера (#747)', () => {
  it('created: инвалидация общего корня notifications (лента + счётчик)', () => {
    const qc = fakeQueryClient();
    handleStreamFrame(CREATED, qc);
    expect(qc.invalidated).toEqual([['notifications']]);
    expect(qc.written).toEqual([]);
  });

  it('unread-count: только мгновенная запись кэша счётчика, без рефеча', () => {
    const qc = fakeQueryClient();
    handleStreamFrame({ kind: 'unread-count', count: 5, occurredAt: '2026-09-19T10:40:00Z' }, qc);
    expect(qc.written).toEqual([{ key: ['notifications', 'unread-count'], data: 5 }]);
    expect(qc.invalidated).toEqual([]);
  });

  it('connected: кадр-сердцебиение ничего не инвалидирует', () => {
    const qc = fakeQueryClient();
    handleStreamFrame({ kind: 'connected' }, qc);
    expect(qc.invalidated).toEqual([]);
    expect(qc.written).toEqual([]);
  });
});

describe('handleStreamFrame — тост-гейт push-настройки категории (#790)', () => {
  it('гейт не передан — прежнее поведение, тост показывается', () => {
    handleStreamFrame(CREATED, fakeQueryClient());
    expect(notify).toHaveBeenCalledTimes(1);
  });

  it('гейт разрешил категорию — тост показывается, предикату уходит категория кадра', () => {
    const seen: string[] = [];
    handleStreamFrame(CREATED, fakeQueryClient(), (category) => {
      seen.push(category);
      return true;
    });
    expect(seen).toEqual(['tasks']);
    expect(notify).toHaveBeenCalledTimes(1);
    expect(notify).toHaveBeenCalledWith(CREATED);
  });

  it('гейт заглушил категорию — тоста нет, лента всё равно инвалидируется', () => {
    const qc = fakeQueryClient();
    handleStreamFrame(CREATED, qc, () => false);
    expect(notify).not.toHaveBeenCalled();
    expect(qc.invalidated).toEqual([['notifications']]);
  });
});
