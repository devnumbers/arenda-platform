'use client';

import { useCallback, useState } from 'react';
import { requestPushPermissionAndSubscribe, type RequestPushPermissionOutcome } from '../lib/request-push';
import { usePushSubscriptionTools } from '../lib/push-tools';

/**
 * High-level hook that drives the explicit push-enablement flow named in
 * spec #183 (`useSubscribePush`). Returns a trigger (call it from a click
 * handler) and the latest outcome so the caller can show the right copy.
 *
 * Internally composed of {@link usePushSubscriptionTools} (VAPID key + POST)
 * and {@link requestPushPermissionAndSubscribe} (permission + subscribe).
 * Подписка возникает только из явного действия (спека #1028 §4) — фоновых
 * вызовов у хука нет.
 */
export function useSubscribePush(): {
  readonly subscribe: () => Promise<RequestPushPermissionOutcome>;
  readonly isPending: boolean;
} {
  const { vapidKey, postSubscription } = usePushSubscriptionTools();
  const [isPending, setIsPending] = useState(false);

  const subscribe = useCallback(async (): Promise<RequestPushPermissionOutcome> => {
    setIsPending(true);
    try {
      return await requestPushPermissionAndSubscribe(vapidKey, postSubscription);
    } finally {
      setIsPending(false);
    }
  }, [vapidKey, postSubscription]);

  return { subscribe, isPending };
}
