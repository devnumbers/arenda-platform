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
import {
  allCategoriesEnabled,
  mapCategoryPreferences,
  type NotificationCategoryPreferences,
} from '@/entities/notification';
import { notificationKeys } from '@/shared/api/query-keys';
import type { components } from '@/shared/api/dto';

type PushPreferencesDto = components['schemas']['PushPreferencesResponse'];
type PushPreferencesRequestDto = components['schemas']['PushPreferencesRequest'];

/** Категорийный дефолт устройства: «всё включено» (решение #738). Мастера в
 * модели нет (спека #1028 §0): строка есть = устройство включено, строки
 * нет = выключено — на этот тип мастеровое состояние не выражается. */
export function defaultPushDevicePreferences(): PushDevicePreferences {
  return { categories: allCategoriesEnabled() };
}

/** Категорийные флаги пуш-канала устройства (решение #738, ADR 0058,
 * спека #1028): ключ устройства — endpoint URL (контракт #743). */
export type PushDevicePreferences = {
  readonly categories: NotificationCategoryPreferences;
};

function mapPushPreferences(dto: PushPreferencesDto): PushDevicePreferences {
  return {
    categories: mapCategoryPreferences(dto.categories),
  };
}

/**
 * Настройки пушей этого устройства (GET /push/subscriptions/preferences,
 * #743). Запрос в воздух не уходит: endpoint появляется, когда в браузере
 * есть живая подписка. Неизвестный бэку endpoint (запись потеряна,
 * аномалия №7 до heal-отписки стартового гейта) — преходящее состояние:
 * категории показываем дефолтные «всё включено», мастеровое состояние
 * рисует сам факт живой подписки.
 */
export function usePushDevicePreferences(
  endpoint: string | undefined,
): UseQueryResult<PushDevicePreferences, ApiError> {
  return useQuery({
    queryKey: notificationKeys.pushPreferences(endpoint ?? ''),
    enabled: endpoint !== undefined,
    queryFn: async () => {
      if (endpoint === undefined) {
        throw new Error('usePushDevicePreferences: нет endpoint подписки');
      }
      const query = new URLSearchParams({ endpoint });
      try {
        const response = await apiClient<PushPreferencesDto>(
          `/push/subscriptions/preferences?${query.toString()}`,
        );
        return mapPushPreferences(response);
      } catch (error) {
        if (error instanceof ApiError && error.status === 404) {
          return defaultPushDevicePreferences();
        }
        throw error;
      }
    },
  });
}

/** Переменные PUT: категории целиком + ключ устройства. Мастер-выключение
 * телом PUT не выражается — это DELETE /push/subscriptions. */
export type UpdatePushDevicePreferencesVars = PushDevicePreferences & {
  readonly endpoint: string;
};

/**
 * Замена настроек устройства (PUT /push/subscriptions/preferences, #743) —
 * оптимистично, как email-матрица (#746): тумблер двигается сразу, при
 * ошибке снимок восстанавливается. Тело несёт только категории (спека
 * #1028 §5); мастер-выключение — жёсткая отписка DELETE (см.
 * useDeletePushSubscription).
 */
export function useUpdatePushDevicePreferences(): UseMutationResult<
  PushDevicePreferences,
  ApiError,
  UpdatePushDevicePreferencesVars
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ endpoint, categories }: UpdatePushDevicePreferencesVars) => {
      const body: PushPreferencesRequestDto = { endpoint, categories };
      const response = await apiClient<PushPreferencesDto>(
        '/push/subscriptions/preferences',
        { method: 'PUT', body: JSON.stringify(body) },
      );
      return mapPushPreferences(response);
    },
    onMutate: async ({ endpoint, categories }) => {
      const queryKey = notificationKeys.pushPreferences(endpoint);
      await queryClient.cancelQueries({ queryKey });
      const previous = queryClient.getQueryData<PushDevicePreferences>(queryKey);
      queryClient.setQueryData(queryKey, { categories });
      return { previous, queryKey };
    },
    onError: (_error, _vars, context) => {
      if (context?.previous !== undefined) {
        queryClient.setQueryData(context.queryKey, context.previous);
      }
    },
    onSuccess: (saved, { endpoint }) => {
      queryClient.setQueryData(notificationKeys.pushPreferences(endpoint), saved);
    },
    onSettled: (_data, _error, { endpoint }) => {
      void queryClient.invalidateQueries({
        queryKey: notificationKeys.pushPreferences(endpoint),
      });
    },
  });
}
