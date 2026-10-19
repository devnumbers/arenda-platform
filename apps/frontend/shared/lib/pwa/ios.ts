/**
 * iOS / iPadOS device detection.
 *
 * This is a shared-layer utility: the platform check is not specific to any
 * single feature (Web Push install requirements), so it lives here and is
 * importable from any layer per Feature-Sliced
 * Design boundaries — mirroring `standalone.ts`.
 *
 * Every helper is SSR-safe — returns a benign default when `window` is
 * undefined. Call sites run inside client components ('use client'), so the
 * guards are a defence against first-paint execution rather than a real
 * server path, but Next.js still evaluates client-module top-level code on
 * the server during the initial render, so the guards are required.
 */

/** `window.MSStream` is the legacy IE/Edge UA spoofing guard for iPadOS. */
type WindowWithMSStream = Window & {
    readonly MSStream?: unknown;
};

/**
 * Detects iOS / iPadOS devices. iPadOS 13+ spoofs desktop Safari — its UA
 * string is byte-identical to desktop macOS Safari (down to the `Macintosh`
 * token), so the Mac signature is read from the UA itself, alongside the
 * classic iDevice tokens, with touch events separating a spoofed iPad from a
 * real desktop Mac. `navigator.platform` — the previous Mac signal — is
 * deprecated and is deliberately not read. The `MSStream` guard stops
 * Edge / IE from false-matching.
 *
 * Consumers care about the platform family, not the vendor: Web Push on iOS
 * requires a home-screen install (standalone mode).
 */
export function isIosDevice(): boolean {
    if (typeof window === 'undefined' || typeof navigator === 'undefined') return false;
    const ua = navigator.userAgent;
    const hasIosToken = /iPad|iPhone|iPod/.test(ua) || (/macintosh|macintel/i.test(ua) && 'ontouchend' in document);
    const isMsStream = (window as WindowWithMSStream).MSStream !== undefined;
    return hasIosToken && !isMsStream;
}
