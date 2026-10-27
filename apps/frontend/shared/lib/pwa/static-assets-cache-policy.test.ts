import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import {
    isCacheableStaticResponse,
    isStaticAssetCacheableRequest,
    STATIC_ASSETS_CACHE_NAME,
    STATIC_ASSET_PUBLIC_PREFIXES,
} from './static-assets-cache-policy';

describe('isStaticAssetCacheableRequest', () => {
    const cacheable = (
        pathname: string,
        search = '',
        method = 'GET',
    ): boolean => isStaticAssetCacheableRequest(method, pathname, new URLSearchParams(search));

    it('accepts direct public images and icons, nested paths included', () => {
        expect(cacheable('/images/empty-properties.webp')).toBe(true);
        expect(cacheable('/images/properties/empty.webp')).toBe(true);
        expect(cacheable('/images/notifications/category-groceries.png')).toBe(true);
        expect(cacheable('/icons/icon-192.png')).toBe(true);
        expect(cacheable('/icons/splash/splash-1290x2796.png')).toBe(true);
        expect(cacheable('/images/logo.svg')).toBe(true);
    });

    it('accepts optimizer responses whose url parameter points at the public statics', () => {
        // next/image кодирует параметр (%2F — слэш); searchParams возвращает
        // уже раскодированное значение, поэтому матчинг — по декодированному.
        expect(cacheable('/_next/image', 'url=%2Fimages%2Fempty-logo.webp&w=640&q=90')).toBe(true);
        expect(cacheable('/_next/image', 'url=%2Ficons%2Ficon-512.png&w=128&q=90')).toBe(true);
        expect(cacheable('/_next/image', 'url=/images/empty-logo.webp&w=640&q=90')).toBe(true);
    });

    it('rejects non-GET methods', () => {
        expect(cacheable('/images/logo.svg', '', 'POST')).toBe(false);
        expect(cacheable('/_next/image', 'url=%2Fimages%2Fx.png', 'HEAD')).toBe(false);
    });

    it('rejects optimizer requests pointing outside the public statics', () => {
        // remotePatterns пуст — внешних картинок через оптимизатор нет;
        // явное правило не должно задеть будущую динамику (#1600).
        expect(cacheable('/_next/image', 'url=https%3A%2F%2Fcdn.example.com%2Fx.png')).toBe(false);
        expect(cacheable('/_next/image', 'url=%2Ffonts%2Finter.woff2')).toBe(false);
        expect(cacheable('/_next/image', 'url=%2Fapi%2Fproperties%2F1%2Fphoto')).toBe(false);
        expect(cacheable('/_next/image', 'url=')).toBe(false);
        expect(cacheable('/_next/image')).toBe(false);
    });

    it('rejects lookalike and foreign paths', () => {
        expect(cacheable('/images')).toBe(false);
        expect(cacheable('/imagesfoo/x.png')).toBe(false);
        expect(cacheable('/fonts/inter.woff2')).toBe(false);
        expect(cacheable('/offline.html')).toBe(false);
        expect(cacheable('/api/me/photo')).toBe(false);
        expect(cacheable('/properties')).toBe(false);
        expect(cacheable('/sw.js')).toBe(false);
    });
});

describe('isCacheableStaticResponse', () => {
    it('accepts image responses only — a 200 error page must not be cached forever', () => {
        expect(isCacheableStaticResponse('image/webp')).toBe(true);
        expect(isCacheableStaticResponse('image/png')).toBe(true);
        expect(isCacheableStaticResponse('image/svg+xml')).toBe(true);
        expect(isCacheableStaticResponse('text/html; charset=utf-8')).toBe(false);
        expect(isCacheableStaticResponse('application/json')).toBe(false);
        expect(isCacheableStaticResponse(null)).toBe(false);
        expect(isCacheableStaticResponse('')).toBe(false);
    });
});

describe('static assets cache constants', () => {
    it('names the cache static-assets-v1 — distinct from offline and photo caches', () => {
        expect(STATIC_ASSETS_CACHE_NAME).toMatch(/^static-assets-v\d+$/);
        expect(STATIC_ASSETS_CACHE_NAME).not.toContain('photo');
        expect(STATIC_ASSETS_CACHE_NAME).not.toContain('offline');
    });

    it('limits the matcher to the public statics prefixes', () => {
        expect([...STATIC_ASSET_PUBLIC_PREFIXES]).toEqual(['/images/', '/icons/']);
    });
});

