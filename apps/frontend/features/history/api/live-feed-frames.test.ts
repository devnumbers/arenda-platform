import { describe, expect, it, vi } from 'vitest';
import { createLiveFeedFrameHandler, type LiveFeedDeps } from './live-feed-frames';

/** Стенд обработчика: prepend возвращает курсор свежей границы из очереди
 * (как это делает реальный результат fetchPreviousPage), пустая очередь —
 * граница упёрлась в дно (null). */
function harness(overrides: Partial<LiveFeedDeps> = {}) {
  const calls: string[] = [];
  const cursorQueue: (string | null)[] = [];
  let enabled = true;
  let hasPreviousPage = true;
  let scopePropertyIds: ReadonlyArray<string> | undefined;

  const prepend = vi.fn((): Promise<{ firstPrevCursor: string | null } | null> => {
    calls.push('prepend');
    return Promise.resolve({ firstPrevCursor: cursorQueue.shift() ?? null });
  });
  const refetch = vi.fn((): Promise<void> => {
    calls.push('refetch');
    return Promise.resolve();
  });
  const state = vi.fn(() => ({ enabled, hasPreviousPage, scopePropertyIds }));

  const handle = createLiveFeedFrameHandler({ state, prepend, refetch, ...overrides });
  return {
    handle,
    calls,
    prepend,
    refetch,
    setEnabled: (value: boolean) => {
      enabled = value;
    },
    setHasPreviousPage: (value: boolean) => {
      hasPreviousPage = value;
    },
    setScopePropertyIds: (value: ReadonlyArray<string> | undefined) => {
      scopePropertyIds = value;
    },
    /** Что вернут следующие prepend'ы как курсор свежей границы страницы. */
    enqueueFirstPrevCursors: (...values: (string | null)[]) => {
      cursorQueue.push(...values);
    },
  };
}

function historyFrame(propertyId: string | null = 'p1') {
  return { entity: 'history', propertyId };
}

describe('createLiveFeedFrameHandler — кадр истории → prepend свежих (тикет #718)', () => {
  it('кадр по объекту ленты prependит свежую страницу', async () => {
    const h = harness();
    h.handle(historyFrame());
    await vi.waitFor(() => expect(h.calls).toStrictEqual(['prepend']));
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

    await vi.waitFor(() => expect(h.calls).toStrictEqual(['prepend']));
  });

  it('кадр без propertyId (безобъектная книга) prependит', async () => {
    const h = harness();
    h.handle(historyFrame(null));
    await vi.waitFor(() => expect(h.calls).toStrictEqual(['prepend']));
  });

  it('запрос не готов (первая загрузка/выключен) — кадр молча пропускается', async () => {
    const h = harness();
    h.setEnabled(false);

    h.handle(historyFrame());

    await Promise.resolve();
    expect(h.calls).toStrictEqual([]);
  });

  it('нет границы свежих (пустая лента) — кадр перечитывает ленту целиком', async () => {
    const h = harness();
    h.setHasPreviousPage(false);

    h.handle(historyFrame());

    await vi.waitFor(() => expect(h.calls).toStrictEqual(['refetch']));
  });

  it('догоняет страницу за страницей, пока свежая граница не упрётся в дно', async () => {
    const h = harness();
    h.enqueueFirstPrevCursors('cursor-1', null);

    h.handle(historyFrame());

    await vi.waitFor(() => expect(h.calls).toStrictEqual(['prepend', 'prepend']));
  });

  it('кадры во время полёта не параллелятся — цепочка промисов', async () => {
    let release!: () => void;
    const gate = new Promise<{ firstPrevCursor: string | null }>((resolve) => {
      release = () => resolve({ firstPrevCursor: null });
    });
    const h = harness();
    let active = 0;
    let overlapped = false;
    h.prepend.mockImplementation(() => {
      if (active > 0) {
        overlapped = true;
      }
      active += 1;
      const result = active === 1 ? gate : Promise.resolve({ firstPrevCursor: null });
      return result.finally(() => {
        active -= 1;
      });
    });

    h.handle(historyFrame());
    h.handle(historyFrame());
    h.handle(historyFrame());

    await vi.waitFor(() => expect(h.prepend).toHaveBeenCalledTimes(1));
    expect(overlapped).toBe(false);
    release();
    await vi.waitFor(() => expect(h.prepend).toHaveBeenCalledTimes(3));
    // Три кадра — три последовательных прогона без единого наложения.
    expect(overlapped).toBe(false);
  });

  it('после потолка доprependов лента перечитывается целиком (редкий burst)', async () => {
    const h = harness();
    h.enqueueFirstPrevCursors('more-1', 'more-2', 'more-3', 'more-4', 'more-5');

    h.handle(historyFrame());

    await vi.waitFor(() => {
      expect(h.calls).toStrictEqual([
        'prepend',
        'prepend',
        'prepend',
        'prepend',
        'prepend',
        'refetch',
      ]);
    });
  });
});
