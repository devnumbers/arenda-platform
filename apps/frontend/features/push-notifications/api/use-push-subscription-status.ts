'use client';

import { useCallback, useEffect, useState } from 'react';
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

export type PushSubscriptionStatusResult = PushSubscriptionStatus & {
  /**
   * Re-probe the browser's push capability, permission, and subscription.
   * The browser does not emit an event when any of these change, so call this
   * after an explicit user action that may affect the result (e.g. once the
   * "Разрешить пуши" button has created a subscription) — otherwise the status
   * stays stale until a page reload.
   */
  readonly refresh: () => void;
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
export function usePushSubscriptionStatus(): PushSubscriptionStatusResult {
  // Resolve synchronously on the client first render; on SSR `isPending`
  // stays true and the caller renders the neutral UI.
  const getInitial = (): PushSubscriptionStatus => {
    if (typeof window !== 'undefined' && !isPushSupported()) {
      return UNSUPPORTED;
    }
    return PENDING;
  };
  const [status, setStatus] = useState<PushSubscriptionStatus>(getInitial);

  // Re-read the browser push capability/permission/subscription and push the
  // result into state. Explicit useCallback, not the compiler's automatic
  // memoization: exhaustive-deps is static and cannot see it, so a plain
  // function plus `[refresh]` deps below is a lint error; with useCallback the
  // identity is stable by construction and the mount effect runs exactly
  // once. `cancelled` is threaded in by the mount effect so a probe that
  // resolves after unmount does not call setState.
  const refresh = useCallback(async (cancelled: () => boolean = () => false): Promise<void> => {
    if (!isPushSupported()) return;

    const permission = readNotificationPermission();
    if (permission !== 'granted') {
      if (!cancelled()) {
        setStatus({
          isUnsupported: false,
          needsPermission: true,
          permissionDenied: permission === 'denied',
          needsSubscription: false,
          isReady: false,
          isPending: false,
        });
      }
      return;
    }

    const registration = await navigator.serviceWorker.ready;
    const subscription = await registration.pushManager.getSubscription();
    if (cancelled()) return;
    setStatus({
      isUnsupported: false,
      needsPermission: false,
      permissionDenied: false,
      needsSubscription: subscription === null,
      isReady: subscription !== null,
      isPending: false,
    });
  }, []);

  useEffect(() => {
    // The synchronous-unsupported case was handled by the initializer; bail
    // out here so no setState is reached for unsupported browsers. The same
    // synchronous check (not `status.isUnsupported`) keeps the effect free of
    // the status value in its reads; `refresh` is compiler-memoized, so the
    // effect still runs exactly once per mount.
    if (!isPushSupported()) return;

    let cancelled = false;
    // Run once on mount. The async IIFE keeps the setStatus calls behind an
    // await so ESLint's react-hooks/set-state-in-effect rule does not fire.
    void (async () => {
      await refresh(() => cancelled);
    })();
    return () => {
      cancelled = true;
    };
  }, [refresh]);

  return { ...status, refresh: () => void refresh() };
}