describe('service worker static assets cache stays in sync', () => {
    // Guards against drift between the TS source of truth and the inline copy
    // kept in public/sw.js (which cannot import TS at runtime). Mirrors the
    // photo-cache-policy.test.ts sync guard.
    const swPath = resolve(process.cwd(), 'public/sw.js');
    const swSource = readFileSync(swPath, 'utf8');

    // Тело одной функции sw.js по балансу фигурных скобок — для срез-based
    // проверок веток, чьи маркеры иначе пересекались бы с фото-веткой.
    const sliceFunction = (source: string, name: string): string => {
        const start = source.indexOf(`function ${name}(`);
        expect(start, `function ${name} not found in sw.js`).toBeGreaterThanOrEqual(0);
        const bodyStart = source.indexOf('{', start);
        let depth = 0;
        for (let i = bodyStart; i < source.length; i += 1) {
            if (source[i] === '{') depth += 1;
            if (source[i] === '}') {
                depth -= 1;
                if (depth === 0) {
                    return source.slice(start, i + 1);
                }
            }
        }
        return source.slice(start);
    };

    it('public/sw.js mirrors the cache name', () => {
        expect(swSource).toContain(`var STATIC_ASSETS_CACHE_NAME = '${STATIC_ASSETS_CACHE_NAME}'`);
    });

    it('public/sw.js mirrors the public prefixes list', () => {
        const start = swSource.indexOf('var STATIC_ASSET_PUBLIC_PREFIXES = [');
        expect(start, 'prefix list marker not found in sw.js').toBeGreaterThanOrEqual(0);
        const end = swSource.indexOf(']', start);
        const swPrefixes = Array.from(
            swSource.slice(start, end).matchAll(/'([^']+)'/g),
            (match) => match[1] ?? '',
        );
        expect(swPrefixes).toEqual([...STATIC_ASSET_PUBLIC_PREFIXES]);
    });

    it('public/sw.js wires the cache-first branch before the navigate branch', () => {
        expect(swSource).toContain('isStaticAssetCacheableRequest');
        expect(swSource).toContain('staticAssetCacheFirst');
        expect(swSource).toContain('isCacheableStaticResponse');
    });

    it('public/sw.js caches only successful image responses in the static helper', () => {
        // 404/500/ошибочные страницы не должны навсегда прятать живой ассет.
        // Срез по телу хелпера — глобальный toContain всегда нашёл бы
        // фото-ветку и прошёл бы впустую.
        const helper = sliceFunction(swSource, 'fetchAndCacheStaticAsset');
        expect(helper).toContain('response.status !== 200');
        expect(helper).toContain('isCacheableStaticResponse');
    });

    it('public/sw.js falls back to the network when a static cache read fails', () => {
        // Отказ cache.match (квота/приватный режим) не должен ронять запрос.
        expect(sliceFunction(swSource, 'staticAssetCacheFirst')).toContain('matchError');
    });

    it('public/sw.js creates the cache on install', () => {
        expect(swSource).toContain('caches.open(STATIC_ASSETS_CACHE_NAME)');
    });

    it('public/sw.js activation keeps the static cache across SW updates', () => {
        // Снос «всего кроме offline-кеша» стёр бы накопленные иллюстрации при
        // каждом обновлении SW — статик-кеш в списке сохранения.
        expect(swSource).toContain('key !== STATIC_ASSETS_CACHE_NAME');
    });

    it('public/sw.js never clears the static cache on logout — it is account-independent', () => {
        // Чистка при логауте — только приватный фото-кеш; публичная статика
        // одинакова для любого аккаунта (#1600).
        expect(swSource).not.toContain('caches.delete(STATIC_ASSETS_CACHE_NAME)');
    });

    it('public/sw.js has no eviction for the static cache — the set is finite', () => {
        // Десятки вариантов на устройство (сотни КБ) — квоты и eviction нет.
        expect(swSource).not.toContain('STATIC_ASSETS_MAX_ENTRIES');
        expect(swSource).not.toContain('trimStatic');
    });
});
