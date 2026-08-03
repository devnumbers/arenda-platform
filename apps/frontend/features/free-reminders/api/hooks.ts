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
import { freeReminderKeys } from './keys';
import type { components } from '@/shared/api/generated';

type FreeReminderResponse = components['schemas']['FreeReminderResponse'];
type FreeReminderCreateRequest = components['schemas']['FreeReminderCreateRequest'];
type FreeReminderUpdateRequest = components['schemas']['FreeReminderUpdateRequest'];

export function useFreeReminder(id: string): UseQueryResult<FreeReminderResponse, ApiError> {
  return useQuery({
    queryKey: freeReminderKeys.detail(id),
    queryFn: () => apiClient<FreeReminderResponse>(`/free-reminders/${id}`),
    enabled: Boolean(id),
  });
}

export function useCreateFreeReminder(): UseMutationResult<
  FreeReminderResponse,
  ApiError,
  { propertyId: string; data: FreeReminderCreateRequest }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ propertyId, data }) =>
      apiClient<FreeReminderResponse>(`/properties/${propertyId}/free-reminders`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: (_, { propertyId }) => {
      queryClient.invalidateQueries({ queryKey: freeReminderKeys.all });
      queryClient.invalidateQueries({ queryKey: freeReminderKeys.byProperty(propertyId) });
      queryClient.invalidateQueries({ queryKey: freeReminderKeys.upcoming(propertyId) });
    },
  });
}

export function useUpdateFreeReminder(): UseMutationResult<
  FreeReminderResponse,
  ApiError,
  { id: string; data: FreeReminderUpdateRequest }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }) =>
      apiClient<FreeReminderResponse>(`/free-reminders/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data),
      }),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: freeReminderKeys.all });
      queryClient.invalidateQueries({ queryKey: freeReminderKeys.detail(data.id) });
    },
  });
}

export function useDeleteFreeReminder(): UseMutationResult<void, ApiError, string> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id) => apiClient<void>(`/free-reminders/${id}`, { method: 'DELETE' }),
    onSuccess: (_, id) => {
      queryClient.invalidateQueries({ queryKey: freeReminderKeys.all });
      queryClient.removeQueries({ queryKey: freeReminderKeys.detail(id) });
    },
  });
}
