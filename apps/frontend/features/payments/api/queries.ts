import { queryOptions, type UseQueryOptions } from '@tanstack/react-query';
import { apiClient, type ApiTransport } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import {
  mapGlobalPaymentFeed,
  mapGlobalPaymentObject,
  mapPayment,
  mapPaymentOperation,
  mapOperationsSummary,
} from '@/entities/payment';
import type {
  GlobalPaymentFeed,
  GlobalPaymentObject,
  OperationsSummary,
  Payment,
  PaymentOperation,
} from '@/entities/payment';
import {
  globalOperationKeys,
  globalPaymentKeys,
  paymentKeys,
  paymentOperationKeys,
  type GlobalOperationScope,
  type OperationsSortKey,
  type PaymentOperationOrder,
  type PaymentOperationScope,
  type PaymentOperationStatusFilter,
} from '@/shared/api/query-keys';
import type { components } from '@/shared/api/dto';
import {
  keysetNextPageParam,
  type KeysetPage,
  type KeysetPagedQueryConfig,
} from '@/shared/lib/keyset';
import { OPERATIONS_PAGE_SIZE, operationsOffsetNextPageParam } from '../lib/operations-pages';

/** Порция глобальной ленты (#597): строки плюс keyset-продолжение —
 * opaque-курсор следующей порции, null = лента исчерпана. */
export type GlobalOperationsPageData = KeysetPage<PaymentOperation>;

type PaymentsResponse = components['schemas']['PaymentsResponse'];
type OperationsResponse = components['schemas']['OperationsResponse'];
type OperationsSummaryResponse = components['schemas']['OperationsSummaryResponse'];
type PaymentResponseDto = components['schemas']['PaymentResponse'];
type OperationResponseDto = components['schemas']['OperationResponse'];

/** Двойной модуль API-слоя payments (без 'use client'): чистые fetch-функции
 * и queryOptions-фабрики канона #887 — общий источник ключ+fetch для
 * клиентских хуков, прогрева хабов #626 и серверного префетча. Хуки — в
 * hooks.ts ('use client'); мутации, поиск и отфильтрованные срезы остаются
 * там же. */

/** Чистый fetch списка платежей объекта — общее горло хука и серверного
 * префетча #887 (transport выбирает окружение). */
export async function fetchPaymentsOfProperty(
  propertyId: string,
  search = '',
  transport: ApiTransport = apiClient,
): Promise<Payment[]> {
  const query = search ? `?search=${encodeURIComponent(search)}` : '';
  const response = await transport<PaymentsResponse>(
    `/properties/${encodeURIComponent(propertyId)}/payments${query}`,
  );
  return response.items.map(mapPayment);
}

/** Опции списка платежей объекта (канон #887): один источник ключ+fetch
 * для хука и серверного префетча. */
export function paymentListQueryOptions({
  propertyId,
  search = '',
  transport = apiClient,
}: {
  readonly propertyId: string;
  readonly search?: string;
  readonly transport?: ApiTransport;
}): UseQueryOptions<Payment[], ApiError, Payment[], ReturnType<typeof paymentKeys.list>> {
  return queryOptions({
    queryKey: paymentKeys.list(propertyId, search),
    queryFn: () => fetchPaymentsOfProperty(propertyId, search, transport),
  });
}

/** Чистый fetch платежа — общее горло хука и серверного префетча #887. */
export async function fetchPayment(
  propertyId: string,
  paymentId: string,
  transport: ApiTransport = apiClient,
): Promise<Payment> {
  const response = await transport<PaymentResponseDto>(
    `/properties/${encodeURIComponent(propertyId)}/payments/${encodeURIComponent(paymentId)}`,
  );
  return mapPayment(response);
}

/** Опции платежа (канон #887): один источник ключ+fetch для хука и
 * серверного префетча. */
export function paymentDetailQueryOptions({
  propertyId,
  paymentId,
  transport = apiClient,
}: {
  readonly propertyId: string;
  readonly paymentId: string;
  readonly transport?: ApiTransport;
}): UseQueryOptions<Payment, ApiError, Payment, ReturnType<typeof paymentKeys.detail>> {
  return queryOptions({
    queryKey: paymentKeys.detail(propertyId, paymentId),
    queryFn: () => fetchPayment(propertyId, paymentId, transport),
  });
}

