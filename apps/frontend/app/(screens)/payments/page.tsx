import type { JSX } from 'react';
import type { Metadata } from 'next';
import { PaymentsGlobalScreen } from '@/widgets/payments';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';
import {
  globalPaymentObjectsQueryOptions,
  globalPaymentsFeedQueryOptions,
} from '@/features/payments';

// Кабинетная страница: куки-сессия и URL-состояние — всегда динамический
// рендер; force-dynamic снимает требование Suspense-границы для
// useSearchParams при статическом экспорте (аменда #1605 — in-page
// Suspense убран ради однофлушного HTML, react/react#37014).
export const dynamic = 'force-dynamic';

export const metadata: Metadata = { title: 'Платежи — Рентли' };

/**
 * Хаб «Платежи» — пионер серверного префетча #887: холодный вход рисует
 * первый кадр с данными. Граница живёт внутри Suspense — hover-префетч
 * динамики рендерит RSC только до неё и серверные запросы впустую не
 * гоняет (research #863).
 */
export default function PaymentsRoutePage(): JSX.Element {
  return (
<ServerPrefetchBoundary
  prefetch={(queryClient) => {
    void queryClient.prefetchQuery(globalPaymentsFeedQueryOptions(serverApiClient));
    void queryClient.prefetchQuery(globalPaymentObjectsQueryOptions({ transport: serverApiClient }));
  }}
>
  <PaymentsGlobalScreen />
</ServerPrefetchBoundary>
  );
}
