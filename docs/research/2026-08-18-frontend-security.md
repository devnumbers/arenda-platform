# Безопасность фронта: прикладной уровень и зависимостный максимум

Дата: 2026-08-18
Вход для: тикет [#329](https://github.com/devnumbers/arenda-platform/issues/329) «[research] Безопасность фронта: прикладной уровень и зависимостный максимум», часть карты [#326](https://github.com/devnumbers/arenda-platform/issues/326) (режим качества фронта и админки).

Файл содержит только фактуру из первоисточников (официальные доки, README, npm registry) и проверенные локальные факты репозитория. Раздел «Выводы для решения» — сухие факты и trade-offs, решение принимает человек.

Стек по тикету: `apps/frontend` (Next.js 16.3.1 standalone, React 19.2.4, PWA SW, HeroUI v3, framer-motion, react-markdown), `apps/admin` (Vite 6 + react-admin 5, за nginx), `apps/landing` (генерированный экспорт Figma Make, за nginx). Уже зелёное — не переоткрывается, только усиливается: npm audit / trivy fs / semgrep p/ci (pre-push + nightly + dispatch, #311/#318), dependabot cooldown 7d.

## 0. Текущее состояние (локальные факты)

Проверено чтением файлов 2026-08-18.

### XSS-поверхности

- `apps/frontend/shared/ui/markdown-content/MarkdownContent.tsx` — единственное место рендера markdown: `<ReactMarkdown>{children}</ReactMarkdown>` без `remarkPlugins`/`rehypePlugins`, без переопределения `urlTransform`, без `rehype-raw` (grep по `react-markdown|rehype|remark` вне node_modules: только этот файл и запись в `package.json` — `react-markdown ^10.1.0`).
- Источники контента — ровно два, оба файлы в репозитории, читаются на сервере в server components: `apps/frontend/app/(cabinet)/profile/info/privacy/page.tsx` (`fs.readFileSync(path.join(process.cwd(), 'content', 'privacy.md'))`), аналогично `terms/page.tsx` → `content/terms.md`. API-источников markdown нет (grep `MarkdownContent` — только эти две страницы).
- `dangerouslySetInnerHTML` — 0 вхождений в `apps/frontend` и `apps/admin/src` (grep `*.ts,*.tsx,*.js` вне node_modules/.next). **Одно вхождение в `apps/landing/src/app/components/ui/chart.tsx:83`** — shadcn-паттерн `ChartStyle`: `<style dangerouslySetInnerHTML>` инжектит CSS custom properties (`--color-*`) из статического конфига тем в коде, не из пользовательского ввода. Landing — генерированный экспорт, по условию тикета линт кода landing не наводим (security-only: nginx + зависимости).
- ESLint-гейта на `dangerouslySetInnerHTML` нет ни в `apps/frontend/eslint.config.mjs` (FSD-boundaries + no-restricted-imports на generated/@heroui), ни в `apps/admin/eslint.config.mjs` (no-explicit-any, запрет абсолютных URL и `/api/`-литералов, generated-шов, fetch только в граничных файлах).

### Заголовки и точка терминирования TLS

- `apps/frontend/next.config.ts` — единственный header: `Cache-Control: no-store` для `/sw.js`. Никаких security-заголовков и CSP.
- `apps/frontend/proxy.ts` (в Next.js 16 middleware переименован в proxy — файл-конвенция `proxy.ts`, см. первоисточник §2) — только auth-маршрутизация (проверка сессии через `/me`, редиректы на `/login?from=`), заголовков безопасности не ставит. `config.matcher` — кабинетные префиксы + `/login`.
- `apps/admin/nginx.conf` и `apps/landing/nginx.conf` — побайтно идентичны: `listen 8080`, `root /usr/share/nginx/html`, `location = /healthz`, `try_files $uri $uri/ /index.html`. Ноль security-заголовков, ноль cache-политик для статики, TLS внутри нет вообще (plaintext HTTP на 8080).
- TLS терминирует Caddy на хосте: `docs/deployment.md` § Caddyfile — `rentlee.ru`, `admin.rentlee.ru`, `dev.*` через ACME (automatic HTTPS), контейнеры за `reverse_proxy 127.0.0.1:1xxxx`. HSTS/security-заголовки в Caddyfile из доки не ставятся.
- Service worker `apps/frontend/public/sw.js`: scope `/`, fetch-хендлер перехватывает **только** `request.mode === 'navigate'` (API/статика проходят мимо), network-first для кабинета, offline-страница из кэша; push-payload жёстко валидируется (`resolveClickTarget` пропускает только same-origin абсолютные пути, режет `//` **и `/\`** — `url[1] === '\\'`); токенов и API-ответов не кэширует; IndexedDB хранит только boolean-флаг standalone.

### Сессии и секреты

- Сессия — cookie, ставит Go-бэкенд: `apps/backend/internal/platform/httpsupport/session.go` — `HttpOnly: true`, `Secure` (динамически по APP_ENV), `SameSite: Lax`, имя `__Host-session_id` при secure / `session_id` в dev (ADR 0018, ADR 0034). Sliding window 7 дней, max TTL 30.
- `apps/frontend/shared/api/client.ts` — `fetch('/api'+path)`, токен не читает и не передаёт (cookie едет автоматически). `localStorage` — только cooldown повторной отправки кода (`features/auth/lib/use-send-cooldown.ts`), `sessionStorage` — черновик логина (`use-login-draft.ts`). Никаких токенов в web storage.
- `apps/admin/src/authProvider.ts` — `credentials: 'include'`, сессия та же cookie; `VITE_API_PREFIX` (fallback `/api`) — единственная Vite-переменная в коде admin (grep: `authProvider.ts`, `dataProvider.ts`, `lib/report-error.ts`), это публичный префикс пути, не секрет. `NEXT_PUBLIC_*` во frontend не используется вообще (grep — 0 вхождений). `.env*` в apps/frontend/apps/admin отсутствуют.
- gitleaks (строгий, blocking) уже в ci.yml на PR с allowlist в `.gitleaks.toml`.

### safe-internal-path

- `apps/frontend/shared/lib/safe-internal-path.ts`: пропускает только строки, начинающиеся с `/`, не начинающиеся с `//` и не `/login`. Используется в двух местах: `proxy.ts:67` (редирект после логина — `NextResponse.redirect(new URL(target, request.url))`) и `app/login/page.tsx:89` (`router.push(target)`).

### Существующие зависимостные гейты

- pre-push (lefthook, `lefthook.yml`): `make test`, migrations lint, govulncheck, `make npm-audit` (`npm audit --audit-level=high` по 6 директориям: frontend, admin, landing, tools/*), `make trivy-fs` (pinned trivy 0.74.0, vuln, HIGH/CRITICAL, ignore-unfixed, skip node_modules/.git/.tmp).
- `.github/workflows/security.yml`: semgrep `p/ci --error` + trivy fs — nightly (cron 17 1 * * *) и dispatch строго, на PR — `continue-on-error`.
- `.github/workflows/ci.yml`: gitleaks через docker CLI (blocking на PR), hadolint + docker build + trivy образа по сервисам.
- `.github/dependabot.yml`: gomod/npm×3/docker×4/github-actions, weekly, cooldown 7d на каждой записи (#318), группы `minor-patch` для npm, target-branch dev, open-pull-requests-limit 5–10. Auto-assign/assignees не настроен.

## 1. XSS: react-markdown и санитизация

Первоисточник: [react-markdown README](https://github.com/remarkjs/react-markdown) (прочитан 2026-08-18).

Официальные позиции:

- «Use of react-markdown is secure by default.» Пакет в списке фич — «safe by default (no dangerouslySetInnerHTML or XSS attacks)»: строит виртуальную DOM из синтаксического дерева, а не `innerHTML`.
- Сырой HTML по умолчанию **не рендерится**: react-markdown «typically escapes HTML (or ignores it, with skipHtml) because it is dangerous». `rehype-raw` нужен только «in a trusted environment (you trust the markdown)», когда хотят рендерить встроенный HTML. У нас `rehype-raw` не подключён → HTML в markdown-файлах уйдёт в вывод экранированным текстом.
- Дефолтный `urlTransform` фильтрует URL: разрешены протоколы http, https, irc, ircs, mailto, xmpp и относительные/протоколо-относительные пути — `javascript:` в ссылках не пройдёт. Предупреждение README: «Overwriting urlTransform to something insecure will open you up to XSS vectors» — у нас не перезаписан.
- Про `rehype-sanitize` README: «To make sure the content is completely safe, even after what plugins do» — то есть санитизация официально позиционируется как защита от **будущих плагинов** и небезопасных `components`-оверрайдов, а не от дефолтного пайплайна.

Факты для решения:

- Текущая конфигурация (контент = два markdown-файла из репозитория, проходят code review; плагинов нет) соответствует «trusted environment» без `rehype-raw`. Вектор появляется только при смене источника на API/БД/пользовательский ввод или при добавлении плагинов — тогда README прямо требует санитизацию.
- Дешёвая альтернатива санитизации «на всякий случай» — ESLint-гейт `no-restricted-imports` на `rehype-raw`/`rehype-sanitize`-семантику: запрет включать raw-HTML плагины в `MarkdownContent` без явного решения (паттерн уже используется в этом конфиге для `@heroui/styles` и generated-клиента). Ноль рантайм-стоимости, ловит момент, когда предусловие «trusted environment» сломается.
- `rehype-sanitize` как страховка: +1 зависимость в бандл (~десятки KB) и проход по hast на каждый рендер; для двух статических страниц выигрыш сегодня нулевой, но он автоматически покрывает будущий сценарий «контент стал приходить из API».

## 2. Линт-запрет dangerouslySetInnerHTML

- Факт: в frontend/admin 0 вхождений (см. §0), в landing — 1 в генерированном коде (CSS-инъекция из статического конфига, не пользовательский ввод).
- Инструмент: правило `react/no-danger` из eslint-plugin-react (входит в поставку `eslint-config-next`, который уже подключён в `apps/frontend/eslint.config.mjs` как `nextVitals`) или селектор `no-restricted-syntax` по JSXAttribute `dangerouslySetInnerHTML` — тот же механизм, что уже несёт admin-конфиг. Оба варианта — изменение только ESLint-конфигов, нулевая рантайм-стоимость.
- Слой подключения: pre-commit (джобы `frontend lint` / `admin lint` уже в `lefthook.yml` — гейт заработает без новых джобов) + тот же lint в ci.yml на PR.
- Landing security-only: вхождение в `chart.tsx` сознательно не трогаем и не линтуем.

## 3. Security-заголовки и CSP для Next.js 16 standalone

Первоисточник: [Next.js 16.3.1 CSP guide](https://nextjs.org/docs/app/guides/content-security-policy) (прочитан 2026-08-18; `version: 16.3.1`, `lastUpdated: 2026-03-20`).

Официальные позиции:

- **Nonce-based CSP живёт в `proxy.ts`** (файл-конвенция, в Next 16 переименована из `middleware.ts`): proxy генерирует nonce на запрос, кладёт его в заголовок `Content-Security-Policy` запроса и `x-nonce`; «Next.js applies nonces during server-side rendering» — автоматически вешает nonce на фреймворк-скрипты, бандлы страниц и инлайн-стили/скрипты, которые генерирует Next. Ручная разметка тегов не нужна.
- Рекомендуемый полис из гайда: `default-src 'self'; script-src 'self' 'nonce-…' 'strict-dynamic'; style-src 'self' 'nonce-…'; img-src 'self' blob: data:; font-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; upgrade-insecure-requests`.
- Жёсткое предусловие: «you must use dynamic rendering to add nonces». Все страницы должны быть динамическими; «Static optimization and ISR are disabled», CDN-кэш недоступен, **«Partial Prerendering (PPR) is incompatible with nonce-based CSP»**. У нас CDN нет; кабинетные страницы и так за `proxy.ts` с auth-проверкой (динамические по построению), но `privacy/terms`-страницы сейчас рендерятся без запросных данных — переход на nonce переводит и их в динамические.
- В dev обязателен `'unsafe-eval'` (React реконструирует серверные стектрейсы), «unsafe-eval is not required for production».
- **CSP в `next.config.ts` headers — только без nonce** (гайд «Without Nonces»): `script-src 'self' 'unsafe-inline' …`. Это сознательно слабее по скриптам (`'unsafe-inline'` сводит защиту script-src на нет), но все остальные директивы (`object-src`, `base-uri`, `form-action`, `frame-ancestors`, `connect-src`, `img-src`) работают и для статики.
- Experimental-альтернатива: hash-based CSP через SRI (`experimental.sri.algorithm: 'sha256'`) — статика сохраняется, но фича экспериментальная, App Router only.
- Matcher из гайда: исключить `api`, `_next/static`, `_next/image`, `favicon.ico` и prefetch-запросы — иначе nonce-заголовок дублируется на статику, где он бессмыслен.
- Troubleshooting гайда прямо называет наши смежные риски: «Inline styles: Use CSS-in-JS libraries that support nonces or move styles to external files»; «Service workers: Add appropriate policies for service worker scripts».
- Официальный пример с полным строгим CSP: [examples/with-strict-csp](https://github.com/vercel/next.js/tree/canary/examples/with-strict-csp) в репо next.js.

Совместимость с нашим стеком — факты и честная неуверенность:

- **PWA SW**: `sw.js` регистрируется как classic script с same-origin URL; `script-src 'self'` из рецепта гайда его покрывает. SW не исполняет чужих скриптов и не инжектит их в страницы (fetch-хендлер только навигации, офлайн-страница — наш же ассет). Отдельный `worker-src` не требуется, пока `'self'` в script-src (SW-скрипт подчиняется script-src/worker-src с фолбэком; `'self'` достаточно).
- **framer-motion / HeroUI v3**: HeroUI v3 — Tailwind-классы (скомпилированный CSS-файл, внешний), CSS Modules у нас тоже внешние файлы. framer-motion применяет анимационные стили через CSSOM (`element.style.*`), а CSP style-src не блокирует CSSOM-манипуляции — но **SSR-рендер React 19 может выдавать инлайн `style`-атрибуты**, которые при `style-src` без `'unsafe-inline'`/nonce блокируются. Это гипотеза, не проверено на нашем коде — правильная проверка: включить CSP в `Content-Security-Policy-Report-Only` на stage и собрать отчёт нарушений до переключения в блокирующий режим. Гайд Next этого режима прямо не описывает, но это стандартный механизм CSP (MDN: Report-Only заголовок — отладочная версия политики).
- **@next-safe/middleware**: мёртв. npm registry (прочитан 2026-08-18): `latest: 0.10.0`, опубликован `2022-08-02`, все последние релизы — июль–август 2022. Про Next 16 (и вообще proxy.ts-конвенцию) библиотека знать не может. Не рассматривать.
- `next-safe-action` — библиотека для валидации Server Actions, к CSP отношения не имеет (в тикете уже помечено «не то»).
- Статичные заголовки (HSTS, X-Content-Type-Options, Referrer-Policy, Permissions-Policy) — в `next.config.ts` `headers()` на любой source; для standalone-деплоя это штатный механизм, они уедут в ответы Next-сервера. Для nginx-приложений (admin/landing) — те же заголовки в `nginx.conf` через `add_header … always`.

MDN-справка по заголовкам (прочитано 2026-08-18):

- [`X-Content-Type-Options: nosniff`](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/X-Content-Type-Options) — запрещает MIME-сниффинг; «prevents XSS-attacks where user-uploaded content is executed as an HTML document»; для script/style-запросов браузер блокирует ответ с неверным MIME.
- [`Strict-Transport-Security`](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Strict-Transport-Security) — HTTPS-only + запрет клика сквозь TLS-ошибки; пример MDN `max-age=31536000; includeSubDomains`; preload требует ≥1 года и отдельной подачи в hstspreload.org; осторожность: includeSubDomains прибивает HTTP-сабдомены (у нас все хосты за Caddy HTTPS — риск низкий, но dev-хосты надо учесть).
- [`Referrer-Policy`](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Referrer-Policy) — строгий вариант `no-referrer`; браузерный дефолт `strict-origin-when-cross-origin` уже не отдаёт полный URL кросс-домейнам.

## 4. nginx admin/landing (и Caddy)

Факты (§0): конфиги идентичны и пусты с точки зрения безопасности — нет HSTS, XCTO, frame-защиты, Referrer-Policy, cache-политик; listen 8080 plaintext; TLS терминирует Caddy (ACME).

- Чего не хватает по MDN/OWASP-набору: `X-Content-Type-Options: nosniff` `always`; `X-Frame-Options: DENY` или CSP `frame-ancestors 'none'` (clickjacking; для admin-панели логично `DENY`); `Referrer-Policy: no-referrer` (admin шлёт запросы только на свой origin — referrer наружу не нужен); `Permissions-Policy` (свернуть камеру/микрофон/геолокацию — не используются); cache-политика: Vite-хэшированные ассеты (`/assets/*`) — `Cache-Control: public, max-age=31536000, immutable`, `index.html` — `no-cache` (иначе релизы не доезжают до пользователей). Стоимость — строки `add_header` в двух `nginx.conf`, ноль зависимости.
- HSTS корректнее ставить на Caddy (точка терминирования TLS: заголовок имеет силу только в HTTPS-ответе; MDN: браузеры игнорируют его по HTTP). Caddyfile живёт на сервере и описан в `docs/deployment.md` — изменение это deploy-процедура, не код контейнеров. Неуверенность: я не смог прочитать первоисточник Caddy (caddyserver.com — таймаут), поэтому не утверждаю, ставит ли Caddy что-то по умолчанию; по нашей доке Caddyfile заголовков не содержит — на stage проверяется `curl -sI`.
- TLS-минимум: контейнеры nginx TLS не занимаются вообще (8080 HTTP за reverse_proxy на localhost) — минимум протокола настраивается только в Caddy (`minimum_tls_version`/protocols). Наша дока Caddyfile этих директив не содержит; фактический минимум на stage/prod надо проверить (`openssl s_client`), прежде чем что-то менять. Не подтверждено первоисточником из-за таймаута — помечено как непроверенное.
- Landing security-only: всё выше применимо к landing одинаково (nginx + зависимости); код landing не трогаем.

## 5. Секреты/токены в клиентском коде

Факты (§0): сессия — httpOnly + Secure + SameSite=Lax + `__Host-`-префикс, ставится и ротируется Go-бэкендом; фронт и admin токен в JS не видят; в web storage секретов нет; `NEXT_PUBLIC_*` не используются, `VITE_API_PREFIX` — публичный префикс.

- Риск утечки токена в PWA/SW: отсутствует по построению — SW не перехватывает `/api` (только navigate), не кэширует API-ответы, IndexedDB хранит один boolean; токен живёт в httpOnly cookie, недоступной из JS и SW. Push-payload открывает только same-origin пути (§0).
- Линт-гейты на env-переменные (закрепить текущий ноль):
  - frontend: `no-restricted-syntax`/`no-restricted-properties` на `process.env.NEXT_PUBLIC_*` — сейчас 0 использований; гейт делает любое появление публичной переменной осознанным решением (паттерн admin-конфига с литералами `/api/`).
  - admin: whitelist на `import.meta.env.VITE_API_PREFIX` (уже de-facto один; eslint-правило перечислит допустимое значение, как сделано для fetch-границ).
  - Слой: pre-commit (существующие lint-джобы), нулевая стоимость.
- Гейт на web storage: `no-restricted-globals`/syntax на `localStorage`/`sessionStorage` вне `features/auth/lib/*` — защищает от «положим токен в localStorage» в будущем. Той же природы, что существующий admin-гейт «fetch только в граничных файлах».

## 6. safe-internal-path: от чего защищает и достаточно ли

- Защищает от: протоколо-относительных редиректов (`//evil.com` → внешний уход) и редирект-лупов на `/login`. Используется на обоих выходах `from`-параметра (proxy-редирект и `router.push` после верификации).
- **Найден обход (проверен локально Node/WHATWG URL 2026-08-18)**: строка `/\evil.com` проходит текущую проверку (`startsWith('/')` true, `startsWith('//')` false), но парсер WHATWG URL нормализует `\` в `/` для special schemes: `new URL('/\evil.com', 'https://app.example')` → `https://evil.com/`. В `proxy.ts:68` `NextResponse.redirect(new URL(target, request.url))` отдаёт 302 на внешний домен — открытый редирект с доверенного origin (фишинговый вектор: rentlee.ru/login?from=/\evil.com).
- Асимметрия с собственным кодом: инлайн-копия `resolveClickTarget` в `public/sw.js:205` этот вектор уже режет (`url[1] === '\\'`), `safe-internal-path.ts` — нет.
- Поведение `router.push('/\evil.com')` в Next router не проверял (неуверенность); proxy-ветка подтверждена.
- Достаточность после фикса: односимвольный фикс (`value.startsWith('/\\')` → null) закрывает нормализационный вектор; более строгий вариант — позитивный whitelist по символам пути. Юнит-тест рядом (паттерн `cabinet-routes.test.ts` — guard-тесты на drift).

## 7. Зависимостный максимум

### npm audit: блокирующий на каждый PR vs ночной/pre-push

- Текущее: pre-push blocking (`--audit-level=high`) + nightly strict в security.yml; в ci.yml на PR аудита нет вовсе.
- Факты для решения: advisory-базы обновляются независимо от кода — блокирующий audit на PR без фильтра путей краснеет на «не своих» изменениях (ночью вышла advisory под закешированную версию → утренние PR стоят). Честная блокирующая форма — job в ci.yml с `paths`-фильтром на `**/package-lock.json` (аудит релевантен только когда lockfile меняется) — та же команда `make npm-audit`, ~30–60 с на пакет (npm уже в раннере). Semgrep-страница про p/ci уже видит наш cooldown-комментарий (#318).
- Не переоткрываемое зелёное: pre-push + nightly остаются как есть.

### dependabot-политика

- Есть: cooldown 7d (все экосистемы), группы `minor-patch` (npm×3), лимиты PR, target dev.
- Отсутствует: `assignees` (auto-assign) — поддерживаемое поле конфига Dependabot; `reviewers`/`assignees` на каждую запись. Цена — 1 строка на запись, выигрыш — PR не висят непросмотренные. Группировка для docker/actions отсутствует (там мажорные обновления реже — можно не группировать).

### SBOM

Первоисточник: [Trivy SBOM docs](https://trivy.dev/docs/latest/supply-chain/sbom/) (через поиск 2026-08-18; прямой fetch trivy.dev вернул 404 на старом пути).

- Trivy генерирует CycloneDX (`--format cyclonedx`) и SPDX (`--format spdx`/`spdx-json`) для тех же подкоманд: `trivy fs --format cyclonedx --output sbom.cdx.json .`; по умолчанию CycloneDX-вывод — чистый SBOM без уязвимостей. Один формат за запуск (второй — через `trivy convert`).
- У нас trivy 0.74.0 уже пиннован в Makefile — SBOM получается той же image без новых инструментов; syft добавил бы второй генератор без новой информации. Естественный слой — deploy-джоба (артефакт релиза: «что реально ушло в prod») или nightly; хранение — GitHub artifacts/релизы. Продукт — record-keeping fintech без регуляторного требования SBOM: выигрыш — форензика «какая версия пакета была в проде в дату X» и быстрая проверка при следующем инциденте типа event-stream.

### semgrep-наборы поверх p/ci

Первоисточники: [Semgrep contributing docs](https://docs.semgrep.dev/contributing/contributing-to-semgrep-rules-repository) (confidence-уровни), [semgrep-rules repo](https://github.com/semgrep/semgrep-rules), [Semgrep blog «Don't leak your secrets»](https://semgrep.dev/blog/2021/dont-leak-your-secrets) (p/secrets), [OWASP-набор и обновление маппингов до Top 10 2025](https://semgrep.dev/blog/2026/owasp-top-10-2025-whats-new). Страницы semgrep.dev/p/* рендерятся клиентским JS — WebFetch их не читает, состав наборов взят по докам/блогам и помечен как вторичный.

- Официальная шкала FP: HIGH — «high true positives, useful in CI/CD»; MEDIUM — «some false positives»; LOW — «expect a fair amount of false positives, similar to audit style rules».
- `p/xss` — taint-правила источник→сток для JS/React (`dangerouslySetInnerHTML` с непровалидированными данными, DOM-XSS паттерны). В нашем коде стоков ноль (§0–2), поэтому находок ожидаемо ~0; ценность — автоматический триггер, если сток появится. Как блокирующий на PR — плохой кандидат: taint-правила дают MEDIUM/LOW-confidence находки; как nightly-набор рядом с p/ci — дёшево.
- `p/typescript` — correctness+security TS; пересекается с `tsc --noEmit` (уже в CI) и ESLint — маржинальная ценность низкая, только nightly-«второе мнение».
- `p/secrets` — порты gitleaks-паттернов; у нас gitleaks уже blocking на PR с выверенным allowlist (`.gitleaks.toml`) — дублирование даст расхождения в FP между двумя движками; разве что nightly без блокировки.
- `p/owasp-top-ten` — широкий набор по A01–A10; большинство категорий серверные (инъекции, SSRF, контроль доступа) — для фронтового репо это шум; Semgrep сам перевёл маппинги на Top 10 2025, так что набор живой, но не про наш код.
- Цена любого набора: +минуты ночной джобы и объём отчёта; блокирующими их делать нельзя (FP-природа LOW/MEDIUM правил), максимум — PR `continue-on-error` как у текущего p/ci.

## 8. Выводы для решения

Сводная таблица (слой: где подключается; «LO» = landing security-only):

| # | Мера | Инструмент | Слой | Цена | Выигрыш |
|---|------|-----------|------|------|---------|
| 1 | Фикс `/\`-обхода в `safe-internal-path` + guard-тест | код (1 строка + тест) | pre-commit (vitest) | минуты | закрывает живой open-redirect через `proxy.ts` |
| 2 | Гейт `dangerouslySetInnerHTML` | ESLint (`react/no-danger` в frontend; selector в admin) | pre-commit + CI lint | ноль | закрепляет текущий ноль стоков |
| 3 | Гейт `rehype-raw` (и переопределения `urlTransform`) | ESLint `no-restricted-imports` в frontend | pre-commit | ноль | страхует «trusted environment»-предусловие react-markdown |
| 4 | Статичные security-заголовки: XCTO, Referrer-Policy, Permissions-Policy, frame-защита | `next.config.ts headers()` (frontend) + `add_header … always` в nginx (admin/landing, LO) | код конфигов | строки конфига | MIME-сниффинг, кликджекинг, referrer-утечки |
| 5 | CSP шаг 1 — без nonce (`'unsafe-inline'` script/style, остальное строго) | `next.config.ts headers()` | код конфига | низкая; риск сломать ассеты мал | object-src/base-uri/form-action/frame-ancestors уже работают |
| 6 | CSP шаг 2 — nonce-based (`proxy.ts` + `strict-dynamic`, dynamic rendering) | рецепт офиц. гайда Next 16 | сначала Report-Only на stage | средняя: все страницы dynamic; style-атрибуты framer-motion — проверить | полноценная XSS-глубина; PPR несовместим |
| 7 | HSTS + TLS-минимум на Caddy | Caddyfile (deploy-процедура, docs/deployment.md) | stage → prod | строка конфига; фактический минимум TLS проверить | SSL-stripping, downgrade |
| 8 | Cache-политики nginx (`/assets` immutable, `index.html` no-cache) | nginx admin/landing (LO) | код конфигов | строки | релизы доезжают; меньше перезагрузок ассетов |
| 9 | Env-гейты: `NEXT_PUBLIC_*`=0, `VITE_*` whitelist, web storage вне auth-lib | ESLint обеих апков | pre-commit | ноль | страхует httpOnly-cookie-модель от регрессий |
| 10 | npm audit в ci.yml с paths-фильтром на lockfiles | `make npm-audit` | CI PR | ~1–2 мин на PR с lockfile | блокирует попадание новой уязвимой зависимости в merge |
| 11 | Dependabot `assignees` (+ при желании группы для docker/actions) | `.github/dependabot.yml` | конфиг | строки | PR не висят без владельца |
| 12 | SBOM при деплое (CycloneDX) | существующий trivy 0.74.0, `trivy fs --format cyclonedx` | deploy-джоба, артефакт релиза | ~минута | форензика состава прода |
| 13 | semgrep p/xss (+ p/typescript/p/secrets по вкусу) | security.yml, PR continue-on-error | nightly | минуты, отчётный шум | триггер на появление XSS-стоков; p/owasp-top-ten — не наш профиль |

Против: 6 несовместим с PPR (если займёмся PPR — выбирать SRI-путь или остаться на шаге 5); 13 не блокирует PR по природе FP; @next-safe/middleware исключён (мёртв с 2022).

## Неуверенности и пробелы первоисточников

- OWASP Secure Headers page (owasp.org) и caddyserver.com недоступны (таймаут/403) — заголовочные рекомендации взяты у MDN, дефолты Caddy (TLS-минимум, отсутствие HSTS) не подтверждены первоисточником; фактическое состояние проверить на stage (`curl -sI`, `openssl s_client`).
- Страницы semgrep.dev/p/* непарсимы WebFetch (JS-рендеринг) — состав наборов описан по docs.semgrep.dev/блогам; перед включением конкретного набора посмотреть его состав локально (`semgrep --config p/xss --dry-run` не устанавливая ничего в репо).
- Поведение инлайн style-атрибутов framer-motion/React 19 под nonce-CSP — гипотеза; решается Report-Only-запуском на stage.
- `router.push('/\evil.com')` в Next router не проверял; proxy-ветка обхода подтверждена Node-экспериментом.
