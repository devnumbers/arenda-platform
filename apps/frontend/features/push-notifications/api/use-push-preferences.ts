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

/** Дефолт устройства (решение #738): мастер включён, все категории включены. */
export function defaultPushDevicePreferences(): PushDevicePreferences {
  return { enabled: true, categories: allCategoriesEnabled() };
}

/** Состояние пуш-канала устройства (решение #738, ADR 0056): мастер-тумблер
 * «Получать пуш-уведомления» и флаги четырёх категорий — на подписке
 * браузера, ключ устройства — endpoint URL (контракт #743). */
export type PushDevicePreferences = {
  readonly enabled: boolean;
  readonly categories: NotificationCategoryPreferences;
};

function mapPushPreferences(dto: PushPreferencesDto): PushDevicePreferences {
  return {
    enabled: dto.enabled,
    categories: mapCategoryPreferences(dto.categories),
  };
}

/**
 * Настройки пушей этого устройства (GET /push/subscriptions/preferences,
 * #743). Запрос в воздух не уходит: endpoint появляется, когда в браузере
 * есть живая подписка. Неизвестный бэку endpoint (запись потеряна) —
 * дефолт «всё включено»: запись пересоздаст фоновый гейт подписки, PUT
 * сохранит состояние.
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

/** Переменные PUT: состояние целиком + ключ устройства. */
export type UpdatePushDevicePreferencesVars = PushDevicePreferences & {
  readonly endpoint: string;
};

/**
 * Замена настроек устройства (PUT /push/subscriptions/preferences, #743) —
 * оптимистично, как email-матрица (#746): тумблер двигается сразу, при
 * ошибке снимок восстанавливается. Выключение мастера — enabled=false:
 * dispatch пропускает устройство, повторное включение мгновенное.
 */
export function useUpdatePushDevicePreferences(): UseMutationResult<
  PushDevicePreferences,
  ApiError,
  UpdatePushDevicePreferencesVars
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ endpoint, enabled, categories }: UpdatePushDevicePreferencesVars) => {
      const body: PushPreferencesRequestDto = { endpoint, enabled, categories };
      const response = await apiClient<PushPreferencesDto>(
        '/push/subscriptions/preferences',
        { method: 'PUT', body: JSON.stringify(body) },
      );
      return mapPushPreferences(response);
    },
    onMutate: async ({ endpoint, enabled, categories }) => {
      const queryKey = notificationKeys.pushPreferences(endpoint);
      await queryClient.cancelQueries({ queryKey });
      const previous = queryClient.getQueryData<PushDevicePreferences>(queryKey);
      queryClient.setQueryData(queryKey, { enabled, categories });
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
