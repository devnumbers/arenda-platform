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
import { mapNotificationPreferencesResponse } from '@/entities/user';
import type { NotificationPreference } from '@/entities/user';
import type { components } from '@/shared/api/dto';

type NotificationPreferencesResponse =
  components['schemas']['NotificationPreferencesResponse'];
type NotificationPreferencesUpdateRequest =
  components['schemas']['NotificationPreferencesUpdateRequest'];

// Note: a separate keys.ts here would be ignored by the bare `api` rule in
// the root .gitignore, so the key lives in this file.
export const notificationPreferencesKeys = {
  all: ['notification-preferences'] as const,
};

export function useNotificationPreferences(): UseQueryResult<
  NotificationPreference[],
  ApiError
> {
  return useQuery({
    queryKey: notificationPreferencesKeys.all,
    queryFn: async () => {
      const res = await apiClient<NotificationPreferencesResponse>(
        '/notification-preferences',
      );
      return mapNotificationPreferencesResponse(res);
    },
  });
}

export function useUpdateNotificationPreferences(): UseMutationResult<
  NotificationPreference[],
  ApiError,
  NotificationPreference[]
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (preferences: NotificationPreference[]) => {
      const payload: NotificationPreferencesUpdateRequest = {
        preferences: preferences.map((preference) => ({
          event_type: preference.eventType,
          email_allowed: preference.emailAllowed,
          push_allowed: preference.pushAllowed,
        })),
      };
      const res = await apiClient<NotificationPreferencesResponse>(
        '/notification-preferences',
        {
          method: 'PUT',
          body: JSON.stringify(payload),
        },
      );
      return mapNotificationPreferencesResponse(res);
    },
    onSuccess: (data) => {
      queryClient.setQueryData(notificationPreferencesKeys.all, data);
      queryClient.invalidateQueries({
        queryKey: notificationPreferencesKeys.all,
      });
    },
  });
}
