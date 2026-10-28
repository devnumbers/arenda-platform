import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import {
    isCacheablePhotoResponse,
    isPhotoThumbCacheableRequest,
    PHOTO_CACHE_CLEAR_ALWAYS_PATH,
    PHOTO_CACHE_CLEAR_ON_SUCCESS_PATH,
    PHOTO_CACHE_MAX_ENTRIES,
    PHOTO_CACHE_NAME,
    PHOTO_CACHE_SCOPES,
    photoCacheClearPolicy,
    photoThumbCacheKey,
    photoThumbRequestSkipsCache,
} from './photo-cache-policy';

describe('isPhotoThumbCacheableRequest', () => {
    const cacheable = (
        pathname: string,
        search = '?size=thumb&v=abc123',
        method = 'GET',
    ): boolean => isPhotoThumbCacheableRequest(method, pathname, new URLSearchParams(search));

    it('accepts every photo stream shape of ADR 0065 with size=thumb and a version', () => {
        expect(cacheable('/api/me/photo')).toBe(true);
        expect(cacheable('/api/properties/abc123/photo')).toBe(true);
        expect(cacheable('/api/contacts/abc123/photo')).toBe(true);
        expect(cacheable('/api/users/abc123/photo')).toBe(true);
    });

    it('accepts any parameter order and the retry nonce', () => {
        expect(cacheable('/api/me/photo', '?v=abc123&size=thumb')).toBe(true);
        expect(cacheable('/api/me/photo', '?v=abc123&size=thumb&r=1')).toBe(true);
    });

    it('rejects non-GET methods', () => {
        expect(cacheable('/api/me/photo', '?size=thumb&v=abc', 'POST')).toBe(false);
        expect(cacheable('/api/me/photo', '?size=thumb&v=abc', 'HEAD')).toBe(false);
    });

    it('rejects requests without the thumb variant', () => {
        // Полный объект (крупный просмотр, если появится) мимо примитива —
        // кешируется только то, что реально показывает CirclePhoto.
        expect(cacheable('/api/me/photo', '?v=abc')).toBe(false);
        expect(cacheable('/api/me/photo', '?size=full&v=abc')).toBe(false);
    });

    it('rejects requests without a non-empty version', () => {
        // Cache-first безопасен только для версионных URL: замена фото —
        // новый ?v=, неизменяемые байты. Неверсионный запрос — не кешируем.
        expect(cacheable('/api/me/photo', '?size=thumb')).toBe(false);
        expect(cacheable('/api/me/photo', '?size=thumb&v=')).toBe(false);
    });

    it('rejects foreign paths', () => {
        expect(cacheable('/api/me')).toBe(false);
        expect(cacheable('/api/me/avatar')).toBe(false);
        expect(cacheable('/api/me/photo/extra')).toBe(false);
        // id отсутствует — «photo» становится вторым сегментом, не четвёртым.
        expect(cacheable('/api/properties/photo')).toBe(false);
        expect(cacheable('/api/properties/abc123/avatar')).toBe(false);
        expect(cacheable('/api/properties/abc123/photos')).toBe(false);
        // v1-форма не поддерживается (ADR 0065, аменд 2026-10-09).
        expect(cacheable('/api/v1/properties/abc123/photo')).toBe(false);
        expect(cacheable('/images/empty-properties.png')).toBe(false);
        expect(cacheable('/_next/image?url=%2Fimages%2Fx.png&w=256')).toBe(false);
        expect(cacheable('/login')).toBe(false);
    });
});

describe('photoThumbCacheKey', () => {
    it('strips the retry nonce so a successful retry becomes the canonical thumb', () => {
        expect(photoThumbCacheKey('https://app.test/api/me/photo?v=abc&size=thumb&r=1')).toBe(
            'https://app.test/api/me/photo?v=abc&size=thumb',
        );
        expect(photoThumbCacheKey('https://app.test/api/me/photo?size=thumb&r=3&v=abc')).toBe(
            'https://app.test/api/me/photo?size=thumb&v=abc',
        );
    });

    it('returns the input unchanged when there is no nonce', () => {
        const href = 'https://app.test/api/me/photo?v=abc&size=thumb';
        expect(photoThumbCacheKey(href)).toBe(href);
    });
});

describe('photoThumbRequestSkipsCache', () => {
    it('sends retry-nonce requests to the network — a retry must be able to heal bad cached bytes', () => {
        // Ключ нормализуется (r вырезан), но cache.match для нонса не
        // выполняется: если в кеше лежат байты, которые не декодируются,
        // ретрай с тем же содержимым не лечит — ломает механизм #1275.
        expect(photoThumbRequestSkipsCache(new URLSearchParams('?v=abc&size=thumb&r=1'))).toBe(true);
        expect(photoThumbRequestSkipsCache(new URLSearchParams('?size=thumb&v=abc&r=3'))).toBe(true);
    });

    it('lets plain requests read the cache', () => {
        expect(photoThumbRequestSkipsCache(new URLSearchParams('?v=abc&size=thumb'))).toBe(false);
        expect(photoThumbRequestSkipsCache(new URLSearchParams(''))).toBe(false);
    });
});

describe('isCacheablePhotoResponse', () => {
    it('accepts image responses only — a 200 error page must not be cached forever', () => {
        expect(isCacheablePhotoResponse('image/jpeg')).toBe(true);
        expect(isCacheablePhotoResponse('image/png')).toBe(true);
        expect(isCacheablePhotoResponse('image/webp')).toBe(true);
        expect(isCacheablePhotoResponse('text/html; charset=utf-8')).toBe(false);
        expect(isCacheablePhotoResponse('application/json')).toBe(false);
        expect(isCacheablePhotoResponse(null)).toBe(false);
        expect(isCacheablePhotoResponse('')).toBe(false);
    });
});

