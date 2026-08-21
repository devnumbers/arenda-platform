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
import type { ApiError } from '@/shared/api/errors';
import { financeKeys, operationKeys, recurringOperationKeys } from '@/shared/api/query-keys';
import type { components } from '@/shared/api/dto';
import { mapRecurringOperationResponse } from '@/entities/operation';
import type {
  RecurringOperation,
  RecurringOperationCreateRequest,
  RecurringOperationUpdateRequest,
} from '@/entities/operation';

type RecurringOperationResponse =
  components['schemas']['RecurringOperationResponse'];
type RecurringOperationCreateWireRequest =
  components['schemas']['RecurringOperationCreateRequest'];
type RecurringOperationUpdateWireRequest =
  components['schemas']['RecurringOperationUpdateRequest'];
type RecurringOperationsResponse =
  components['schemas']['RecurringOperationsResponse'];

// Команды приходят из виджетов в camelCase; wire-формат (snake_case) живёт
// только внутри этого модуля.
export function toCreateWireRequest(
  data: RecurringOperationCreateRequest,
): RecurringOperationCreateWireRequest {
  return {
    type: data.type,
    category_id: data.categoryId,
    name: data.name,
    amount_kopecks: data.amountKopecks,
    start_date: data.startDate,
    payment_day: data.paymentDay,
    end_date: data.endDate,
    comment: data.comment,
    periodicity: data.periodicity,
    reminder_offset_days: data.reminderOffsetDays,
  };
}

export function toUpdateWireRequest(
  data: RecurringOperationUpdateRequest,
): RecurringOperationUpdateWireRequest {
  return {
    type: data.type,
    category_id: data.categoryId,
    name: data.name,
    amount_kopecks: data.amountKopecks,
    start_date: data.startDate,
    payment_day: data.paymentDay,
    end_date: data.endDate,
    comment: data.comment,
    periodicity: data.periodicity,
    apply_from_date: data.applyFromDate,
    reminder_offset_days: data.reminderOffsetDays,
  };
}

function fetchRecurringOperation(id: string): Promise<RecurringOperation> {
  return apiClient<RecurringOperationResponse>(`/recurring-operations/${id}`).then(
    mapRecurringOperationResponse,
  );
}

function invalidateOperationLists(queryClient: QueryClient): void {
  queryClient.invalidateQueries({ queryKey: operationKeys.lists() });
  queryClient.invalidateQueries({ queryKey: operationKeys.infiniteLists() });
}

function invalidateRecurringOperationLists(queryClient: QueryClient): void {
  queryClient.invalidateQueries({ queryKey: recurringOperationKeys.lists() });
}

export function useRecurringOperations(): UseQueryResult<
  RecurringOperation[],
  ApiError
> {
  return useQuery({
    queryKey: recurringOperationKeys.recurringOperations(),
    queryFn: async () => {
      const response = await apiClient<RecurringOperationsResponse>('/recurring-operations');
      return response.items.map(mapRecurringOperationResponse);
    },
  });
}

export function useRecurringOperationsByProperty(
  propertyId: string,
): UseQueryResult<RecurringOperation[], ApiError> {
  return useQuery({
    queryKey: recurringOperationKeys.byProperty(propertyId),
    queryFn: async () => {
      const response = await apiClient<RecurringOperationsResponse>(
        `/properties/${propertyId}/recurring-operations`,
      );
      return response.items.map(mapRecurringOperationResponse);
    },
    enabled: Boolean(propertyId),
  });
}

export function useRecurringOperation(
  id: string,
): UseQueryResult<RecurringOperation, ApiError> {
  return useQuery({
    queryKey: recurringOperationKeys.detail(id),
    queryFn: () => fetchRecurringOperation(id),
    enabled: Boolean(id),
  });
}

export function useCreateRecurringOperation(): UseMutationResult<
  RecurringOperation,
  ApiError,
  { propertyId: string; data: RecurringOperationCreateRequest }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ propertyId, data }) =>
      mapRecurringOperationResponse(
        await apiClient<RecurringOperationResponse>(
          `/properties/${propertyId}/recurring-operations`,
          {
            method: 'POST',
            body: JSON.stringify(toCreateWireRequest(data)),
          },
        ),
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
  RecurringOperation,
  ApiError,
  { id: string; propertyId?: string; data: RecurringOperationUpdateRequest }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, data }) =>
      mapRecurringOperationResponse(
        await apiClient<RecurringOperationResponse>(`/recurring-operations/${id}`, {
          method: 'PATCH',
          body: JSON.stringify(toUpdateWireRequest(data)),
        }),
      ),
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
  RecurringOperation,
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
      ).then(mapRecurringOperationResponse),
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
  RecurringOperation,
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
      ).then(mapRecurringOperationResponse),
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
