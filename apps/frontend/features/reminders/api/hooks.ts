'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import { reminderKeys } from './keys';
import type { components } from '@/shared/api/generated';

type ReminderResponse = components['schemas']['ReminderResponse'];
type ReminderUpdateRequest = components['schemas']['ReminderUpdateRequest'];
type RemindersResponse = components['schemas']['RemindersResponse'];

export function useReminders(limit = 100, offset = 0) {
  return useQuery({
    queryKey: reminderKeys.all,
    queryFn: () =>
      apiClient<RemindersResponse>(
        `/reminders?limit=${limit}&offset=${offset}`,
      ),
  });
}

export function useReminder(id: string) {
  return useQuery({
    queryKey: reminderKeys.detail(id),
    queryFn: () => apiClient<ReminderResponse>(`/reminders/${id}`),
    enabled: Boolean(id),
  });
}

export function useUpdateReminder() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      data,
    }: {
      id: string;
      data: ReminderUpdateRequest;
    }) =>
      apiClient<ReminderResponse>(`/reminders/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(data),
      }),
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({ queryKey: reminderKeys.all });
      queryClient.invalidateQueries({ queryKey: reminderKeys.detail(id) });
    },
  });
}

export function useDeleteReminder() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient<void>(`/reminders/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: reminderKeys.all });
    },
  });
}
