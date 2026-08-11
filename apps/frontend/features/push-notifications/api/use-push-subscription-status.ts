'use client';

import { useEffect, useState } from 'react';
import {
  isPushSupported,
  readNotificationPermission,
} from '../lib/platform';

export type PushSubscriptionStatus = {
  /** The browser cannot receive push at all. */
  readonly isUnsupported: boolean;
  /** Notification permission has not been granted. */
  readonly needsPermission: boolean;
  /** The user explicitly denied notification permission — the system prompt cannot be re-shown. */
  readonly permissionDenied: boolean;
  /** Permission granted but no active subscription on this device. */
  readonly needsSubscription: boolean;
  /** Push is supported, permission granted, and an active subscription exists. */
  readonly isReady: boolean;
  /** Initial SSR-safe state — no decision yet. */
  readonly isPending: boolean;
};

const PENDING: PushSubscriptionStatus = {
  isUnsupported: false,
  needsPermission: false,
  permissionDenied: false,
  needsSubscription: false,
  isReady: false,
  isPending: true,
};

const UNSUPPORTED: PushSubscriptionStatus = {
  isUnsupported: true,
  needsPermission: false,
  permissionDenied: false,
  needsSubscription: false,
  isReady: false,
  isPending: false,
};

/**
 * Inspect the browser push capability and return a coarse-grained status the
 * notification-settings UI can branch on.
 *
 * `isUnsupported` is resolved eagerly during the first render (it is a
 * synchronous capability check) so the effect never calls `setState` in the
 * unsupported branch — ESLint's `react-hooks/set-state-in-effect` rule fires
 * on unconditional `setState` calls inside effects. The async subscription
 * check lives in `.then` callbacks, which the rule treats as legitimate
 * (state settles after an awaited operation, not synchronously on mount).
 */
export function usePushSubscriptionStatus(): PushSubscriptionStatus {
  // Resolve synchronously on the client first render; on SSR `isPending`
  // stays true and the caller renders the neutral UI.
  const getInitial = (): PushSubscriptionStatus => {
    if (typeof window !== 'undefined' && !isPushSupported()) {
      return UNSUPPORTED;
    }
    return PENDING;
  };
  const [status, setStatus] = useState<PushSubscriptionStatus>(getInitial);

  useEffect(() => {
    // The synchronous-unsupported case was handled by the initializer; bail
    // out here so no setState is reached for unsupported browsers.
    if (status.isUnsupported) return;

    let cancelled = false;
    const resolve = (next: PushSubscriptionStatus): void => {
      if (!cancelled) setStatus(next);
    };

    void (async () => {
      const permission = readNotificationPermission();
      if (permission !== 'granted') {
        resolve({
          isUnsupported: false,
          needsPermission: true,
          permissionDenied: permission === 'denied',
          needsSubscription: false,
          isReady: false,
          isPending: false,
        });
        return;
      }

      const registration = await navigator.serviceWorker.ready;
      const subscription = await registration.pushManager.getSubscription();
      resolve({
        isUnsupported: false,
        needsPermission: false,
        permissionDenied: false,
        needsSubscription: subscription === null,
        isReady: subscription !== null,
        isPending: false,
      });
    })();

    return () => {
      cancelled = true;
    };
    // Re-run only on mount; the capability/permission state is stable for the
    // lifetime of this view and a manual re-check is not needed here.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return status;
}
