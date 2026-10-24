import type { Metadata } from 'next';
import {
  PaymentsDueGlobalScreen,
  parseDueSortParams,
} from '@/widgets/payments';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';
import { paymentsDueQueryOptions } from '@/features/payments';

// Кабинетная страница: куки-сессия и URL-состояние — всегда динамический
// рендер; force-dynamic снимает требование Suspense-границы для
// useSearchParams при статическом экспорте (аменда #1605 — in-page
// Suspense убран ради однофлушного HTML, react/react#37014).
export const dynamic = 'force-dynamic';

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
<ServerPrefetchBoundary
  prefetch={(queryClient) => {
    void queryClient.prefetchQuery(paymentsDueQueryOptions(serverApiClient));
  }}
>
  <PaymentsDueGlobalScreen initialSort={sort} />
</ServerPrefetchBoundary>
  );
}
