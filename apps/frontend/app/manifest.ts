import type { MetadataRoute } from 'next';

/**
 * Web App Manifest for the app PWA.
 *
 * Served at `/manifest.webmanifest`; `<link rel="manifest">` is added by Next.js
 * automatically. Decisions are documented in issue #179 and the research note
 * `docs/research/pwa-manifest-installability.md`.
 *
 * `id` and `start_url` MUST NOT change after publication — changing them breaks
 * recognition of already-installed apps (web.dev, Web app manifest).
 * start_url historically pointed at легаси /dashboard; маршрут снесён
 * (карта #862, тикет #865), дом кабинета — /properties. Публикации не было
 * (пуш = деплой stage не делался), поэтому смена безопасна.
 */
export default function manifest(): MetadataRoute.Manifest {
    return {
        id: '/',
        name: 'Рентли',
        short_name: 'Рентли',
        description: 'Управление арендной недвижимостью',
        lang: 'ru',
        start_url: '/properties',
        scope: '/',
        display: 'standalone',
        // White so the macOS PWA title-bar strip behind the traffic-light
        // buttons renders white instead of the brand blue. Pairs with the
        // theme-color meta (app/layout.tsx viewport.themeColor) and
        // background_color below. Android/iOS also pick this up.
        theme_color: '#ffffff',
        background_color: '#ffffff',
        // URLs carry a ?v=YYYYMMDD version: /icons/* is served with
        // `Cache-Control: immutable` for a year (next.config.ts), so changing
        // the art under the same URL never reaches clients — browser caches
        // and installed WebAPKs keep the old bytes, and reinstalling the PWA
        // does not clear Chrome's HTTP cache. New art = bump ?v= here and in
        // the push-icon consumers (shared/lib/pwa/push-payload.ts,
        // public/sw.js); guard test — shared/lib/pwa/icon-urls.test.ts.
        icons: [
            {
                src: '/icons/icon-192.png?v=20261028',
                sizes: '192x192',
                type: 'image/png',
                purpose: 'any',
            },
            {
                src: '/icons/icon-512.png?v=20261028',
                sizes: '512x512',
                type: 'image/png',
                purpose: 'any',
            },
            {
                // Android Adaptive Icons — opaque square, content in the 80%
                // safe zone. Assembled from the master
                // assets/icons/source-maskable-android.png (downscale
                // 1024→512). Current process and file mapping —
                // assets/icons/README.md.
                src: '/icons/icon-maskable-512.png?v=20261028',
                sizes: '512x512',
                type: 'image/png',
                purpose: 'maskable',
            },
            {
                // Android 13+ Material You themed icons — house silhouette only,
                // system tints it with the wallpaper color. Glass effects cannot
                // survive the tint, so this is a separate source. The same file
                // is the Android push-notification badge (PUSH_BADGE_PATH in
                // shared/lib/pwa/push-payload.ts and public/sw.js) — the badge
                // is also rendered as a solid silhouette (owner decision
                // 28.10.2026: one art for both roles).
                src: '/icons/icon-monochrome-512.png?v=20261028',
                sizes: '512x512',
                type: 'image/png',
                purpose: 'monochrome',
            },
        ],
    };
}
