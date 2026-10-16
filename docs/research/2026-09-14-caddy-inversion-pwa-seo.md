# Research: PWA/SEO-последствия инверсии фолбэка Caddy и переезд favicon лендинга на `/assets`

- Тикет: #652 (карта #649 «Прокси-схема Caddy — инверсия фолбэка»)
- Дата: 2026-09-14
- Вопрос: проверить по первоисточникам (MDN/web.dev для PWA, Google Search Central для SEO, vite.dev для ассетов), что инверсия фолбэка (лендинг = явное исключение `/` + `/assets/*`, фронт-кабинет = catch-all) и новый контракт статики не ломают PWA и SEO `rentlee.ru`: (1) поведение service worker; (2) robots.txt/sitemap — кто и как должен отдавать; (3) SEO-хвост старой схемы (мягкие 404); (4) канонический перенос favicon лендинга с `/icon.png` на `/assets/*`.
- Метод: разбор кода (`apps/frontend/public/sw.js`, `shared/lib/pwa/*`, `apps/frontend/app/{page.tsx,manifest.ts}`, `next.config.ts`, `apps/landing/{index.html,nginx.conf,vite.config.ts,src/app/App.tsx}`, Caddyfile из `docs/deployment.md` §Caddyfile) + первоисточники (developer.mozilla.org, web.dev, developers.google.com, vite.dev, whatwg/w3c-спека service workers) + эмпирическая сборка на **точном пине Vite 6.4.3** из `apps/landing/package.json` (минимальный проект в `/tmp`, node_modules лендинга — README-подобных источников не хватало, поведение `index.html` проверено сборкой).

## 0. Факты о текущем состоянии (база для выводов)