/** Чистый fetch просрочек объекта — общее горло хука и серверного
 * префетча #887. */
export async function fetchPropertyOperationsOverdue(
  propertyId: string,
  search = '',
  transport: ApiTransport = apiClient,
): Promise<PaymentOperation[]> {
  const query = search ? `&search=${encodeURIComponent(search)}` : '';
  const response = await transport<OperationsResponse>(
    `/properties/${encodeURIComponent(propertyId)}/operations?status=overdue&order=asc${query}`,
  );
  return response.items.map(mapPaymentOperation);
}

/** Опции просрочек объекта (канон #887): один источник ключ+fetch для
 * хука и серверного префетча. */
export function paymentOperationsOverdueQueryOptions({
  propertyId,
  search = '',
  transport = apiClient,
}: {
  readonly propertyId: string;
  readonly search?: string;
  readonly transport?: ApiTransport;
}): UseQueryOptions<PaymentOperation[], ApiError, PaymentOperation[], ReturnType<typeof paymentOperationKeys.overdueByProperty>> {
  return queryOptions({
    queryKey: paymentOperationKeys.overdueByProperty(propertyId, search),
    queryFn: () => fetchPropertyOperationsOverdue(propertyId, search, transport),
  });
}

/** Чистый fetch сводки объекта — общее горло хука и серверного префетча
 * #887. Категорийный фильтр уходит в запрос: сводка сужается категорией
 * вместе со списком (решение владельца 01.10). */
export async function fetchPropertyOperationsSummary(
  propertyId: string,
  scope: PaymentOperationScope,
  transport: ApiTransport = apiClient,
): Promise<OperationsSummary> {
  const params = new URLSearchParams();
  params.set('status', scope.status);
  if (scope.sort !== undefined) {
    params.set('sort', scope.sort);
  }
  if (scope.type !== undefined) {
    params.set('type', scope.type);
  }
  if (scope.categories !== undefined && scope.categories.length > 0) {
    params.set('category', scope.categories.join(','));
  }
  if (scope.dateFrom !== undefined) {
    params.set('date_from', scope.dateFrom);
  }
  if (scope.dateTo !== undefined) {
    params.set('date_to', scope.dateTo);
  }
  if (scope.search !== undefined && scope.search !== '') {
    params.set('search', scope.search);
  }
  const response = await transport<OperationsSummaryResponse>(
    `/properties/${encodeURIComponent(propertyId)}/operations/summary?${params.toString()}`,
  );
  return mapOperationsSummary(response);
}

/** Опции сводки объекта (канон #887): один источник ключ+fetch для хука
 * и серверного префетча. */
export function paymentOperationsSummaryQueryOptions({
  propertyId,
  scope,
  transport = apiClient,
}: {
  readonly propertyId: string;
  readonly scope: PaymentOperationScope;
  readonly transport?: ApiTransport;
}): UseQueryOptions<OperationsSummary, ApiError, OperationsSummary, ReturnType<typeof paymentOperationKeys.summary>> {
  return queryOptions({
    queryKey: paymentOperationKeys.summary(propertyId, scope),
    queryFn: () => fetchPropertyOperationsSummary(propertyId, scope, transport),
  });
}

/** Чистый fetch фида «Платежей» — общее горло хука, прогрева хабов
 * #626 и серверного префетча #887 (кэш прогревается тем же кодом, что
 * читает экран). */
export async function fetchGlobalPaymentsFeed(
  transport: ApiTransport = apiClient,
): Promise<GlobalPaymentFeed> {
  const response = await transport<components['schemas']['PaymentsGlobalResponse']>(
    '/payments',
  );
  return mapGlobalPaymentFeed(response);
}

/** Опции фида «Платежей» (канон #887): один источник ключ+fetch для хука
 * и серверного префетча. */
