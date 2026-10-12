/**
 * Platform capability detection for Web Push.
 *
 * Every helper is SSR-safe — returns a benign default when `window` is
 * undefined. Call sites run inside client components ('use client'), so the
 * guards are a defence against first-paint execution rather than a real
 * server path, but Next.js still evaluates client-module top-level code on
 * the server during the initial render, so the guards are required.
 */

// Standalone-mode detection is a shared-layer concern (used by login UI, the
// service worker guard, and push). Imported here so existing push feature call
// sites keep working without crossing FSD layer boundaries.
import { isStandaloneMode } from '@/shared/lib/pwa/standalone';
export { isStandaloneMode };

// iOS device detection moved to the shared layer (12.10.2026): the
// pull-to-refresh gate in shared/ui needs it too (ADR 0031), and shared/ui
// cannot import from a feature. Not re-exported: within this slice only
// requiresInstallOnIos() consumes it, and the slice exports only what other
// elements actually import.
import { isIosDevice } from '@/shared/lib/pwa/ios';

/**
 * True when the current browser exposes everything Web Push needs: an active
 * service worker container, a `PushManager`, and the `Notification` API.
 * Used to hide push UI on browsers that cannot receive push at all.
 */
export function isPushSupported(): boolean {
    if (typeof window === 'undefined' || typeof navigator === 'undefined') return false;
    return (
        'serviceWorker' in navigator &&
        'PushManager' in window &&
        'Notification' in window
    );
}

/**
 * On iOS, push notifications are only delivered to installed PWAs (standalone
 * mode). When the user tries to enable push from a regular Safari tab, we
 * surface an "add to Home Screen" instruction instead of a doomed system
 * prompt. This helper centralises that check.
 */
export function requiresInstallOnIos(): boolean {
    return isIosDevice() && !isStandaloneMode();
}

export type NotificationPermissionState = NotificationPermission | 'unsupported';

/**
 * Read the current notification permission, or `'unsupported'` when the
 * Notification API is missing. Never throws.
 */
export function readNotificationPermission(): NotificationPermissionState {
    if (typeof window === 'undefined' || typeof Notification === 'undefined') {
        return 'unsupported';
    }
    return Notification.permission;
}

/** Отписка от событий пробы (для useSyncExternalStore-cleanup). */
export type PermissionChangesUnsubscribe = () => void;

/**
 * Live permission updates (спека #1028 §6, слайс 2 #1038): the browser emits
 * no event for `Notification.permission` itself, but
 * `permissions.query({name:'notifications'})` returns a PermissionStatus
 * whose `change` event fires when the permission is granted/revoked outside
 * the page (late opt-in from Site Settings, Chrome auto-revoke). The
 * subscription wires that event; without the Permissions API (Safari < 16,
 * SSR) it degrades to a no-op and the probe simply never updates live.
 *
 * The wiring itself is async (the query resolves in a promise): a change
 * landing in the same tick as the subscribe call can be missed — the store
 * re-reads the permission on every change anyway, and permission flips in
 * that window are not a real scenario.
 */
export function subscribeToPermissionChanges(
    onChange: () => void,
): PermissionChangesUnsubscribe {
    // permissions может отсутствовать в рантайме (старый Safari), хотя
    // lib.dom считает его обязательным — проверяем через локальный срез.
    type NavigatorWithOptionalPermissions = Omit<Navigator, 'permissions'> & {
        readonly permissions?: Permissions;
    };
    if (
        typeof window === 'undefined' ||
        typeof navigator === 'undefined'
    ) {
        return () => {};
    }
    const permissions = (navigator as NavigatorWithOptionalPermissions).permissions;
    if (permissions?.query === undefined) {
        return () => {};
    }
    let removeListener: () => void = () => {};
    permissions
        .query({ name: 'notifications' })
        .then((status) => {
            status.addEventListener('change', onChange);
            removeListener = () => status.removeEventListener('change', onChange);
        })
        .catch(() => {
            // Permissions API rejects for 'notifications' on browsers without
            // live updates — the synchronous probe still works.
        });
    return () => removeListener();
}
