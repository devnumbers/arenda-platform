import { Suspense } from 'react';
import type { Metadata } from 'next';
import type { QueryClient } from '@tanstack/react-query';
import { PaymentDetailLoading, PaymentDetailScreen } from '@/widgets/payments';
import {
  paymentDetailQueryOptions,
  paymentOperationsGateQueryOptions,
  paymentOperationsPagedQueryOptions,
} from '@/features/payments';
import { propertyDetailQueryOptions } from '@/features/properties';
import { OPERATIONS_FEED_SORT, paymentKeys } from '@/shared/api/query-keys';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';

/**
 * Страница платежа (#465): карточка правила, мутации паузы/оплаты/избранного,
 * секции «Ближайший платеж» и «Просроченные платежи», секция «История
 * платежа» (макет 3214:76417; лексика — #1338). Оболочка новых экранов — из layout группы
 * (screens).
 */

export const metadata: Metadata = {
  title: 'Платеж — Рентли',
};

/**
 * Серверная раскладка первого кадра (#887): деталь объекта и деталь
 * платежа — параллельная волна; после успеха платежа монолитно ждём
 * остальное содержимое экрана — гасилки «Оплатить» (те же ключи, что
 * читает экран) и первую порцию истории (решение владельца 22.10):
 * секции приходят с данными сразу, скелетон сменяется контентом одним
 * кадром без волн догрузки (страница не дёргается: секция истории не
 * прыгает после гидрации, «Просроченные» не мигают пустым).
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
  await Promise.all([
    queryClient.prefetchQuery(overdue),
    queryClient.prefetchQuery(planned),
    queryClient.prefetchInfiniteQuery(paymentOperationsPagedQueryOptions({
      propertyId,
      paymentId,
      status: 'paid',
      order: 'desc',
      sort: OPERATIONS_FEED_SORT,
      transport: serverApiClient,
    })),
  ]);
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
