'use client';

import {
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query';
import { useGuardedMutation } from '@/shared/lib/hooks/use-guarded-mutation';
import { apiClient } from '@/shared/api/client';
import type { ApiError } from '@/shared/api/errors';
import { notificationKeys } from '@/shared/api/query-keys';
import type { components } from '@/shared/api/dto';
import type { NotificationCategoryPreferences } from '@/entities/notification';
import { getBrowserSubscription } from '../lib/browser-subscription';
import { isPushSupported } from '../lib/platform';
import type { PushSubscriptionPayload } from '../lib/subscribe';

type VapidPublicKeyResponse = components['schemas']['VapidPublicKeyResponse'];
type PushSubscriptionCreateRequest =
  components['schemas']['PushSubscriptionCreateRequest'];
type PushSubscriptionDeleteRequest =
  components['schemas']['PushSubscriptionDeleteRequest'];
type PushSubscriptionResponse =
  components['schemas']['PushSubscriptionResponse'];

// Note: a separate keys.ts here would be ignored by the bare `api` rule in the
// root .gitignore, so the keys live in this file.
export const pushSubscriptionKeys = {
  vapid: ['push', 'vapid-public-key'] as const,
  /** Проба подписки браузера: данные — endpoint строка или null. Один ключ
   * для всех потребителей пробы: heal-отписка и авто-подписка стартового
   * гейта обновляют экран настроек и тост-гейт мгновенно (слайс 2, #1038). */
  subscription: ['push', 'subscription'] as const,
};

/**
 * Fetch the server VAPID public key (base64url, RFC 8292). The key is static
 * for the lifetime of a deployment — rotating it invalidates every existing
 * subscription — so it is cached indefinitely (staleTime: Infinity).
 */
export function useVapidPublicKey(): UseQueryResult<string, ApiError> {
  return useQuery({
    queryKey: pushSubscriptionKeys.vapid,
    queryFn: async () => {
      const res = await apiClient<VapidPublicKeyResponse>('/push/vapid-public-key');
      return res.public_key;
    },
    staleTime: Infinity,
    gcTime: Infinity,
  });
}

/**
 * Проба подписки браузера: endpoint живой PushSubscription или null. Ключ —
 * тот же, что пишут мутации подписки: любое изменение подписки (heal,
 * авто-промпт, мастер-выключение экрана) инвалидирует одну запись — все
 * потребители пробы честны без перезагрузки. Ошибка пробы (SW не
 * зарегистрировался) — деградация «подписки нет», не ретраится.
 */
export function usePushSubscriptionProbe(options: {
  readonly enabled?: boolean;
} = {}): UseQueryResult<string | null> {
  return useQuery({
    queryKey: pushSubscriptionKeys.subscription,
    enabled: options.enabled ?? true,
    queryFn: async () => {
      if (!isPushSupported()) {
        return null;
      }
      try {
        return (await getBrowserSubscription())?.endpoint ?? null;
      } catch {
        return null;
      }
    },
  });
}

/** Переменные POST: тело подписки + категории, которые фронт шлёт явно
 * (спека #1028 §5; omitted-дефолт «все ВКЛ» остаётся чужим клиентам). */
export type CreatePushSubscriptionVars = PushSubscriptionPayload & {
  readonly categories?: NotificationCategoryPreferences;
};

/** Register (upsert) a push subscription on the backend. Idempotent by endpoint. */
export function useCreatePushSubscription(): UseMutationResult<
  PushSubscriptionResponse,
  ApiError,
  CreatePushSubscriptionVars
> {
  const queryClient = useQueryClient();
  return useGuardedMutation({
    mutationFn: async ({ categories, ...payload }: CreatePushSubscriptionVars) => {
      const body: PushSubscriptionCreateRequest = {
        endpoint: payload.endpoint,
        p256dh: payload.p256dh,
        auth: payload.auth,
        expiration_time: payload.expirationTime,
        ...(categories === undefined ? {} : { categories }),
      };
      return apiClient<PushSubscriptionResponse>('/push/subscriptions', {
        method: 'POST',
        body: JSON.stringify(body),
      });
    },
    onSuccess: (_data, { endpoint }) => {
      queryClient.setQueryData(pushSubscriptionKeys.subscription, endpoint);
    },
  });
}

/** Unregister a push subscription by endpoint. Idempotent since #1028: 204 both for an existing and a missing row. */
export function useDeletePushSubscription(): UseMutationResult<
  void,
  ApiError,
  string
> {
  const queryClient = useQueryClient();
  return useGuardedMutation({
    mutationFn: async (endpoint: string) => {
      const body: PushSubscriptionDeleteRequest = { endpoint };
      await apiClient<void>('/push/subscriptions', {
        method: 'DELETE',
        body: JSON.stringify(body),
      });
    },
    onSuccess: (_data, endpoint) => {
      queryClient.setQueryData(pushSubscriptionKeys.subscription, null);
      void queryClient.invalidateQueries({ queryKey: pushSubscriptionKeys.subscription });
      void queryClient.invalidateQueries({
        queryKey: notificationKeys.pushPreferences(endpoint),
      });
    },
  });
}
