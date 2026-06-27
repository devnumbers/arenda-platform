'use client';

import { useMemo } from 'react';
import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { ApiError } from '@/shared/api/errors';
import { operationKeys } from './keys';
import type { components } from '@/shared/api/generated';

type OperationResponse = components['schemas']['OperationResponse'];
type OperationCreateRequest = components['schemas']['OperationCreateRequest'];
type OperationUpdateRequest = components['schemas']['OperationUpdateRequest'];
type OperationsResponse = components['schemas']['OperationsResponse'];
type PropertyOperationsSummaryResponse =
  components['schemas']['PropertyOperationsSummaryResponse'];

type RecurringOperationResponse =
  components['schemas']['RecurringOperationResponse'];
type RecurringOperationCreateRequest =
  components['schemas']['RecurringOperationCreateRequest'];

type ReminderCreateRequest = components['schemas']['ReminderCreateRequest'];
type ReminderResponse = components['schemas']['ReminderResponse'];
type RemindersResponse = components['schemas']['RemindersResponse'];

export type OperationsFilters = {
  type?: 'income' | 'expense';
  status?: string;
  category?: string;
  property_id?: string;
  from?: string;
  to?: string;
  recurring_operation_id?: string;
  limit?: string;
  offset?: string;
};

export function useOperations(filters: OperationsFilters = {}) {
  const queryString = useMemo(() => {
    const params = new URLSearchParams();
    Object.entries(filters).forEach(([key, value]) => {
      if (value) params.set(key, value);
    });
    return params.toString();
  }, [filters]);

  return useQuery({
    queryKey: operationKeys.operations({ ...filters }),
    queryFn: () =>
      apiClient<OperationsResponse>(`/operations${queryString ? `?${queryString}` : ''}`),
  });
}

export function useOperationsByProperty(
  propertyId: string,
): UseQueryResult<OperationsResponse, ApiError> {
  return useQuery({
    queryKey: operationKeys.byProperty(propertyId),
    queryFn: () =>
      apiClient<OperationsResponse>(`/properties/${propertyId}/operations`),
    enabled: Boolean(propertyId),
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
    onSuccess: (_, { id, propertyId }) => {
      queryClient.invalidateQueries({ queryKey: operationKeys.detail(id) });
      queryClient.invalidateQueries({ queryKey: ['operations'] });
      if (propertyId) {
        queryClient.invalidateQueries({ queryKey: operationKeys.byProperty(propertyId) });
        queryClient.invalidateQueries({ queryKey: operationKeys.summary(propertyId) });
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
      queryClient.invalidateQueries({
        queryKey: operationKeys.byProperty(propertyId),
      });
      queryClient.invalidateQueries({
        queryKey: operationKeys.summary(propertyId),
      });
    },
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
      queryClient.invalidateQueries({
        queryKey: operationKeys.recurringByProperty(propertyId),
      });
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
    onSuccess: (_, { id, propertyId }) => {
      queryClient.invalidateQueries({
        queryKey: operationKeys.byProperty(propertyId),
      });
      queryClient.invalidateQueries({ queryKey: operationKeys.detail(id) });
      queryClient.invalidateQueries({
        queryKey: operationKeys.summary(propertyId),
      });
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
      queryClient.invalidateQueries({
        queryKey: operationKeys.byProperty(propertyId),
      });
      queryClient.invalidateQueries({ queryKey: operationKeys.detail(id) });
      queryClient.invalidateQueries({
        queryKey: operationKeys.summary(propertyId),
      });
    },
  });
}

export function useOperationReminders(
  propertyId: string,
  operationId: string,
): UseQueryResult<RemindersResponse, ApiError> {
  return useQuery({
    queryKey: operationKeys.reminders(propertyId, operationId),
    queryFn: () =>
      apiClient<RemindersResponse>(
        `/properties/${propertyId}/operations/${operationId}/reminders`,
      ),
    enabled: Boolean(propertyId) && Boolean(operationId),
  });
}

export function useCreateOperationReminder(): UseMutationResult<
  ReminderResponse,
  ApiError,
  { propertyId: string; operationId: string; data: ReminderCreateRequest }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ propertyId, operationId, data }) =>
      apiClient<ReminderResponse>(
        `/properties/${propertyId}/operations/${operationId}/reminders`,
        {
          method: 'POST',
          body: JSON.stringify(data),
        },
      ),
    onSuccess: (_, { propertyId, operationId }) => {
      queryClient.invalidateQueries({
        queryKey: operationKeys.reminders(propertyId, operationId),
      });
    },
  });
}

export function useCreateRecurringOperationReminder(): UseMutationResult<
  RemindersResponse,
  ApiError,
  {
    propertyId: string;
    recurringOperationId: string;
    data: ReminderCreateRequest;
  }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ propertyId, recurringOperationId, data }) =>
      apiClient<RemindersResponse>(
        `/properties/${propertyId}/recurring-operations/${recurringOperationId}/reminders`,
        {
          method: 'POST',
          body: JSON.stringify(data),
        },
      ),
    onSuccess: (_, { propertyId, recurringOperationId }) => {
      queryClient.invalidateQueries({
        queryKey: operationKeys.recurringReminders(propertyId, recurringOperationId),
      });
    },
  });
}
