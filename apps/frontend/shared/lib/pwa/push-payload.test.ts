import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import {
    buildShowNotificationOptions,
    DEFAULT_PUSH_CLICK_URL,
    DEFAULT_PUSH_TAG,
    DEFAULT_PUSH_TITLE,
    parsePushPayload,
    pickClickTargetClient,
    resolveClickTarget,
    type SwClientLike,
} from './push-payload';

describe('parsePushPayload', () => {
    it('parses a full valid payload', () => {
        const payload = parsePushPayload(
            JSON.stringify({
                title: 'Оплата подписки',
                body: 'Списание за тариф не удалось — проверьте карту',
                tag: 'subscription_grace',
                url: '/profile/tariff',
                eventType: 'subscription_grace',
            }),
        );
        expect(payload).toEqual({
            title: 'Оплата подписки',
            body: 'Списание за тариф не удалось — проверьте карту',
            tag: 'subscription_grace',
            url: '/profile/tariff',
            eventType: 'subscription_grace',
        });
    });

    it('applies defaults for missing optional fields', () => {
        const payload = parsePushPayload(JSON.stringify({ body: 'Привет' }));
        expect(payload).toEqual({
            title: DEFAULT_PUSH_TITLE,
            body: 'Привет',
            tag: DEFAULT_PUSH_TAG,
            url: DEFAULT_PUSH_CLICK_URL,
            eventType: null,
        });
    });

    it('falls back to the default tag when the tag is an empty string', () => {
        const payload = parsePushPayload(JSON.stringify({ title: 'T', body: 'B', tag: '' }));
        expect(payload?.tag).toBe(DEFAULT_PUSH_TAG);
    });

    it('returns null for undefined eventData', () => {
        expect(parsePushPayload(undefined)).toBeNull();
    });

    it('returns null for null eventData', () => {
        expect(parsePushPayload(null)).toBeNull();
    });

    it('returns null for an empty string', () => {
        expect(parsePushPayload('')).toBeNull();
    });

    it('returns null for invalid JSON', () => {
        expect(parsePushPayload('{not json')).toBeNull();
    });

    it('returns null when JSON parses to a non-object', () => {
        expect(parsePushPayload('"a string"')).toBeNull();
        expect(parsePushPayload('42')).toBeNull();
        expect(parsePushPayload('null')).toBeNull();
    });

    it('rejects external URLs in the payload.url, falling back to the default', () => {
        const payload = parsePushPayload(
            JSON.stringify({ title: 'T', body: 'B', url: 'https://evil.example/path' }),
        );
        expect(payload?.url).toBe(DEFAULT_PUSH_CLICK_URL);
    });
});

describe('buildShowNotificationOptions', () => {
    it('maps payload fields onto NotificationOptions', () => {
        const options = buildShowNotificationOptions({
            title: 'T',
            body: 'Тело уведомления',
            tag: 'subscription_grace',
            url: '/profile/tariff',
            eventType: 'subscription_grace',
        });
        expect(options).toEqual({
            body: 'Тело уведомления',
            tag: 'subscription_grace',
            icon: '/icons/icon-192.png',
            badge: '/icons/icon-192.png',
            data: { url: '/profile/tariff' },
        });
    });

    it('carries the resolved (already-safe) url into data', () => {
        const options = buildShowNotificationOptions({
            title: 'T',
            body: 'B',
            tag: 't',
            url: DEFAULT_PUSH_CLICK_URL,
            eventType: null,
        });
        expect(options.data.url).toBe(DEFAULT_PUSH_CLICK_URL);
    });
});

