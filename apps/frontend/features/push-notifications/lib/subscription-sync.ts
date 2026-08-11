'use client';

import { useCallback } from 'react';
import { useCreatePushSubscription, useVapidPublicKey } from '../api/hooks';
import {
  hasRequiredKeys,
  subscribeToPush,
  subscriptionToPayload,
} from './subscribe';
import { isPushSupported, readNotificationPermission } from './platform';

/**
 * Background push-subscription synchronisation.
 *
 * On every cabinet mount (via {@link PushPermissionGate}) we check that an
 * active subscription still exists for the browser. If the browser revoked or
 * lost the subscription (storage wipe, browser-managed rotation) we silently
 * re-subscribe and re-POST it. This keeps push delivery alive without
 * bothering the user — and crucially never shows the system permission prompt,
 * which can only be triggered from an explicit user action.
 *
 * The function is intentionally non-throwing: any failure returns a typed
 * `reason` so the caller can `reportClientError` and move on.
 */

export type EnsureSubscriptionResult =
  | { readonly outcome: 'active'; readonly subscription: PushSubscription; readonly isNew: boolean }
  | {
      readonly outcome: 'created';
      readonly subscription: PushSubscription;
      readonly isNew: true;
    }
  | { readonly outcome: 'reason'; readonly reason: EnsureSubscriptionReason };

export type EnsureSubscriptionReason =
  | 'unsupported'
  | 'no-permission'
  | 'no-vapid-key'
  | 'missing-keys'
  | 'network-error';

export type PostSubscriptionFn = (payload: ReturnType<typeof subscriptionToPayload>) => Promise<void>;

/**
 * Reuse the react-query hooks to resolve the VAPID key and a POST function.
 * Kept as a factory so the gate can call it from an effect without dragging
 * hook rules into a plain async helper.
 */
export function useEnsureSubscriptionTools(): {
  readonly vapidKey: string | undefined;
  readonly postSubscription: PostSubscriptionFn;
} {
  const { data: vapidKey } = useVapidPublicKey();
  const createSubscription = useCreatePushSubscription();
  // `useCallback` keeps the function referentially stable across renders so
  // the consumer's `useEffect` does not loop on every render.
  const postSubscription = useCallback<PostSubscriptionFn>(
    async (payload) => {
      await createSubscription.mutateAsync(payload);
    },
    [createSubscription],
  );
  return { vapidKey, postSubscription };
}

/**
 * Verify and (re)establish the active push subscription.
 *
 * @param vapidKey base64url VAPID public key from the backend.
 * @param postSubscription callback that POSTs the payload to `/push/subscriptions`.
 */
export async function ensureActiveSubscription(
  vapidKey: string | undefined,
  postSubscription: PostSubscriptionFn,
): Promise<EnsureSubscriptionResult> {
  if (!isPushSupported()) {
    return { outcome: 'reason', reason: 'unsupported' };
  }
  if (readNotificationPermission() !== 'granted') {
    return { outcome: 'reason', reason: 'no-permission' };
  }
  if (!vapidKey) {
    return { outcome: 'reason', reason: 'no-vapid-key' };
  }

  const registration = await navigator.serviceWorker.ready;
  let subscription = await registration.pushManager.getSubscription();

  if (subscription) {
    return { outcome: 'active', subscription, isNew: false };
  }

  // No active subscription — create one and register it with the backend.
  try {
    subscription = await subscribeToPush(registration, vapidKey);
  } catch {
    // subscribe() throws on denial or transient errors; nothing to persist.
    return { outcome: 'reason', reason: 'no-permission' };
  }

  const payload = subscriptionToPayload(subscription);
  if (!hasRequiredKeys(payload)) {
    return { outcome: 'reason', reason: 'missing-keys' };
  }

  try {
    await postSubscription(payload);
  } catch {
    return { outcome: 'reason', reason: 'network-error' };
  }

  return { outcome: 'created', subscription, isNew: true };
}
