'use client';

import { useCallback, useState } from 'react';
import { requestPushPermissionAndSubscribe, type RequestPushPermissionOutcome } from '../lib/request-push';
import { useEnsureSubscriptionTools } from '../lib/subscription-sync';

/**
 * High-level hook that drives the explicit push-enablement flow named in
 * spec #183 (`useSubscribePush`). Returns a trigger (call it from a click
 * handler) and the latest outcome so the caller can show the right copy.
 *
 * Internally composed of {@link useEnsureSubscriptionTools} (VAPID key + POST)
 * and {@link requestPushPermissionAndSubscribe} (permission + subscribe). The
 * split lets `PushPermissionGate` reuse the same tools for the background
 * re-subscribe path without re-triggering the system prompt.
 */
export function useSubscribePush(): {
  readonly subscribe: () => Promise<RequestPushPermissionOutcome>;
  readonly isPending: boolean;
} {
  const { vapidKey, postSubscription } = useEnsureSubscriptionTools();
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
