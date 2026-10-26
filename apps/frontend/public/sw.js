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
 * PWA-изоляция (standalone-клиент не видит лендинг) живёт НЕ здесь: SW не
 * может узнать display-mode навигационного запроса, а прежний механизм с
 * IndexedDB-флагом навсегда закрывал лендинг в браузере на Android (WebAPK
 * делит хранилище профиля с вкладками; флаг писался только в true — карта
 * #1610, research #1611). Гард — инлайн-скрипт в layout лендинга
 * (apps/landing/app/layout.tsx, display-mode + navigator.standalone);
 * activate ниже лишь сносит легаси-БД флага у уже пострадавших.
 *
 * The app route prefixes below mirror
 * `apps/frontend/shared/lib/pwa/app-routes.ts` — the TS source of truth.
 * The unit test `app-routes.test.ts` guards against drift between the TS
 * source and this inline copy.
 */

const CACHE_VERSION = 'v2';
const CACHE_NAME = `rentli-offline-${CACHE_VERSION}`;
const OFFLINE_URL = '/offline.html';

// App route prefixes — keep in sync with shared/lib/pwa/app-routes.ts.
// Легаси /dashboard снесён (карта #862, тикет #865): start_url манифеста
// переведён на /properties, старые ссылки уходят в сеть как 404.
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
  '/history',
  '/notifications',
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
    Promise.all([
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
      }),
      // Снос легаси-БД флага изоляции «rentli-pwa» (standalone-store.ts
      // снесён): больше ничего её не читает, а залипший флаг у пострадавших
      // (Android WebAPK) не должен ожить при откате старого кода из
      // HTTP-кеша. Ошибки удаления не блокируют активацию.
      new Promise(function (resolve) {
        if (!('indexedDB' in self)) {
          resolve();
          return;
        }
        var del;
        try {
          del = indexedDB.deleteDatabase('rentli-pwa');
        } catch (error) {
          void error;
          resolve();
          return;
        }
        del.onsuccess = function () {
          resolve();
        };
        del.onerror = function () {
          resolve();
        };
        del.onblocked = function () {
          resolve();
        };
      }),
    ])
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

  // Only intercept navigations to app routes; landing routes and other
  // origins go straight to the network. Изоляция standalone-клиента от
  // лендинга — на странице лендинга (layout.tsx): SW не знает display-mode
  // навигации, прежний редирект снесён (карта #1610, research #1611).
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
