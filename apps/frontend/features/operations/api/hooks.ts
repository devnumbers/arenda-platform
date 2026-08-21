'use client';

import { useMemo } from 'react';
import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
  type InfiniteData,
  type QueryClient,
  type UseMutationResult,
  type UseInfiniteQueryResult,
  type UseQueryResult,
} from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import { categoryKeys, financeKeys, leaseKeys, operationKeys } from '@/shared/api/query-keys';
import type { components } from '@/shared/api/dto';
import { mapOperationResponse, mapOperationsResponse } from '@/entities/operation';
import type {
  Operation,
  OperationCreateRequest,
  OperationUpdateRequest,
  OperationsPage,
} from '@/entities/operation';
import { mapPropertyOperationsSummaryResponse } from '@/entities/property';
import type { PropertyOperationsSummary } from '@/entities/property';

type OperationResponse = components['schemas']['OperationResponse'];
type OperationCreateWireRequest = components['schemas']['OperationCreateRequest'];
type OperationUpdateWireRequest = components['schemas']['OperationUpdateRequest'];
export type OperationListSort = 'operation_date_desc' | 'operation_date_asc';
type OperationsResponse = components['schemas']['OperationsResponse'];
type PropertyOperationsSummaryResponse =
  components['schemas']['PropertyOperationsSummaryResponse'];

export type OperationsFilters = {
  type?: 'income' | 'expense' | ('income' | 'expense')[];
  status?: string | string[];
  category_id?: string | string[];
  property_id?: string;
  lease_id?: string;
  from?: string;
  to?: string;
  recurring_operation_id?: string;
  sort?: OperationListSort;
  limit?: number;
  offset?: number;
  exclude_archived_properties?: boolean;
};

// Rent operations affect lease state, so lease keys must be invalidated when a
// lease-linked rent operation changes. The rent category id lives in the
// categories cache; if it is not loaded yet, fall back to invalidating lease
// keys for any lease-linked operation change (safe over-invalidation).
function shouldInvalidateLeaseKeys(
  queryClient: QueryClient,
  operation: Pick<Operation, 'leaseId' | 'categoryId'>,
): boolean {
  if (!operation.leaseId) {
    return false;
  }
  const incomeCategories = queryClient.getQueryData<components['schemas']['OperationCategory'][]>(
    categoryKeys.list('income'),
  );
  if (!incomeCategories) {
    return true;
  }
  // Rent operations affect lease state. A member may have multiple rent
  // categories (own + each shared-access owner's), so check membership across
  // all of them rather than the first find() — otherwise a rent operation on a
  // shared object (owner's rent category) would not invalidate lease keys.
  const rentCategoryIds = incomeCategories
    .filter((category) => category.code === 'rent')
    .map((category) => category.id);
  return (
    rentCategoryIds.length > 0 &&
    rentCategoryIds.includes(operation.categoryId)
  );
}

function normalizeOperationsFilters(
  filters: OperationsFilters,
): Record<string, string | string[]> {
  const result: Record<string, string | string[]> = {};
  Object.entries(filters).forEach(([key, value]) => {
    if (value === undefined || value === '') {
      return;
    }
    if (typeof value === 'number' || typeof value === 'boolean') {
      result[key] = String(value);
      return;
    }
    if (Array.isArray(value)) {
      const filtered = value.filter((v) => v !== '');
      if (filtered.length > 0) {
        result[key] = filtered;
      }
      return;
    }
    result[key] = value;
  });
  return result;
}

function operationsQueryString(filters: Record<string, string | string[]>): string {
  const params = new URLSearchParams();
  Object.entries(filters).forEach(([key, value]) => {
    if (Array.isArray(value)) {
      value.forEach((v) => params.append(key, v));
    } else {
      params.set(key, value);
    }
  });
  return params.toString();
}

function invalidateOperationLists(queryClient: QueryClient): void {
  queryClient.invalidateQueries({ queryKey: operationKeys.lists() });
  queryClient.invalidateQueries({ queryKey: operationKeys.infiniteLists() });
  queryClient.invalidateQueries({ queryKey: financeKeys.reports() });
}

// Команды приходят из виджетов в camelCase; wire-формат (snake_case) живёт
// только внутри этого модуля.
export function toCreateWireRequest(
  data: OperationCreateRequest,
): OperationCreateWireRequest {
  return {
    type: data.type,
    category_id: data.categoryId,
    name: data.name,
    amount_kopecks: data.amountKopecks,
    operation_date: data.operationDate,
    comment: data.comment,
    lease_id: data.leaseId,
    reminder_offset_days: data.reminderOffsetDays,
  };
}

