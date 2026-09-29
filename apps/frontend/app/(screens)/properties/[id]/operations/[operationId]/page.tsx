import { Suspense } from 'react';
import type { Metadata } from 'next';
import type { QueryClient } from '@tanstack/react-query';
import { OperationDetailLoading, OperationDetailScreen } from '@/widgets/payments';
import {
  paymentOperationQueryOptions,
  paymentOperationsGateQueryOptions,
} from '@/features/payments';
import { propertyDetailQueryOptions } from '@/features/properties';
import { paymentOperationKeys } from '@/shared/api/query-keys';
import type { PaymentOperation } from '@/entities/payment';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';

/**
 * Страница операции: вхождение правила с «Отметить оплаченной» (и экраном
 * успеха «Платеж оплачен») по макетам 1386:67731 / 1419:25859 /
 * 1419:25645 / 1444:65733. Оболочка новых экранов — из layout группы (screens).
 */

export const metadata: Metadata = {
  title: 'Операция — Рентли',
};

/**
 * Серверная раскладка первого кадра (#887): деталь объекта и деталь
 * операции — параллельный первый кадр; статусные списки операций правила
 * (кнопка «Оплатить») — после успеха операции, только если у неё есть
 * правило (enabled по paymentId у экрана).
 */
async function prefetchOperationScreen(
  queryClient: QueryClient,
  propertyId: string,
  operationId: string,
): Promise<void> {
  await Promise.all([
    queryClient.prefetchQuery(propertyDetailQueryOptions({ id: propertyId, transport: serverApiClient })),
    queryClient.prefetchQuery(paymentOperationQueryOptions({ propertyId, operationId, transport: serverApiClient })),
  ]);
  const operation = queryClient.getQueryData<PaymentOperation>(
    paymentOperationKeys.byId(propertyId, operationId),
  );
  if (operation === undefined || operation.paymentId === null) {
    return;
  }
  const [overdue, planned] = paymentOperationsGateQueryOptions({
    propertyId, paymentId: operation.paymentId, transport: serverApiClient,
  });
  void queryClient.prefetchQuery(overdue);
  void queryClient.prefetchQuery(planned);
}

export default async function OperationRoutePage({ params }: PageProps<'/properties/[id]/operations/[operationId]'>) {
  const { id, operationId } = await params;

  return (
    <Suspense fallback={<OperationDetailLoading propertyId={id} />}>
      <ServerPrefetchBoundary prefetch={(queryClient) => prefetchOperationScreen(queryClient, id, operationId)}>
        <OperationDetailScreen propertyId={id} operationId={operationId} />
      </ServerPrefetchBoundary>
    </Suspense>
  );
}
