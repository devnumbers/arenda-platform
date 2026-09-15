# Research: TS-экосистема после 18.08.2026 — версии, новые правила, advisory-замеры фронта и админки

- Тикет: #639 (карта #636 «Усиление линтеров»)
- Дата: 2026-09-13
- Предыдущий срез: [2026-08-18-eslint-ts-strictness.md](./2026-08-18-eslint-ts-strictness.md) (#328)
- Метод: advisory-прогоны на фактическом коде через временные конфиги `apps/*/eslint.advisory.config.mjs`, расширяющие основные конфиги приложений (кандидаты в `warn`, type-checked — через `projectService`); после замера оба временных файла удалены, трекаемые файлы не правились. Первоисточники: GitHub Releases eslint/eslint, typescript-eslint/typescript-eslint, vercel/next.js; react.dev; typescript-eslint.io; eslint.org; knip.dev; версии и даты — npm registry и `node_modules` приложений.

## 0. Версии на 13.09.2026 и что вышло после 18.08

| Пакет | Фронт (установл.) | Админка (установл.) | npm latest | После 18.08 |
|---|---|---|---|---|
| eslint | 9.39.4 | 9.39.5 | **10.10.0** (04.09) | 10.9.0 (21.08), 10.9.1 (24.08), 10.10.0 (04.09) — без новых правил |
| typescript-eslint | 8.62.0 | 8.67.0 | **8.70.0** (07.09) | 8.68.0 (24.08), 8.69.0 (31.08), 8.70.0 (07.09) |
| eslint-plugin-react-hooks | 7.1.1 | 7.1.1 | **7.1.1** (17.04) | ничего |
| next / eslint-config-next | 16.3.1 | — | **16.3.5** (11.09) | 16.3.2 (21.08), 16.3.3 (25.08), 16.3.4 (31.08), 16.3.5 (11.09) |
| eslint-plugin-jsx-a11y | 6.10.2 | не установлен | 6.10.2 (26.10.2024) | ничего, пакет стабилен ~2 года |
| eslint-plugin-boundaries | 7.2.0 | не установлен | 7.2.0 (09.08) | 7.2.0 и есть актуальная |
| knip | не установлен | не установлен | **6.35.1** (09.09) | активное развитие, ~релиз в неделю |
| typescript | 5.9.3 | 5.9.3 | — | — |
| babel-plugin-react-compiler | **1.0.0** (dependencies!) | не установлен | 1.0.0 (08.05) | GA-версия компилятора уже во фронте |

Источники: [Releases eslint](https://github.com/eslint/eslint/releases), [Releases typescript-eslint](https://github.com/typescript-eslint/typescript-eslint/releases), [npm next](https://www.npmjs.com/package/next), [npm knip](https://www.npmjs.com/package/knip).

### 0.1. ESLint: мажор 10.x, ветка 9.x заморожена

- **ESLint 10.0.0 вышел 06.02.2026**; обе наши апки остаются на 9.39.x — последней версии 9.x (9.39.5, 10.07.2026; фронт на 9.39.4). После 18.08 вышли только миноры/патчи 10.x: [v10.9.0](https://github.com/eslint/eslint/releases) — опция `checkConditionalExpressions` у `no-unmodified-loop-condition`; v10.10.0 — поддержка флагов `d`/`v` в `no-unexpected-multiline`, усиления `new-cap`/`no-extra-bind`.
- **Ни одного нового core-правила во всей линейке 10.x (10.1–10.10) нет** — проверены все релизы. Активность ушла в опции существующих правил и API (`includeIgnoreFile()`, bulk suppressions, `meta.languages`). Миграция 9→10 — про инфраструктуру, не про правила: [breaking changes 10.0.0](https://github.com/eslint/eslint/releases/tag/v10.0.0) — удалена eslintrc-поддержка, Node `^20.19 || ^22.13 || >=24`, удалены deprecated API (`SourceCode`, rule context), `eslint:recommended` обновлён, комментарии `eslint-env` стали ошибками, включён JSX reference tracking.
- Новые core-правила линейки 9.x, которые мы ещё не трогали (оба уже доступны на 9.39): **`preserve-caught-error`** (v9.35.0, [блог ESLint](https://eslint.org/blog/2025/09/eslint-v9.35.0-released/) — требует `cause` при перебрасывании пойманной ошибки; в 10.7.0 добавлена опция `errorClassNames`) и **`no-unassigned-vars`** (v9.27.0, [блог ESLint](https://eslint.org/blog/2025/05/eslint-v9.27.0-released/) — `let`/`var` без присваиваний, но читаемые).

### 0.2. typescript-eslint 8.63–8.70: breaking нет, одно новое правило

По [релизам](https://github.com/typescript-eslint/typescript-eslint/releases): breaking changes отсутствуют во всех шести релизах после установленной у нас базы. Значимое:

- **8.70.0 — новое правило `no-generated-empty-object-type`** (в `strictTypeChecked`): ловит type-операции, схлопывающиеся в `{}` (`Omit` по union, `NonNullable<unknown>`); [docs](https://typescript-eslint.io/rules/no-generated-empty-object-type). Для нас: не доступно ни на 8.62 (фронт), ни на 8.67 (админка) — требуется подъём до 8.70.
- 8.69.0 — `no-misused-promises` получил опции `checkConditionals` + `flagUnions`.
- 8.68.0 — fix-suggestions для `strict-void-return`.
- 8.64.0 — deprecated `no-loop-func`; 8.65.0 — deprecated extension-rule `no-restricted-imports` (мы базовое core-правило и используем — не задето).
- **`strict-void-return`** — «новое» правило для нас, хотя добавлено ещё [PR #9707](https://github.com/typescript-eslint/typescript-eslint/pull/9707) (мерж 12.01.2026, релиз 8.53.0): «Disallow passing a value-returning function in a position accepting a void function» ([docs](https://typescript-eslint.io/rules/strict-void-return)). TypeScript считает `() => T` присваиваемым к `() => void`, и возвращённое значение молча теряется; для async-функций это скрытый floating promise, для генераторов — никогда не стартующее тело. Вне пресетов; доступно в обеих установленных версиях.
- **`no-unsafe-type-assertion`** — тоже вне пресетов, доступно в 8.62/8.67 (проверено дампом плагина): ловит сужающие `as` (тип более узкий исходного) и `as` из `any` — [docs](https://typescript-eslint.io/rules/no-unsafe-type-assertion).
- Фронт отстаёт от админки на 5 миноров (8.62 против 8.67) и оба — от latest (8.70). В 8.63–8.67 новых правил нет, т.е. сам по себе подъём ничего не включает, но нужен для `no-generated-empty-object-type`.

### 0.3. next 16.3.2–16.3.5: только фиксы, eslint-config-next не менялся

[Release notes](https://github.com/vercel/next.js/releases): 16.3.5 (11.09) — фиксы disk-cache `next/image` (0-byte записи/чтения), CSP nonce для loading/template script-тегов, standalone NFT; 16.3.2–16.3.4 — аналогичные патчи без пометок security. Изменений в `eslint-config-next` в notes нет — состав lint-пресетов 16.3.1 остаётся актуальным. Патч-подъём `next`/`eslint-config-next` 16.3.1 → 16.3.5 — рутинный, но не предмет этого тикета.

### 0.4. react-hooks: 7.1.1 — последняя, compiler-эра стабильна

После 18.08 релизов нет ([npm timeline](https://www.npmjs.com/package/eslint-plugin-react-hooks)): 7.1.1 (17.04.2026) остаётся latest в обоих приложениях. Наблюдение: [react.dev](https://react.dev/reference/eslint-plugin-react-hooks) уже документирует в пресете `recommended` правило `component-hook-factories` (фабрики компонентов в HOF), которого в установленной 7.1.1 ещё нет — признак ближайшего релиза, следить.

## 1. Compiler-powered правила react-hooks v7: что ловят и как включаются

Факт из установленного `eslint-plugin-react-hooks@7.1.1` (дамп `configs.flat.recommended`, фронт): **16 правил**, из них 14 error: `rules-of-hooks`, `static-components`, `use-memo`, `preserve-manual-memoization`, `immutability`, `globals`, `refs`, `set-state-in-effect`, `error-boundaries`, `purity`, `set-state-in-render`, `config`, `gating`; warn: `exhaustive-deps`, `incompatible-library`, `unsupported-syntax`.

Ключевой ответ на вопрос тикета — **включаются ли они отдельно от React Compiler: нет, и не нужно**. Пресет `recommended` включает всё семейство сразу, и, по [react.dev](https://react.dev/reference/eslint-plugin-react-hooks): «React Compiler diagnostics are automatically surfaced by this ESLint plugin, and can be used even if your app hasn't adopted the compiler yet». То есть compiler-powered правила — это diagnostics движка компилятора, доступные без его включения; при включённом же компиляторе компонент с нарушением просто пропускается компилятором (skip), а не ломает сборку.

Состояние у нас:

- **Фронт**: всё семейство уже активно через `eslint-config-next` 16.3.1 (проверено дампом), и сверх этого во фронте стоит `babel-plugin-react-compiler: 1.0.0` (dependencies) — компилятор включён, правила гоняются не как «холодная» диагностика, а в связке с реальной компиляцией. Дельты к react-hooks нет; единственный открытый пункт — `exhaustive-deps` уже флипнут в error (#390).
- **Админка**: react-hooks 7.1.1 `recommended` включён (#391), `exhaustive-deps` — warn. Ничего нового с 18.08 не появилось.

## 2. Дельта strictTypeChecked vs cherry-pick фронта

Честная дельта считана дампом пресетов установленной 8.62: `strictTypeChecked` (100 правил) ∖ `stylisticTypeChecked` (46) ∖ `recommended` (уже активен во фронте через `eslint-config-next/typescript`) ∖ 13 правил, включённых вручную (§0 доков #328/#330). Остаток — 39 правил, из них:

- **18 правил из recommendedTypeChecked** — фронт их не включил поштучно, но админка имеет их даром через `extends: recommendedTypeChecked`: `await-thenable`, `no-array-delete`, `no-duplicate-type-constituents`, `no-for-in-array`, `no-implied-eval`, `no-redundant-type-constituents`, `no-unsafe-argument/-assignment/-call/-enum-comparison/-member-access/-return/-unary-minus`, `only-throw-error`, `prefer-promise-reject-errors`, `restrict-plus-operands`, `unbound-method`. Примечание: в установленных версиях `no-floating-promises` и `unbound-method` уже входят в `recommendedTypeChecked` (проверено дампом) — вопреки представлению, что `no-floating-promises` строго strict-only.
- **13 strict-only корректностных**: `no-misused-spread`, `no-mixed-enums`, `no-unnecessary-type-conversion`, `no-unnecessary-type-parameters`, `no-unnecessary-type-arguments`, `no-unnecessary-boolean-literal-compare`, `prefer-reduce-type-parameter`, `prefer-return-this-type`, `related-getter-setter-pairs`, `use-unknown-in-catch-callback-variable`, `return-await`, `no-meaningless-void-operator`, `no-non-null-asserted-nullish-coalescing` (+ чистая стилистика strict-блока, не кандидат: `no-dynamic-delete`, `no-invalid-void-type` и др.).
- **2 правила вне пресетов**: `strict-void-return`, `no-unsafe-type-assertion` (+ недоступная на 8.62 `no-generated-empty-object-type` из 8.70).

## 3. Advisory-замеры (13.09.2026)

Прогон: временный `eslint.advisory.config.mjs` поверх основного конфига каждого приложения, кандидаты в warn, type-checked через `projectService`; generated-каталоги админки исключены (как в основном конфиге). Примеры — файл:строка.

### 3.1. Сводная таблица: правило — фронт — админка — сигнал

| Правило | Фронт | Админка | Сигнал |
|---|---|---|---|
| `@typescript-eslint/no-unsafe-type-assertion` (вне пресетов) | **59** (56 app + 3 e2e) | **35** (19 `dataProvider.ts`, 4 `authProvider.ts`, 4 `tariffs.tsx`) | **сильный** — сужающие `as`; топ фронта: `shared/ui/text-field/TextField.tsx` (9), `shared/ui/select/Select.tsx` (6), `shared/api/client.ts` (3) |
| `@typescript-eslint/strict-void-return` (вне пресетов, 8.53+) | **37** (35 app + 2 e2e) | 0 | **средний** — возвращаемые значения, теряемые в void-контекстах; топ: `widgets/property-detail/ui/PropertyDetailPage.tsx` (7), `app/ui-kit/page.tsx` (5, демо-страница), тесты (6) |
| `preserve-caught-error` (core 9.35+, требует `cause`) | 0 | **4** — все `src/authProvider.ts:88,100,108,110` | **средний** — перебрасывание ошибки без причины; фронт чист |
| `@typescript-eslint/no-unnecessary-type-conversion` | 0 | **4** (`authProvider.ts:93`, `tariffs.tsx:173,232`, `userSubscription.tsx:429`) | слабый, точечные фиксы |
| `@typescript-eslint/no-deprecated` | 0 (уже error) | **2** (`LoginPage.tsx:28,51` — deprecated `FormEvent`) | слабый |
| `@typescript-eslint/no-unnecessary-condition` | 0 (уже error) | **1** (`dataProvider.ts:94`) | слабый |
| `@typescript-eslint/no-unnecessary-type-parameters` | 0 | **1** (`dataProvider.ts:206`) | слабый |
| `@typescript-eslint/no-redundant-type-constituents` | **1** (`shared/lib/notifications/base.ts:51`) | 0 | бесплатный фикс |
| `@typescript-eslint/no-unnecessary-boolean-literal-compare` | **1** (`widgets/rentals/ui/wizard-chrome.tsx:357`) | 0 | бесплатный фикс |
| `@typescript-eslint/no-mixed-enums`, `no-misused-spread`, `related-getter-setter-pairs`, `use-unknown-in-catch-callback-variable`, `prefer-reduce-type-parameter`, `prefer-return-this-type`, `return-await`, `no-meaningless-void-operator`, `no-non-null-asserted-nullish-coalescing`, `no-unnecessary-type-arguments` | 0 | 0 | гейты без текущей цены |
| recommendedTypeChecked-остаток (18 правил: `no-unsafe-*`, `unbound-method`, `restrict-plus-operands`, `only-throw-error`, `await-thenable`, `no-for-in-array`, `no-implied-eval`, `no-array-delete`, `no-duplicate-type-constituents`, `prefer-promise-reject-errors`, `no-unsafe-enum-comparison`, `no-unsafe-unary-minus`) | 0 | уже включено (пресет) | подтверждение: svg-декларация `shared/assets/svg.d.ts` погасила `no-unsafe-*` (в августе было 34) |
| `@typescript-eslint/switch-exhaustiveness-check` | 0 (уже error) | 0 | админке — бесплатно |
| `react-hooks/exhaustive-deps` | error (уже флипнут) | 0 нарушений на warn | админке флип бесплатен |
| `preserve-caught-error`-компаньон `no-unassigned-vars` (core 9.27+) | 0 | 0 | оба гейта без цены |
| jsx-a11y `recommended` (в админку — замер через node_modules фронта, зависимость не добавлялась) | error (уже включён) | **2** — только `no-autofocus` (`LoginPage.tsx:101,135`) | админке решение копеечное: автофокус логин-формы — осознанный UX, вопрос flip/опции, не работы |
| **Стилистика (контроль, не кандидаты):** `consistent-type-definitions` | **319** (в августе 240) | 0 | рост подтверждает «не брать» |
| `array-type` | **201** (в августе 31) | 0 | рост подтверждает «не брать» |
| `consistent-indexed-object-style` | **137** (было 140, теперь в т.ч. app-код: `app/(screens)/*/page.tsx`) | 0 | не брать |
| `no-invalid-void-type` | **20** (все `features/*/api/hooks.ts` — react-query-паттерн) | 0 | не брать без конфиг-опций |

Попутная находка (вне кандидатов, базовый next-пресет, warn): **`@next/next/no-img-element` — 8** во фронте (свойства/контакты/платежи/задачи аватары и селекты, напр. `entities/property/ui/PropertyAvatar.tsx:59`) — в августе прогон был чист; кандидаты на `next/image` или явное решение.

### 3.2. Выводы по замерам

1. **Корректностный остаток strictTypeChecked на фронте почти исчерпан**: из 31 correctness-кандидата шум дают только два правила вне пресетов (`no-unsafe-type-assertion` 59, `strict-void-return` 37) и два одиночных фикса. Вся «пресетная» часть — ноль. Это меняет фокус карты: следующая еда — не пресеты, а поимённые правила вне них.
2. **`no-unsafe-type-assertion` — главный кандидат обеих апок** (59+35). Ловит реальный класс: сужающий `as` в UI-китах и мапперах фронта; в админке 19 из 35 — в `dataProvider.ts` (парсинг ответов). Фиксы локальные, правило type-checked с projectService.
3. **`preserve-caught-error` — единственное новое core-правило 9.x с сигналом**, причём только в админке (4 в `authProvider.ts`); во фронте 0. Дёшево и полезно: сохранение `cause` при перебрасывании — диагностируемость ошибок.
4. **`strict-void-return`** — 37 на фронте, но треть — демо-страница ui-kit и тесты; в app-коде 35, подозрительных мест немного (обычно «onX: () => что-то-возвращающее»). Дешёвый partial: включить и почистить, либо подождать fix-suggestions (8.68+ требует подъём).
5. **Стилистические числа фронта выросли с августа** (240→319, 31→201) — ещё одно подтверждение решения «stylistic-пресеты не брать целиком»: тело растёт быстрее, чем успеваем чистить.
6. **Админка**: сигналов мало, все точечные (49 сообщений суммарно на 9 файлов); `dataProvider.ts`/`authProvider.ts` — концентратор (19+4 unsafe-assertions, 4 preserve-caught-error, 1+1).

## 4. Админ-дельта: фактура для grilling-тикета «Админка»

Сводка расхождений с фронтом (помимо §3):

1. **`exhaustive-deps` не флипнут в error** (фронт — #390). Нарушений 0 — флип бесплатен, переводит будущие пропуски в блокер.
2. **jsx-a11y отсутствует полностью** (плагин не установлен). Замер recommended (плагин подгружен из node_modules фронта, зависимость не добавлялась): 2 нарушения (`no-autofocus` в LoginPage). Фронтовский вердикт августа «в админку jsx-a11y не добавлять» подтверждается цифрой 2 — цена символическая, но и выигрыш мал (экраны на MUI/react-admin дают семантику сами). Решение — за grilling: либо рекомендованный пресет с 2 фиксами, либо ничего.
3. **Core-правила 9.x не включены**: `preserve-caught-error` (сигнал 4) и `no-unassigned-vars` (0, гейт).
4. **`switch-exhaustiveness-check` не включён** (0 — бесплатно).
5. **recommendedTypeChecked-база у админки шире фронта** — парадоксально, но админка уже гоняет всё семейство `no-unsafe-*`/`unbound-method`/`no-floating-promises` через extends, тогда как фронт держит это поимённо. Зеркальная дельта — у фронта (§2).
6. **strict-only остаток** админки — те же 26 правил, что и у фронта (§2), с нулевой текущей ценой кроме перечисленных в §3.1.
7. **typescript-eslint 8.67 vs latest 8.70** — подъём мелкий; `no-generated-empty-object-type` доступен только с 8.70.
8. **ESLint 10** — обе апки на 9.39.x; миграция отдельно от правил (§0.1), спешки нет: 9.39.5 — финальная ветка 9.x, новых правил в 10.x ноль.

## 5. Итог: кандидаты по волнам

**Волна A — бесплатно (0–1 фикс на правило):**
фронт — `no-redundant-type-constituents` (1), `no-unnecessary-boolean-literal-compare` (1), core `no-unassigned-vars` (0, гейт); админка — `switch-exhaustiveness-check` (0), `exhaustive-deps` → error (0), core `no-unassigned-vars` (0). Попутно фронт: разобрать 8 warn `@next/next/no-img-element` (фиксы или осознанное решение).

**Волна B — средняя цена, главный сигнал:**
`no-unsafe-type-assertion` (фронт 59 / админка 35 — фикс-волна, потом error); core `preserve-caught-error` (0/4, `cause` в authProvider); админка — `no-unnecessary-type-conversion` (4), `no-deprecated` (2), `no-unnecessary-condition` (1), `no-unnecessary-type-parameters` (1).

**Волна C — решения карты:**
`strict-void-return` (37/0 — чистить app-код и тесты либо ждать 8.68+ suggestions с подъёмом); подъём typescript-eslint 8.62/8.67 → 8.70 ради `no-generated-empty-object-type`; jsx-a11y в админку (2 находки — вопрос «да/нет», не цены); ESLint 9→10 — отдельная миграция без линт-дивидендов.

**Не брать (подтверждение августовских решений свежими числами):** stylistic-пресеты целиком (фронтовые счётчики выросли: 319/201/137/20), `knip` как линт-гейт пока не обсуждался в тикетах — инструмент ([knip.dev](https://knip.dev/), 6.35.1) зрелый, но это отдельный grilling о dead-code-политике, не про правила линтера.

## 6. Ограничения замера

- `no-generated-empty-object-type` не мерилась (нет ни в 8.62, ни в 8.67).
- jsx-a11y в админке мерился плагином из node_modules фронта (6.10.2) — для постоянного включения это будет новая devDependency.
- Фронтовые счётчики включают e2e/ (разбивка в §3.1); advisory-конфиг фронта не изолировал демо-страницу `app/ui-kit/page.tsx` — её вклад указан отдельно (5 из 37 `strict-void-return`).
