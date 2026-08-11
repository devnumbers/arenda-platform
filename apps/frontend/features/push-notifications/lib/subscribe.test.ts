import { describe, expect, it } from 'vitest';
import {
    hasRequiredKeys,
    subscriptionToPayload,
    type SubscriptionLike,
} from './subscribe';

function makeSubscription(overrides: Partial<SubscriptionLike> = {}): SubscriptionLike {
    return {
        endpoint: 'https://fcm.googleapis.com/fcm/send/abc',
        expirationTime: null,
        getKey(name) {
            if (name === 'p256dh') {
                // 65 bytes of 0x01
                const buf = new ArrayBuffer(65);
                const view = new Uint8Array(buf);
                view.fill(1);
                return buf;
            }
            if (name === 'auth') {
                // 16 bytes of 0x02
                const buf = new ArrayBuffer(16);
                const view = new Uint8Array(buf);
                view.fill(2);
                return buf;
            }
            return null;
        },
        ...overrides,
    };
}

describe('subscriptionToPayload', () => {
    it('maps endpoint and base64url-encodes p256dh and auth', () => {
        const payload = subscriptionToPayload(makeSubscription());
        expect(payload.endpoint).toBe('https://fcm.googleapis.com/fcm/send/abc');
        expect(payload.p256dh.length).toBeGreaterThan(0);
        expect(payload.auth.length).toBeGreaterThan(0);
        // base64url alphabet: no +, /, or padding
        expect(payload.p256dh).not.toMatch(/[+/=]/);
        expect(payload.auth).not.toMatch(/[+/=]/);
    });

    it('emits ISO 8601 expirationTime when the browser reports one', () => {
        const fixed = Date.UTC(2026, 0, 1, 12, 0, 0);
        const payload = subscriptionToPayload(makeSubscription({ expirationTime: fixed }));
        expect(payload.expirationTime).toBe('2026-01-01T12:00:00.000Z');
    });

    it('emits null expirationTime when the browser reports null', () => {
        const payload = subscriptionToPayload(makeSubscription({ expirationTime: null }));
        expect(payload.expirationTime).toBeNull();
    });

    it('produces empty key strings when the browser hides p256dh/auth', () => {
        const payload = subscriptionToPayload(
            makeSubscription({
                getKey: () => null,
            }),
        );
        expect(payload.p256dh).toBe('');
        expect(payload.auth).toBe('');
    });

    it('round-trips known ECDH bytes through the base64url encoder', () => {
        const p256dh = Uint8Array.of(1, 2, 3, 250);
        const auth = Uint8Array.of(10, 20, 30, 40, 50);
        const payload = subscriptionToPayload(
            makeSubscription({
                getKey(name) {
                    if (name === 'p256dh') return p256dh.slice().buffer;
                    if (name === 'auth') return auth.slice().buffer;
                    return null;
                },
            }),
        );
        // Decode back and compare with the original bytes
        const decode = (b64url: string): number[] =>
            Array.from(Buffer.from(b64url.replace(/-/g, '+').replace(/_/g, '/'), 'base64'));
        expect(decode(payload.p256dh)).toEqual([1, 2, 3, 250]);
        expect(decode(payload.auth)).toEqual([10, 20, 30, 40, 50]);
    });
});

describe('hasRequiredKeys', () => {
    it('returns true when both keys are present', () => {
        expect(hasRequiredKeys({ p256dh: 'a', auth: 'b', endpoint: 'e', expirationTime: null })).toBe(true);
    });

    it('returns false when p256dh is empty', () => {
        expect(hasRequiredKeys({ p256dh: '', auth: 'b', endpoint: 'e', expirationTime: null })).toBe(false);
    });

    it('returns false when auth is empty', () => {
        expect(hasRequiredKeys({ p256dh: 'a', auth: '', endpoint: 'e', expirationTime: null })).toBe(false);
    });
});
