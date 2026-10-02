'use client';

import { useCallback } from 'react';
import type { NotificationCategoryPreferences } from '@/entities/notification';
import { requestPushPermissionAndSubscribe, type RequestPushPermissionOutcome } from '../lib/request-push';
import { usePushSubscriptionTools } from '../lib/push-tools';

/**
 * High-level hook that drives the explicit push-enablement flow named in
 * spec #183 (`useSubscribePush`). Returns a trigger (call it from a click
 * handler); the caller reacts to the outcome (тост успеха/ошибки, красный
 * слот блокировки).
 *
 * Internally composed of {@link usePushSubscriptionTools} (VAPID key + POST)
 * and {@link requestPushPermissionAndSubscribe} (permission + subscribe).
 * Категории первого POST несёт вызывающий — клик мастера шлёт все ВКЛ,
 * клик категории — одну кликнутую ВКЛ (спека #1028 §2, слайс 3 #1039).
 * Подписка возникает только из явного действия (спека #1028 §4) — фоновых
 * вызовов у хука нет.
 */
export function useSubscribePush(): {
  readonly subscribe: (
    categories?: NotificationCategoryPreferences,
  ) => Promise<RequestPushPermissionOutcome>;
} {
  const { vapidKey, postSubscription } = usePushSubscriptionTools();

  const subscribe = useCallback(
    async (
      categories?: NotificationCategoryPreferences,
    ): Promise<RequestPushPermissionOutcome> =>
      requestPushPermissionAndSubscribe(vapidKey, postSubscription, categories),
    [vapidKey, postSubscription],
  );

  return { subscribe };
}
