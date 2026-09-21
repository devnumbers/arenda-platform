/*
 * Рентли app service worker.
 *
 * Scope: "/" (script lives at /sw.js, so the default scope is the origin root).
 * This means the SW becomes the active controller for the whole origin once
 * registered, including the landing pages. To stay safe, the fetch handler
 * strictly filters by path and only intercepts app navigations — everything
 * else (landing routes, /api, /_next static, fonts, images) is passed through
 * to the network untouched. See docs/research/pwa-manifest-installability.md
 * §4 "Подводные камни scope на общем origin с лендингом".
 *
 * The app route prefixes below mirror
 * `apps/frontend/shared/lib/pwa/app-routes.ts` and the `@frontend path`
 * matcher in the Caddyfile (docs/deployment.md). The unit test
 * `app-routes.test.ts` guards against drift between the TS source and this
 * inline copy.
 */

const CACHE_VERSION = 'v2';
const CACHE_NAME = `rentli-offline-${CACHE_VERSION}`;
const OFFLINE_URL = '/offline.html';

// Standalone-flag persistence — the page writes the flag via
// `shared/lib/pwa/standalone-store.ts` and the SW reads it here to redirect
// PWA navigations to `/` back into the app. Constants MUST match
// standalone-store.ts; the guard test `standalone-store marker sync with
// service worker` catches drift on all three markers (DB name, store, key).
var STANDALONE_DB_NAME = 'rentli-pwa';
var STANDALONE_DB_STORE = 'pwa';
var STANDALONE_DB_KEY = 'standalone';

// Reads the standalone flag. Resolves `false` on any error / missing DB so a
// broken IndexedDB never blocks navigation — the worst case is the redirect
// not firing and the user seeing the landing once.
function readStandaloneFlag() {
  return new Promise(function (resolve) {
    if (!('indexedDB' in self)) {
      resolve(false);
      return;
    }
    var open;
    try {
      open = indexedDB.open(STANDALONE_DB_NAME, 1);
    } catch (error) {
      void error;
      resolve(false);
      return;
    }
    open.onupgradeneeded = function () {
      var db = open.result;
      if (!db.objectStoreNames.contains(STANDALONE_DB_STORE)) {
        db.createObjectStore(STANDALONE_DB_STORE);
      }
    };
    open.onsuccess = function () {
      var db = open.result;
      try {
        var tx = db.transaction(STANDALONE_DB_STORE, 'readonly');
        var store = tx.objectStore(STANDALONE_DB_STORE);
        var req = store.get(STANDALONE_DB_KEY);
        req.onsuccess = function () {
          db.close();
          resolve(req.result === true);
        };
        req.onerror = function () {
          db.close();
          resolve(false);
        };
      } catch (error) {
        void error;
        db.close();
        resolve(false);
      }
    };
    open.onerror = function () {
      resolve(false);
    };
  });
}

// App route prefixes — keep in sync with shared/lib/pwa/app-routes.ts.
// /dashboard остаётся: это постоянный редирект на /properties, зашитый в
// start_url манифеста PWA.
const APP_ROUTE_PREFIXES = [
  '/login',
  '/properties',
  '/profile',
  '/subscription',
  '/ui-kit',
  '/tasks',
  '/operations',
  '/contacts',
  '/payments',
  '/participants',
  '/dashboard',
];

