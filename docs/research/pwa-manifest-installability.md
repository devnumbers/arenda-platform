# Installable PWA кабинета: манифест, installability, iOS, service worker (Next.js 16)

Дата: 2026-08-10
Вход для: тикет https://github.com/devnumbers/arenda-platform/issues/173 (installable PWA для кабинета `apps/frontend`; Web Push — отдельный тикет, здесь только то, что нужно заложить в SW).
Метод: первичные источники — W3C/MDN/web.dev (Learn PWA), WebKit Blog, Microsoft Learn, документация Next.js и Apple; ключевые утверждения сверены минимум по двум независимым источникам. Неподтверждённое помечено ⚠️.

## Резюме

- Для installability в Chrome/Edge сегодня **service worker не требуется**: достаточно HTTPS + манифест с `name`/`short_name`, иконками 192 и 512 px, `start_url`, `display: standalone|fullscreen|minimal-ui|window-controls-overlay`. Требование offline-capable SW анонсировали в Chrome 93 (2021) и отменили в том же году; актуальные критерии SW не содержат. SW нам всё равно нужен — под будущий Web Push и branded offline-экран, но не ради кнопки «Установить».
- На iOS никакого авто-промпта нет и не будет: установка только вручную через Share → «На экран Домой». Чтобы сайт стал Home Screen web app (standalone), нужен манифест с `display: standalone|fullscreen` **или** capable-мета-тег. Сплеш-скрин на iOS генерируется не из манифеста, а из `apple-touch-startup-image` — этот механизм актуален и в 2025.
- Главный проектный подводный камень — общий origin с лендингом: SW в scope `/` будет контролировать и страницы лендинга (fetch-события пойдут через него), а `/manifest.webmanifest` и `/sw.js` по текущему Caddyfile упадут на лендинг-контейнер, пока их не добавить в `@frontend path` (см. раздел «Контекст проекта»).
- Манифест подаём через `app/manifest.ts` (тип `MetadataRoute.Manifest`), SW — статикой из `public/`, apple-теги — через `appleWebApp`/`icons` в Metadata API. Ничего PWA-специфичного в Next.js 16 не появилось.

## Контекст проекта: общий origin с лендингом

Фактура из `docs/deployment.md` (раздел «Caddyfile», :232-307):

- Origin `rentlee.ru` (prod) и `dev.rentlee.ru` (stage) обслуживает Caddy на хосте: `handle_path /api/*` и `/webhooks/*` → backend, пути кабинета (`/login*`, `/dashboard*`, `/properties*`, `/leases*`, `/tenants*`, `/finance*`, `/profile*`, `/subscription*`, `/support*`, `/ui-kit*`, `/calendar*`, `/reminders*`, `/_next/*`, `/fonts/*`, `/images/*`, статические svg и `/icon.png`) → Next.js-фронт (`docs/deployment.md:250-253`), **всё остальное — fallback на лендинг** (`docs/deployment.md:255-257`).
- Прямо в доке зафиксировано правило: «При добавлении нового top-level роута в Next.js-фронт его нужно добавить в `@frontend path` и перечитать Caddy» (`docs/deployment.md:306-307`). Следствие для тикета: пути манифеста (`/manifest.webmanifest`), SW-скрипта (`/sw.js` или `/pwa/*`), PWA-иконок и offline-страницы **надо добавить в `@frontend path` в обоих блоках (prod и stage)**, иначе они уйдут на лендинг-контейнер (nginx, `127.0.0.1:13002/23002`, `docs/deployment.md:44`).
- Во фронте `next@16.2.11`, `react@19.2.4` (`apps/frontend/package.json`). Уже есть `apps/frontend/app/icon.png` (файловая конвенция Next → `<link rel="icon">`); `apple-icon`, манифеста и SW нет.

## 1. Web App Manifest

### Обязательный минимум (по критериям Chrome, см. раздел 2)

По спецификации все члены манифеста опциональны ([MDN, Web app manifests](https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Manifest)), но для установки практически обязательны:

- `name` и/или `short_name` (Chrome принимает любое из двух; `short_name` держать ≤ 12 символов, чтобы не обрезался на домашнем экране — [web.dev, Web app manifest](https://web.dev/learn/pwa/web-app-manifest));
- `icons` — минимум 192×192 и 512×512;
- `start_url`;
- `display`: `standalone` (наш случай), `fullscreen`, `minimal-ui` или `window-controls-overlay`;
- `prefer_related_applications` — отсутствует или `false`.

### Поля

- **`id`** — строка-идентификатор PWA, уникальная в пределах origin; если не задан, равен `start_url`. Зафиксировать явно (`"id": "/"`), иначе смена `start_url` (например, добавление query) сломает распознавание уже установленного приложения ([web.dev, Web app manifest](https://web.dev/learn/pwa/web-app-manifest)). iOS использует `id` для синхронизации Focus-настроек с 16.4 ([WebKit Blog](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/)).
- **`start_url`** — рекомендован абсолютный путь (`/dashboard` для кабинета; `/` попадёт на лендинг!). Если не задан, браузер возьмёт URL страницы, с которой установили (может быть deep link).
- **`scope`** — навигационный scope установленного приложения: переходы вне него открываются во встроенном браузере с browser UI. **Не влияет на scope service worker'а** ([web.dev, Web app manifest](https://web.dev/learn/pwa/web-app-manifest)). У нас кабинет — десяток top-level роутов, одним префиксом уже `/` не сузить; `scope: "/"` с `start_url: "/dashboard"` — корректный вариант. Побочный эффект: ссылка на лендинг внутри установленного PWA откроется в том же окне приложения (лендинг тоже в scope) — приемлемо.
- **`display`** — `standalone`. **`display_override`** — массив режимов, которые браузер перебирает до `display` (пример: `["window-controls-overlay", "standalone"]`); значения: `browser`, `fullscreen`, `minimal-ui`, `standalone`, `tabbed`, `window-controls-overlay` ([MDN, display_override](https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Manifest/Reference/display_override)). Для первой итерации не нужен.
- **`theme_color`** — цвет title bar на десктопе и статус-бара на Android; переопределяется `<meta name="theme-color">`. **`background_color`** — фон сплеша до загрузки стилей; Safari на iOS его игнорирует ([web.dev, Web app manifest](https://web.dev/learn/pwa/web-app-manifest)). Цвета — имена CSS, hex, `rgb()`/`hsl()` **без альфа-канала**.
- **`description`, `lang`, `dir`, `orientation`, `categories`** — опционально; `description` + `screenshots` включают «богатый» install-диалог на Android (см. ниже). `dir`/`lang`/`iarc_rating_id` не имплементированы в браузерах ([MDN, Web app manifests](https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Manifest)).

### Иконки, `purpose`, maskable

- Массив объектов: обязателен только `src`; `sizes` (`"192x192"`, для SVG — `"any"`), `type` (`image/png` — явный `type` экономит браузеру sniffing), `purpose` ([MDN, icons](https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Manifest/Reference/icons)).
- Если давать только один размер — 512×512; рекомендовано 192, 384, 512, 1024 ([web.dev, Web app manifest](https://web.dev/learn/pwa/web-app-manifest)).
- **Maskable**: адаптивные иконки Android (маски круг/сквircle и т.п.): квадратный PNG, основной логотип внутри «безопасной зоны» — круга радиусом 40% ширины иконки; минимум 512×512. Отдельная иконка с `"purpose": "maskable"`, либо одна универсальная с `"purpose": "any maskable"` ([web.dev, Web app manifest](https://web.dev/learn/pwa/web-app-manifest); значения `purpose` — [MDN, icons](https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Manifest/Reference/icons): `any` (дефолт), `maskable`, `monochrome`). Проверять через maskable.app.
- Сплеш-скрин на Android генерируется из `name` + `background_color` + `theme_color` + иконки манифеста автоматически ([MDN, Web app manifests — Splash screens](https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Manifest)).

### Shortcuts и screenshots

- **`shortcuts`** — статические deep links (контекстное меню по иконке на десктопе и Android с WebAPK): `name`, `url` (обязан быть внутри `scope`), опционально `short_name`, `description`, `icons`. Гарантий показа нет — порядок по приоритету ([web.dev, Enhancements](https://web.dev/learn/pwa/enhancements)). Кандидаты для нас: «Объекты», «Финансы», «Календарь».
- **`screenshots`** — массив `{src, sizes, type}` без ограничений по размерам; вместе с `description` превращает install-диалог на Android в rich UI «как в сторе» ([web.dev, Web app manifest](https://web.dev/learn/pwa/web-app-manifest)).

### Подача файла

- Официальное расширение — `.webmanifest`, Content-Type `application/manifest+json`; `.json` тоже работает во всех браузерах ([MDN, Web app manifests — Deploying](https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Manifest)).
- Подключается `<link rel="manifest">` на всех страницах, с которых возможна установка. Если манифест требует креды — нужен `crossorigin="use-credentials"` даже same-origin (нам не нужно).

## 2. Критерии installability (Chrome/Edge/Android, Safari)

### Chrome (актуальные критерии, страница обновлена 2024-09-19)

Источник — [web.dev, What does it take to be installable?](https://web.dev/articles/install-criteria). Для срабатывания `beforeinstallprompt` и показа install-UI:

- приложение ещё не установлено;
- engagement-эвристики: пользователь кликнул/тапнул по странице хотя бы раз и суммарно провёл на ней ≥ 30 секунд;
- HTTPS;
- манифест с `name` или `short_name`; `icons` с 192px и 512px; `start_url`; `display` ∈ {`fullscreen`, `standalone`, `minimal-ui`, `window-controls-overlay`}; `prefer_related_applications` отсутствует или `false`.

**Service worker в списке отсутствует.** История: в начале 2021 Chrome анонсировал требование реальной offline-работоспособности с Chrome 93, но 14 апреля 2021 план заморозили («put those plans on hold») — [Chrome Developers, Improving PWA offline support detection](https://developer.chrome.com/blog/improved-pwa-offline-detection/). Что SW больше не нужен для install-промпта в Chrome и Edge, подтверждает и [Web Almanac 2025, PWA](https://almanac.httparchive.org/en/2025/pwa), и [Дока](https://doka.guide/tools/pwa/).

### Edge

Официальная дока Microsoft прямо говорит: «A Progressive Web App doesn't need to have a service worker for Microsoft Edge to be able to install the app» ([Microsoft Learn, Get started with PWAs](https://learn.microsoft.com/en-us/microsoft-edge/progressive-web-apps/how-to/)). Критерии манифеста — те же, что у Chrome (общий движок Chromium).

### Как ломается installability (типовые причины)

- иконок 192/512 нет или они недоступны (404 по вине прокси — наш кейс с Caddy); `display: browser`; `start_url` вне `scope` или 404; манифест не парсится/не отдаётся.
- Отдельная ловушка Android WebAPK: если сервер перестал отдавать манифест (404), Chrome не проверяет обновления минимум 30 дней ([web.dev, Update](https://web.dev/learn/pwa/update)).
- Диагностика: Chrome DevTools → Application → Manifest (показывает ошибки installability) — [web.dev, Web app manifest](https://web.dev/learn/pwa/web-app-manifest). PWA-аудит из Lighthouse удалён (с Lighthouse 12, май 2024) ⚠️ — источник непервичный, но массово подтверждается; проверять через DevTools.

### Safari

- **iOS/iPadOS**: `beforeinstallprompt` не существует, автопромпта нет — только ручное Share → «На экран "Домой"». Сайт становится Home Screen web app (открывается standalone, отдельная карточка в App Switcher), если есть манифест с `display: standalone|fullscreen` **или** capable-мета-тег; иначе это просто bookmark, который с iOS 16.4 открывается в браузере по умолчанию ([WebKit Blog, Web Push for Web Apps on iOS and iPadOS](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/)). До iOS 15.4 манифест загружался только при открытии share sheet — если не успевал, ставился bookmark вместо web app ([web.dev, Enhancements](https://web.dev/learn/pwa/enhancements)).
- **macOS**: Safari 17 (Sonoma, 2023) — File → «Add to Dock» для любого сайта; web app получает собственное окно, иконку в Dock/Launchpad/Spotlight ([WebKit, WebKit Features in Safari 17.0](https://webkit.org/blog/14445/webkit-features-in-safari-17-0/)).
- Контекст для следующего тикета: Web Push на iOS работает с 16.4 **только для установленных на домашний экран web apps** ([WebKit Blog](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/)).

## 3. iOS-специфика (без пушей)

### Иконки: `apple-touch-icon`

- `<link rel="apple-touch-icon">`, PNG; рекомендованные размеры: 180×180 (iPhone retina), 167×167 (iPad Pro), 152×152 (iPad) ([Apple, Configuring Web Applications](https://developer.apple.com/library/archive/documentation/AppleApplications/Reference/SafariWebContent/ConfiguringWebApplications/ConfiguringWebApplications.html)). С iOS 15.4 манифестные иконки тоже поддерживаются, но **`apple-touch-icon` имеет приоритет** над ними ([WebKit Blog](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/)). Без иконок iOS раньше делала скриншот страницы, с 16.4 — монограмму из первой буквы имени (там же).
- В Next.js это файловая конвенция `app/apple-icon.png` (автоматически генерит `<link rel="apple-touch-icon">`) или `metadata.icons.apple` (см. раздел 5).

### Capable-мета: standalone-режим

- Классический Apple-тег `<meta name="apple-mobile-web-app-capable" content="yes">` включает standalone ([Apple, Meta Tags](https://developer.apple.com/library/archive/documentation/AppleApplications/Reference/SafariHTMLRef/Articles/MetaTags.html)); определить режим в рантайме — нестандартное `window.navigator.standalone` (только WebKit; `undefined` вне iOS) ([web.dev, Enhancements](https://web.dev/learn/pwa/enhancements)).
- Chrome с конца 2024 помечает apple-префиксный тег как deprecated и просит стандартный `<meta name="mobile-web-app-capable" content="yes">` (массовые баг-репорты: [flutter#154596](https://github.com/flutter/flutter/issues/154596), [next.js#70272](https://github.com/vercel/next.js/issues/70272)). Next.js исправил генерацию: `appleWebApp` в Metadata API теперь эмитит именно `mobile-web-app-capable` (см. раздел 5). Практика 2025-2026: эмитить оба тега — стандартный для Chromium, apple-вариант для старых iOS ⚠️ (официального подтверждения Apple, что Safari понимает стандартный тег, не найдено; см. «Открытые моменты»).

### Status bar

`<meta name="apple-mobile-web-app-status-bar-style" content="default|black|black-translucent">` — работает только при включённом standalone. `black-translucent`: контент рендерится под полупрозрачным статус-баром на весь экран; иконки статус-бара всегда белые, обязателен контраст фона и `env(safe-area-inset-*)` ([web.dev, Enhancements](https://web.dev/learn/pwa/enhancements); [Apple, Meta Tags](https://developer.apple.com/library/archive/documentation/AppleApplications/Reference/SafariHTMLRef/Articles/MetaTags.html)). Для первой итерации — `default`.

### Splash screens: `apple-touch-startup-image` — механизм не изменился

- iOS/iPadOS **не используют манифест** для сплеша (в отличие от Android). Нужны статические картинки через `<link rel="apple-touch-startup-image" href="..." media="...">`, где `media` — media query под конкретный размер окна устройства; размер картинки должен точно совпадать с размером окна запуска (для iPad — ещё и варианты мультизадачности 1/3, 1/2, 2/3 экрана) ([web.dev, Enhancements](https://web.dev/learn/pwa/enhancements); [Apple, Configuring Web Applications — Specifying a Launch Screen Image](https://developer.apple.com/library/archive/documentation/AppleApplications/Reference/SafariWebContent/ConfiguringWebApplications/ConfiguringWebApplications.html): без тега показывается скриншот последнего состояния приложения).
- Генерация набора: [PWA Asset Generator](https://github.com/elegantapp/pwa-asset-generator) (статическая, в build) или PWA Compat (клиентская JS-библиотека) ([web.dev, Enhancements](https://web.dev/learn/pwa/enhancements)).
- Актуальность в 2025: подтверждается практикой — вопросы «startupImage не показывается на iOS» актуальны и для Next.js 15/16 (Stack Overflow, сент 2025, там же замечено, что для показа splash может требоваться и apple-префиксный capable-тег ⚠️ — один источник).
- `<meta name="apple-mobile-web-app-title" content="...">` — имя под иконкой на домашнем экране (по умолчанию берётся `<title>`) ([Apple, Configuring Web Applications](https://developer.apple.com/library/archive/documentation/AppleApplications/Reference/SafariWebContent/ConfiguringWebApplications/ConfiguringWebApplications.html)).

## 4. Service worker для online-only приложения

### Минимальный корректный SW (без кэширования данных приложения)

SW для installability не нужен (раздел 2), но нужен как носитель будущего push-функционала и branded offline-экрана. Состав:

- `install`: precache **только** статической offline-страницы (`/offline.html` + её ассеты), `skipWaiting()` опционально;
- `activate`: чистка старых версий cache; `clients.claim()` опционально;
- `fetch`: обрабатывать **только navigation-запросы** (`request.mode === 'navigate'`) и только к роутам кабинета — network-first с fallback на закэшированную offline-страницу при ошибке сети. Всё остальное (API `/api/*`, `_next/*`, чужие пути) — не трогать (`return` без `respondWith`, запрос уходит в сеть напрямую). Никакого кэширования данных приложения/API.
- `push`/`notificationclick` — заглушки под следующий тикет.

Такой подход не конфликтует с installability: SW без fetch-офлайна сегодня ни на что не влияет, а offline-fallback — UX-улучшение (вместо Chrome Dino).

### Стратегия обновления SW

- Браузер сам запускает update-алгоритм (byte-compare) при навигации на страницу в scope и не чаще раза в 24ч при событиях; новая версия устанавливается в фоне и висит в waiting, пока открыты страницы со старым SW ([MDN, Service Worker API](https://developer.mozilla.org/en-US/docs/Web/API/Service_Worker_API)).
- **Auto-update**: `self.skipWaiting()` в `install` + `clients.claim()` в `activate` — новый SW сразу перехватывает контроль; на странице слушать `controllerchange` и при необходимости перезагружать. **Промпт «новая версия»**: слушать `updatefound` на регистрации → `statechange` нового SW в `installed` → показать UI → по клику `postMessage({type:'SKIP_WAITING'})` → reload по `controllerchange` ([web.dev, Update](https://web.dev/learn/pwa/update)).
- Для online-only без кэша данных риск рассинхрона «старый клиент vs новый API» минимален (HTML/JS всегда свежие из сети) — авто-обновление с тихим reload при смене контроллера достаточно; промпт можно добавить позже.
- `updateViaCache: 'none'` в `register()` гарантирует обход HTTP-кэша для скрипта и импортов ([MDN, register()](https://developer.mozilla.org/en-US/docs/Web/API/ServiceWorkerContainer/register)); впрочем, Next.js и так отдаёт `public/` с `Cache-Control: public, max-age=0, must-revalidate` ([документировано в Next.js docs по кэшированию public-ассетов; подтверждение](https://adriel.dev/posts/2025-04-17-properly-caching-images-in-your-mdx-based-nextjs-blog)).

### Подводные камни scope на общем origin с лендингом

- Дефолтный scope регистрации — **директория скрипта**: `/sw.js` → scope `/` ([MDN, register()](https://developer.mozilla.org/en-US/docs/Web/API/ServiceWorkerContainer/register)). Scope определяет, **какие URL SW может контролировать** — это чисто URL-префикс на origin, браузер не знает, что часть путей обслуживает другой бэкенд. Как только SW зарегистрирован (с любой страницы кабинета), он становится active для всего scope `/`, и **все последующие навигации в scope — включая страницы лендинга — становятся контролируемыми клиентами**: fetch-события лендинга пойдут через наш SW.
- Последствия при scope `/`: (1) наш fetch-handler обязан первым делом фильтровать пути (пропускать всё, что не навигации кабинета), иначе offline-fallback сработает и для лендинга; (2) баг в SW затронет лендинг — лечится только unregister/новой версией SW; (3) push-уведомления и badge — без путевой фильтрации тоже общие.
- **Сузить scope можно без всяких заголовков**: `register('/sw.js', { scope: '/dashboard/' })` — scope уже расположения скрипта разрешён всегда ([MDN, register()](https://developer.mozilla.org/en-US/docs/Web/API/ServiceWorkerContainer/register)). Проблема: scope — один префикс, а кабинет живёт на ~15 top-level роутах (`/login`, `/dashboard`, `/properties`…) — осмысленно не сузить. Реалистично: scope `/` + строгая фильтрация в fetch-handler.
- **Заголовок `Service-Worker-Allowed` нужен для противоположного** — расширения scope шире директории скрипта (например, скрипт в `/pwa/sw.js`, scope `/`); сервер ставит его на ответ SW-скрипта, иначе регистрация падает ([MDN, register()](https://developer.mozilla.org/en-US/docs/Web/API/ServiceWorkerContainer/register)). Нам он не нужен, если скрипт лежит в корне (`/sw.js`). Раскладка «`/pwa/sw.js` + `Service-Worker-Allowed: /`» эквивалентна по итоговому scope, но добавляет требование заголовка — смысла нет.
- Не забыть: путь SW-скрипта (и `/offline.html`, и PWA-иконки) добавить в Caddy `@frontend path`, иначе отдача уйдёт на лендинг (см. «Контекст проекта»). Дополнительно проверить, что Caddy не переписывает `Cache-Control` для `/sw.js` (⚠️ проверить на stage).

## 5. Подача manifest/SW из Next.js 16 App Router

### Манифест

- **`app/manifest.ts`** — функция, возвращающая `MetadataRoute.Manifest`; это special route handler, **кэшируется по умолчанию**, если не использует request-time API ([Next.js, manifest file convention](https://nextjs.org/docs/app/api-reference/file-conventions/metadata/manifest)). Отдаётся по `/manifest.webmanifest`; `<link rel="manifest">` добавляется автоматически. Тип `MetadataRoute.Manifest` покрывает поля спеки (name, icons, display, shortcuts, screenshots и т.д.).
- Альтернатива — статический `app/manifest.json` / `app/manifest.webmanifest` с тем же эффектом. Для статичного набора полей разницы нет; `manifest.ts` удобнее типизацией и возможностью собрать значения из констант.
- Либо вообще без файла: `metadata.manifest = '/manifest.webmanifest'` в layout — только добавляет `<link>` ([Next.js, generate-metadata — manifest](https://nextjs.org/docs/app/api-reference/functions/generate-metadata)).
- Во всех вариантах URL `/manifest.webmanifest` должен быть добавлен в Caddy `@frontend path` (см. «Контекст проекта»).

### Иконки и apple-теги через Metadata API

- Файловые конвенции: `app/icon.png` (уже есть в проекте), `app/apple-icon.png` → автоматические `<link rel="icon">` / `<link rel="apple-touch-icon">`. Рекомендуются как основной способ ([Next.js, generate-metadata — icons](https://nextjs.org/docs/app/api-reference/functions/generate-metadata)).
- Программно: `metadata.icons.apple` — массив `{url, sizes, type}` (для 180×180/167×167/152×152) (там же).
- **`metadata.appleWebApp`**: `{capable, title, statusBarStyle, startupImage}`. По документации объект генерит: `<meta name="mobile-web-app-capable" content="yes">` (стандартный тег — фикс после [next.js#70272](https://github.com/vercel/next.js/issues/70272)), `<meta name="apple-mobile-web-app-title">`, `<meta name="apple-mobile-web-app-status-bar-style">`, и `<link rel="apple-touch-startup-image">` для каждого `startupImage` (с `media` при передаче объекта `{url, media}`) ([Next.js, generate-metadata — appleWebApp](https://nextjs.org/docs/app/api-reference/functions/generate-metadata)).
- Apple-префиксный `apple-mobile-web-app-capable` Next больше не эмитит; если понадобится для старых iOS — добавить через `metadata.other: { 'apple-mobile-web-app-capable': 'yes' }` (там же, секция `other`).
- `<meta name="theme-color">`: в Next 14+ задаётся через отдельный экспорт `viewport` (`export const viewport = { themeColor: '#...' }`); `themeColor` внутри `metadata` deprecated ([Next.js, generate-metadata — themeColor](https://nextjs.org/docs/app/api-reference/functions/generate-metadata)). `theme_color` манифеста — независимое поле, задаётся в `manifest.ts`.

### Service worker

- Стандартный способ — статический файл `public/sw.js`, доступный как `/sw.js` (scope `/` по умолчанию). Регистрация — из клиентского компонента (`useEffect`, `if ('serviceWorker' in navigator) navigator.serviceWorker.register('/sw.js')`) — например, в корневом layout через `'use client'` обёртку; регистрировать только на роутах кабинета, чтобы не будить SW на лендинге без необходимости (хотя scope всё равно `/`).
- Кастомные заголовки (если понадобятся `Service-Worker-Allowed` или явный `Cache-Control`) — через `headers()` в `next.config.ts` ([Next.js, next.config.js headers](https://nextjs.org/docs/app/api-reference/config/next-config-js/headers)).
- `public/` отдаётся с `Cache-Control: public, max-age=0, must-revalidate` — SW-скрипт не залипнет в HTTP-кэше между деплоями; дополнительно `updateViaCache: 'none'` при регистрации (раздел 4).
- **`Serwist`/`next-pwa` не рассматриваем**: для online-only сценария (одна offline-страница + push-заглушка) рукописный SW в 30-50 строк проще и не тащит Workbox-зависимость.

### Что нового в Next 16 для PWA

Ничего специфичного: релиз (октябрь 2025) — Turbopack по умолчанию, Cache Components, архитектурные изменения ([Next.js Blog, Next.js 16](https://nextjs.org/blog/next-16)); file conventions `manifest.ts`/`apple-icon.png` и Metadata API (`appleWebApp`, `viewport.themeColor`) работают как в 15.x. В проекте `next@16.2.11` — все описанные механизмы доступны.

## Открытые/спорные моменты

1. ⚠️ **Стандартный `mobile-web-app-capable` и Safari**: официального подтверждения Apple, что iOS понимает непрефиксный тег, не найдено; Chromium deprecated-предупреждение требует стандартный тег, iOS-standalone исторически — apple-префиксный. Безопасный вариант — эмитить оба (через `appleWebApp` + `metadata.other`). Нужна проверка на реальном устройстве iOS 17/18/26.
2. ⚠️ **Splash на iOS**: `apple-touch-startup-image` остаётся единственным механизмом (web.dev, 2024 + практика SO, сент 2025), но есть единичные сообщения, что для показа splash нужен именно apple-префиксный capable-тег. Набор размеров/media queries под актуальные устройства генерировать инструментом (PWA Asset Generator), а не вручную.
3. ⚠️ **Engagement-эвристики Chrome** (клик + 30 сек) взяты со страницы web.dev, обновлённой 2024-09-19; формальной спеки нет, эвристики могут меняться без анонсов. Перед релизом проверить в Chrome DevTools → Application → Manifest.
4. ⚠️ **W3C-спека** `https://www.w3.org/TR/appmanifest/` отдала 403 при фетче; поля манифеста сверены по MDN-референсу (зеркалит спеку) и web.dev — наборы полей согласованы, но при сомнениях в тонкостях (`display_override`, `purpose`) смотреть спеку напрямую из браузера.
5. ⚠️ **Цепочка Caddy → Next для `/sw.js`**: убедиться на stage, что Caddy не модифицирует `Cache-Control` и что `Content-Type` — `text/javascript`/`application/javascript` (иначе регистрация SW упадёт — скрипт обязан отдаваться с валидным JS MIME, [MDN, register()](https://developer.mozilla.org/en-US/docs/Web/API/ServiceWorkerContainer/register)).
6. ⚠️ **Кэширование `app/manifest.ts`**: документировано, что route handler кэшируется по умолчанию — после изменения манифеста уже установленные WebAPK обновятся по своему алгоритму (Chrome на Android — по расписанию, desktop — при перезапуске PWA, iOS — только переустановкой; [web.dev, Update](https://web.dev/learn/pwa/update)). Менять `id`/`start_url` после публикации нельзя — зафиксировать в спеке тикета.
