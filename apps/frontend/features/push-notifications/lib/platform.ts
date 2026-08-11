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

/** `window.MSStream` is the legacy IE/Edge UA spoofing guard for iPadOS. */
type WindowWithMSStream = Window & {
    readonly MSStream?: unknown;
};

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
