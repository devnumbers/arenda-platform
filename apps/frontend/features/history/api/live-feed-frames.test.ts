import { describe, expect, it, vi } from 'vitest';
import { createLiveFeedFrameHandler, type LiveFeedDeps } from './live-feed-frames';

function historyFrame(propertyId: string | null = 'p1') {
  return { entity: 'history', propertyId };
}

/** Стенд обработчика: fetchFresh возвращает полноту порции из очереди
 * (пустая очередь — «дно»: порция легла не в полный размер). */
function harness(overrides: Partial<LiveFeedDeps> = {}) {
  const calls: string[] = [];
  const portionQueue: boolean[] = [];
  let enabled = true;
  let hasPages = true;
  let scopePropertyIds: ReadonlyArray<string> | undefined;

  const fetchFresh = vi.fn((): Promise<{ hasMore: boolean } | null> => {
    calls.push('fetchFresh');
    return Promise.resolve({ hasMore: portionQueue.shift() ?? false });
  });
  const refetch = vi.fn((): Promise<void> => {
    calls.push('refetch');
    return Promise.resolve();
  });
  const state = vi.fn(() => ({ enabled, hasPages, scopePropertyIds }));

  const handle = createLiveFeedFrameHandler({ state, fetchFresh, refetch, ...overrides });
  return {
    handle,
    calls,
    fetchFresh,
    refetch,
    setEnabled: (value: boolean) => {
      enabled = value;
    },
    setHasPages: (value: boolean) => {
      hasPages = value;
    },
    setScopePropertyIds: (value: ReadonlyArray<string> | undefined) => {
      scopePropertyIds = value;
    },
    /** Полнота порций следующих догона (true — порция ровно в pageSize). */
    enqueuePortions: (...values: boolean[]) => {
      portionQueue.push(...values);
    },
  };
}

describe('createLiveFeedFrameHandler — кадр истории → догон снизу (тикет #718)', () => {
  it('кадр по объекту ленты запускает догон', async () => {
    const h = harness();
    h.handle(historyFrame());
    await vi.waitFor(() => expect(h.calls).toStrictEqual(['fetchFresh']));
  });

  it('кадр по чужому объекту при фильтре объектов ленты игнорируется', async () => {
    const h = harness();
    h.setScopePropertyIds(['p1', 'p2']);

    h.handle(historyFrame('p3'));

    await Promise.resolve();
    expect(h.calls).toStrictEqual([]);
  });

  it('кадр по своему объекту проходит фильтр объектов', async () => {
    const h = harness();
    h.setScopePropertyIds(['p1', 'p2']);

    h.handle(historyFrame('p2'));

    await vi.waitFor(() => expect(h.calls).toStrictEqual(['fetchFresh']));
  });

  it('кадр без propertyId (безобъектная книга) догоняет', async () => {
    const h = harness();
    h.handle(historyFrame(null));
    await vi.waitFor(() => expect(h.calls).toStrictEqual(['fetchFresh']));
  });

  it('запрос не готов (первая загрузка/выключен) — кадр молча пропускается', async () => {
    const h = harness();
    h.setEnabled(false);

    h.handle(historyFrame());

    await Promise.resolve();
    expect(h.calls).toStrictEqual([]);
  });

  it('лента пуста (вливать не во что) — кадр перечитывает ленту целиком', async () => {
    const h = harness();
    h.setHasPages(false);

    h.handle(historyFrame());

    await vi.waitFor(() => expect(h.calls).toStrictEqual(['refetch']));
  });

  it('полная порция тянет следующий догон, неполная — дно', async () => {
    const h = harness();
    h.enqueuePortions(true, false);

    h.handle(historyFrame());

    await vi.waitFor(() => expect(h.calls).toStrictEqual(['fetchFresh', 'fetchFresh']));
  });

  it('после потолка полных порций лента перечитывается целиком (редкий burst)', async () => {
    const h = harness();
    h.enqueuePortions(true, true, true, true, true);

    h.handle(historyFrame());

    await vi.waitFor(() => {
      expect(h.calls).toStrictEqual([
        'fetchFresh',
        'fetchFresh',
        'fetchFresh',
        'fetchFresh',
        'fetchFresh',
        'refetch',
      ]);
    });
  });

  it('кадры во время полёта догона не параллелятся — цепочка промисов', async () => {
    let release!: () => void;
    const gate = new Promise<{ hasMore: boolean } | null>((resolve) => {
      release = () => resolve({ hasMore: false });
    });
    const h = harness();
    let active = 0;
    let overlapped = false;
    h.fetchFresh.mockImplementation(() => {
      if (active > 0) {
        overlapped = true;
      }
      active += 1;
      const result = active === 1 ? gate : Promise.resolve({ hasMore: false });
      return result.finally(() => {
        active -= 1;
      });
    });

    h.handle(historyFrame());
    h.handle(historyFrame());
    h.handle(historyFrame());

    await vi.waitFor(() => expect(h.fetchFresh).toHaveBeenCalledTimes(1));
    expect(overlapped).toBe(false);
    release();
    await vi.waitFor(() => expect(h.fetchFresh).toHaveBeenCalledTimes(3));
    // Три кадра — три последовательных догона без единого наложения.
    expect(overlapped).toBe(false);
  });
});
