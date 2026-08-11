'use client';

import {
  ensureActiveSubscription,
  type EnsureSubscriptionReason,
  type EnsureSubscriptionResult,
  type PostSubscriptionFn,
} from './subscription-sync';
import {
  isPushSupported,
  readNotificationPermission,
  requiresInstallOnIos,
} from './platform';

/**
 * Explicit push-enablement flow triggered by a user action (the "Разрешить
 * пуши" button in onboarding or notification settings).
 *
 * Unlike {@link ensureActiveSubscription}, this helper MAY show the system
 * permission prompt when the user has not yet decided. It also returns a
 * structured result so the caller can show the right copy (iOS install
 * instruction, denial message, success toast).
 */

export type RequestPushPermissionOutcome =
  | { readonly outcome: 'ios-needs-install' }
  | { readonly outcome: 'unsupported' }
  | { readonly outcome: 'denied' }
  | { readonly outcome: 'subscribed'; readonly subscription: PushSubscription }
  | { readonly outcome: 'already-subscribed'; readonly subscription: PushSubscription }
  | { readonly outcome: 'error'; readonly reason: EnsureSubscriptionReason };

/**
 * Request notification permission and (re)subscribe. `vapidKey` and
 * `postSubscription` come from the react-query hooks resolved by the caller.
 */
export async function requestPushPermissionAndSubscribe(
  vapidKey: string | undefined,
  postSubscription: PostSubscriptionFn,
): Promise<RequestPushPermissionOutcome> {
  if (!isPushSupported()) {
    return { outcome: 'unsupported' };
  }

  // iOS only delivers push to installed PWAs; surface the instruction instead
  // of a system prompt that would silently fail.
  if (requiresInstallOnIos()) {
    return { outcome: 'ios-needs-install' };
  }

  if (readNotificationPermission() === 'default') {
    const permission = await Notification.requestPermission();
    if (permission !== 'granted') {
      return { outcome: 'denied' };
    }
  } else if (readNotificationPermission() === 'denied') {
    return { outcome: 'denied' };
  }

  const result: EnsureSubscriptionResult = await ensureActiveSubscription(
    vapidKey,
    postSubscription,
  );

  if (result.outcome === 'active') {
    return { outcome: 'already-subscribed', subscription: result.subscription };
  }
  if (result.outcome === 'created') {
    return { outcome: 'subscribed', subscription: result.subscription };
  }
  return { outcome: 'error', reason: result.reason };
}
