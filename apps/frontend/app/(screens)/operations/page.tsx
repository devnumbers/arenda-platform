import { Suspense } from 'react';
import type { Metadata } from 'next';
import { OperationsGlobalScreen, OperationsLoading } from '@/widgets/payments';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';
import { OPERATIONS_FEED_SORT } from '@/shared/api/query-keys';
import {
  globalOperationsPagedQueryOptions,
  globalOperationsSummaryQueryOptions,
} from '@/features/payments';
import { propertiesListQueryOptions } from '@/features/properties';

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
    <Suspense fallback={<OperationsLoading />}>
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
    </Suspense>
  );
}