export function globalPaymentsFeedQueryOptions(
  transport: ApiTransport = apiClient,
): UseQueryOptions<GlobalPaymentFeed, ApiError, GlobalPaymentFeed, typeof globalPaymentKeys.feed> {
  return queryOptions({
    queryKey: globalPaymentKeys.feed,
    queryFn: () => fetchGlobalPaymentsFeed(transport),
  });
}

/** Чистый fetch стопок объектов «Платежей» — общее горло хука, prefetch
 * и серверного префетча #887. */
export async function fetchGlobalPaymentObjects(
  search = '',
  transport: ApiTransport = apiClient,
): Promise<ReadonlyArray<GlobalPaymentObject>> {
  const query = search ? `?search=${encodeURIComponent(search)}` : '';
  const response = await transport<components['schemas']['PaymentObjectsGlobalResponse']>(
    `/payments/objects${query}`,
  );
  return response.items.map(mapGlobalPaymentObject);
}

/** Опции стопок объектов «Платежей» (канон #887): один источник ключ+fetch
 * для хука и серверного префетча. */
export function globalPaymentObjectsQueryOptions({
  search = '',
  transport = apiClient,
}: {
  readonly search?: string;
  readonly transport?: ApiTransport;
} = {}): UseQueryOptions<ReadonlyArray<GlobalPaymentObject>, ApiError, ReadonlyArray<GlobalPaymentObject>, ReturnType<typeof globalPaymentKeys.objects>> {
  return queryOptions({
    queryKey: globalPaymentKeys.objects(search),
    queryFn: () => fetchGlobalPaymentObjects(search, transport),
  });
}

/** Чистый fetch порции глобальной ленты — общее горло хука, прогрева
 * хабов #626 и серверного префетча #887. cursor — keyset-продолжение
 * прошлого ответа (#597); undefined читает ленту с начала. */
export async function fetchGlobalOperationsPage(
  scope: GlobalOperationScope,
  cursor?: string,
  transport: ApiTransport = apiClient,
): Promise<GlobalOperationsPageData> {
  const params = operationsScopeParams(scope);
  params.set('order', scope.order);
  params.set('limit', String(OPERATIONS_PAGE_SIZE));
  if (cursor) {
    params.set('cursor', cursor);
  }
  const response = await transport<OperationsResponse>(`/operations?${params.toString()}`);
  return {
    items: response.items.map(mapPaymentOperation),
    nextCursor: response.nextCursor ?? null,
  };
}

/**
 * Конфиг keyset-обхода глобальной ленты (#887) — возвращаемый тип фабрики
 * globalOperationsPagedQueryOptions: экспорты features/ несут явные
 * возвращаемые типы (apps/frontend/AGENTS.md), форма — общий
 * KeysetPagedQueryConfig из shared/lib/keyset.
 */
export type GlobalOperationsPagedQueryConfig = KeysetPagedQueryConfig<
  ReturnType<typeof globalOperationKeys.listPaged>,
  GlobalOperationsPageData
>;

/** Опции глобальной ленты (канон #887): один источник ключ+fetch для хука
 * и серверного префетча. */
export function globalOperationsPagedQueryOptions({
  scope,
  transport = apiClient,
}: {
  readonly scope: GlobalOperationScope;
  readonly transport?: ApiTransport;
}): GlobalOperationsPagedQueryConfig {
  return {
    queryKey: globalOperationKeys.listPaged(scope),
    queryFn: ({ pageParam }) => fetchGlobalOperationsPage(scope, pageParam, transport),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: keysetNextPageParam,
  };
}

/** Чистый fetch сводки операций — общее горло хука, прогрева хабов #626
 * и серверного префетча #887. */
export async function fetchGlobalOperationsSummary(
  scope: GlobalOperationScope,
  transport: ApiTransport = apiClient,
): Promise<OperationsSummary> {
  const params = operationsScopeParams(scope);
  const query = params.toString();
  const response = await transport<OperationsSummaryResponse>(
    `/operations/summary${query.length > 0 ? `?${query}` : ''}`,
  );
  return mapOperationsSummary(response);
}

/** Опции сводки глобальной ленты (канон #887): один источник ключ+fetch
 * для хука и серверного префетча. */
