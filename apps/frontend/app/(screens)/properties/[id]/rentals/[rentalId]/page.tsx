import { Suspense } from 'react';
import type { Metadata } from 'next';
import type { QueryClient } from '@tanstack/react-query';
import { RentalCompletedLoading, RentalCompletedScreen } from '@/widgets/rentals';
import { propertyDetailQueryOptions } from '@/features/properties';
import { rentalsQueryOptions } from '@/features/rentals';
import { paymentOperationsPagedQueryOptions } from '@/features/payments';
import { rentalKeys } from '@/shared/api/query-keys';
import type { Rental } from '@/entities/rental';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';

/**
 * Завершённая детализация «Прошлых аренд» (#535): карточка прошлой аренды
 * по id. Незавершённая по прямой ссылке уводится на текущий экран аренды.
 */

export const metadata: Metadata = {
  title: 'Аренда — Рентли',
};

/**
 * Серверная раскладка первого кадра (#887): список аренд (экран выбирает
 * карточку из него) и деталь объекта — параллельный первый кадр; история
 * платежа аренды — после успеха списка, только когда аренда найдена.
 */
async function prefetchRentalScreen(
  queryClient: QueryClient,
  propertyId: string,
  rentalId: string,
): Promise<void> {
  await Promise.all([
    queryClient.prefetchQuery(propertyDetailQueryOptions({ id: propertyId, transport: serverApiClient })),
    queryClient.prefetchQuery(rentalsQueryOptions({ propertyId, transport: serverApiClient })),
  ]);
  const rentals = queryClient.getQueryData<Rental[]>(rentalKeys.list(propertyId));
  const rental = rentals?.find((item) => item.id === rentalId);
  if (rental === undefined) {
    return;
  }
  void queryClient.prefetchInfiniteQuery(paymentOperationsPagedQueryOptions({
    propertyId,
    paymentId: rental.rentPayment.paymentId,
    status: 'paid',
    order: 'desc',
    transport: serverApiClient,
  }));
}

export default async function RentalCompletedRoutePage({
  params,
}: PageProps<'/properties/[id]/rentals/[rentalId]'>) {
  const { id, rentalId } = await params;

  return (
    <Suspense fallback={<RentalCompletedLoading propertyId={id} />}>
      <ServerPrefetchBoundary prefetch={(queryClient) => prefetchRentalScreen(queryClient, id, rentalId)}>
        <RentalCompletedScreen propertyId={id} rentalId={rentalId} />
      </ServerPrefetchBoundary>
    </Suspense>
  );
}
