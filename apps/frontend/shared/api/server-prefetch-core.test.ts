import { describe, expect, it } from 'vitest';
import { QueryClient, dehydrate, hydrate } from '@tanstack/react-query';
import {
  createServerQueryClient,
  dehydrateServerPrefetch,
  dropUnhydratableQueries,
  settleServerPrefetch,
} from './server-prefetch-core';
import { ApiError } from './errors';

describe('createServerQueryClient', () => {
  it('не ретраит — провал считается сразу, бюджет TTFB не жжётся попытками', async () => {
    const queryClient = createServerQueryClient();
    let attempts = 0;
    await queryClient
      .fetchQuery({
        queryKey: ['failing'],
        queryFn: () => {
          attempts += 1;
          return Promise.reject(new ApiError('boom', 'сбой'));
        },
      })
      .catch(() => undefined);
    expect(attempts).toBe(1);
  });
});

describe('settleServerPrefetch', () => {
  it('ждёт все запущенные запросы и оседает на успешных', async () => {
    const queryClient = createServerQueryClient();
    void queryClient.prefetchQuery({
      queryKey: ['slow'],
      queryFn: () => new Promise<string>((resolve) => setTimeout(() => resolve('ok'), 20)),
    });
    void queryClient.prefetchQuery({ queryKey: ['fast'], queryFn: () => Promise.resolve(1) });
    await expect(settleServerPrefetch(queryClient)).resolves.toBe(true);
    expect(queryClient.getQueryData(['slow'])).toBe('ok');
    expect(queryClient.getQueryData(['fast'])).toBe(1);
  });

  it('выходит по бюджету, если запрос висит дольше', async () => {
    const queryClient = createServerQueryClient();
    void queryClient.prefetchQuery({
      queryKey: ['hung'],
      queryFn: () => new Promise<string>(() => undefined),
    });
    await expect(settleServerPrefetch(queryClient, 20)).resolves.toBe(false);
  });
});

describe('dropUnhydratableQueries + dehydrateServerPrefetch', () => {
  it('в hydration попадают только успешные запросы', async () => {
    const queryClient = createServerQueryClient();
    await queryClient.prefetchQuery({ queryKey: ['ok'], queryFn: () => Promise.resolve(42) });
    void queryClient.prefetchQuery({
      queryKey: ['failed'],
      queryFn: () => Promise.reject(new ApiError('not_found', 'нет')),
    });
    void queryClient.prefetchQuery({
      queryKey: ['hung'],
      queryFn: () => new Promise<string>(() => undefined),
    });
    await settleServerPrefetch(queryClient, 20);
    dropUnhydratableQueries(queryClient);

    const state = dehydrateServerPrefetch(queryClient);
    expect(state.queries.map((query) => query.queryKey)).toStrictEqual([['ok']]);
    expect(queryClient.getQueryData(['ok'])).toBe(42);
    expect(queryClient.getQueryState(['failed'])).toBeUndefined();
    expect(queryClient.getQueryState(['hung'])).toBeUndefined();
  });
});

describe('гонка SSE×гидратация (канон #887)', () => {
  /**
   * SSE-кадр кладёт данные в клиентский кэш setQueryData'ой между ответом
   * сервера (dehydrate) и маунтом границы. Гидратация свежее затирать не
   * имеет права — экранные данные мигнули бы назад. Гард — dataUpdatedAt
   * и dehydratedAt в hydrate (query-core).
   */
  it('свежий SSE-кадр в кэше не затирается гидратацией серверного снимка', () => {
    const serverTime = 1000;
    const sseTime = 2000;

    const serverClient = createServerQueryClient();
    serverClient.setQueryData(['payments'], { total: 1, server: true });
    const dehydrated = dehydrate(serverClient, {
      shouldDehydrateQuery: () => true,
    });
    // Снимок сервера старше SSE-кадра: правим временные метки на каноничные.
    for (const query of dehydrated.queries) {
      query.state = { ...query.state, dataUpdatedAt: serverTime };
      query.dehydratedAt = serverTime;
    }

    const browserClient = new QueryClient();
    browserClient.setQueryData(['payments'], { total: 2, sse: true }, { updatedAt: sseTime });
    expect(browserClient.getQueryState(['payments'])?.dataUpdatedAt).toBe(sseTime);

    hydrate(browserClient, dehydrated);

    expect(browserClient.getQueryData(['payments'])).toStrictEqual({ total: 2, sse: true });
  });

  it('без клиентских данных гидратация приносит серверный кадр', () => {
    const serverClient = createServerQueryClient();
    serverClient.setQueryData(['payments'], { total: 1, server: true });
    const dehydrated = dehydrate(serverClient, {
      shouldDehydrateQuery: () => true,
    });
    for (const query of dehydrated.queries) {
      query.state = { ...query.state, dataUpdatedAt: 1000 };
      query.dehydratedAt = 1000;
    }

    const browserClient = new QueryClient();
    hydrate(browserClient, dehydrated);

    expect(browserClient.getQueryData(['payments'])).toStrictEqual({ total: 1, server: true });
  });
});
