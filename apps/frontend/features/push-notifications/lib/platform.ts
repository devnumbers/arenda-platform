/**
 * Platform capability detection for Web Push.
 *
 * Every helper is SSR-safe — returns a benign default when `window` is
 * undefined. Call sites run inside client components ('use client'), so the
 * guards are a defence against first-paint execution rather than a real
 * server path, but Next.js still evaluates client-module top-level code on
 * the server during the initial render, so the guards are required.
 */

/** iOS Safari exposes a non-standard `navigator.standalone` boolean. */
type NavigatorWithStandalone = Navigator & {
    readonly standalone?: boolean;
};

/** `window.MSStream` is the legacy IE/Edge UA spoofing guard for iPadOS. */
type WindowWithMSStream = Window & {
    readonly MSStream?: unknown;
};

/**
 * True when the app runs as an installed PWA — either Chrome's
 * `(display-mode: standalone)` media query matches, or iOS Safari reports
 * `navigator.standalone === true`. Web Push on iOS requires standalone mode.
 */
export function isStandaloneMode(): boolean {
    if (typeof window === 'undefined') return false;
    if (window.matchMedia?.('(display-mode: standalone)').matches) return true;
    return (navigator as NavigatorWithStandalone).standalone === true;
}

/**
 * Detects iOS / iPadOS devices. iPadOS 13+ spoofs desktop Safari in its UA
 * string, so the check includes the Mac platform alongside the classic
 * iDevice tokens. The `MSStream` guard stops Edge / IE from false-matching.
 */
export function isIosDevice(): boolean {
    if (typeof window === 'undefined' || typeof navigator === 'undefined') return false;
    const ua = navigator.userAgent;
    const platform = typeof navigator.platform === 'string' ? navigator.platform : '';
    const hasIosToken = /iPad|iPhone|iPod/.test(ua) || (/macintosh|macintel/i.test(platform) && 'ontouchend' in document);
    const isMsStream = (window as WindowWithMSStream).MSStream !== undefined;
    return hasIosToken && !isMsStream;
}

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