function isAppPath(pathname) {
  return APP_ROUTE_PREFIXES.some(function (prefix) {
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

  // PWA hard-isolation: when an installed-PWA client lands on `/` (via
  // history, an external link, or manual URL entry), bounce it back into the
  // app. The standalone flag is written by the page after SW registration
  // (see ServiceWorkerRegister.tsx + standalone-store.ts). Non-standalone
  // (browser) clients fall through and see the landing as normal.
  if (url.origin === self.location.origin && url.pathname === '/') {
    event.respondWith(
      readStandaloneFlag().then(function (isStandalone) {
        if (isStandalone) {
          return Response.redirect('/properties', 302);
        }
        // Not a PWA client — let the request go to the network (landing).
        return fetch(request);
      })
    );
    return;
  }

  // Only intercept navigations to app routes; landing routes and other
  // origins go straight to the network.
  if (url.origin !== self.location.origin || !isAppPath(url.pathname)) {
    return;
  }

  // Network-first for app navigations; on network failure (offline), serve
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
 * Web Push handlers (push + notificationclick).
 *
 * The payload parsing, option building, and click-target resolution logic
 * below is an inline copy of the pure functions in
 * `apps/frontend/shared/lib/pwa/push-payload.ts`. The SW is a static script
 * served from /sw.js and cannot import TypeScript at runtime, so it keeps its
 * own copy; the guard test in `push-payload.test.ts` ("service worker handlers
 * stay in sync") fails if the two drift on the key markers (showNotification,
 * clients.openWindow, default click url, branded icon). Keep the logic in sync
 * when changing behaviour.
 */

// Fallbacks — must mirror DEFAULT_PUSH_TITLE / DEFAULT_PUSH_CLICK_URL in
// shared/lib/pwa/push-payload.ts.
var DEFAULT_PUSH_TITLE = 'Рентли';
var DEFAULT_PUSH_CLICK_URL = '/properties';
var DEFAULT_PUSH_TAG = 'rentli-notification';
var PUSH_ICON_PATH = '/icons/icon-192.png';

// Inline copy of resolveClickTarget: only same-origin absolute paths are kept,
// everything else falls back to the default. Stops a hostile or malformed
// payload from opening an arbitrary page on tap.
function resolveClickTarget(url) {
  if (typeof url !== 'string' || url.length === 0) {
    return DEFAULT_PUSH_CLICK_URL;
  }
  if (url[0] !== '/' || url[1] === '/' || url[1] === '\\') {
    return DEFAULT_PUSH_CLICK_URL;
  }
  return url;
}

// Inline copy of parsePushPayload + buildShowNotificationOptions.
function parsePushPayload(eventData) {
  if (eventData == null || eventData === '') {
    return null;
  }
  var parsed;
  try {
    parsed = JSON.parse(eventData);
  } catch (error) {
    // Invalid JSON — caller (push handler) shows a default notification.
    void error;
    return null;
  }
  if (!parsed || typeof parsed !== 'object') {
    return null;
  }
  var title = typeof parsed.title === 'string' ? parsed.title : DEFAULT_PUSH_TITLE;
  var body = typeof parsed.body === 'string' ? parsed.body : '';
  var tag =
    typeof parsed.tag === 'string' && parsed.tag.length > 0
      ? parsed.tag
      : DEFAULT_PUSH_TAG;
  var url = resolveClickTarget(parsed.url);
  return { title: title, body: body, tag: tag, url: url };
}

self.addEventListener('push', function (event) {
  // A visible notification is mandatory on every push event: Chrome otherwise
  // shows a generic "updated in background" system notice, and Safari revokes
  // the push permission after a silent push.
  var payload = parsePushPayload(event.data ? event.data.text() : null);
  var title = payload ? payload.title : DEFAULT_PUSH_TITLE;
  var options = {
    body: payload ? payload.body : '',
    tag: payload ? payload.tag : DEFAULT_PUSH_TAG,
    icon: PUSH_ICON_PATH,
    badge: PUSH_ICON_PATH,
    data: { url: payload ? payload.url : DEFAULT_PUSH_CLICK_URL },
  };
  event.waitUntil(self.registration.showNotification(title, options));
});

// Focus an existing same-origin client that shows the app, or open one.
// postMessage lets the React app perform client-side navigation when a window
// is already open on a different cabinet route. Mirrors pickClickTargetClient
// in shared/lib/pwa/push-payload.ts.
function focusOrOpenClient(url) {
  return self.clients
    .matchAll({ type: 'window', includeUncontrolled: true })
    .then(function (clientList) {
      for (var i = 0; i < clientList.length; i += 1) {
        var client = clientList[i];
        if (
          typeof client.url === 'string' &&
          client.url.indexOf(self.location.origin) === 0 &&
          'focus' in client
        ) {
          // Ask the page to navigate; the SW itself cannot use the router.
          client.postMessage({ type: 'PUSH_NOTIFICATION_CLICK', url: url });
          return client.focus();
        }
      }
      if (self.clients.openWindow) {
        return self.clients.openWindow(url);
      }
      return undefined;
    });
}

self.addEventListener('notificationclick', function (event) {
  event.notification.close();
  var url = resolveClickTarget(
    event.notification && event.notification.data ? event.notification.data.url : undefined,
  );
  event.waitUntil(focusOrOpenClient(url));
});

self.addEventListener('message', function (event) {
  // Allow the page to trigger an immediate SW takeover on update.
  if (event.data && event.data.type === 'SKIP_WAITING') {
    self.skipWaiting();
  }
});
