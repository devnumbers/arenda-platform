/*
 * Рентли cabinet service worker.
 *
 * Scope: "/" (script lives at /sw.js, so the default scope is the origin root).
 * This means the SW becomes the active controller for the whole origin once
 * registered, including the landing pages. To stay safe, the fetch handler
 * strictly filters by path and only intercepts cabinet navigations — everything
 * else (landing routes, /api, /_next static, fonts, images) is passed through
 * to the network untouched. See docs/research/pwa-manifest-installability.md
 * §4 "Подводные камни scope на общем origin с лендингом".
 *
 * The cabinet route prefixes below mirror
 * `apps/frontend/shared/lib/pwa/cabinet-routes.ts` and the `@frontend path`
 * matcher in the Caddyfile (docs/deployment.md). The unit test
 * `cabinet-routes.test.ts` guards against drift between the TS source and this
 * inline copy.
 */

const CACHE_VERSION = 'v1';
const CACHE_NAME = `rentli-offline-${CACHE_VERSION}`;
const OFFLINE_URL = '/offline.html';

// Cabinet route prefixes — keep in sync with shared/lib/pwa/cabinet-routes.ts.
const CABINET_ROUTE_PREFIXES = [
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

function isCabinetPath(pathname) {
  return CABINET_ROUTE_PREFIXES.some(function (prefix) {
    return pathname === prefix || pathname.startsWith(prefix + '/');
  });
}

self.addEventListener('install', function (event) {
  event.waitUntil(
    caches.open(CACHE_NAME).then(function (cache) {
      return cache.add(OFFLINE_URL);
    })
  );
});

self.addEventListener('activate', function (event) {
  event.waitUntil(
    caches.keys().then(function (keys) {
      return Promise.all(
        keys
          .filter(function (key) {
            return key !== CACHE_NAME;
          })
          .map(function (key) {
            return caches.delete(key);
          })
      );
    })
  );
  self.clients.claim();
});

self.addEventListener('fetch', function (event) {
  const request = event.request;

  // Only handle navigation (document) requests — leave API calls, static
  // assets, fonts, images, and everything else to the network.
  if (request.mode !== 'navigate') {
    return;
  }

  const url = new URL(request.url);

  // Only intercept navigations to cabinet routes; landing routes and other
  // origins go straight to the network.
  if (url.origin !== self.location.origin || !isCabinetPath(url.pathname)) {
    return;
  }

  // Network-first for cabinet navigations; on network failure (offline), serve
  // the branded offline page from cache.
  event.respondWith(
    fetch(request)
      .then(function (response) {
        return response;
      })
      .catch(function () {
        return caches.match(OFFLINE_URL).then(function (cached) {
          return cached ?? Response.error();
        });
      })
  );
});

/*
 * Push and notificationclick handlers are stubs for the Web Push ticket (#7).
 * They are intentionally no-ops here so the SW stays forward-compatible with
 * the push feature without implementing it prematurely.
 */
self.addEventListener('push', function (event) {
  // Implemented in the Web Push ticket.
});

self.addEventListener('notificationclick', function (event) {
  // Implemented in the Web Push ticket.
});

self.addEventListener('message', function (event) {
  // Allow the page to trigger an immediate SW takeover on update.
  if (event.data && event.data.type === 'SKIP_WAITING') {
    self.skipWaiting();
  }
});
