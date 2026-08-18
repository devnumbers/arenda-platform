# Research: внешний бенчмарк практик Next.js 16 против нашего apps/frontend

- Тикет: #327 (карта wayfinder #326)
- Дата: 2026-08-18
- Метод: web-research по первоисточникам — официальная документация Next.js **16.3.1** (страницы несут `version: 16.3.1`, lastUpdated 2026-08-11), `facebook/react` (README + CHANGELOG `eslint-plugin-react-hooks`), react.dev, и **реальные файлы эталонных репозиториев** через GitHub Contents API (`raw`). Локальная верификация нашего стека: `apps/frontend/{AGENTS.md,CODING_STANDARDS.md,eslint.config.mjs,tsconfig.json,next.config.ts,package.json}`, содержимое `node_modules/eslint-config-next@16.3.1/dist` и фактический `eslint --print-config`.
- Эталонный набор: vercel/commerce, steven-tey/dub, calcom/cal.com, midday-ai/midday, дефолтный шаблон create-next-app (vercel/next.js canary). **Documenso выбыл**: веб-приложение переехало на React Router 7 («Remix») + Hono (`apps/remix/package.json`: `@react-router/node ^7.18.1`, `react-router build`) — Next.js там больше нет; ниже учитываются только его общие TS-практики.

## 1. Официальный бар Next.js 16.3.1

### 1.1 Граница сервер/клиент ([docs](https://nextjs.org/docs/app/getting-started/server-and-client-components))

Ключевые предписания (цитаты дословные):

- «Use **Client Components** when you need: State and event handlers … Lifecycle logic (useEffect). Browser-only APIs … Custom hooks» — «Use **Server Components** when you need: Fetch data … Use API keys … Reduce the amount of JavaScript sent to the browser».
- «`"use client"` is used to declare a **boundary** between the Server and Client module graphs … Once a file is marked with `"use client"`, **all of its imports and the components it directly renders are included in the client bundle**» — директива нужна не на каждом клиентском файле, а на границе.
- Про размещение границы: «To reduce the size of your client JavaScript bundles, add `'use client'` to specific interactive components instead of marking large parts of the UI as Client Components» — тянуть границу к листьям.
- Сериализуемость пропсов: «Props passed to Client Components need to be serializable».
- Провайдеры: «render providers as deep as possible in the tree … This makes it easier for Next.js to optimize the static parts».
- `server-only` / `client-only` пакеты — build-time защита от «environment poisoning» (серверный код случайно попал в клиентский бандл): «if you try to import the module into a Client Component, there will be a build-time error».

Наш `AGENTS.md` («Server Components by default; use `'use client'` only for interactivity, browser APIs, or hooks») и правило CODING_STANDARDS про `await searchParams` в серверной странице соответствуют документации один-в-один.

### 1.2 Async-интерфейсы (Promise-based `params`/`searchParams`)

Документ 16.3.1 показывает сигнатуру страниц как `params: Promise<{ id: string }>` и `const { id } = await params` прямо в примерах Server Component. Наше правило ревью «Synchronous `searchParams` — reading the Next 16 promise directly → `await` it in the server page» повторяет официальный пример. Гейта на это в eslint-config-next нет — у нас он остаётся правилом ревью (дёшево, т.к. нарушение роняет рендер страницы сразу и ловится e2e/build).

### 1.3 Паттерны данных: server fetch vs react-query