export function globalOperationsSummaryQueryOptions({
  scope,
  transport = apiClient,
}: {
  readonly scope: GlobalOperationScope;
  readonly transport?: ApiTransport;
}): UseQueryOptions<OperationsSummary, ApiError, OperationsSummary, ReturnType<typeof globalOperationKeys.summary>> {
  return queryOptions({
    queryKey: globalOperationKeys.summary(scope),
    queryFn: () => fetchGlobalOperationsSummary(scope, transport),
  });
}

/** Чистый fetch статусного списка операций платежа — общее горло хука
 * и серверного префетча #887. */
export async function fetchPaymentOperationsByStatus(
  propertyId: string,
  paymentId: string,
  status: PaymentOperationStatusFilter,
  transport: ApiTransport = apiClient,
): Promise<PaymentOperation[]> {
  const response = await transport<OperationsResponse>(
    `/properties/${encodeURIComponent(propertyId)}/payments/${encodeURIComponent(paymentId)}`
      + `/operations?status=${status}&order=asc`,
  );
  return response.items.map(mapPaymentOperation);
}

/** Опции статусного списка операций платежа (канон #887): один источник
 * ключ+fetch для хука и серверного префетча. */
export function paymentOperationsByStatusQueryOptions({
  propertyId,
  paymentId,
  status,
  transport = apiClient,
}: {
  readonly propertyId: string;
  readonly paymentId: string;
  readonly status: PaymentOperationStatusFilter;
  readonly transport?: ApiTransport;
}): UseQueryOptions<PaymentOperation[], ApiError, PaymentOperation[], ReturnType<typeof paymentOperationKeys.byPaymentWithStatus>> {
  return queryOptions({
    queryKey: paymentOperationKeys.byPaymentWithStatus(propertyId, paymentId, status),
    queryFn: () => fetchPaymentOperationsByStatus(propertyId, paymentId, status, transport),
  });
}

/**
 * Конфиг пары гасящих списков «Оплатить» — возвращаемый тип фабрики
 * paymentOperationsGateQueryOptions: экспорты features/ несут явные
 * возвращаемые типы (apps/frontend/AGENTS.md).
 */
export type PaymentOperationsGateQueryOptions = readonly [
  ReturnType<typeof paymentOperationsByStatusQueryOptions>,
  ReturnType<typeof paymentOperationsByStatusQueryOptions>,
];

/**
 * Пара гасящих списков «Оплатить» (#465): просроченные и плановые операции
 * правила — те же ключи, что читает usePaymentOperationsByStatus экрана,
 * поэтому серверный прогрев пары гасит кнопку без перезапроса. Порядок
 * закреплён за страницами: просроченные первыми (приоритет долга —
 * порядок накопления, канон «Оплатить»).
 */
export function paymentOperationsGateQueryOptions({
  propertyId,
  paymentId,
  transport = apiClient,
}: {
  readonly propertyId: string;
  readonly paymentId: string;
  readonly transport?: ApiTransport;
}): PaymentOperationsGateQueryOptions {
  return [
    paymentOperationsByStatusQueryOptions({ propertyId, paymentId, status: 'overdue', transport }),
    paymentOperationsByStatusQueryOptions({ propertyId, paymentId, status: 'planned', transport }),
  ] as const;
}

/** Чистый fetch операции — общее горло хука и серверного префетча #887. */
export async function fetchOperation(
  propertyId: string,
  operationId: string,
  transport: ApiTransport = apiClient,
): Promise<PaymentOperation> {
  const response = await transport<OperationResponseDto>(
    `/properties/${encodeURIComponent(propertyId)}`
      + `/operations/${encodeURIComponent(operationId)}`,
  );
  return mapPaymentOperation(response);
}

/** Опции операции (канон #887): один источник ключ+fetch для хука и
 * серверного префетча. */
export function paymentOperationQueryOptions({
  propertyId,
  operationId,
  transport = apiClient,
}: {
  readonly propertyId: string;
  readonly operationId: string;
  readonly transport?: ApiTransport;
}): UseQueryOptions<PaymentOperation, ApiError, PaymentOperation, ReturnType<typeof paymentOperationKeys.byId>> {
  return queryOptions({
    queryKey: paymentOperationKeys.byId(propertyId, operationId),
    queryFn: () => fetchOperation(propertyId, operationId, transport),
  });
}