export function toUpdateWireRequest(
  data: OperationUpdateRequest,
): OperationUpdateWireRequest {
  return {
    type: data.type,
    category_id: data.categoryId,
    name: data.name,
    amount_kopecks: data.amountKopecks,
    operation_date: data.operationDate,
    comment: data.comment,
    lease_id: data.leaseId,
    reminder_offset_days: data.reminderOffsetDays,
  };
}

export function useOperations(
  filters: OperationsFilters = {},
  options: { enabled?: boolean } = {},
): UseQueryResult<OperationsPage, ApiError> {
  const normalized = useMemo(() => normalizeOperationsFilters(filters), [filters]);

  const queryString = useMemo(() => {
    return operationsQueryString(normalized);
  }, [normalized]);

  return useQuery({
    queryKey: operationKeys.operations(normalized),
    queryFn: async () =>
      mapOperationsResponse(
        await apiClient<OperationsResponse>(
          `/operations${queryString ? `?${queryString}` : ''}`,
        ),
      ),
    enabled: options.enabled,
  });
}

export function useOperationsByProperty(
  propertyId: string,
  filters?: Omit<OperationsFilters, 'property_id'>,
): UseQueryResult<OperationsPage, ApiError> {
  const normalized = useMemo(() => {
    return filters ? normalizeOperationsFilters(filters) : {};
  }, [filters]);

  const queryString = useMemo(() => {
    return operationsQueryString(normalized);
  }, [normalized]);

  return useQuery({
    queryKey: operationKeys.byProperty(propertyId, normalized),
    queryFn: async () =>
      mapOperationsResponse(
        await apiClient<OperationsResponse>(
          `/properties/${propertyId}/operations${queryString ? `?${queryString}` : ''}`,
        ),
      ),
    enabled: Boolean(propertyId),
  });
}

export function useInfiniteOperations(
  filters: Omit<OperationsFilters, 'offset'> = {},
  options: { enabled?: boolean } = {},
): UseInfiniteQueryResult<InfiniteData<OperationsPage>, ApiError> {
  const normalized = useMemo(() => normalizeOperationsFilters(filters), [filters]);

  return useInfiniteQuery<
    OperationsPage,
    ApiError,
    InfiniteData<OperationsPage>,
    ReturnType<typeof operationKeys.infiniteOperations>,
    number
  >({
    queryKey: operationKeys.infiniteOperations(normalized),
    initialPageParam: 0,
    queryFn: async ({ pageParam }) => {
      const queryString = operationsQueryString({
        ...normalized,
        offset: String(pageParam),
      });
      return mapOperationsResponse(
        await apiClient<OperationsResponse>(
          `/operations${queryString ? `?${queryString}` : ''}`,
        ),
      );
    },
    getNextPageParam: (lastPage) => lastPage.nextOffset ?? undefined,
    enabled: options.enabled,
  });
}

export function useOperation(
  id: string,
): UseQueryResult<Operation, ApiError> {
  return useQuery({
    queryKey: operationKeys.detail(id),
    queryFn: async () =>
      mapOperationResponse(await apiClient<OperationResponse>(`/operations/${id}`)),
    enabled: Boolean(id),
  });
}

export function usePropertyOperationsSummary(
  propertyId: string,
): UseQueryResult<PropertyOperationsSummary, ApiError> {
  return useQuery({
    queryKey: operationKeys.summary(propertyId),
    queryFn: async () =>
      mapPropertyOperationsSummaryResponse(
        await apiClient<PropertyOperationsSummaryResponse>(
          `/properties/${propertyId}/operations/summary`,
        ),
      ),
    enabled: Boolean(propertyId),
  });
}

const PROPERTY_OPERATIONS_PAGE_LIMIT = 100;

/**
 * Вытягивает все постраничные операции объекта агрегированной entity-страницей
 * (wire-формат не покидает этот модуль).
 */
export async function fetchAllPropertyOperations(
  propertyId: string,
): Promise<OperationsPage> {
  const items: Operation[] = [];
  let offset = 0;

  for (;;) {
    const params = new URLSearchParams({
      limit: String(PROPERTY_OPERATIONS_PAGE_LIMIT),
      offset: String(offset),
    });
    const page = mapOperationsResponse(
      await apiClient<OperationsResponse>(
        `/properties/${propertyId}/operations?${params.toString()}`,
      ),
    );
    items.push(...page.items);

    if (!page.hasMore || page.nextOffset === undefined) {
      break;
    }
    offset = page.nextOffset;
  }

  return {
    items,
    limit: PROPERTY_OPERATIONS_PAGE_LIMIT,
    offset: 0,
    hasMore: false,
    nextOffset: undefined,
  };
}

