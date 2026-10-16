import { base64UrlToUint8Array } from '@/shared/lib/pwa/base64url';

/**
 * Push subscription orchestration.
 *
 * `subscribeToPush` wraps `pushManager.subscribe` with the VAPID application
 * server key. `subscriptionToPayload` is a pure mapper from a `PushSubscription`
 * to the backend request body — extracted so it can be unit-tested without a
 * live browser. Both run on the client only.
 */

/** Minimal view of `PushSubscription` consumed by the mapper. */
export type SubscriptionLike = {
    readonly endpoint: string;
    readonly expirationTime: number | null;
    getKey(name: 'p256dh' | 'auth'): ArrayBuffer | null;
};

/** Payload for `POST /push/subscriptions` (camelCase view of the OpenAPI type). */
export type PushSubscriptionPayload = {
    readonly endpoint: string;
    readonly p256dh: string;
    readonly auth: string;
    readonly expirationTime: string | null;
};

function toBase64Url(buffer: ArrayBuffer | null): string {
    if (!buffer) return '';
    const bytes = new Uint8Array(buffer);
    let binary = '';
    for (const byte of bytes) {
        binary += String.fromCharCode(byte);
    }
    // `btoa` is available in the browser and in Node ≥16 (vitest).
    return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

/**
 * Convert a browser `PushSubscription` into the backend request body.
 *
 * `getKey('p256dh')` / `getKey('auth')` return `null` on the rare browsers
 * that do not expose them; in that case the caller treats an empty `p256dh`/
 * `auth` as a hard error (RFC 8291 requires both) and refuses to POST.
 * `expirationTime` is sent as ISO 8601 or `null` — the backend column is
 * nullable and the push service may report a future expiry (RFC 8030).
 */
export function subscriptionToPayload(subscription: SubscriptionLike): PushSubscriptionPayload {
    return {
        endpoint: subscription.endpoint,
        p256dh: toBase64Url(subscription.getKey('p256dh')),
        auth: toBase64Url(subscription.getKey('auth')),
        expirationTime:
            subscription.expirationTime !== null
                ? new Date(subscription.expirationTime).toISOString()
                : null,
    };
}

/**
 * Returns `true` when the payload carries the mandatory ECDH keys.
 * The backend rejects subscriptions without `p256dh`/`auth`, so callers skip
 * the POST when this returns `false`.
 */
export function hasRequiredKeys(payload: PushSubscriptionPayload): boolean {
    return payload.p256dh.length > 0 && payload.auth.length > 0;
}

/**
 * Subscribe the browser push manager using the server's VAPID public key.
 *
 * `userVisibleOnly: true` is the Chrome contract: every push must surface a
 * visible notification (the SW `push` handler enforces this). The
 * applicationServerKey is base64url-decoded into the `BufferSource` the API
 * requires.
 *
 * The function returns the raw `PushSubscription`; the caller decides whether
 * to POST it to the backend (so the same primitive serves both first-time
 * subscription and re-subscription).
 */
export async function subscribeToPush(
    registration: ServiceWorkerRegistration,
    applicationServerKeyB64Url: string,
): Promise<PushSubscription> {
    const applicationServerKey = base64UrlToUint8Array(applicationServerKeyB64Url);
    return registration.pushManager.subscribe({
        userVisibleOnly: true,
        // `pushManager.subscribe` expects a `BufferSource` backed by a concrete
        // `ArrayBuffer`. `base64UrlToUint8Array` returns `Uint8Array<ArrayBufferLike>`,
        // so copy into a fresh `ArrayBuffer` to satisfy the DOM lib type.
        applicationServerKey: applicationServerKey.buffer as ArrayBuffer,
    });
}