describe('resolveClickTarget', () => {
    it('accepts an absolute same-origin path', () => {
        expect(resolveClickTarget('/properties')).toBe('/properties');
        expect(resolveClickTarget('/properties/abc-123')).toBe('/properties/abc-123');
        expect(resolveClickTarget('/profile/notifications')).toBe('/profile/notifications');
    });

    it('falls back when the value is missing', () => {
        expect(resolveClickTarget(undefined)).toBe(DEFAULT_PUSH_CLICK_URL);
        expect(resolveClickTarget(null)).toBe(DEFAULT_PUSH_CLICK_URL);
        expect(resolveClickTarget('')).toBe(DEFAULT_PUSH_CLICK_URL);
    });

    it('rejects non-string values', () => {
        expect(resolveClickTarget(42)).toBe(DEFAULT_PUSH_CLICK_URL);
        expect(resolveClickTarget({ url: '/properties' })).toBe(DEFAULT_PUSH_CLICK_URL);
    });

    it('rejects external absolute URLs', () => {
        expect(resolveClickTarget('https://example.com')).toBe(DEFAULT_PUSH_CLICK_URL);
        expect(resolveClickTarget('http://example.com/path')).toBe(DEFAULT_PUSH_CLICK_URL);
    });

    it('rejects scheme-relative and backslash URLs', () => {
        expect(resolveClickTarget('//example.com')).toBe(DEFAULT_PUSH_CLICK_URL);
        expect(resolveClickTarget('/\\example.com')).toBe(DEFAULT_PUSH_CLICK_URL);
    });

    it('rejects javascript: and other schemes', () => {
        expect(resolveClickTarget('javascript:alert(1)')).toBe(DEFAULT_PUSH_CLICK_URL);
        expect(resolveClickTarget('data:text/html,<x>')).toBe(DEFAULT_PUSH_CLICK_URL);
    });

    it('rejects relative paths without a leading slash', () => {
        expect(resolveClickTarget('properties')).toBe(DEFAULT_PUSH_CLICK_URL);
        expect(resolveClickTarget('./properties')).toBe(DEFAULT_PUSH_CLICK_URL);
    });
});

describe('pickClickTargetClient', () => {
    const origin = 'https://app.rentli.ru';
    const makeClient = (url: string): SwClientLike => ({
        url,
        focus: () => Promise.resolve(undefined),
        postMessage: () => undefined,
    });

    it('returns the first same-origin client', () => {
        const clients = [
            makeClient('https://other.example.com/foo'),
            makeClient('https://app.rentli.ru/properties'),
            makeClient('https://app.rentli.ru/profile'),
        ];
        const result = pickClickTargetClient(clients, origin);
        expect(result?.client.url).toBe('https://app.rentli.ru/properties');
    });

    it('returns null when no client matches the origin', () => {
        const clients = [makeClient('https://other.example.com/foo')];
        expect(pickClickTargetClient(clients, origin)).toBeNull();
    });

    it('returns null for an empty client list', () => {
        expect(pickClickTargetClient([], origin)).toBeNull();
    });

    it('matches a same-origin client with a path', () => {
        const clients = [makeClient(`${origin}/properties/123`)];
        const result = pickClickTargetClient(clients, origin);
        expect(result?.client.url).toBe(`${origin}/properties/123`);
    });
});

describe('service worker handlers stay in sync', () => {
    // Guards against drift between the TS source of truth and the inline copy
    // kept in public/sw.js (which cannot import TS at runtime). Mirrors the
    // app-routes.test.ts sync guard.
    const swPath = resolve(process.cwd(), 'public/sw.js');
    const swSource = readFileSync(swPath, 'utf8');

    it('public/sw.js shows a notification on every push event', () => {
        expect(swSource, 'push handler must call showNotification').toContain(
            'showNotification',
        );
    });

    it('public/sw.js handles notificationclick with clients.openWindow fallback', () => {
        expect(swSource, 'notificationclick handler must call clients.openWindow').toContain(
            'clients.openWindow',
        );
        expect(swSource).toContain('notificationclick');
    });

    it('public/sw.js references the shared default click url', () => {
        expect(swSource, 'SW must fall back to the default click url').toContain(
            DEFAULT_PUSH_CLICK_URL,
        );
    });

    it('public/sw.js references the shared default tag', () => {
        expect(swSource, 'SW must fall back to the default tag').toContain(
            `var DEFAULT_PUSH_TAG = '${DEFAULT_PUSH_TAG}'`,
        );
    });

    it('public/sw.js uses the branded 192px icon', () => {
        expect(swSource).toContain('/icons/icon-192.png');
    });
});
