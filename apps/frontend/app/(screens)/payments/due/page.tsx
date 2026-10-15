import { Suspense } from 'react';
import type { Metadata } from 'next';
import {
  PaymentsDueGlobalScreen,
  PaymentsDueLoading,
  parseDueSortParams,
} from '@/widgets/payments';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';
import { paymentsDueQueryOptions } from '@/features/payments';

export const metadata: Metadata = { title: 'Платежи на оплату — Рентли' };

/**
 * Глобальная страница «Платежи на оплату» (тикет #1347). Единый хром —
 * ScreenLayout группы (screens); вход — секция «На оплату» главного экрана.
 * Направление строк живёт в query строки (?sort=old, дефолт «Сначала
 * новые» не пишется) — переживает перезагрузку. Холодный вход рисует
 * первый кадр с данными: серверный префетч канона #887 внутри Suspense.
 */
export default async function PaymentsDueRoutePage({
  searchParams,
}: PageProps<'/payments/due'>) {
  const resolved = await searchParams;
  const sort = parseDueSortParams(resolved.sort);

  return (
    <Suspense fallback={<PaymentsDueLoading />}>
      <ServerPrefetchBoundary
        prefetch={(queryClient) => {
          void queryClient.prefetchQuery(paymentsDueQueryOptions(serverApiClient));
        }}
      >
        <PaymentsDueGlobalScreen initialSort={sort} />
      </ServerPrefetchBoundary>
    </Suspense>
  );
}
