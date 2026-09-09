/**
 * App route prefixes — the single source of truth for which top-level paths
 * belong to the app (Next.js frontend) and must be handled by the service
 * worker. Everything else (landing `/`, marketing pages, `/api`, `/_next`)
 * is treated as passthrough.
 *
 * This list mirrors the `@frontend path` matcher in the Caddyfile
 * (see `docs/deployment.md`); the top-level surface is the unified chrome
 * (map #556): global sections from `shared/config/navigation.ts` plus
 * auth/subscription/profile and utility routes.
 *
 * The service worker (`public/sw.js`) keeps its own inline copy because a
 * static SW script cannot import TypeScript at runtime. The unit test
 * `cabinet-routes.test.ts` guards against drift between the two.
 */
export const CABINET_ROUTE_PREFIXES: ReadonlyArray<string> = [
    '/login',
    '/properties',
    '/profile',
    '/subscription',
    '/support',
    '/ui-kit',
    // Глобальные разделы единого хрома (карта #556): ленты и заглушки.
    '/tasks',
    '/operations',
    '/contacts',
    '/payments',
    '/participants',
    // /dashboard — постоянный редирект на /properties (PWA start_url и старые
    // ссылки); роут живой, поэтому остаётся в списке сервис-воркера.
    '/dashboard',
];

/** The offline fallback page precached by the service worker. */
export const OFFLINE_URL = '/offline.html';

/**
 * Returns true when `pathname` belongs to a cabinet route — i.e. it starts with
 * one of the registered cabinet prefixes. Query string and hash are ignored.
 */
export function isCabinetRoute(pathname: string): boolean {
    const cleanPath = pathname.split('?')[0]?.split('#')[0] ?? pathname;
    return CABINET_ROUTE_PREFIXES.some((prefix) => cleanPath === prefix || cleanPath.startsWith(`${prefix}/`));
}