- **Caddy сегодня** (`docs/deployment.md` §Caddyfile): `@frontend path` — белый список (~15 top-level роутов фронта + `/_next/*`, `/fonts/*`, `/images/*`, `/icon.png`, `/manifest.webmanifest`, `/sw.js`, `/offline.html`, `/icons/*`, `/apple-icon.png` и служебные svg) → `127.0.0.1:13000`; **всё остальное** (в т.ч. `/` и любой мусорный путь) → лендинг `127.0.0.1:13002`.
- **Фронт, корень `/`**: `apps/frontend/app/page.tsx` — `redirect(ROUTES.properties)` (`next/navigation.redirect` без `permanent` = **307**; [Next.js, redirect](https://nextjs.org/docs/app/api-reference/functions/redirect)). Сегодня этот редирект в проде недостижим: `/` перехватывает лендинг-дефолт Caddy.
- **Фронт, чужие пути**: `app/not-found.tsx` в репо **нет** → Next отдаёт дефолтную страницу 404 с HTTP-статусом 404 ([Next.js, not-found file convention](https://nextjs.org/docs/app/api-reference/file-conventions/not-found)).
- **Лендинг — не строго одна страница**: `apps/landing/src/app/App.tsx` (react-router) имеет роуты **`/privacy` и `/terms`** (юрстраницы, `legal-pages.tsx`; ссылки из футера и contact-modal) и `*` → `Navigate to="/"` (клиентский редирект). Сегодня прямые заходы на `/privacy`, `/terms` работают через SPA-fallback лендинга. ⚠️ **Это не было зафиксировано в #649/#653** («лендинг = одна страница») — см. чеклист п.1.
- **Шрифты лендинга**: `src/styles/fonts.css` ссылается на `public/landing-fonts/*` **абсолютными путями** → после инверсии `/landing-fonts/*` обязан остаться в лендинг-исключении (уже отмечено в #649/#653).
- **Живая коллизия favicon**: `apps/landing/index.html:10` → `<link rel="icon" href="/icon.png">`, но `/icon.png` стоит в Caddy-whitelist **фронта** и резолвится в `apps/frontend/app/icon.png` (Next file convention). Лендинг показывает иконку кабинета; собственный `apps/landing/public/icon.png` в проде недостижим.
- **robots.txt**: нет ни в whitelist, ни в `public/` лендинга → сегодня `/robots.txt` уходит в SPA-fallback лендинга и отдаёт `index.html` с **200**.
- **SW** (`apps/frontend/public/sw.js`): fetch-handler обрабатывает **только** `request.mode === 'navigate'`; для `/` — спец-ветка «hard isolation» (standalone-клиент → `Response.redirect('/properties', 302)`; не-standalone → `fetch(request)` без изменений); для ~15 префиксов кабинета — network-first с fallback на кэшированный `/offline.html`; **всё остальное — pass-through** (`return` без `respondWith`). Регистрация — `ServiceWorkerRegister.tsx` внутри `ScreenLayout` (только роуты кабинета), `register('/sw.js', { scope: '/', updateViaCache: 'none' })`; standalone-флаг пишется в IndexedDB после регистрации.

---

## 1. Service worker: поведение после инверсии не меняется

### 1.1 Что происходит по коду

SW-логика полностью **путевая и режимная** (path + `request.mode`), она не зависит от того, *кто* за ней отвечает на запрос — Caddy-роутинг для SW прозрачен. Разбор `sw.js` по сценариям «сегодня → после»:

| Сценарий | Сегодня | После инверсии |
|---|---|---|
| Навигация на `/`, standalone-PWA | SW перехватывает (`request.mode==='navigate'`, path `/`) → IndexedDB-флаг → **302 `/properties`**, до сети не доходит | **Без изменений** — та же ветка кода |
| Навигация на `/`, обычный браузер | SW отвечает `fetch(request)` → сеть → Caddy → **лендинг** | Тот же `fetch(request)` → сеть → Caddy → лендинг (**`/` остаётся в лендинг-исключении по #653**, smoke «лендинг `/` = 200») |
| Навигация на префиксы кабинета | SW: network-first, при ошибке сети → cached `/offline.html` | Без изменений |
| Навигация на мусорный путь | pass-through → сеть → **лендинг (200)** | pass-through → сеть → **Next 404**. SW тут по-прежнему ни при чём: путь не в `APP_ROUTE_PREFIXES`, `respondWith` не вызывается. Отличие только в ответе сети |
| Ненавигационные запросы (`/api`, `/_next`, шрифты, картинки) | pass-through | Без изменений |

**Вывод: правки `sw.js` после инверсии не нужны вообще.** Единственное содержательное изменение — ответ сети на мусорные пути (200 лендинга → 404 фронта), и SW к нему безразличен: он эти пути пропускает.

⚠️ **Расхождение в тикетах, зафиксировать в решении**: #652 формулирует «после `/` отдаёт фронт с redirect на `/properties`», но #649 (решения гриллинга) и #653 (smoke «лендинг `/` = 200») фиксируют, что `/` **остаётся лендингом**. Второе — согласованное решение: иначе лендинг лишается единственного URL (противоречит решению «лендинг остаётся одной страницей»), а фронтовский `app/page.tsx`-redirect остаётся dev/резервным механизмом. Спец-ветка SW для `/` при этом **остаётся load-bearing** в обоих прочтениях: именно она, а не фронтовый редирект, возвращает установленного PWA-пользователя с `/` в приложение (и делает это до обращения к сети — работает офлайн).

### 1.2 Сужать scope не нужно; `/dashboard/sw.js` — не делать

- Scope — **один URL-префикс**: «A string representing a URL that defines a service worker's registration scope; that is, what range of URLs a service worker can control» ([MDN, register()](https://developer.mozilla.org/en-US/docs/Web/API/ServiceWorkerContainer/register)). Дефолт — директория скрипта: скрипт в `/dashboard/sw.js` → максимальный scope `/dashboard/` без всяких заголовков. Но кабинет живёт на ~15 top-level префиксах (`/login`, `/properties`, …) — один префикс их не накрывает. Вывод повторяет вывод `docs/research/pwa-manifest-installability.md` §4: **scope `/` + строгая путевая фильтрация в fetch-handler** — реалистичная схема, и фильтрация уже реализована.
- Сужение до `/dashboard/` реально **ломает функционал**: (а) offline-fallback на `/offline.html` и перехват навигаций исчезают на всех роутах вне `/dashboard/*` — включая `start_url`-родственный `/properties` и `/login`; (б) web.dev прямо рекомендует обратное: «Keep the service worker's scope as close to the root of your app as possible» ([web.dev/learn/pwa/service-workers](https://web.dev/learn/pwa/service-workers), раздел Scope; «only one service worker is allowed per scope»).
- `Service-Worker-Allowed` — про **расширение** scope шире директории скрипта, не про сужение: «A service worker can't have a scope broader than its own location, unless the server specifies a broader maximum scope in a `Service-Worker-Allowed` header» ([MDN, register()](https://developer.mozilla.org/en-US/docs/Web/API/ServiceWorkerContainer/register)). Для `/sw.js` в корне он не нужен и после инверсии не понадобится.
- **Переезд скрипта** (`/sw.js` → `/dashboard/sw.js`) — отдельная миграционная мина для установленных клиентов: это другая registration (scriptURL+scope). Старая регистрация при обновлении, получив на `/sw.js` 404/410, снимается по спеке (Update algorithm → Clear Registration; [W3C Service Workers](https://www.w3.org/TR/service-workers/), история решения — [w3c/ServiceWorker#204](https://github.com/w3c/ServiceWorker/issues/204)), но до этого момента старый SW продолжает контролировать origin, а параллельная вторая регистрация на том же origin — лишнее состояние. Выгода нулевая.

### 1.3 Риски для установленного PWA после инверсии

- **Манифест не трогаем**: `id: '/'`, `start_url: '/dashboard'`, `scope: '/'` — менять после публикации нельзя, ломает распознавание установленных приложений (уже зафиксировано в `apps/frontend/app/manifest.ts` и `docs/research/pwa-manifest-installability.md` §5/§6). Инверсия их не затрагивает.
- **Обновление SW**: браузер обновляет SW «when a navigation to an in-scope page occurs» ([MDN, Service Worker API](https://developer.mozilla.org/en-US/docs/Web/API/Service_Worker_API)). Лендинг-навигации и после инверсии происходят в scope `/` — частота update-check не меняется. На Next-404-страницах SW не регистрируется (регистрация живёт в `ScreenLayout` кабинетных роутов) — не проблема.
- **Доступность `/sw.js`, `/offline.html`, `/manifest.webmanifest`, `/icons/*`**: после инверсии эти пути выпадают из whitelist в **дефолт → фронт**, т.е. попадают куда нужно автоматически — whitelist просто сокращается. Обратное движение (лендинг-исключение должно покрывать `/assets/*`, `/landing-fonts/*`, `/robots.txt` и юрстраницы — см. §2/§4) их не задевает.
- **`Cache-Control: no-store` на `/sw.js`** (открытый вопрос #649 «нужен ли дубль на уровне Caddy»): достаточно заголовка от Next (`next.config.ts` `headers()`) — Caddy `reverse_proxy` передаёт апстрим-заголовки как есть, ничего не переписывая. Дубль на уровне Caddy не требуется; контроль — разовый curl на stage: `curl -sSI https://dev.rentlee.ru/sw.js | grep -i cache-control` → `no-store` + `Content-Type: text/javascript|application/javascript` (невалидный JS MIME валит регистрацию — [MDN, register()](https://developer.mozilla.org/en-US/docs/Web/API/ServiceWorkerContainer/register)).
- Итоговый риск инверсии для PWA: **нулевой при соблюдении п.1–2 чеклиста** (не трогать манифест; не трогать `sw.js`; `/` остаётся лендингом).

---

## 2. robots.txt / sitemap: отдаёт лендинг, файл заведомо создать

### 2.1 Требования Google к robots.txt

- Файл обязан лежать в корне хоста: «You must place the robots.txt file in the top-level directory of a site» — ровно `https://rentlee.ru/robots.txt`; действие распространяется «only [on] the host, protocol, and port number where the robots.txt file is hosted» ([Google, How Google interprets the robots.txt specification](https://developers.google.com/crawling/docs/robots-txt-spec)). Никакой «общий» robots.txt на поддомене/подпути не поможет.
- Сегодняшнее состояние (`/robots.txt` → `index.html` с 200) Google разбирает так: «if the content downloaded is HTML instead of robots.txt rules, Google will try to parse the content and extract rules, and ignore everything else» (там же) — HTML-шаблон Vite не содержит валидных robots-строк → фактически «robots.txt нет, ограничений нет». Не фатально, но и не то, что нам нужно.
- После инверсии без файла `/robots.txt` уходил бы фронту → Next 404 → «Google's crawlers treat all 4xx errors, except 429, as if a valid robots.txt file didn't exist» (там же) — опять «ограничений нет». Валидно, но теряем disallow кабинета.
- Google кэширует robots.txt до 24 часов («Google generally caches the contents of robots.txt file for up to 24 hours», там же) — после выката эффект не мгновенный.

### 2.2 Кто должен отдавать после инверсии: лендинг

robots.txt — файл уровня **хоста**, а публичную (маркетинговую) поверхность хоста после инверсии владеет лендинг: его контракт — «явно перечисленное исключение». Логично и практически:

- файл кладётся в `apps/landing/public/robots.txt` — по доке Vite `public/` как раз для ассетов, которые «Never referenced in source code (e.g. robots.txt)» и «must retain the exact same file name (without hashing)» ([vite.dev, The public directory](https://vite.dev/guide/assets.html#the-public-directory));
- в Caddy-исключение лендинга добавляется `path` `/robots.txt` (+ `/sitemap.xml`, если делаем). Это ещё один явный пункт контракта — в духе принятого решения «лендинг = перечисленное исключение».
- Альтернатива — `app/robots.ts` на фронте ([Next.js, robots file convention](https://nextjs.org/docs/app/api-reference/file-conventions/metadata/robots)) — функционально эквивалентна (дефолт-роутинг сам приведёт к фронту), но размывает владение: SEO-политика публичной части уехала бы в кабинет-приложение, а файл перестал бы жить рядом с лендингом, которым описывается. Отклоняем.

### 2.3 Рекомендуемое содержимое

```text
User-agent: *
Allow: /
Disallow: /dashboard
Disallow: /properties
Disallow: /operations
Disallow: /payments
Disallow: /participants
Disallow: /tasks
Disallow: /contacts
Disallow: /subscription
Disallow: /profile
Disallow: /support
Disallow: /ui-kit
Disallow: /api/
Disallow: /webhooks/
Sitemap: https://rentlee.ru/sitemap.xml
```

- Список `Disallow` зеркалит `APP_ROUTE_PREFIXES` из `sw.js` / `shared/lib/pwa/app-routes.ts` (источник истины для кабинетных префиксов) — тот же принцип синхронизации, что в guard-тестах SW.
- **`/login` сознательно не заблокирован**: единственная публичная страница кабинета, цель CTA лендинга («Войти»); пусть остаётся доступной для обхода и выдачи. Альтернатива — закрыть и её, если не хотим выдачи формы логина; UX-аргумент за открытую.
- `Disallow` — префиксное сопоставление: `/properties` закрывает и несуществующие `/properties-xyz` — это желаемое поведение.
- **Не комбинировать `Disallow` с `noindex` на кабинете**: disallow мешает обходу, noindex требует обхода — Google прямо не рекомендует их смешивать; кабинет за аутентификацией неиндексируем по определению, robots.txt-запрета достаточно. (Тонкость из доков Google: URL, закрытый disallow, может всё же попасть в индекс без контента, если на него ведут внешние ссылки, — для auth-кабинета это приемлемо и неизбежно.)
- `Allow: /` избыточен при отсутствии других блоков (default — разрешено), но безвреден и самодокументируем; можно опустить.

### 2.4 Sitemap: опционально, минимальный

- Для маленького сайта sitemap не обязателен: Google считает его избыточным, «если сайт не содержит более ~500 страниц» и все страницы достижимы ссылками с главной ([Google, Learn about sitemaps](https://developers.google.com/search/docs/crawling-indexing/sitemaps/overview)); sitemap помогает «поисковым системам обнаруживать URL» на новых/крупных сайтах, но «не гарантирует индексацию» (там же).
- У нас публичных страниц три: `/`, `/privacy`, `/terms`. Рекомендация: **сделать минимальный** `apps/landing/public/sitemap.xml` с этими тремя URL — цена нулевая, даёт Google явный список + заставляет зафиксировать канонические адреса (только канонические URL того же хоста, что и sitemap). Плюс — юрстраницы получают явный маршрут обнаружения после инверсии (см. ниже про `/privacy`).
- Если решено без sitemap — просто убрать `Sitemap:`-строку из robots.txt.

---

## 3. SEO-хвост старой схемы: 404 фронта корректен, noindex не нужен

- **Что было**: любой мусорный путь → SPA-fallback → `index.html` с **200**, затем клиентский `Navigate to="/"` (react-router `*`-роут). Для Google это классический **soft 404**: «2xx (success)» + «If the content suggests an error for Google Search, an empty page or an error message» → «Search Console will show a soft 404 error» ([Google, How HTTP status codes affect Google's crawlers](https://developers.google.com/search/docs/advanced/crawling/soft-404-errors); Search Console Help «Fix soft 404 errors» редиректит туда же). Плюс неограниченное число URL с идентичным контентом без canonical — duplicate content и пустой расход краулинга.
- **Что станет**: мусорный путь → Next-дефолт 404 (статус 404). По Google: «Google doesn't index URLs that return a 4xx status code», а ранее проиндексированные URL, «returning 4xx status code, are removed from the index» ([та же страница](https://developers.google.com/search/docs/advanced/crawling/soft-404-errors)). **Подтверждено: 404 фронта — корректный сигнал**, мягкие 404 уйдут, легаси-URL сами выпадут из индекса; ручных действий не требуется.
- **noindex не нужен**: `noindex` — сигнал для страниц, которые *отдаются с 200* и должны исчезнуть из индекса; для отсутствующих страниц правильный и достаточный сигнал — сам статус 404/410. Ставить `noindex` на 404-страницу — бессмысленно (она и так не индексируется).
- **Не смягчать 404 редиректом на `/`**: редирект несуществующих страниц на главную — задокументированный анти-паттерн (Google считает такие ответы мягкими 404; см. «Fix soft 404 errors», Search Console Help — https://support.google.com/webmasters/answer/181708). У нас он к тому же разрушил бы саму инверсию.
- Дефолтная 404-страница Next (без `app/not-found.tsx`) отдаёт корректный **статус 404** ([Next.js, not-found file convention](https://nextjs.org/docs/app/api-reference/file-conventions/not-found)) — для SEO этого достаточно; кастомная брендированная 404 — опциональный UX-кандидат, не SEO-требование.

---

## 4. Favicon лендинга: относительная ссылка в `index.html` → `/assets/<name>-<hash>`

### 4.1 Механизмы Vite (проверено на пине 6.4.3)

| Механизм | Как работает | Оценка |
|---|---|---|
| **Относительная ссылка в `index.html` на файл исходников** (`href="./src/assets/icon.png"`) | Vite берёт `index.html` как entry build'а и переразрешает ассет-ссылки: при указании `base` «asset references in your .html files are all automatically adjusted» ([vite.dev/guide/build](https://vite.dev/guide/build.html)); ассет попадает в граф сборки и получает «hashed file names» ([vite.dev/guide/assets](https://vite.dev/guide/assets.html)) | **Рекомендуется.** Эмпирика на Vite 6.4.3: `<link rel="icon" href="./src/assets/icon.png">` в собранном `dist/index.html` → `<link rel="icon" href="/assets/icon-BLCRa3-k.png">`, файл лежит в `dist/assets/` |
| Файл в `public/` с уникальным именем (`/landing-icon.png`) | «served at root path / during dev, and copied to the root of the dist directory as-is» — **без хэширования**: «must retain the exact same file name (without hashing)»; ссылка — только root-absolute ([vite.dev/guide/assets](https://vite.dev/guide/assets.html#the-public-directory)) | Работает, но: без cache-busting + **требует отдельного path в Caddy-исключении** (дефолт после инверсии уводит `/landing-icon.png` на фронт). Общая доктрина Vite: «prefer importing assets unless you specifically need the guarantees provided by the public directory» |
| `import iconUrl from './icon.png'` в JS | Даёт хэшированный URL, но `<link rel="icon">` надо проставлять **исполнением JS** — фавиконка запрашивается браузером/краулером до и вне JS; SEO-нестабильно | Не канонично для favicon |
| Сторонний `vite-plugin-favicon` и т.п. | Генерация набора favicon/manifest-иконок | Избыточно: у лендинга одна PNG-иконка, не PWA |

### 4.2 Рекомендация

Перенести `apps/landing/public/icon.png` → `apps/landing/src/assets/icon.png`, в `apps/landing/index.html`:

```html
<link rel="icon" href="./src/assets/icon.png" />
```

- Результат сборки — `/assets/icon-<hash>.png`: **уже покрыт** лендинг-исключением Caddy `/assets/*` и immutable-локацией nginx (`location /assets/` с `Cache-Control: public, max-age=31536000, immutable`) — нулевые правки прокси, консистентно с бандлами. Dev-сервер Vite отдаёт `./src/assets/icon.png` из исходников без изменений.
- `public/icon.png` лендинга после переноса **удалить** — иначе останется мёртвый путь, который после инверсии молча уйдёт на фронт.
- `/icon.png` остаётся фронту (`app/icon.png` — Next file convention, им фронт и пользуется): коллизия исчезает не переименованием, а тем, что лендинг больше не ссылается на чужой путь.
- **Google-гайдлайны по favicon** соблюдаются ([Google, Favicon guidelines for Search](https://developers.google.com/search/docs/appearance/favicon-in-search)): «square (1:1)… at least 8x8px, we recommend… larger than 48x48px»; «The favicon URL must be stable (don't change the URL frequently)» — контент-хэш меняет URL только при изменении самой иконки, что и является моментом обновления кэша; PNG поддерживается; «Add a `<link>` tag to the header of your home page» — как у нас; «Googlebot-Image must be able to crawl the favicon file» — наш robots.txt `/assets/` не запрещает; «Google Search only supports one favicon per site» (hostname) — после переноса у `rentlee.ru` остаётся один лендинговый favicon на `/`, иконки кабинета живут на кабинетных страницах и в манифесте.

---

## Чеклист правок для тикета «Переработка» (#653)

1. **Caddy, лендинг-исключение** — финальный набор: `/` (exact), `/assets/*`, `/landing-fonts/*`, **`/privacy`**, **`/terms`**, **`/robots.txt`**, `/sitemap.xml` (если делаем п.4). ⚠️ `/privacy` и `/terms` — **новое требование поверх #653**: юрстраницы лендинга живут в react-router (`apps/landing/src/app/App.tsx`), сегодня доступны только через SPA-fallback; без них в исключении прямые заходы (ссылки футера/contact-modal, Google, договоры) получат 404 фронта — с юридическим риском. Whitelist-пути фронта (`/icon.png`, `/sw.js`, `/offline.html`, `/manifest.webmanifest`, `/icons/*`, `/apple-icon.png`, `/fonts/*`, `/images/*`, svg) из конфига просто уходят — дефолт сам приводит к фронту.
2. **Favicon**: `git mv apps/landing/public/icon.png apps/landing/src/assets/icon.png`; в `apps/landing/index.html:10` → `<link rel="icon" href="./src/assets/icon.png" />`. Проверить сборкой: в `dist/index.html` ссылка `/assets/icon-*.png`.
3. **robots.txt**: создать `apps/landing/public/robots.txt` по §2.3 (Disallow: кабинетные префиксы = `APP_ROUTE_PREFIXES` минус `/login`, + `/api/`, `/webhooks/`; `Sitemap:` — если п.4). Место: лендинг — владелец публичной поверхности хоста.
4. **sitemap.xml (опционально, рекомендовано)**: `apps/landing/public/sitemap.xml` — три URL: `https://rentlee.ru/`, `/privacy`, `/terms`.
5. **SW/PWA — без правок кода**: `public/sw.js`, `ServiceWorkerRegister.tsx`, `manifest.ts` не трогать (`id`/`start_url`/`scope` неприкосновенны). Отдельного `Cache-Control` для `/sw.js` на уровне Caddy **не делать** — достаточно `no-store` от Next; проверить один раз на stage (`curl -I`: `no-store` + JS MIME).
6. **Smoke-чеки #653 дополнить**: `/robots.txt` → 200 + `text/plain` и валидное содержимое (не HTML); `/privacy`, `/terms` → 200 (лендинг); `/favicon`-URL из собранного `index.html` (`/assets/icon-*.png`) → 200 + immutable; `/garbage-path` → 404 (фронт); `/` → 200 лендинга.
7. **docs/deployment.md**: убрать дубликат Caddyfile, описать новый контракт («фронт = дефолт, лендинг = перечисленное исключение: `/`, `/assets/*`, `/landing-fonts/*`, `/privacy`, `/terms`, `/robots.txt`[, `/sitemap.xml`]»), правило добавления новых top-level роутов фронта («работают без правок прокси» — теперь правда) и лендинга (новый публичный путь лендинга = явная правка исключения).
8. **Зафиксировать расхождение #652 ↔ #653**: `/` после инверсии остаётся лендингом (по решениям гриллинга #649 и smoke #653); фронтовый `app/page.tsx` redirect — dev/резерв. Ветки `sw.js` для `/` и `APP_ROUTE_PREFIXES` не менять.

## Источники

- [MDN, ServiceWorkerContainer.register()](https://developer.mozilla.org/en-US/docs/Web/API/ServiceWorkerContainer/register) — дефолтный scope, max-scope, `Service-Worker-Allowed`, регистрация с любой страницы.
- [MDN, Service Worker API](https://developer.mozilla.org/en-US/docs/Web/API/Service_Worker_API) — update-check по навигации в scope.
- [web.dev, Learn PWA — Service workers (scope)](https://web.dev/learn/pwa/service-workers) — «one service worker per scope», держать scope у корня.
- [W3C, Service Workers](https://www.w3.org/TR/service-workers/) + [w3c/ServiceWorker#204](https://github.com/w3c/ServiceWorker/issues/204) — Update: 404/410 скрипта → Clear Registration.
- [Google, How Google interprets the robots.txt specification](https://developers.google.com/crawling/docs/robots-txt-spec) — расположение, 4xx/5xx/429, HTML-с-200, кэш 24ч.
- [Google, Learn about sitemaps](https://developers.google.com/search/docs/crawling-indexing/sitemaps/overview) — когда sitemap не обязателен, что даёт.
- [Google, How HTTP status codes affect Google's crawlers](https://developers.google.com/search/docs/advanced/crawling/soft-404-errors) (туда же редиректит «Fix soft 404 errors»: https://support.google.com/webmasters/answer/181708) — soft 404, «Google doesn't index URLs that return a 4xx».
- [Google, Favicon guidelines for Search](https://developers.google.com/search/docs/appearance/favicon-in-search) — размер, стабильный URL, crawlability, `<link>` в head, один favicon на hostname.
- [vite.dev, Guide — Build](https://vite.dev/guide/build.html) и [Guide — Static Asset Handling](https://vite.dev/guide/assets.html) — обработка index.html, public/ («exact same file name (without hashing)», «prefer importing assets»).
- [Next.js, redirect()](https://nextjs.org/docs/app/api-reference/functions/redirect), [not-found file convention](https://nextjs.org/docs/app/api-reference/file-conventions/not-found), [metadata/robots](https://nextjs.org/docs/app/api-reference/file-conventions/metadata/robots).
- Локально: `apps/frontend/public/sw.js`, `apps/frontend/shared/lib/pwa/{ServiceWorkerRegister.tsx,app-routes.ts}`, `apps/frontend/app/{page.tsx,manifest.ts}`, `apps/frontend/next.config.ts`, `apps/landing/{index.html,nginx.conf,vite.config.ts}`, `apps/landing/src/app/App.tsx`, `docs/deployment.md` §Caddyfile, `docs/research/pwa-manifest-installability.md` §4.
