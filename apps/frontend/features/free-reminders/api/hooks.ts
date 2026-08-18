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
import { freeReminderKeys } from '@/shared/api/query-keys';
import type { components } from '@/shared/api/dto';
import { mapFreeReminderResponse, mapUpcomingFreeReminderResponse } from '../model/mappers';
import type {
  FreeReminder,
  FreeReminderCreateRequest,
  FreeReminderUpdateRequest,
  UpcomingFreeReminder,
} from '../model/types';

type FreeReminderResponse = components['schemas']['FreeReminderResponse'];
type FreeReminderCreateWireRequest = components['schemas']['FreeReminderCreateRequest'];
type FreeReminderUpdateWireRequest = components['schemas']['FreeReminderUpdateRequest'];
type UpcomingFreeRemindersResponse = components['schemas']['UpcomingFreeRemindersResponse'];

// Команды приходят из виджетов в camelCase; wire-формат (snake_case) живёт
// только внутри этого модуля.
export function toCreateWireRequest(
  data: FreeReminderCreateRequest,
): FreeReminderCreateWireRequest {
  return {
    title: data.title,
    trigger_at: data.triggerAt,
    periodicity: data.periodicity,
  };
}

export function toUpdateWireRequest(
  data: FreeReminderUpdateRequest,
): FreeReminderUpdateWireRequest {
  return {
    title: data.title,
    trigger_at: data.triggerAt,
    periodicity: data.periodicity,
  };
}

export function useFreeReminder(id: string): UseQueryResult<FreeReminder, ApiError> {
  return useQuery({
    queryKey: freeReminderKeys.detail(id),
    queryFn: () =>
      apiClient<FreeReminderResponse>(`/free-reminders/${id}`).then(
        mapFreeReminderResponse,
      ),
    enabled: Boolean(id),
  });
}

export function useUpcomingFreeReminders(
  propertyId: string,
): UseQueryResult<UpcomingFreeReminder[], ApiError> {
  return useQuery({
    queryKey: freeReminderKeys.upcoming(propertyId),
    queryFn: async () => {
      const response = await apiClient<UpcomingFreeRemindersResponse>(
        `/properties/${propertyId}/free-reminders/upcoming?limit=3`,
      );
      return response.items.map(mapUpcomingFreeReminderResponse);
    },
    enabled: Boolean(propertyId),
  });
}

export function useCreateFreeReminder(): UseMutationResult<
  FreeReminder,
  ApiError,
  { propertyId: string; data: FreeReminderCreateRequest }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ propertyId, data }) =>
      apiClient<FreeReminderResponse>(`/properties/${propertyId}/free-reminders`, {
        method: 'POST',
        body: JSON.stringify(toCreateWireRequest(data)),
      }).then(mapFreeReminderResponse),
    onSuccess: (_, { propertyId }) => {
      queryClient.invalidateQueries({ queryKey: freeReminderKeys.all });
      queryClient.invalidateQueries({ queryKey: freeReminderKeys.byProperty(propertyId) });
      queryClient.invalidateQueries({ queryKey: freeReminderKeys.upcoming(propertyId) });
    },
  });
}

export function useUpdateFreeReminder(): UseMutationResult<
  FreeReminder,
  ApiError,
  { id: string; data: FreeReminderUpdateRequest }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }) =>
      apiClient<FreeReminderResponse>(`/free-reminders/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(toUpdateWireRequest(data)),
      }).then(mapFreeReminderResponse),
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
