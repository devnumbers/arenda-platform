'use client';

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
type ReminderCreateRequest = components['schemas']['ReminderCreateRequest'];
type ReminderResponse = components['schemas']['ReminderResponse'];
type RemindersResponse = components['schemas']['RemindersResponse'];

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
    onSuccess: (_, { propertyId }) => {
      queryClient.invalidateQueries({
        queryKey: operationKeys.byProperty(propertyId),
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
