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
import { authKeys } from '@/features/auth/api/keys';
import {
  mapMeResponse,
  mapNotificationPreferencesResponse,
} from '@/entities/user/model/mappers';
import type {
  User,
  UserUpdateCommand,
  SendPhoneChangeCodeCommand,
  ChangePhoneCommand,
  NotificationPreference,
} from '@/entities/user/model/types';
import type { components } from '@/shared/api/generated';

type NotificationPreferencesResponse =
  components['schemas']['NotificationPreferencesResponse'];
type NotificationPreferencesUpdateRequest =
  components['schemas']['NotificationPreferencesUpdateRequest'];

// Note: a separate keys.ts here would be ignored by the bare `api` rule in
// the root .gitignore, so the key lives in this tracked file.
export const notificationPreferencesKeys = {
  all: ['notification-preferences'] as const,
};

export function useUpdateMe(): UseMutationResult<
  User,
  ApiError,
  UserUpdateCommand
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (data: UserUpdateCommand) => {
      const res = await apiClient<Parameters<typeof mapMeResponse>[0]>('/me', {
        method: 'PATCH',
        body: JSON.stringify(data),
      });
      return mapMeResponse(res);
    },
    onSuccess: (data) => {
      queryClient.setQueryData(authKeys.me, data);
      queryClient.invalidateQueries({ queryKey: authKeys.all });
    },
  });
}

export function useChangePhoneSendCode(): UseMutationResult<
  void,
  ApiError,
  SendPhoneChangeCodeCommand
> {
  return useMutation({
    mutationFn: async ({ phone }: SendPhoneChangeCodeCommand) => {
      await apiClient<void>('/me/phone/send-code', {
        method: 'POST',
        body: JSON.stringify({ phone }),
      });
    },
  });
}

export function useChangePhone(): UseMutationResult<
  User,
  ApiError,
  ChangePhoneCommand
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ phone, code }: ChangePhoneCommand) => {
      const res = await apiClient<Parameters<typeof mapMeResponse>[0]>(
        '/me/phone/change',
        {
          method: 'POST',
          body: JSON.stringify({ phone, code }),
        },
      );
      return mapMeResponse(res);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: authKeys.all });
    },
  });
}

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
          allowed: preference.allowed,
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
