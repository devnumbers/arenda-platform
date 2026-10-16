/**
 * App route prefixes — the single source of truth for which top-level paths
 * belong to the app (Next.js frontend) and must be handled by the service
 * worker. Everything else (landing `/`, marketing pages, `/api`, `/_next`)
 * is treated as passthrough.
 *
 * In Caddy the app has no per-path matcher of its own: the cabinet is the
 * catch-all complement of the `@landing` exceptions (see the routing
 * contract in `docs/deployment.md` and `deploy/caddy/rentlee.caddy`), so
 * this list is not a proxy mirror. The top-level surface is the unified
 * chrome (map #556): global sections from `shared/config/navigation.ts`
 * plus auth/subscription/profile and utility routes.
 *
 * The list is mirrored by the inline copy in the service worker
 * (`public/sw.js` — a static SW script cannot import TypeScript at
 * runtime) and by the `Disallow` block of the landing `robots.txt`; the
 * `proxy.ts` /me-gate matcher covers every prefix except `/subscription`
 * (one-way coverage — see `app-routes.test.ts`).
 */
export const APP_ROUTE_PREFIXES: ReadonlyArray<string> = [
    '/login',
    '/properties',
    '/profile',
    '/subscription',
    '/ui-kit',
    // Глобальные разделы единого хрома (карта #556): ленты и заглушки.
    '/tasks',
    '/operations',
    '/contacts',
    '/payments',
    '/participants',
    '/history',
    '/notifications',
];

/** The offline fallback page precached by the service worker. */
export const OFFLINE_URL = '/offline.html';

/**
 * Returns true when `pathname` belongs to an app route — i.e. it starts with
 * one of the registered app prefixes. Query string and hash are ignored.
 */
export function isAppRoute(pathname: string): boolean {
    const cleanPath = pathname.split('?')[0]?.split('#')[0] ?? pathname;
    return APP_ROUTE_PREFIXES.some((prefix) => cleanPath === prefix || cleanPath.startsWith(`${prefix}/`));
}
