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
import { ApiError } from '@/shared/api/errors';
import { operationKeys } from './keys';
import { financeKeys } from '@/features/finance/api/keys';
import { leaseKeys } from '@/features/leases/api/keys';
import type { components } from '@/shared/api/generated';

type OperationResponse = components['schemas']['OperationResponse'];
type OperationCreateRequest = components['schemas']['OperationCreateRequest'];
type OperationUpdateRequest = components['schemas']['OperationUpdateRequest'];
type OperationListSort = components['schemas']['OperationListSort'];
type OperationsResponse = components['schemas']['OperationsResponse'];
type PropertyOperationsSummaryResponse =
  components['schemas']['PropertyOperationsSummaryResponse'];

export type OperationsFilters = {
  type?: 'income' | 'expense' | ('income' | 'expense')[];
  status?: string | string[];
  category?: string | string[];
  property_id?: string;
  lease_id?: string;
  from?: string;
  to?: string;
  recurring_operation_id?: string;
  sort?: OperationListSort;
  limit?: number;
  offset?: number;
};

function normalizeOperationsFilters(
  filters: OperationsFilters,
): Record<string, string | string[]> {
  const result: Record<string, string | string[]> = {};
  Object.entries(filters).forEach(([key, value]) => {
    if (value === undefined || value === '') {
      return;
    }
    if (typeof value === 'number') {
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

export function useOperations(
  filters: OperationsFilters = {},
  options: { enabled?: boolean } = {},
): UseQueryResult<OperationsResponse, ApiError> {
  const normalized = useMemo(() => normalizeOperationsFilters(filters), [filters]);

  const queryString = useMemo(() => {
    return operationsQueryString(normalized);
  }, [normalized]);

  return useQuery({
    queryKey: operationKeys.operations(normalized),
    queryFn: () =>
      apiClient<OperationsResponse>(`/operations${queryString ? `?${queryString}` : ''}`),
    enabled: options.enabled,
  });
}

export function useOperationsByProperty(
  propertyId: string,
  filters?: Omit<OperationsFilters, 'property_id'>,
): UseQueryResult<OperationsResponse, ApiError> {
  const normalized = useMemo(() => {
    return filters ? normalizeOperationsFilters(filters) : {};
  }, [filters]);

  const queryString = useMemo(() => {
    return operationsQueryString(normalized);
  }, [normalized]);

  return useQuery({
    queryKey: operationKeys.byProperty(propertyId, normalized),
    queryFn: () =>
      apiClient<OperationsResponse>(
        `/properties/${propertyId}/operations${queryString ? `?${queryString}` : ''}`,
      ),
    enabled: Boolean(propertyId),
  });
}

export function useInfiniteOperations(
  filters: Omit<OperationsFilters, 'offset'> = {},
  options: { enabled?: boolean } = {},
): UseInfiniteQueryResult<InfiniteData<OperationsResponse>, ApiError> {
  const normalized = useMemo(() => normalizeOperationsFilters(filters), [filters]);

  return useInfiniteQuery<
    OperationsResponse,
    ApiError,
    InfiniteData<OperationsResponse>,
    ReturnType<typeof operationKeys.infiniteOperations>,
    number
  >({
    queryKey: operationKeys.infiniteOperations(normalized),
    initialPageParam: 0,
    queryFn: ({ pageParam }) => {
      const queryString = operationsQueryString({
        ...normalized,
        offset: String(pageParam),
      });
      return apiClient<OperationsResponse>(
        `/operations${queryString ? `?${queryString}` : ''}`,
      );
    },
    getNextPageParam: (lastPage) => lastPage.next_offset ?? undefined,
    enabled: options.enabled,
  });
}

export function useOperation(
  id: string,
): UseQueryResult<OperationResponse, ApiError> {
  return useQuery({
    queryKey: operationKeys.detail(id),
    queryFn: () => apiClient<OperationResponse>(`/operations/${id}`),
    enabled: Boolean(id),
  });
}

export function usePropertyOperationsSummary(
  propertyId: string,
): UseQueryResult<PropertyOperationsSummaryResponse, ApiError> {
  return useQuery({
    queryKey: operationKeys.summary(propertyId),
    queryFn: () =>
      apiClient<PropertyOperationsSummaryResponse>(
        `/properties/${propertyId}/operations/summary`,
      ),
    enabled: Boolean(propertyId),
  });
}

export function useCompleteOperation(): UseMutationResult<
  OperationResponse,
  ApiError,
  { id: string; propertyId?: string }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id }) =>
      apiClient<OperationResponse>(`/operations/${id}/complete`, { method: 'POST' }),
    onSuccess: (operation, { id, propertyId }) => {
      queryClient.setQueryData(operationKeys.detail(id), operation);
      queryClient.invalidateQueries({ queryKey: operationKeys.detail(id) });
      invalidateOperationLists(queryClient);
      if (propertyId) {
        queryClient.invalidateQueries({ queryKey: operationKeys.byProperty(propertyId) });
        queryClient.invalidateQueries({ queryKey: operationKeys.summary(propertyId) });
      }
      if (operation.lease_id && operation.category === 'rent') {
        queryClient.invalidateQueries({ queryKey: leaseKeys.all });
      }
    },
  });
}

export function useMarkOperationIncomplete(): UseMutationResult<
  OperationResponse,
  ApiError,
  { id: string; propertyId?: string }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id }) =>
      apiClient<OperationResponse>(`/operations/${id}/mark-incomplete`, {
        method: 'POST',
      }),
    onSuccess: (operation, { id, propertyId }) => {
      queryClient.setQueryData(operationKeys.detail(id), operation);
      queryClient.invalidateQueries({ queryKey: operationKeys.detail(id) });
      invalidateOperationLists(queryClient);
      if (propertyId) {
        queryClient.invalidateQueries({ queryKey: operationKeys.byProperty(propertyId) });
        queryClient.invalidateQueries({ queryKey: operationKeys.summary(propertyId) });
      }
      if (operation.lease_id && operation.category === 'rent') {
        queryClient.invalidateQueries({ queryKey: leaseKeys.all });
      }
    },
  });
}

export function useCreateOperation(): UseMutationResult<
  OperationResponse,
  ApiError,
  { propertyId: string; data: OperationCreateRequest }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ propertyId, data }) =>
      apiClient<OperationResponse>(`/properties/${propertyId}/operations`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
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
  OperationResponse,
  ApiError,
  { id: string; propertyId: string; data: OperationUpdateRequest }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }) =>
      apiClient<OperationResponse>(`/operations/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data),
      }),
    onSuccess: (operation, { id, propertyId }) => {
      queryClient.setQueryData(operationKeys.detail(id), operation);
      invalidateOperationLists(queryClient);
      queryClient.invalidateQueries({
        queryKey: operationKeys.byProperty(propertyId),
      });
      queryClient.invalidateQueries({ queryKey: operationKeys.detail(id) });
      queryClient.invalidateQueries({
        queryKey: operationKeys.summary(propertyId),
      });
      if (operation.lease_id && operation.category === 'rent') {
        queryClient.invalidateQueries({ queryKey: leaseKeys.all });
      }
    },
  });
}

export function useDeleteOperation(): UseMutationResult<
  void,
  ApiError,
  { id: string; propertyId: string }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id }) =>
      apiClient<void>(`/operations/${id}`, { method: 'DELETE' }),
    onSuccess: (_, { id, propertyId }) => {
      queryClient.removeQueries({ queryKey: operationKeys.detail(id), exact: true });
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
