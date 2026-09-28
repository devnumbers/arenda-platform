import { describe, expect, it } from 'vitest';
import {
  handleStreamFrame,
  notificationHandlers,
  type ShowNotificationToast,
  type StreamQueryClient,
} from './notification-handlers';
import type { NotificationCreatedFrame, StreamFrame } from './stream-frame';

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

/** Лежачий тост-эффект (инъекция вместо DOM): фиксирует кадры, дошедшие до
 * показа. */
function fakeToast(): { shown: NotificationCreatedFrame[]; showToast: ShowNotificationToast } {
  const shown: NotificationCreatedFrame[] = [];
  return { shown, showToast: (frame) => shown.push(frame) };
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

describe('handleStreamFrame — кадровая логика стрима (#747)', () => {
  it('created: инвалидация общего корня notifications (лента + счётчик), кэш не пишется', () => {
    const qc = fakeQueryClient();
    handleStreamFrame(CREATED, qc, fakeToast().showToast);
    expect(qc.invalidated).toEqual([['notifications']]);
    expect(qc.written).toEqual([]);
  });

  it('unread-count: только мгновенная запись кэша счётчика, без рефеча', () => {
    const qc = fakeQueryClient();
    const { showToast } = fakeToast();
    handleStreamFrame({ kind: 'unread-count', count: 5, occurredAt: '2026-09-19T10:40:00Z' }, qc, showToast);
    expect(qc.written).toEqual([{ key: ['notifications', 'unread-count'], data: 5 }]);
    expect(qc.invalidated).toEqual([]);
  });

  it('connected: кадр-сердцебиение ничего не делает', () => {
    const qc = fakeQueryClient();
    const { showToast } = fakeToast();
    handleStreamFrame({ kind: 'connected' }, qc, showToast);
    expect(qc.invalidated).toEqual([]);
    expect(qc.written).toEqual([]);
  });
});

describe('handleStreamFrame — тост-гейт push-настройки категории (#790)', () => {
  it('гейт не передан — прежнее поведение, тост показывается', () => {
    const { shown, showToast } = fakeToast();
    handleStreamFrame(CREATED, fakeQueryClient(), showToast);
    expect(shown).toEqual([CREATED]);
  });

  it('гейт разрешил категорию — тост показывается, предикату уходит категория кадра', () => {
    const { shown, showToast } = fakeToast();
    const seen: string[] = [];
    handleStreamFrame(CREATED, fakeQueryClient(), showToast, (category) => {
      seen.push(category);
      return true;
    });
    expect(seen).toEqual(['tasks']);
    expect(shown).toEqual([CREATED]);
  });

  it('гейт заглушил категорию — тоста нет, лента всё равно инвалидируется', () => {
    const qc = fakeQueryClient();
    const { shown, showToast } = fakeToast();
    handleStreamFrame(CREATED, qc, showToast, () => false);
    expect(shown).toEqual([]);
    expect(qc.invalidated).toEqual([['notifications']]);
  });
});

describe('notificationHandlers — открытые-хендлеры стрима (replay-курсора нет, ADR 0060)', () => {
  it('onOpen перечитывает живое — инвалидация общего корня notifications', () => {
    const qc = fakeQueryClient();
    const { showToast } = fakeToast();
    notificationHandlers({ queryClient: qc, showToast, toastGate: () => true }).onOpen?.();
    expect(qc.invalidated).toEqual([['notifications']]);
  });

  it('onFrame диспатчит в handleStreamFrame: инвалидация и тост идут по кадру', () => {
    const qc = fakeQueryClient();
    const { shown, showToast } = fakeToast();
    notificationHandlers({ queryClient: qc, showToast, toastGate: () => true }).onFrame?.(CREATED);
    expect(qc.invalidated).toEqual([['notifications']]);
    expect(shown).toEqual([CREATED]);
  });

  it('гейт фабрики глушит тост, не трогая инвалидацию ленты', () => {
    const qc = fakeQueryClient();
    const { shown, showToast } = fakeToast();
    notificationHandlers({ queryClient: qc, showToast, toastGate: () => false }).onFrame?.(CREATED);
    expect(shown).toEqual([]);
    expect(qc.invalidated).toEqual([['notifications']]);
  });
});
