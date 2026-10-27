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
 *
 * Приватный photo-thumb кеш (карта #1587, тикет #1608; решение #1593):
 * cache-first для версионных thumb-выдач ADR 0065 — повторные показы
 * после полной перезагрузки и «Назад» приходят из Cache Storage, не по
 * сети (в том числе офлайн и после вытеснения HTTP-кеша Safari ITP).
 * Кеш приватный: логаут чистит его всегда, логин (verify) — при успехе,
 * фото одного аккаунта не достаются другому. Matching/policy logic below
 * is an inline copy of the pure functions in
 * `shared/lib/pwa/photo-cache-policy.ts`; the guard test
 * `photo-cache-policy.test.ts` fails if the two drift on the key markers
 * (cache name, entry cap, photo scopes, auth clear paths). Keep the logic
 * in sync when changing behaviour. Хранилище не пересекается с offline-
 * кешем и с будущим публичным `static-assets-v1` (#1600).
 */

const CACHE_VERSION = 'v2';
const CACHE_NAME = `rentli-offline-${CACHE_VERSION}`;
const OFFLINE_URL = '/offline.html';

// Photo-thumb cache constants — must mirror photo-cache-policy.ts.
var PHOTO_CACHE_NAME = 'photo-thumbs-v1';
var PHOTO_CACHE_MAX_ENTRIES = 300;
var PHOTO_CACHE_SCOPES = ['properties', 'contacts', 'users'];
var PHOTO_CACHE_CLEAR_ALWAYS_PATH = '/api/auth/logout';
var PHOTO_CACHE_CLEAR_ON_SUCCESS_PATH = '/api/auth/verify';

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
              // Offline-кеш текущей версии и приватный фото-кеш (переживает
              // обновления SW; смена его версии — бамп PHOTO_CACHE_NAME,
              // старый уедет сюда же на снос) сохраняются; прочие — легаси.
              return key !== CACHE_NAME && key !== PHOTO_CACHE_NAME;
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
  const url = new URL(request.url);

  if (url.origin === self.location.origin) {
    // Приватный photo-thumb кеш: cache-first для версионных тумбов
    // (photo-cache-policy.ts — источник правил).
    if (isPhotoThumbCacheableRequest(request.method, url.pathname, url.searchParams)) {
      event.respondWith(photoThumbCacheFirst(request, url));
      return;
    }
    // Логаут/логин: passthrough с очисткой приватного фото-кеша.
    if (photoCacheClearPolicy(request.method, url.pathname) !== 'never') {
      event.respondWith(authRequestClearingPhotoCache(request, url.pathname));
      return;
    }
  }

  // Only handle navigation (document) requests — leave API calls, static
  // assets, fonts, images, and everything else to the network.
  if (request.mode !== 'navigate') {
    return;
  }

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
 * Private photo-thumb cache helpers (карта #1587, тикет #1608).
 *
 * The three policy functions below are an inline copy of the pure functions
 * in `shared/lib/pwa/photo-cache-policy.ts` (same names, same behaviour);
 * the guard test `photo-cache-policy.test.ts` fails on drift between the
 * copies — keep them in sync. All Cache Storage operations degrade to a
 * plain network pass-through on failure (private browsing, quota).
 */

// Inline copy of isPhotoStreamPath: the ADR 0065 photo stream shapes —
// /api/me/photo and /api/<scope>/<id>/photo for PHOTO_CACHE_SCOPES.
function isPhotoStreamPath(pathname) {
  var segments = pathname.split('/').filter(function (segment) {
    return segment !== '';
  });
  if (segments.length === 3) {
    return segments[0] === 'api' && segments[1] === 'me' && segments[2] === 'photo';
  }
  if (segments.length === 4) {
    return (
      segments[0] === 'api' &&
      PHOTO_CACHE_SCOPES.indexOf(segments[1]) !== -1 &&
      segments[3] === 'photo'
    );
  }
  return false;
}

// Inline copy of isPhotoThumbCacheableRequest: GET + size=thumb + non-empty
// versioned ?v= (cache-first is safe only for versioned URLs — photo
// replacement is a new ?v=, so cached bytes never go stale).
function isPhotoThumbCacheableRequest(method, pathname, searchParams) {
  if (method !== 'GET') {
    return false;
  }
  if (searchParams.get('size') !== 'thumb') {
    return false;
  }
  if ((searchParams.get('v') ?? '') === '') {
    return false;
  }
  return isPhotoStreamPath(pathname);
}

// Inline copy of photoThumbCacheKey: the CirclePhoto retry nonce (&r=N) is
// stripped — a successful retry becomes the canonical cached thumb.
function photoThumbCacheKey(href) {
  var url;
  try {
    url = new URL(href);
  } catch (error) {
    void error;
    return href;
  }
  if (!url.searchParams.has('r')) {
    return href;
  }
  url.searchParams.delete('r');
  return url.href;
}

// Inline copy of photoThumbRequestSkipsCache: retry requests bypass
// cache.match — a retry must re-ask the network, otherwise bad cached
// bytes can never heal (двухударный фолбэк #1275). The successful retry
// result is still cached under the stripped key.
function photoThumbRequestSkipsCache(searchParams) {
  return searchParams.has('r');
}

// Inline copy of isCacheablePhotoResponse: only image responses are stored —
// a proxy error page with 200 and text/html must not hide a living photo
// behind a permanent cache entry.
function isCacheablePhotoResponse(contentType) {
  return contentType != null && contentType.indexOf('image/') === 0;
}

// Inline copy of photoCacheClearPolicy: logout clears unconditionally,
// login (verify) clears on success, everything else leaves the cache alone.
function photoCacheClearPolicy(method, pathname) {
  if (method !== 'POST') {
    return 'never';
  }
  if (pathname === PHOTO_CACHE_CLEAR_ALWAYS_PATH) {
    return 'always';
  }
  if (pathname === PHOTO_CACHE_CLEAR_ON_SUCCESS_PATH) {
    return 'on-success';
  }
  return 'never';
}

function photoThumbCacheFirst(request, url) {
  var opened;
  try {
    opened = caches.open(PHOTO_CACHE_NAME);
  } catch (error) {
    void error;
    return fetch(request);
  }
  return opened.then(
    function (cache) {
      var keyRequest = new Request(photoThumbCacheKey(url.href));
      // Ретрай с нонсом — сразу в сеть (мимо cache.match), см. выше.
      if (photoThumbRequestSkipsCache(url.searchParams)) {
        return fetchAndCacheThumb(request, cache, keyRequest);
      }
      return cache.match(keyRequest).then(
        function (cached) {
          if (cached) {
            return cached;
          }
          return fetchAndCacheThumb(request, cache, keyRequest);
        },
        function (matchError) {
          // Ошибка чтения кеша (квота, приватный режим) — сеть как обычно.
          void matchError;
          return fetchAndCacheThumb(request, cache, keyRequest);
        }
      );
    },
    function (openError) {
      // Cache Storage недоступен — сеть как обычно (сегодняшнее поведение).
      void openError;
      return fetch(request);
    }
  );
}

// fetch + запись в кеш: только 200 с image/-контентом (404/500 и ошибочные
// страницы не должны навсегда прятать живое фото — фолбэк #1275); запись и
// trim не могут уронить сам запрос (квота, приватный режим).
function fetchAndCacheThumb(request, cache, keyRequest) {
  return fetch(request).then(function (response) {
    if (
      !response ||
      response.status !== 200 ||
      !isCacheablePhotoResponse(response.headers.get('content-type'))
    ) {
      return response;
    }
    return cache
      .put(keyRequest, response.clone())
      .then(function () {
        return trimPhotoCache(cache);
      })
      .then(function () {
        return response;
      })
      .catch(function (putError) {
        void putError;
        return response;
      });
  });
}

// FIFO-эвикция по порядку вставки Cache API: записи иммутабельны (ключ —
// версионный URL), LRU-семантика не нужна; только что положенная запись —
// в хвосте и остаётся.
function trimPhotoCache(cache) {
  return cache.keys().then(function (keys) {
    var excess = keys.length - PHOTO_CACHE_MAX_ENTRIES;
    if (excess <= 0) {
      return undefined;
    }
    return Promise.all(
      keys.slice(0, excess).map(function (key) {
        return cache.delete(key);
      })
    );
  });
}

// Логаут чистит фото-кеш при любом завершённом ответе (4xx/5xx тоже —
// перестраховка в приватную сторону); логин — только при успехе. Сетевой
// провал сессию не закрывает и чистки не делает. Один SW на origin —
// очистка накрывает все вкладки.
function authRequestClearingPhotoCache(request, pathname) {
  var policy = photoCacheClearPolicy(request.method, pathname);
  return fetch(request).then(function (response) {
    var shouldClear = policy === 'always' || (policy === 'on-success' && response.ok);
    if (!shouldClear) {
      return response;
    }
    return caches
      .delete(PHOTO_CACHE_NAME)
      .catch(function (deleteError) {
        void deleteError;
      })
      .then(function () {
        return response;
      });
  });
}

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
