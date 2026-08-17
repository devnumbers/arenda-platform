'use client';

import {
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ApiError } from '@/shared/api/errors';
import { financeKeys, operationKeys, recurringOperationKeys } from '@/shared/api/query-keys';
import type { components } from '@/shared/api/dto';

type RecurringOperationResponse =
  components['schemas']['RecurringOperationResponse'];
type RecurringOperationCreateRequest =
  components['schemas']['RecurringOperationCreateRequest'];
type RecurringOperationUpdateRequest =
  components['schemas']['RecurringOperationUpdateRequest'];
type RecurringOperationsResponse =
  components['schemas']['RecurringOperationsResponse'];

function invalidateOperationLists(queryClient: QueryClient): void {
  queryClient.invalidateQueries({ queryKey: operationKeys.lists() });
  queryClient.invalidateQueries({ queryKey: operationKeys.infiniteLists() });
}

function invalidateRecurringOperationLists(queryClient: QueryClient): void {
  queryClient.invalidateQueries({ queryKey: recurringOperationKeys.lists() });
}

export function useRecurringOperations(): UseQueryResult<
  RecurringOperationsResponse,
  ApiError
> {
  return useQuery({
    queryKey: recurringOperationKeys.recurringOperations(),
    queryFn: () => apiClient<RecurringOperationsResponse>('/recurring-operations'),
  });
}

export function useRecurringOperationsByProperty(
  propertyId: string,
): UseQueryResult<RecurringOperationsResponse, ApiError> {
  return useQuery({
    queryKey: recurringOperationKeys.byProperty(propertyId),
    queryFn: () =>
      apiClient<RecurringOperationsResponse>(
        `/properties/${propertyId}/recurring-operations`,
      ),
    enabled: Boolean(propertyId),
  });
}

export function useRecurringOperation(
  id: string,
): UseQueryResult<RecurringOperationResponse, ApiError> {
  return useQuery({
    queryKey: recurringOperationKeys.detail(id),
    queryFn: () =>
      apiClient<RecurringOperationResponse>(`/recurring-operations/${id}`),
    enabled: Boolean(id),
  });
}

export function useCreateRecurringOperation(): UseMutationResult<
  RecurringOperationResponse,
  ApiError,
  { propertyId: string; data: RecurringOperationCreateRequest }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ propertyId, data }) =>
      apiClient<RecurringOperationResponse>(
        `/properties/${propertyId}/recurring-operations`,
        {
          method: 'POST',
          body: JSON.stringify(data),
        },
      ),
    onSuccess: (_, { propertyId }) => {
      invalidateRecurringOperationLists(queryClient);
      queryClient.invalidateQueries({
        queryKey: recurringOperationKeys.byProperty(propertyId),
      });
    },
  });
}

export function useUpdateRecurringOperation(): UseMutationResult<
  RecurringOperationResponse,
  ApiError,
  { id: string; propertyId?: string; data: RecurringOperationUpdateRequest }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }) =>
      apiClient<RecurringOperationResponse>(`/recurring-operations/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data),
      }),
    onSuccess: (_, { id, propertyId }) => {
      invalidateRecurringOperationLists(queryClient);
      invalidateOperationLists(queryClient);
      queryClient.invalidateQueries({
        queryKey: recurringOperationKeys.detail(id),
      });
      queryClient.invalidateQueries({ queryKey: financeKeys.reports() });
      if (propertyId) {
        queryClient.invalidateQueries({
          queryKey: recurringOperationKeys.byProperty(propertyId),
        });
        queryClient.invalidateQueries({
          queryKey: operationKeys.summary(propertyId),
        });
        queryClient.invalidateQueries({
          queryKey: operationKeys.byProperty(propertyId),
        });
      }
    },
  });
}

export function usePauseRecurringOperation(): UseMutationResult<
  RecurringOperationResponse,
  ApiError,
  { id: string; propertyId: string }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id }) =>
      apiClient<RecurringOperationResponse>(`/recurring-operations/${id}/pause`,
        {
          method: 'POST',
        },
      ),
    onSuccess: (_, { id, propertyId }) => {
      invalidateRecurringOperationLists(queryClient);
      queryClient.invalidateQueries({
        queryKey: recurringOperationKeys.byProperty(propertyId),
      });
      queryClient.invalidateQueries({
        queryKey: recurringOperationKeys.detail(id),
      });
    },
  });
}

export function useResumeRecurringOperation(): UseMutationResult<
  RecurringOperationResponse,
  ApiError,
  { id: string; propertyId: string }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id }) =>
      apiClient<RecurringOperationResponse>(
        `/recurring-operations/${id}/resume`,
        {
          method: 'POST',
        },
      ),
    onSuccess: (_, { id, propertyId }) => {
      invalidateRecurringOperationLists(queryClient);
      queryClient.invalidateQueries({
        queryKey: recurringOperationKeys.byProperty(propertyId),
      });
      queryClient.invalidateQueries({
        queryKey: recurringOperationKeys.detail(id),
      });
    },
  });
}

export function useDeleteRecurringOperation(): UseMutationResult<
  void,
  ApiError,
  { id: string; propertyId?: string }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id }) =>
      apiClient<void>(`/recurring-operations/${id}`, { method: 'DELETE' }),
    onSuccess: (_, { id, propertyId }) => {
      queryClient.removeQueries({
        queryKey: recurringOperationKeys.detail(id),
        exact: true,
      });
      invalidateRecurringOperationLists(queryClient);
      invalidateOperationLists(queryClient);
      if (propertyId) {
        queryClient.invalidateQueries({
          queryKey: recurringOperationKeys.byProperty(propertyId),
        });
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
