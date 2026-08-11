/**
 * Cabinet route prefixes — the single source of truth for which top-level paths
 * belong to the cabinet (Next.js frontend) and must be handled by the service
 * worker. Everything else (landing `/`, marketing pages, `/api`, `/_next`)
 * is treated as passthrough.
 *
 * This list mirrors the `@frontend path` matcher in the Caddyfile
 * (see `docs/deployment.md`) and the navigation items in
 * `widgets/cabinet-layout/lib/nav-items.ts`.
 *
 * The service worker (`public/sw.js`) keeps its own inline copy because a
 * static SW script cannot import TypeScript at runtime. The unit test
 * `cabinet-routes.test.ts` guards against drift between the two.
 */
export const CABINET_ROUTE_PREFIXES: ReadonlyArray<string> = [
    '/login',
    '/dashboard',
    '/properties',
    '/leases',
    '/tenants',
    '/finance',
    '/profile',
    '/subscription',
    '/support',
    '/ui-kit',
    '/calendar',
    '/reminders',
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
