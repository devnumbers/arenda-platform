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
import { reminderKeys } from './keys';
import type { components } from '@/shared/api/generated';

type ReminderResponse = components['schemas']['ReminderResponse'];
type ReminderUpdateRequest = components['schemas']['ReminderUpdateRequest'];
type RemindersResponse = components['schemas']['RemindersResponse'];
type CalendarRemindersResponse = components['schemas']['CalendarRemindersResponse'];

// Календарь напоминаний — первый потребитель «существующего хука напоминаний».
// from/to — локальные даты 'YYYY-MM-DD' (полуоткрытый диапазон [from, to)).
export function useCalendarReminders(
  from: string,
  to: string,
): UseQueryResult<CalendarRemindersResponse, ApiError> {
  return useQuery({
    queryKey: reminderKeys.calendar(from, to),
    queryFn: () =>
      apiClient<CalendarRemindersResponse>(
        `/reminders/calendar?from=${from}&to=${to}`,
      ),
    enabled: Boolean(from && to),
  });
}

export function useReminders(
  limit = 100,
  offset = 0,
): UseQueryResult<RemindersResponse, ApiError> {
  return useQuery({
    queryKey: reminderKeys.list(limit, offset),
    queryFn: () =>
      apiClient<RemindersResponse>(
        `/reminders?limit=${limit}&offset=${offset}`,
      ),
  });
}

export function useReminder(
  id: string,
): UseQueryResult<ReminderResponse, ApiError> {
  return useQuery({
    queryKey: reminderKeys.detail(id),
    queryFn: () => apiClient<ReminderResponse>(`/reminders/${id}`),
    enabled: Boolean(id),
  });
}

export function useUpdateReminder(): UseMutationResult<
  ReminderResponse,
  ApiError,
  { id: string; data: ReminderUpdateRequest }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }) =>
      apiClient<ReminderResponse>(`/reminders/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: reminderKeys.all });
    },
  });
}

export function useDeleteReminder(): UseMutationResult<
  void,
  ApiError,
  string
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id) =>
      apiClient<void>(`/reminders/${id}`, { method: 'DELETE' }),
    onSuccess: (_, id) => {
      queryClient.invalidateQueries({ queryKey: reminderKeys.all });
      queryClient.invalidateQueries({ queryKey: reminderKeys.detail(id) });
    },
  });
}
