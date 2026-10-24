import type { Metadata } from 'next';
import { OperationsGlobalScreen } from '@/widgets/payments';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';
import { OPERATIONS_FEED_SORT } from '@/shared/api/query-keys';
import {
  globalOperationsPagedQueryOptions,
  globalOperationsSummaryQueryOptions,
} from '@/features/payments';
import { propertiesListQueryOptions } from '@/features/properties';

// Кабинетная страница: куки-сессия и URL-состояние — всегда динамический
// рендер; force-dynamic снимает требование Suspense-границы для
// useSearchParams при статическом экспорте (аменда #1605 — in-page
// Suspense убран ради однофлушного HTML, react/react#37014).
export const dynamic = 'force-dynamic';

/** Экран «Операции» — глобальная лента по всем объектам (карта #545,
 * тикет #541); на едином хроме экранов (#564), вход — «Операции» в
 * сайдбаре ПК и в шите «Еще» на мобайле и планшете. Фильтры — в
 * query-параметрах, поэтому клиентский экран со useSearchParams стоит за
 * Suspense-границей — требование App Router (иначе прод-сборка падает на
 * пререндере). */
export const metadata: Metadata = {
  title: 'Операции — Рентли',
};

export default function OperationsRoutePage() {
  return (
<ServerPrefetchBoundary
  prefetch={(queryClient) => {
    // Дефолтный срез ленты — весь период (#670), все объекты: те же
    // ключи, что читает экран без применённого периода; сводка без
    // периода — она же all-time (гейт «Операций еще не было»).
    // Лента платёжных фактов читается по paid_date (#933/#994).
    void queryClient.prefetchInfiniteQuery(globalOperationsPagedQueryOptions({
      scope: { order: 'desc', sort: OPERATIONS_FEED_SORT },
      transport: serverApiClient,
    }));
    void queryClient.prefetchQuery(globalOperationsSummaryQueryOptions({
      scope: { order: 'desc', sort: OPERATIONS_FEED_SORT },
      transport: serverApiClient,
    }));
    void queryClient.prefetchQuery(propertiesListQueryOptions(serverApiClient));
  }}
>
  <OperationsGlobalScreen />
</ServerPrefetchBoundary>
  );
}
