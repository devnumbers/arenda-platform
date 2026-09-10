'use client';

import { useEffect, type JSX } from 'react';
import { reportClientError } from '@/shared/lib/error-reporting/report-client-error';
import { useNotificationPreferences } from '@/features/notification-preferences';
import {
  ensureActiveSubscription,
  useEnsureSubscriptionTools,
} from '@/features/push-notifications';
import { readNotificationPermission } from '@/features/push-notifications';

/**
 * Invisible side-effect component: keeps the push subscription alive.
 *
 * Mounted once in the screen layout (ScreenLayout, ex-cabinet since #568).
 * On every app load it checks whether
 * the user has at least one `pushAllowed === true` preference, and if so (and
 * permission is already granted) runs {@link ensureActiveSubscription} in the
 * background. The system permission prompt is NEVER triggered from here —
 * only from explicit user actions in the onboarding modal and notification
 * settings — so the gate cannot resurrect a prompt the user dismissed.
 *
 * Any failure is reported via `reportClientError` and swallowed; push is a
 * best-effort channel and the email path is unaffected.
 */
export function PushPermissionGate(): JSX.Element | null {
  const { data } = useNotificationPreferences();
  const { vapidKey, postSubscription } = useEnsureSubscriptionTools();

  useEffect(() => {
    if (!data) return;

    const anyPushAllowed = data.some((preference) => preference.pushAllowed);
    if (!anyPushAllowed) return;

    if (readNotificationPermission() !== 'granted') return;

    let cancelled = false;
    ensureActiveSubscription(vapidKey, postSubscription)
      .then((result) => {
        if (cancelled) return;
        if (result.outcome === 'reason') {
          // unsupported / no-permission / no-vapid-key are expected runtime
          // states and not worth reporting; network-error and missing-keys
          // are surfaced for observability.
          if (result.reason === 'network-error' || result.reason === 'missing-keys') {
            reportClientError(
              `push subscription sync failed: ${result.reason}`,
            );
          }
        }
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        reportClientError(
          'push subscription sync threw',
          error instanceof Error ? error.stack : undefined,
        );
      });

    return () => {
      cancelled = true;
    };
  }, [data, vapidKey, postSubscription]);

  return null;
}
