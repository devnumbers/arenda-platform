import { Suspense } from 'react';
import type { Metadata } from 'next';
import type { QueryClient } from '@tanstack/react-query';
import { PropertyDetailPage, PropertyDetailRouteLoading } from '@/widgets/property-detail';
import { ServerPrefetchBoundary } from '@/shared/api/server-prefetch';
import { serverApiClient } from '@/shared/api/server-client';
import { dateToIsoLocal } from '@/shared/lib/calendar';
import { propertyKeys } from '@/shared/api/query-keys';
import {
  paymentListQueryOptions,
  paymentOperationsOverdueQueryOptions,
  paymentOperationsSummaryQueryOptions,
  operationsMonthOf,
  operationsMonthRange,
} from '@/features/payments';
import {
  propertiesListQueryOptions,
  propertyDetailQueryOptions,
} from '@/features/properties';
import { rentalsQueryOptions } from '@/features/rentals';
import { subscriptionQueryOptions } from '@/features/subscription';
import { contactsListQuery } from '@/features/contacts';
import { activeTasksQueryOptions } from '@/features/tasks';

export const metadata: Metadata = {
  title: 'Объект — Рентли',
  description: 'Просмотр объекта недвижимости',
};

/**
 * Серверная раскладка первого кадра страницы объекта (пионер #887). Гейт
 * #769 повторяется серверно: деталь решает читаемость объекта, зависимые
 * секции раскладываются только после его успеха — на нечитаемом (404,
 * suspended) объекте секции не простреливают 404-волнами, а сама деталь
 * не гидратируется и клиент перечитывает её своим запросом: not-found,
 * suspended и ErrorCard-экраны работают как сегодня.
 */
async function prefetchPropertyScreen(queryClient: QueryClient, id: string): Promise<void> {
  await queryClient.prefetchQuery(propertyDetailQueryOptions({ id, transport: serverApiClient }));
  if (queryClient.getQueryState(propertyKeys.detail(id))?.status !== 'success') {
    return;
  }

  void queryClient.prefetchQuery(propertiesListQueryOptions(serverApiClient));
  void queryClient.prefetchQuery(subscriptionQueryOptions(serverApiClient));
  void queryClient.prefetchQuery(rentalsQueryOptions({ propertyId: id, transport: serverApiClient }));
  void queryClient.prefetchQuery(paymentListQueryOptions({ propertyId: id, transport: serverApiClient }));
  void queryClient.prefetchQuery(paymentOperationsOverdueQueryOptions({ propertyId: id, transport: serverApiClient }));
  // Сводка секции «Операции в <месяц>» — тот же срез, что у экрана
  // (карта #669), плюс all-time — выбор «Операций еще не было».
  const today = dateToIsoLocal(new Date());
  const month = operationsMonthRange(operationsMonthOf(today));
  void queryClient.prefetchQuery(paymentOperationsSummaryQueryOptions({
    propertyId: id,
    scope: { status: 'paid', order: 'desc', dateFrom: month.from, dateTo: month.to },
    transport: serverApiClient,
  }));
  void queryClient.prefetchQuery(paymentOperationsSummaryQueryOptions({
    propertyId: id,
    scope: { status: 'paid', order: 'desc' },
    transport: serverApiClient,
  }));
  // Дефолтный срез книги объекта — тот же ключ, что читает useContacts
  // экрана (сортировка экрана живёт в URL, префетчится дефолт «Имя ↑»).
  void queryClient.prefetchInfiniteQuery(contactsListQuery({
    propertyId: id,
    search: '',
    sort: 'name',
    order: 'asc',
    transport: serverApiClient,
  }));
  void queryClient.prefetchQuery(activeTasksQueryOptions({ propertyId: id, transport: serverApiClient }));
}

export default async function PropertyDetailRoutePage({
  params,
}: PageProps<'/properties/[id]'>) {
  const { id } = await params;

  return (
    <Suspense fallback={<PropertyDetailRouteLoading />}>
      <ServerPrefetchBoundary prefetch={(queryClient) => prefetchPropertyScreen(queryClient, id)}>
        <PropertyDetailPage />
      </ServerPrefetchBoundary>
    </Suspense>
  );
}
