import type { MetadataRoute } from 'next';

/**
 * Web App Manifest for the cabinet PWA.
 *
 * Served at `/manifest.webmanifest`; `<link rel="manifest">` is added by Next.js
 * automatically. Decisions are documented in issue #179 and the research note
 * `docs/research/pwa-manifest-installability.md`.
 *
 * `id` and `start_url` MUST NOT change after publication — changing them breaks
 * recognition of already-installed apps (web.dev, Web app manifest).
 */
export default function manifest(): MetadataRoute.Manifest {
    return {
        id: '/',
        name: 'Рентли',
        short_name: 'Рентли',
        description: 'Управление арендной недвижимостью',
        lang: 'ru',
        start_url: '/dashboard',
        scope: '/',
        display: 'standalone',
        theme_color: '#2b7fff',
        background_color: '#ffffff',
        icons: [
            {
                src: '/icons/icon-192.png',
                sizes: '192x192',
                type: 'image/png',
                purpose: 'any',
            },
            {
                src: '/icons/icon-512.png',
                sizes: '512x512',
                type: 'image/png',
                purpose: 'any',
            },
        ],
    };
}