export function useCompleteOperation(): UseMutationResult<
  Operation,
  ApiError,
  { id: string; propertyId?: string }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ id }) =>
      mapOperationResponse(
        await apiClient<OperationResponse>(`/operations/${id}/complete`, { method: 'POST' }),
      ),
    onSuccess: (operation, { id, propertyId }) => {
      queryClient.setQueryData(operationKeys.detail(id), operation);
      queryClient.invalidateQueries({ queryKey: operationKeys.detail(id) });
      invalidateOperationLists(queryClient);
      if (propertyId) {
        queryClient.invalidateQueries({ queryKey: operationKeys.byProperty(propertyId) });
        queryClient.invalidateQueries({ queryKey: operationKeys.summary(propertyId) });
      }
      if (shouldInvalidateLeaseKeys(queryClient, operation)) {
        queryClient.invalidateQueries({ queryKey: leaseKeys.all });
      }
    },
  });
}

export function useMarkOperationIncomplete(): UseMutationResult<
  Operation,
  ApiError,
  { id: string; propertyId?: string }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ id }) =>
      mapOperationResponse(
        await apiClient<OperationResponse>(`/operations/${id}/mark-incomplete`, {
          method: 'POST',
        }),
      ),
    onSuccess: (operation, { id, propertyId }) => {
      queryClient.setQueryData(operationKeys.detail(id), operation);
      queryClient.invalidateQueries({ queryKey: operationKeys.detail(id) });
      invalidateOperationLists(queryClient);
      if (propertyId) {
        queryClient.invalidateQueries({ queryKey: operationKeys.byProperty(propertyId) });
        queryClient.invalidateQueries({ queryKey: operationKeys.summary(propertyId) });
      }
      if (shouldInvalidateLeaseKeys(queryClient, operation)) {
        queryClient.invalidateQueries({ queryKey: leaseKeys.all });
      }
    },
  });
}

export function useCreateOperation(): UseMutationResult<
  Operation,
  ApiError,
  { propertyId: string; data: OperationCreateRequest }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ propertyId, data }) =>
      mapOperationResponse(
        await apiClient<OperationResponse>(`/properties/${propertyId}/operations`, {
          method: 'POST',
          body: JSON.stringify(toCreateWireRequest(data)),
        }),
      ),
    onSuccess: (_, { propertyId }) => {
      invalidateOperationLists(queryClient);
      queryClient.invalidateQueries({
        queryKey: operationKeys.byProperty(propertyId),
      });
      queryClient.invalidateQueries({
        queryKey: operationKeys.summary(propertyId),
      });
    },
  });
}

export function useUpdateOperation(): UseMutationResult<
  Operation,
  ApiError,
  { id: string; propertyId?: string; data: OperationUpdateRequest }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, data }) =>
      mapOperationResponse(
        await apiClient<OperationResponse>(`/operations/${id}`, {
          method: 'PATCH',
          body: JSON.stringify(toUpdateWireRequest(data)),
        }),
      ),
    onSuccess: (operation, { id, propertyId }) => {
      queryClient.setQueryData(operationKeys.detail(id), operation);
      invalidateOperationLists(queryClient);
      queryClient.invalidateQueries({ queryKey: operationKeys.detail(id) });
      if (propertyId) {
        queryClient.invalidateQueries({
          queryKey: operationKeys.byProperty(propertyId),
        });
        queryClient.invalidateQueries({
          queryKey: operationKeys.summary(propertyId),
        });
      }
      if (shouldInvalidateLeaseKeys(queryClient, operation)) {
        queryClient.invalidateQueries({ queryKey: leaseKeys.all });
      }
    },
  });
}

export function useDeleteOperation(): UseMutationResult<
  void,
  ApiError,
  { id: string; propertyId?: string }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id }) =>
      apiClient<void>(`/operations/${id}`, { method: 'DELETE' }),
    onSuccess: (_, { id, propertyId }) => {
      queryClient.removeQueries({ queryKey: operationKeys.detail(id), exact: true });
      invalidateOperationLists(queryClient);
      if (propertyId) {
        queryClient.invalidateQueries({
          queryKey: operationKeys.byProperty(propertyId),
        });
        queryClient.invalidateQueries({
          queryKey: operationKeys.summary(propertyId),
        });
      }
    },
  });
}
