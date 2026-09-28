import {
  QueryClient,
  defaultShouldDehydrateQuery,
  dehydrate,
  type DehydratedState,
} from '@tanstack/react-query';

/**
 * Ядро серверного слоя префетча #887 — без серверных зависимостей (cookies,
 * 'server-only'), чтобы жило в vitest. RSC-обёртка — server-prefetch.tsx.
 *
 * Канон (решение #866, тикет #887): на каждый серверный рендер новый
 * QueryClient; префетч через queryOptions-фабрики; перед dehydrate запросы
 * оседают в бюджете. Неуспешные (ошибка, таймаут бюджета) и неуспевшие
 * запросы снимаются с hydration — клиент перечитывает их сам и ведёт себя
 * как сегодня: typed ApiError приходит из его собственного запроса, 404
 * объекта рисует not-found-экран, 401/403 деградируют в «нет гидратации»,
 * а не в 500 RSC. Успешные hydrate'ятся в кэш до первого рендера виджета —
 * HydrationBoundary кладёт новые запросы в кэш синхронно (useMemo), поэтому
 * SSR-HTML рисует данные, а не скелетон.
 */

/**
 * Бюджет оседания серверного префетча: сколько RSC-рендер ждёт ответы бэка
 * перед dehydrate. Типичный локальный/стендовый бэк укладывается в десятки
 * мс; при медленном бэке Suspense-граница страницы показывает скелетон до
 * бюджета, дальше страница отдаётся с тем, что успело, — остальное клиент
 * перечитывает. Дольше 30с (таймаут серверного транспорта) ждать нельзя;
 * 5с держат TTFB конечным при деградации бэка.
 */
export const SERVER_PREFETCH_BUDGET_MS = 5000;

export function createServerQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        // Серверный рендер не ретраит: каждая попытка сидит в бюджете TTFB,
        // а провалившийся запрос всё равно снимается с hydration.
        retry: false,
        staleTime: 30 * 1000,
        refetchOnWindowFocus: false,
      },
    },
  });
}

/**
 * Оседание префетчнутых запросов: ждём все запущенные на клиенте fetch в
 * пределах бюджета. Возвращает, успел ли весь набор (для тестов/логов).
 */
export async function settleServerPrefetch(
  queryClient: QueryClient,
  budgetMs: number = SERVER_PREFETCH_BUDGET_MS,
): Promise<boolean> {
  const inflight = (): ReadonlyArray<Promise<unknown>> =>
    queryClient
      .getQueryCache()
      .getAll()
      .filter((query) => query.state.fetchStatus === 'fetching')
      .flatMap((query) => (query.promise ? [query.promise] : []));

  const settleWithin = async (ms: number): Promise<void> => {
    let timeoutId: ReturnType<typeof setTimeout> | undefined;
    try {
      await Promise.race([
        (async () => {
          // Запросы могут доцепиться в кэш после первого снятия среза —
          // крутим до плато или бюджета.
          while (inflight().length > 0) {
            await Promise.allSettled(inflight());
          }
        })(),
        new Promise<void>((resolve) => {
          timeoutId = setTimeout(resolve, ms);
        }),
      ]);
    } finally {
      if (timeoutId !== undefined) {
        clearTimeout(timeoutId);
      }
    }
  };

  await settleWithin(budgetMs);
  return inflight().length === 0;
}

/**
 * Снятие негидратируемых запросов с кэша: ошибка и неуспевшие в бюджете
 * не попадают в dehydrate — клиент перечитает их сам («нет гидратации»
 * канона #887). pending в кэше после снятия не остаётся, поэтому фильтр
 * dehydrate — дефолтный (только success).
 */
export function dropUnhydratableQueries(queryClient: QueryClient): void {
  for (const query of queryClient.getQueryCache().getAll()) {
    if (query.state.status !== 'success') {
      queryClient.removeQueries({ queryKey: query.queryKey });
    }
  }
}

/** Итоговое состояние для HydrationBoundary — только успешные запросы. */
export function dehydrateServerPrefetch(queryClient: QueryClient): DehydratedState {
  return dehydrate(queryClient, {
    shouldDehydrateQuery: defaultShouldDehydrateQuery,
  });
}
