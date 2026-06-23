'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { operationKeys } from './keys';
import type { components } from '@/shared/api/generated';

type OperationResponse = components['schemas']['OperationResponse'];
type OperationCreateRequest = components['schemas']['OperationCreateRequest'];
type OperationUpdateRequest = components['schemas']['OperationUpdateRequest'];
type OperationsResponse = components['schemas']['OperationsResponse'];
type ReminderCreateRequest = components['schemas']['ReminderCreateRequest'];
type ReminderResponse = components['schemas']['ReminderResponse'];
type RemindersResponse = components['schemas']['RemindersResponse'];

export function useOperationsByProperty(propertyId: string) {
  return useQuery({
    queryKey: operationKeys.byProperty(propertyId),
    queryFn: () =>
      apiClient<OperationsResponse>(`/properties/${propertyId}/operations`),
    enabled: Boolean(propertyId),
  });
}

export function useOperation(id: string) {
  return useQuery({
    queryKey: operationKeys.detail(id),
    queryFn: () => apiClient<OperationResponse>(`/operations/${id}`),
    enabled: Boolean(id),
  });
}

export function useCreateOperation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      propertyId,
      data,
    }: {
      propertyId: string;
      data: OperationCreateRequest;
    }) =>
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

export function useUpdateOperation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: OperationUpdateRequest }) =>
      apiClient<OperationResponse>(`/operations/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data),
      }),
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({ queryKey: operationKeys.detail(id) });
    },
  });
}

export function useDeleteOperation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, propertyId }: { id: string; propertyId: string }) =>
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
) {
  return useQuery({
    queryKey: operationKeys.reminders(propertyId, operationId),
    queryFn: () =>
      apiClient<RemindersResponse>(
        `/properties/${propertyId}/operations/${operationId}/reminders`,
      ),
    enabled: Boolean(propertyId) && Boolean(operationId),
  });
}

export function useCreateOperationReminder() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      propertyId,
      operationId,
      data,
    }: {
      propertyId: string;
      operationId: string;
      data: ReminderCreateRequest;
    }) =>
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
