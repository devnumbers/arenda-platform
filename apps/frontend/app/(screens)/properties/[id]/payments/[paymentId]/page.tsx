import { Suspense } from 'react';
import type { Metadata } from 'next';
import type { QueryClient } from '@tanstack/react-query';
import { PaymentDetailLoading, PaymentDetailScreen } from '@/widgets/payments';
import {
  paymentDetailQueryOptions,
  paymentOperationsGateQueryOptions,
} from '@/features/payments';
import { propertyDetailQueryOptions } from '@/features/properties';
import { paymentKeys } from '@/shared/api/query-keys';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';

/**
 * Страница платежа (#465): карточка правила, мутации паузы/оплаты/избранного,
 * секции «Ближайший платеж» и «Просроченные», плитки подэкранов. Оболочка
 * новых экранов — из layout группы (screens).
 */

export const metadata: Metadata = {
  title: 'Платеж — Рентли',
};

/**
 * Серверная раскладка первого кадра (#887): деталь объекта и деталь
 * платежа — параллельный первый кадр; статусные списки операций (гасилки
 * «Оплатить») — после успеха платежа, как у экрана (enabled по id правила).
 */
async function prefetchPaymentScreen(
  queryClient: QueryClient,
  propertyId: string,
  paymentId: string,
): Promise<void> {
  await Promise.all([
    queryClient.prefetchQuery(propertyDetailQueryOptions({ id: propertyId, transport: serverApiClient })),
    queryClient.prefetchQuery(paymentDetailQueryOptions({ propertyId, paymentId, transport: serverApiClient })),
  ]);
  if (queryClient.getQueryState(paymentKeys.detail(propertyId, paymentId))?.status !== 'success') {
    return;
  }
  const [overdue, planned] = paymentOperationsGateQueryOptions({
    propertyId, paymentId, transport: serverApiClient,
  });
  void queryClient.prefetchQuery(overdue);
  void queryClient.prefetchQuery(planned);
}

export default async function PaymentRoutePage({ params }: PageProps<'/properties/[id]/payments/[paymentId]'>) {
  const { id, paymentId } = await params;

  return (
    <Suspense fallback={<PaymentDetailLoading propertyId={id} />}>
      <ServerPrefetchBoundary prefetch={(queryClient) => prefetchPaymentScreen(queryClient, id, paymentId)}>
        <PaymentDetailScreen propertyId={id} paymentId={paymentId} />
      </ServerPrefetchBoundary>
    </Suspense>
  );
}
