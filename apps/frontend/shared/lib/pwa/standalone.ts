/**
 * Standalone (PWA) mode detection.
 *
 * This is shared-layer utility: PWA standalone state is not specific to any
 * single feature (push notifications, login, offline), so it lives here and is
 * importable from any layer per Feature-Sliced Design boundaries.
 *
 * Every helper is SSR-safe — returns a benign default when `window` is
 * undefined. Call sites run inside client components ('use client'), so the
 * guards are a defence against first-paint execution rather than a real
 * server path, but Next.js still evaluates client-module top-level code on
 * the server during the initial render, so the guards are required.
 */

/** iOS Safari exposes a non-standard `navigator.standalone` boolean. */
export type NavigatorWithStandalone = Navigator & {
    readonly standalone?: boolean;
};

/**
 * True when the app runs as an installed PWA — either Chrome's
 * `(display-mode: standalone)` media query matches, or iOS Safari reports
 * `navigator.standalone === true`.
 *
 * NOTE: this has nothing to do with Next.js `output: 'standalone'` (a build
 * format). This refers to the browser PWA display mode where the site is
 * launched from the home screen without a browser chrome.
 */
export function isStandaloneMode(): boolean {
    if (typeof window === 'undefined') return false;
    if (window.matchMedia('(display-mode: standalone)').matches) return true;
    return (navigator as NavigatorWithStandalone).standalone === true;
}