/** Чистый fetch порции операций платежа — общее горло хука и серверного
 * префетча #887. sort — ключ даты (#992): истории платёжных фактов просят
 * paid_date (дополнение #994), просрочки остаются на плановой. */
export async function fetchPaymentOperationsPagedPage(
  propertyId: string,
  paymentId: string,
  params: {
    readonly status: PaymentOperationStatusFilter;
    readonly order: PaymentOperationOrder;
    readonly sort?: OperationsSortKey;
  },
  offset: number,
  transport: ApiTransport = apiClient,
): Promise<ReadonlyArray<PaymentOperation>> {
  const sortQuery = params.sort !== undefined ? `&sort=${params.sort}` : '';
  const response = await transport<OperationsResponse>(
    `/properties/${encodeURIComponent(propertyId)}/payments/${encodeURIComponent(paymentId)}`
      + `/operations?status=${params.status}&order=${params.order}${sortQuery}`
      + `&limit=${OPERATIONS_PAGE_SIZE}&offset=${String(offset)}`,
  );
  return response.items.map(mapPaymentOperation);
}

/**
 * Конфиг offset-обхода операций платежа (#887) — возвращаемый тип фабрики
 * paymentOperationsPagedQueryOptions: экспорты features/ несут явные
 * возвращаемые типы (apps/frontend/AGENTS.md), члены — их выведенная форма.
 */
export type PaymentOperationsPagedQueryConfig = {
  readonly queryKey: ReturnType<typeof paymentOperationKeys.byPaymentPaged>;
  readonly queryFn: (context: {
    readonly pageParam: number;
  }) => Promise<ReadonlyArray<PaymentOperation>>;
  readonly initialPageParam: number;
  readonly getNextPageParam: typeof operationsOffsetNextPageParam;
};

/** Опции порций операций платежа (канон #887): один источник ключ+fetch
 * для хука и серверного префетча. */
export function paymentOperationsPagedQueryOptions({
  propertyId,
  paymentId,
  status,
  order,
  sort,
  transport = apiClient,
}: {
  readonly propertyId: string;
  readonly paymentId: string;
  readonly status: PaymentOperationStatusFilter;
  readonly order: PaymentOperationOrder;
  readonly sort?: OperationsSortKey;
  readonly transport?: ApiTransport;
}): PaymentOperationsPagedQueryConfig {
  return {
    queryKey: paymentOperationKeys.byPaymentPaged(propertyId, paymentId, status, order, sort),
    queryFn: ({ pageParam }) =>
      fetchPaymentOperationsPagedPage(
        propertyId, paymentId, { status, order, sort }, pageParam, transport,
      ),
    initialPageParam: 0,
    getNextPageParam: operationsOffsetNextPageParam,
  };
}

/** Скоуп глобальной ленты → общая часть query-параметров (ключ даты,
 * объекты, период, направление, архив, поиск); пагинация и порядок — у
 * порции, сводка их не принимает. Общее горло хуков и прогрева хабов #626. */
function operationsScopeParams(scope: GlobalOperationScope): URLSearchParams {
  const params = new URLSearchParams();
  if (scope.sort !== undefined) {
    params.set('sort', scope.sort);
  }
  if (scope.propertyIds !== undefined && scope.propertyIds.length > 0) {
    params.set('propertyIds', scope.propertyIds.join(','));
  }
  if (scope.categories !== undefined && scope.categories.length > 0) {
    params.set('category', scope.categories.join(','));
  }
  if (scope.type !== undefined) {
    params.set('type', scope.type);
  }
  if (scope.includeArchived) {
    params.set('includeArchived', 'true');
  }
  if (scope.dateFrom !== undefined) {
    params.set('date_from', scope.dateFrom);
  }
  if (scope.dateTo !== undefined) {
    params.set('date_to', scope.dateTo);
  }
  if (scope.search !== undefined && scope.search !== '') {
    params.set('search', scope.search);
  }
  return params;
}
