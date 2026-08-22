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
import type { components } from '@/shared/api/dto';
import type { PushSubscriptionPayload } from '../lib/subscribe';

type VapidPublicKeyResponse = components['schemas']['VapidPublicKeyResponse'];
type PushSubscriptionCreateRequest =
  components['schemas']['PushSubscriptionCreateRequest'];
type PushSubscriptionDeleteRequest =
  components['schemas']['PushSubscriptionDeleteRequest'];
type PushSubscriptionResponse =
  components['schemas']['PushSubscriptionResponse'];

// Note: a separate keys.ts here would be ignored by the bare `api` rule in the
// root .gitignore, so the keys live in this file (same pattern as the
// notification-preferences feature).
export const pushSubscriptionKeys = {
  vapid: ['push', 'vapid-public-key'] as const,
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

/** Register (upsert) a push subscription on the backend. Idempotent by endpoint. */
export function useCreatePushSubscription(): UseMutationResult<
  PushSubscriptionResponse,
  ApiError,
  PushSubscriptionPayload
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (payload: PushSubscriptionPayload) => {
      const body: PushSubscriptionCreateRequest = {
        endpoint: payload.endpoint,
        p256dh: payload.p256dh,
        auth: payload.auth,
        expiration_time: payload.expirationTime,
      };
      return apiClient<PushSubscriptionResponse>('/push/subscriptions', {
        method: 'POST',
        body: JSON.stringify(body),
      });
    },
    onSuccess: (data) => {
      queryClient.setQueryData(pushSubscriptionKeys.subscription, data);
    },
  });
}

/** Unregister a push subscription by endpoint. 404 on a missing endpoint is propagated as an error. */
export function useDeletePushSubscription(): UseMutationResult<
  void,
  ApiError,
  string
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (endpoint: string) => {
      const body: PushSubscriptionDeleteRequest = { endpoint };
      await apiClient<void>('/push/subscriptions', {
        method: 'DELETE',
        body: JSON.stringify(body),
      });
    },
    onSuccess: () => {
      queryClient.setQueryData(pushSubscriptionKeys.subscription, null);
      void queryClient.invalidateQueries({ queryKey: pushSubscriptionKeys.subscription });
    },
  });
}