У Next.js 16 два равноправных официальных пути ([client-side data fetching](https://nextjs.org/docs/app/guides/client-side-data-fetching), [tanstack-query](https://nextjs.org/docs/app/guides/client-side-data-fetching/tanstack-query)):

- **Server Components** — дефолт для первичного рендера (fetch близко к источнику, меньше JS в браузере).
- **TanStack Query на клиенте** — официальный гайд, не компромисс: «TanStack Query can fetch entirely in the browser **when the initial view can wait for a browser request after hydration**» — то есть чисто-клиентский fetch после гидратации легитимен, когда первый экран может ждать. Гайд описывает три паттерна: `useQuery` с inline loading/error, `useSuspenseQuery` за Suspense-границей, и серверный prefetch + `dehydrate`/`HydrationBoundary` с pending-дегидрацией (TanStack Query ≥5.40).
- Официальный «cache contract»: ключ и опции держат вместе — «Keep the key and query options together so both call sites share the same identity» через `queryOptions({...})` (`productCache.key`/`productCache.options`); для Cache Components к нему добавляется `tag`. Провайдер: «Create a new query client for each server render and reuse one query client in the browser» (`getQueryClient()` c singleton в `window`).

Наша карта (`react-query` на клиенте, единый реестр ключей `shared/api/query-keys.ts`, `QueryClient` в `shared/providers/query-provider.tsx` c `staleTime: 30_000`, `refetchOnWindowFocus: false`) — соответствует официальному «browser-only fetch» паттерну. Отличия от гайда: (а) ключи у нас в общем реестре, а не colocated `queryOptions`-контракты — сознательная плата за запрет cross-slice импортов (реестр в `shared` — единственное место, которое видят все фичи); (б) серверного prefetch/гидратации нет — все запросы после гидратации; гайд это разрешает, для cabinet-приложения за логином цена первого запроса приемлема. `useSuspenseQuery` не используем — inline `isLoading`+skeleton закреплён нашим рубриком «Naked data surface».

### 1.4 Каталог правил eslint-config-next / @next/eslint-plugin-next ([docs](https://nextjs.org/docs/app/api-reference/config/eslint))

Состав пакета: `@next/eslint-plugin-next` + `recommended` сеты `eslint-plugin-react` и `eslint-plugin-react-hooks`. Пресеты:

- `eslint-config-next` — база (Next + React + hooks).
- `eslint-config-next/core-web-vitals` — «upgrades rules that impact Core Web Vitals from warnings to errors. Recommended for most projects» (дефолт create-next-app).
- `eslint-config-next/typescript` — «TypeScript-specific linting rules from types-eslint … based on `plugin:@typescript-eslint/recommended`».

Полный каталог `@next/eslint-plugin-next` (recommended): `google-font-display`, `google-font-preconnect`, `inline-script-id`, `next-script-for-ga`, `no-assign-module-variable`, **`no-async-client-component`**, `no-before-interactive-script-outside-document`, `no-css-tags`, `no-document-import-in-page`, `no-duplicate-head`, `no-head-element`, `no-head-import-in-document`, `no-html-link-for-pages`, **`no-img-element`**, `no-page-custom-font`, `no-script-component-in-head`, `no-styled-jsx-in-document`, `no-sync-scripts`, `no-title-in-document-head`, `no-typos`, `no-unwanted-polyfillio`. Многие — pages-router-легаси; App-Router-релевантные: `no-async-client-component`, `no-img-element`, `no-html-link-for-pages`, `no-head-element`, `no-css-tags`, `no-sync-scripts`, `inline-script-id`.

Важно: «Starting with Next.js 16, `next lint` is removed» — ESLint только через CLI; `eslint`-опция в next.config больше не нужна. У нас так и есть (`npm run lint` → eslint CLI).

## 2. React-бар: react.dev и eslint-plugin-react-hooks

### 2.1 react.dev, [Rules of React](https://react.dev/reference/rules)

- «Components must be idempotent — … always return the same output with respect to their inputs — props, state, and context».
- «Side effects must run outside of render — Side effects should not run in render, as React can render components multiple times».
- «Props and state are immutable»; «Values are immutable after being passed to JSX».
- «They are rules — and not just guidelines — in the sense that if you broken them, your app likely has bugs». Рекомендованное средство принуждения: Strict Mode + `eslint-plugin-react-hooks`.

### 2.2 eslint-plugin-react-hooks v6 → v7 (мы уже на v7)

Тикет спрашивал v6; фактически наш стек новее. Цепочка (верифицировано по файлам):

- `apps/frontend/package.json`: `eslint-config-next: 16.3.1`; его зависимости: `eslint-plugin-react-hooks: ^7.0.0`; установленная версия — **7.1.1**.
- `node_modules/eslint-config-next/dist/index.js` (строка 168): rules = спред `...eslint-plugin-react-hooks.configs.recommended.rules` — т.е. пресет `recommended` плагина целиком.
- CHANGELOG плагина: 6.1.0 — flat-config-by-default, новые violations (`use` в try/catch, `useEffectEvent` в произвольных замыканиях); **7.0.0: «slims down presets to just 2 configurations (`recommended` and `recommended-latest`)», и «all compiler rules are enabled by default»** в `recommended`; 7.1.x — ESLint v10, улучшенные `set-state-in-effect`, ref-валидация, отчёт всех ошибок компилятора.
- Фактическая проверка `npx eslint --print-config shared/lib/navigation.ts` в apps/frontend подтверждает активные правила: `react-hooks/rules-of-hooks: error`, `exhaustive-deps: warn`, **`purity: error`, `set-state-in-effect: error`, `set-state-in-render: error`, `static-components: error`, `use-memo: error`, `immutability: error`, `refs: error`, `preserve-manual-memoization: error`, `globals: error`, `error-boundaries: error`, `gating: error`, `incompatible-library: warn`, `unsupported-syntax: warn`**.

Вывод: наш CODING_STANDARDS-рубрика («useEffect-synchronizer», «manual memoization», «useState for derived state») уже **автоматизирована** compiler-правилами через один лишь `eslint-config-next` — «derive during render» и «не мутируй» теперь ловит линт, а не только ревью. Отдельно ставить `recommended-latest` не нужно (это bleeding-edge поверх уже полного набора). Документировать этот факт в CODING_STANDARDS стоит — агенты сейчас считают эти запахи «только ревью».

## 3. Эталонные проекты (конфиги — цитаты из реальных файлов)

Сводка:

| Проект | Структура | Линт | tsconfig-строгость | Security-заголовки |
|---|---|---|---|---|
| **create-next-app (шаблон `app/ts`, next.js canary)** | `app/` плоско | ESLint flat: core-web-vitals + typescript, **плюс biome.json в шаблоне** | `strict: true`, `bundler`, `react-jsx` (идентично нашему) | нет |
| **vercel/commerce** | `app/ components/ lib/` | **нет ESLint** — `test` = `prettier --check` | `strict: true` + **`noUncheckedIndexedAccess: true`**, `forceConsistentCasingInFileNames` | нет; `experimental: { ppr, inlineCss, useCache }` |
| **dub (steven-tey/dub)** | apps/web: `app/ lib/ ui/ pages/` (плоско, без фич-слоёв) | **нет ESLint-конфига и eslint-депс**; только prettier; скрипт `"lint": "next lint"` (в Next 16 эта команда удалена — легаси) | `strict: false` (!), но `strictNullChecks: true` | `Referrer-Policy: no-referrer-when-downgrade`, `X-Frame-Options: DENY`, CSP `frame-ancestors *` только для `/embed/*` |
| **cal.com** | apps/web: `app/ components/ lib/ modules/ server/ pages/` (feature-папки `modules/*` + общие `components`), monorepo-пакеты | **Biome** (`biome.json`), ESLint отсутствует | base: `strict: true`, `useUnknownInCatchVariables: true`, `noUnusedLocals/Parameters: false` | `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`, `X-Frame-Options: DENY` на `/auth/*` и `/signup`, CORP `cross-origin` на embed |
| **midday-ai/midday** (apps/dashboard) | monorepo-пакеты (`ui`, `api`, …) | **Biome** в корне | `strict` в tsconfig, но **`typescript.ignoreBuildErrors: true`** в next.config | `X-Frame-Options: DENY` (всё, кроме api/proxy); **`poweredByHeader: false`**, `reactStrictMode: true` явно |
| **documenso** | `apps/remix` = **React Router 7 + Hono** — не Next.js | Biome | strict | — (выбыл из сравнения) |

Детали, важные для бенчмарка:

- **vercel/commerce** (эталон Vercel!) вообще не линтится ESLint'ом: `package.json` `"test": "pnpm prettier:check"`, deps без eslint. Зато единственный с `noUncheckedIndexedAccess: true` — и работает на canary-стеке (`ppr`, `useCache`, `inlineCss`).
- **dub** (`apps/web/tsconfig.json`): `"strict": false, "strictNullChecks": true` — крупный продакшн-Next-код живёт и так; вся строгость у них договорная. ESLint-пакетов в `apps/web/package.json` нет (проверено по deps), в корне — только `prettier.config.js`.
- **cal.com**: `AGENTS.md` в репо содержит агентные правила уровня наших: «Never use `as any`», «Import directly from source files, not barrel files», «Put permission checks in `page.tsx`, never in `layout.tsx`», «Use Biome for formatting and linting». Показательно: их запрет barrel-импортов («not `@calcom/ui`» а `@calcom/ui/components/button`) — противоположность нашему FSD-правилу «слайс импортируется только через index.ts»: у них мотив — tree-shaking/циклы в большом UI-пакете, у нас — контракт слайса. Оба подхода мейнстримны; конфликт не про «кто прав», а про то, что публичный API слайса у нас — единственная дверь, и это осознанное решение (см. §4).
- **midday**: `next.config.ts` — `output: "standalone"` (как у нас), `poweredByHeader: false`, `generateBuildId` от GIT SHA, но при этом `typescript.ignoreBuildErrors: true` — типы проверяются отдельно (`typecheck`-скрипт), не билдом.

## 4. Против нашего apps/frontend: где отклоняемся и что переносим

Наш бар: ESLint = core-web-vitals + typescript + eslint-plugin-boundaries (FSD-гейты) + no-restricted-imports (DTO-изоляция, запрет @heroui/styles); tsconfig = create-next-app-дефолт (`strict`, без дополнительных флагов); next.config = `output: 'standalone'`, `reactCompiler: true`, единственный header — `Cache-Control: no-store` для `/sw.js`.

### 4.1 Где наш бар ВЫШЕ мейнстрима (не подтягивать вниз)

1. **Архитектурные гейты.** Ни один эталон (commerce, dub, cal.com, midday) не имеет линт-гейтов на структуру: у них плоские `app/ lib/ ui/` или `modules/*` без формальных границ; cal.com живёт с `strict`-культурой без eslint вовсе (Biome). Наш FSD + boundaries + публичные API слайсов — строго всего эталонного набора. В агентном контексте это правильно: граница, enforcement которой живёт в `eslint.config.mjs`, ловит циклический/утекающий импорт от ИИ-агента **до ревью**, тогда как у эталонов это только конвенция/ревью. Вывод: FSD-девиация от мейнстрима — осознанная и для агентного кода более сильная; ничего откатывать не надо.
2. **strict TS.** У нас `strict: true` (дефолт cna); dub — `strict: false`; midday — `ignoreBuildErrors`. Наш бар выше или равен всем.
3. **react-hooks compiler-линт.** Благодаря v7-цепочке (§2.2) мы автоматически получаем compiler-правила — у Biome-эталонов (cal.com, documenso, midday) аналога React-compiler-проверок нет (Biome не запускает React Compiler). Скрытое преимущество нашего ESLint-стека; отметить в CODING_STANDARDS.
4. **react-query.** Официальный гайд Next.js легитимен для нашего паттерна (browser-fetch после гидратации, единый реестр ключей). Отклонение от гайдового `queryOptions`-контракта — вынужденное (cross-slice запрет) и дешёвое; рассинхрон ключей у нас ловится тем, что ключ живёт в одном реестре, а не в N вызовах.

### 4.2 Что переносим (цена/выигрыш в агентном контексте)

| # | Практика | Источник | Цена | Выигрыш | Агентная ценность |
|---|---|---|---|---|---|
| 1 | **Security-заголовки**: `X-Frame-Options: DENY` (или frame-ancestors), `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin` на `/:path*` в `next.config.ts` | dub, cal.com, midday — все три живых Next-эталона | ~10 строк в `next.config.ts` (у нас уже есть секция `headers`) | кликджекинг/MIME-сниффинг/referrer-утечки закрыты дефолтом | средняя: агент сам их не добавит и не сломает; это продакшн-гигиена, а не ловец ошибок |
| 2 | **`poweredByHeader: false`** | midday `next.config.ts` | 1 строка | минус `X-Powered-By: Next.js` из ответов | низкая; чистая гигиена |
| 3 | **`noUncheckedIndexedAccess: true`** в tsconfig | vercel/commerce `tsconfig.json` | разовый прогон `tsc` и правка мест, где `arr[i]` используется без проверки (генерированный `shared/api/generated.ts` и `shared/api/**` придётся исключить или чинить регенерацией) | `arr[i]` становится `T | undefined` на этапе компиляции — класс ИИ-ошибок «обращение по индексу без проверки» умирает в `npm run build` | высокая: ловит частый агентный баг до ревью; самый ценный пункт после заголовков |
| 4 | **Фиксация факта в CODING_STANDARDS**: compiler-правила react-hooks уже активны (§2.2), рубрики «useEffect-synchronizer / manual memoization / derived state» — теперь дублируются линтом | eslint-config-next 16.3.1 + наша верификация print-config | пара строк документации | агенты перестанут считать эти запахи «только-ревью» и будут чинить их по линту до пуша | высокая, бесплатно |
| 5 | (опционально) `useUnknownInCatchVariables` | cal.com `packages/tsconfig/nextjs.json` | входит в `strict: true` (см. TS-доки: это часть strict) — у нас уже неявно активно | — | проверено: пункт уже покрыт нашим `strict` |

### 4.3 Что НЕ переносим

- **Biome-миграция** (cal.com, documenso, midday, и даже шаблон `biome.json` появился в create-next-app): мотив эталонов — скорость и форматирование, но мы теряем `eslint-plugin-boundaries`, DTO-гейты и, главное, React-compiler-правила react-hooks v7 (Biome их не умеет). Наша связка строгее; миграция — шаг вниз для агентного качества.
- **`useSuspenseQuery` + HydrationBoundary/prefetch** (официальный гайд): решает проблему «первый экран ждёт клиентский fetch». У нас экраны за авторизацией и skeleton-паттерн закреплён; переход усложнит каждую страницу ради экономии одного клиентского раунда. Вернуться к этому имеет смысл только если LCP-метрики кабинета станут проблемой.
- **Barrel-запрет cal.com**: противоречит FSD-контракту слайса; их мотивация (tree-shaking большого UI-пакета) к нам не применима — у нас `shared/ui` маленький и tree-shaking'ся через per-folder `index.ts`.
- **CSP** (dub делает только `frame-ancestors` для embed; cal.com «Strict CSP policy (for style-src) is not yet supported in production» — цитата из их next.config): полноценный nonce-CSP с HeroUI/Tailwind — дорогой и хрупкий; ни один эталон не держит строгий CSP в проде. Не сейчас.
- **`reactStrictMode` явно**: у нас не задан; midday задаёт явно. Однозначного дефолта в этом исследовании не проверял (страницу reactStrictMode не фетчили) — при желании задать `true` явно стоит копейки, но и вреда от неявного состояния не зафиксировано. Низкий приоритет.

### 4.4 Пробелы, которые НЕ закрываются ничем из эталонов

- **Гейт на размещение `'use client'`** («к листьям») — ни в официальных правилах, ни у эталонов нет готового линт-правила; `@next/next/no-async-client-component` покрывает только подмножество. Остаётся правилом ревью (наш AGENTS.md уже формулирует). Самодельное правило (запрет `'use client'` вне `ui/`/`api/`-подпапок) — возможный будущий гейт, но ни один первоисточник такого не предписывает; не выдумываем.
- **`await` для Promise-пропсов** (`searchParams`/`params`) — линта нет ни у Next, ни у эталонов; наше ревью-правило остаётся единственной защитой (ошибка громкая — рендер падает).

## 5. Неуверенность и ограничения

- Документация nextjs.org отражает версию 16.3.1 (указана на страницах), но отдельные страницы обновляются независимо (eslint-страница lastUpdated 2025-11-10); каталог правил совпадает с таблицей на странице, но новые правила могли добавиться после этой даты.
- Дефолт `reactStrictMode` в App Router не проверялся по первоисточнику — суждение в §4.3 сформулировано без утверждения о дефолте.
- vercel/commerce работает на canary-стеке (next 15.6.0-canary, `ppr`/`useCache`) — их `noUncheckedIndexedAccess` перенесён независимо от canary-фич.
- «Расширь список по разведке» — добавлен midday-ai/midday (популярный Next.js-16 SaaS-монорепо); documenso выбыл по факту фреймворка. Других кандидатов уровня cal.com/dub на Next 16 в разумном времени не разведано — список эталонов не претендует на полноту.
- Все цитаты конфигов — снапшот GitHub Contents API на 2026-08-18 (ветки по умолчанию: main у dub/cal.com/midday/commerce, canary у vercel/next.js).

## 6. Резюме для карты #326

Наш frontend-бар по строгости **выше любого из эталонов** (ESLint-гейты границ + DTO-изоляция + strict TS + react-hooks compiler-правила из коробки) — откатывать FSD к «плоским feature-папкам мейнстрима» не нужно: в агентном контексте формальные гейты ценнее конвенций. Реальные заимствования, подтверждённые практикой эталонов: (1) security-заголовки в next.config — все три живых Next-эталона, у нас нет; (2) `poweredByHeader: false`; (3) `noUncheckedIndexedAccess: true` из vercel/commerce — самый ценный линт-уровень защиты от типовых ИИ-ошибок; (4) документирование уже активных compiler-правил react-hooks. Biome-миграция, prefetch-гидратация react-query, строгий CSP и откат barrel-контрактов — отклонены с обоснованием выше.
