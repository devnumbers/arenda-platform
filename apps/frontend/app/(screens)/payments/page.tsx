import { Suspense, type JSX } from 'react';
import type { Metadata } from 'next';
import { PaymentsGlobalScreen, PaymentsLoading } from '@/widgets/payments';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';
import {
  globalPaymentObjectsQueryOptions,
  globalPaymentsFeedQueryOptions,
} from '@/features/payments';

export const metadata: Metadata = { title: 'Платежи — Рентли' };

/**
 * Хаб «Платежи» — пионер серверного префетча #887: холодный вход рисует
 * первый кадр с данными. Граница живёт внутри Suspense — hover-префетч
 * динамики рендерит RSC только до неё и серверные запросы впустую не
 * гоняет (research #863).
 */
export default function PaymentsRoutePage(): JSX.Element {
  return (
    <Suspense fallback={<PaymentsLoading />}>
      <ServerPrefetchBoundary
        prefetch={(queryClient) => {
          void queryClient.prefetchQuery(globalPaymentsFeedQueryOptions(serverApiClient));
          void queryClient.prefetchQuery(globalPaymentObjectsQueryOptions({ transport: serverApiClient }));
        }}
      >
        <PaymentsGlobalScreen />
      </ServerPrefetchBoundary>
    </Suspense>
  );
}
