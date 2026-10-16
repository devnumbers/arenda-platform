# CSP шаг 2: разведка nonce + strict-dynamic в Report-Only

Дата: 2026-08-23
Вход для: тикет [#406](https://github.com/devnumbers/arenda-platform/issues/406) «Режим качества: CSP шаг 2 — Report-Only с nonce на stage», часть спеки [#378](https://github.com/devnumbers/arenda-platform/issues/378) (карта [#326](https://github.com/devnumbers/arenda-platform/issues/326), решение [#331](https://github.com/devnumbers/arenda-platform/issues/331)). Фактура для будущего блокирующего решения по nonce-CSP.

Файл фиксирует: механизм разведки (что включается и где), данные прогона на production-билде, выводы для блокирующего решения. Прогон локальный (тот же образ кода, что уедет на stage) — стейджинговые данные накопятся в Uptrace после следующего деплоя stage; локальный срез отвечает на все вопросы гипотез из `2026-08-18-frontend-security.md` §3.

## Механизм (что приземлилось)

- `apps/frontend/proxy.ts` — при `CSP_REPORT_ONLY=true` на каждый запрос генерируется nonce (`shared/lib/csp.ts`, рецепт [гайда Next 16](https://nextjs.org/docs/app/guides/content-security-policy): base64(randomUUID)). Заголовок запроса `Content-Security-Policy-Report-Only` Next разбирает при SSR и вешает nonce на фреймворк-скрипты и инлайн-стили — проверено по исходнику Next 16.3.1 (`dist/server/app-render/app-render.js:209`: `headers['content-security-policy'] || headers['content-security-policy-report-only']`). Заголовок ответа — одноимённый, браузер его не блокирует, только репортит.
- Полис: `script-src 'self' 'nonce-…' 'strict-dynamic'; style-src 'self' 'nonce-…'` + общий с шагом 1 хвост `CSP_BASE_DIRECTIVES` (единственный источник для новых origin'ов: `default-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; upgrade-insecure-requests`) + `report-uri /api/csp-report` (+`'unsafe-eval'` в script-src только в dev). Источники зеркалят шаг 1; nonce только в script/style — ровно будущая блокирующая полиса.
- Сбор отчётов: `app/api/csp-report/route.ts` (статический роут выигрывает у BFF-catch-all `[...path]`, на бэкенд не проксируется) — POST, обе проволочные формы (`application/csp-report` и Reporting-API-массив), каждая находка — одна stdout-строка JSON `{"msg":"csp_report",…}`; stdout контейнера собирает Vector и уносит в Uptrace (`docs/deployment.md` § Observability). Битое тело → 204 без исключения, GET → 405.
- Включение: `deploy/docker-compose.stage.yml` (env `CSP_REPORT_ONLY: "true"` сервису frontend, паттерн `BACKEND_URL`). Prod-compose переменную не получает; отсутствие переменной = байтово прежнее поведение. Matcher proxy не менялся (кабинетные префиксы + `/login`) — auth-логика не тронута.
- Юнит-швы: `shared/lib/csp.test.ts` (13 тестов: полиса, nonce-уникальность, парсер обеих форм, усечение sample, устойчивость к мусору).

## Верификация механизма (production-билд, curl)

С флагом (`CSP_REPORT_ONLY=true next start`):

- `/login` и редирект `/dashboard` → `/login?from=` несут `Content-Security-Policy-Report-Only` с nonce + strict-dynamic + `report-uri /api/csp-report`; nonce уникален на запрос (два запроса — два nonce).
- Блокирующий CSP шага 1 и остальные security-заголовки (`X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, `Permissions-Policy`) рядом, без изменений.
- Динамическая страница (`/tenants/[id]`): 26 атрибутов `nonce=` в HTML, значение совпадает с nonce заголовка запроса.
- POST `/api/csp-report` с телом отчёта → 204 + одна строка `"msg":"csp_report"` в stdout; мусор → 204; обращений к бэкенду по этому пути нет.

Без флага: заголовков Report-Only нет (0), nonce в HTML нет, шаг-1 CSP на месте, auth-редирект `/dashboard` → `/login?from=` прежний (307). Прод-конфигурация не изменилась.

## Данные прогона (Chromium через Playwright, 16 страниц кабинетных + анонимный /login, production-билд, фейковый бэкенд `/me`→200 для сессии)

Сводка по директивам (in-page события `securitypolicyviolation`, disposition=report): **215 script-src-elem** (193 same-origin чанка `/_next/static/chunks/*.js` + `turbopack-*`, 22 inline) и **5 style-src** (4 style-src-attr, 1 style-src-elem — на `/leases`; в логе эндпоинта 3+3 за тот же проход). report-uri доставил 206 POST'ов за проход — конвейер «браузер → эндпоинт → stdout» работает сквозным образом.

Разбивка по страницам:

| Класс страниц | Страницы прогона | Нарушений |
|---|---|---|
| В matcher'е proxy, **динамический рендер** | `/properties`, `/finance/operations`, `/tenants/[id]` | **0** |
| В matcher'е proxy, **статический пререндер** (○ в таблице маршрутов) | `/login` (аноним: 16), `/dashboard`, `/leases`, `/tenants`, `/finance`, `/profile`, `/profile/info`, `/profile/info/privacy`, `/profile/info/terms`, `/profile/tariff`, `/ui-kit` | 15–23 на страницу, все script-src-elem |
| Вне matcher'а proxy | `/support`, `/calendar` | заголовка нет вовсе (см. пробелы) |

Главные факты:

1. **Динамические страницы чисты.** Ноль нарушений на всех динамически рендерящихся страницах: nonce из заголовка запроса Next вешает на SSR-скрипты, а `strict-dynamic` пропагирует доверие на все чанки, догружаемые в рантайме (единственный источник скриптов — непронonce-енный пререндер). Service worker чист: 0 отчётов с `sw.js` за весь проход — регистрация `/sw.js` под `script-src 'self' … 'strict-dynamic'` не репортится.
2. **Вся масса script-нарушений — статический пререндер.** У статических (○) страниц HTML сгенерирован на билде без nonce, а `strict-dynamic` заставляет браузер игнорировать `'self'` — поэтому каждый чанк и каждый inline-скрипт репортится. Это не «наш код плохой», это класс: для блокирующего режима все статические страницы придётся перевести в динамический рендер (гайд: `await connection()`; кабинетные страницы и так должны рендериться в контексте сессии). Оценка класса по таблице маршрутов билда: почти все страницы кабинета сейчас ○ — перевод в динамику и есть основная работа блокирующего шага.
3. **Инлайн-стили — единственный реальный код-фикс.** Все style-нарушения сосредоточены на `/leases`: инлайн-атрибуты прогресс-бара аренды (`entities/lease/ui/LeaseInfo.tsx`: `style={{width: …%}}`, `style={{left: calc(…)}}`) плюс один инжект `style`-элемента. Важно: **nonce не покрывает style-атрибуты** — CSP разрешает их только через `'unsafe-inline'`/`'unsafe-hashes'`. Блокирующий режим потребует переписать прогресс-бар (CSSOM-манипуляция через ref или nonced `<style>`), а не просто добавить nonce.
4. Гипотеза research-2026-08-18 про framer-motion/HeroUI SSR-стили: на прогоне не подтвердилась массово — HeroUI/Tailwind ходят внешним CSS-файлом, framer-motion применяет стили через CSSOM (CSP не регулирует). Единственный style-очаг — LeaseInfo выше.
5. Слепые зоны Report-Only: `upgrade-insecure-requests` браузер игнорирует (консольное предупреждение, ожидаемо по спеке CSP — директива без репорт-семантики), `frame-ancestors` — тоже: embedding-вектор эта разведка не измеряет вовсе. Обе оставлены в полисе для зеркальности с будущей блокирующей. Механизм доставки — `report-uri` (deprecated в пользу Reporting API `report-to`), но поддерживается всеми актуальными браузерами и для разведки достаточен.

## Пробелы, найденные прогоном (вне скоупа тикета, зафиксировать)

- **`/support` и `/calendar` не входят в matcher proxy** — на них не только нет Report-Only заголовка, но и (pre-existing) нет auth-редиректа прокси; данные они тянут через backend-auth (`/api/*` → 401), так что утечки нет, но auth-периметр кабинета неровный. Отдельный тикет на выравнивание matcher'а.
- `/` и `/subscription/payments/[id]/success|fail` тоже вне matcher'а (у корня — server-side `redirect()` на `/dashboard`, у subscription-страниц свой сценарий) — при блокирующем шаге решение про их покрытие нужно принять явно.

## Что дальше (вход в блокирующее решение, отложено до стейдж-данных)

После ближайшего деплоя stage (compose уже несёт флаг) отчёты пойдут в Uptrace с реальных сессий; запрос — по `msg=csp_report` в логах сервиса frontend окружения stage. Ожидание по локальному срезу: картина та же (масса script-src-elem со статических страниц, единичные style-src с `/leases`). Блокирующее решение = (1) перевод кабинета в динамический рендер, (2) фикс LeaseInfo-стилей, (3) явное решение по покрытию matcher'а — после чего flip `CSP_REPORT_ONLY` → блокирующая полиса в `proxy.ts` механический.
