/**
 * Decode a base64url string (RFC 4648 §5, no padding) into a `Uint8Array`.
 *
 * The browser Push API needs the VAPID application server key as an
 * `ArrayBuffer`/`Uint8Array`. The backend delivers it as base64url without
 * padding (RFC 8292); `atob` only understands canonical base64 with padding,
 * so this helper first rewrites the alphabet (`-`→`+`, `_`→`/`) and restores
 * the padding before decoding.
 *
 * Pure stdlib — no dependencies, safe for the service worker and unit tests.
 *
 * @see https://datatracker.ietf.org/doc/html/rfc8292 VAPID (applicationServerKey)
 * @see https://datatracker.ietf.org/doc/html/rfc4648#section-5 base64url
 */
export function base64UrlToUint8Array(value: string): Uint8Array {
    // base64url → canonical base64: replace URL-safe alphabet and add padding
    // so that the final length is a multiple of 4 characters.
    const base64 = value.replace(/-/g, '+').replace(/_/g, '/');
    const padded = base64.padEnd(base64.length + ((4 - (base64.length % 4)) % 4), '=');

    // `atob` is available in the browser, in the service worker global scope,
    // and in Node ≥16 (so vitest can exercise this without polyfills).
    const binary = atob(padded);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i += 1) {
        bytes[i] = binary.charCodeAt(i);
    }
    return bytes;
}