describe('photoCacheClearPolicy', () => {
    it('clears unconditionally on the logout request', () => {
        expect(photoCacheClearPolicy('POST', PHOTO_CACHE_CLEAR_ALWAYS_PATH)).toBe('always');
    });

    it('clears on success on the login (verify) request — the previous device user never leaves photos behind even without an explicit logout', () => {
        expect(photoCacheClearPolicy('POST', PHOTO_CACHE_CLEAR_ON_SUCCESS_PATH)).toBe(
            'on-success',
        );
    });

    it('ignores other auth endpoints, methods, and paths', () => {
        expect(photoCacheClearPolicy('POST', '/api/auth/send')).toBe('never');
        expect(photoCacheClearPolicy('GET', PHOTO_CACHE_CLEAR_ALWAYS_PATH)).toBe('never');
        expect(photoCacheClearPolicy('PUT', PHOTO_CACHE_CLEAR_ON_SUCCESS_PATH)).toBe('never');
        expect(photoCacheClearPolicy('POST', '/api/me/photo')).toBe('never');
        expect(photoCacheClearPolicy('POST', '/auth/logout')).toBe('never');
    });
});

describe('photo cache constants', () => {
    it('caps entries at a few megabytes of thumbs', () => {
        expect(PHOTO_CACHE_MAX_ENTRIES).toBeGreaterThan(0);
        expect(PHOTO_CACHE_MAX_ENTRIES).toBeLessThanOrEqual(1000);
    });

    it('names the cache photo-thumbs-v1 — distinct from offline and static-assets caches', () => {
        expect(PHOTO_CACHE_NAME).toMatch(/^photo-thumbs-v\d+$/);
        expect(PHOTO_CACHE_NAME).not.toContain('static-assets');
        expect(PHOTO_CACHE_NAME).not.toContain('offline');
    });

    it('scopes the matcher to the ADR 0065 photo owners', () => {
        expect([...PHOTO_CACHE_SCOPES].sort()).toEqual(['contacts', 'properties', 'users']);
    });
});

describe('service worker photo cache stays in sync', () => {
    // Guards against drift between the TS source of truth and the inline copy
    // kept in public/sw.js (which cannot import TS at runtime). Mirrors the
    // app-routes.test.ts / push-payload.test.ts sync guards.
    const swPath = resolve(process.cwd(), 'public/sw.js');
    const swSource = readFileSync(swPath, 'utf8');

    it('public/sw.js mirrors the cache name and entry cap', () => {
        expect(swSource).toContain(`var PHOTO_CACHE_NAME = '${PHOTO_CACHE_NAME}'`);
        expect(swSource).toContain(`var PHOTO_CACHE_MAX_ENTRIES = ${PHOTO_CACHE_MAX_ENTRIES}`);
    });

    it('public/sw.js mirrors the photo scope list', () => {
        const start = swSource.indexOf('var PHOTO_CACHE_SCOPES = [');
        expect(start, 'scope list marker not found in sw.js').toBeGreaterThanOrEqual(0);
        const end = swSource.indexOf(']', start);
        const swScopes = Array.from(
            swSource.slice(start, end).matchAll(/'([^']+)'/g),
            (match) => match[1] ?? '',
        );
        expect(swScopes).toEqual([...PHOTO_CACHE_SCOPES]);
    });

    it('public/sw.js mirrors the auth clear paths', () => {
        expect(swSource).toContain(
            `var PHOTO_CACHE_CLEAR_ALWAYS_PATH = '${PHOTO_CACHE_CLEAR_ALWAYS_PATH}'`,
        );
        expect(swSource).toContain(
            `var PHOTO_CACHE_CLEAR_ON_SUCCESS_PATH = '${PHOTO_CACHE_CLEAR_ON_SUCCESS_PATH}'`,
        );
    });

    it('public/sw.js wires the cache-first branch before the navigate branch', () => {
        expect(swSource).toContain('isPhotoThumbCacheableRequest');
        expect(swSource).toContain('photoThumbCacheFirst');
        expect(swSource).toContain('photoThumbCacheKey');
        expect(swSource).toContain('trimPhotoCache');
        // Ретрай-нонс идёт мимо cache.match (network-first) — ретрай обязан
        // перепросить сеть, иначе битая запись не вылечится.
        expect(swSource).toContain('photoThumbRequestSkipsCache');
    });

    it('public/sw.js wires the logout/login cleanup branch', () => {
        expect(swSource).toContain('photoCacheClearPolicy');
        expect(swSource).toContain('authRequestClearingPhotoCache');
    });

    it('public/sw.js only caches successful image responses', () => {
        // 404/500/ошибочные страницы не должны навсегда прятать живое фото.
        expect(swSource).toContain('response.status !== 200');
        expect(swSource).toContain('isCacheablePhotoResponse');
    });

    it('public/sw.js falls back to the network when a cache read fails', () => {
        // Отказ cache.match (квота/приватный режим) не должен ронять запрос.
        expect(swSource).toContain('matchError');
    });

    it('public/sw.js activation keeps the photo cache across SW updates', () => {
        // Снос «всего кроме offline-кеша» вытер бы накопленные тумбы при
        // каждом обновлении SW — фото-кеш в списке сохранения.
        expect(swSource).toContain('key !== PHOTO_CACHE_NAME');
    });
});
