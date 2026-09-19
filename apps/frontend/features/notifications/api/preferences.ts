'use client';

import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query';
import { apiClient } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import {
  mapCategoryPreferences,
  type NotificationCategoryPreferences,
} from '@/entities/notification';
import { notificationKeys } from '@/shared/api/query-keys';
import type { components } from '@/shared/api/dto';

type PreferencesDto = components['schemas']['NotificationPreferencesResponse'];
type PreferencesRequestDto =
  components['schemas']['NotificationPreferencesRequest'];

/**
 * Матрица email-настроек аккаунта (GET /notification-preferences, #743):
 * одна конфигурация на все устройства, нет строки — всё включено (решение
 * #738, ADR 0056). Тариф и Системные бэк не отдаёт — они всегда включены.
 */
export function useEmailNotificationPreferences(): UseQueryResult<
  NotificationCategoryPreferences,
  ApiError
> {
  return useQuery({
    queryKey: notificationKeys.emailPreferences(),
    queryFn: async () => {
      const response = await apiClient<PreferencesDto>('/notification-preferences');
      return mapCategoryPreferences(response.email);
    },
  });
}

/**
 * Замена email-матрицы (PUT /notification-preferences, #743) —
 * оптимистично: тумблер двигается сразу, при ошибке снимок
 * восстанавливается (требование #746). Серверный ответ — истина:
 * onSuccess кладёт его в кэш, onSettled перечитывает.
 */
export function useUpdateEmailPreferences(): UseMutationResult<
  NotificationCategoryPreferences,
  ApiError,
  NotificationCategoryPreferences
> {
  const queryClient = useQueryClient();
  const queryKey = notificationKeys.emailPreferences();
  return useMutation({
    mutationFn: async (email: NotificationCategoryPreferences) => {
      const body: PreferencesRequestDto = { email };
      const response = await apiClient<PreferencesDto>('/notification-preferences', {
        method: 'PUT',
        body: JSON.stringify(body),
      });
      return mapCategoryPreferences(response.email);
    },
    onMutate: async (next) => {
      await queryClient.cancelQueries({ queryKey });
      const previous = queryClient.getQueryData<NotificationCategoryPreferences>(queryKey);
      queryClient.setQueryData(queryKey, next);
      return { previous };
    },
    onError: (_error, _next, context) => {
      if (context?.previous !== undefined) {
        queryClient.setQueryData(queryKey, context.previous);
      }
    },
    onSuccess: (saved) => {
      queryClient.setQueryData(queryKey, saved);
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey });
    },
  });
}
