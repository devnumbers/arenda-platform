# Research: строгость ESLint/TypeScript — полный каталог против текущих конфигов фронта и админки

- Тикет: #328 (карта #326, режим качества фронта/админки; пересматривает решения #295 с полномочиями zero-tolerance)
- Дата: 2026-08-18
- Метод: пробные прогоны на фактическом коде через временные конфиги вне репо (`/tmp/*.mjs` — ESLint, `/tmp/tsconfig.*.json` — tsc; ничего не установлено, трекаемые файлы не правились) + первоисточники: react.dev, GitHub Releases facebook/react, docs typescript-eslint, README jsx-a11y; фактические версии — из `apps/*/node_modules` (typescript-eslint 8.62.0 во фронте — транзитивно с eslint-config-next 16.3.1; 8.67.0 в админке; eslint-plugin-react-hooks 7.1.1 и jsx-a11y 6.10.2 во фронте, в админке обоих нет; eslint-plugin-react-compiler не установлен нигде).

## 0. Базовая линия: что уже включено (факты, не документация)

Фронт (`apps/frontend/eslint.config.mjs`: core-web-vitals + typescript + boundaries) разворачивается в (дамп пресетов из `node_modules/eslint-config-next/dist/`):

- **typescript-eslint `recommended` (без type-checked)**: `no-explicit-any`, `ban-ts-comment`, `no-unused-vars` и ещё ~20 правил — источник: `eslint-config-next/typescript` = `tseslint.configs.recommended` (проверено дампом).
- **react-hooks 7.1.1 `recommended` — 16 правил**, из них 14 в error: `rules-of-hooks`, `static-components`, `use-memo`, `preserve-manual-memoization`, `immutability`, `globals`, `refs`, `set-state-in-effect`, `error-boundaries`, `purity`, `set-state-in-render`, `config`, `gating`; warn: `exhaustive-deps`, `incompatible-library`, `unsupported-syntax`. То есть **compiler-powered правила React Compiler уже гоняются** — включая `purity`/`set-state-in-render`/`refs`, прямо закрывающие пункты рубрики CODING_STANDARDS «useEffect synchronizer» и «useState for derived state».
- **jsx-a11y: 6 правил, все warn** (`alt-text`, `aria-props`, `aria-proptypes`, `aria-unsupported-elements`, `role-has-required-aria-props`, `role-supports-aria-props`).
- Текущий прогон фронта: **0 ошибок, 0 предупреждений** (варнинги next-пресетов тоже пусты).

Админка (`apps/admin/eslint.config.mjs`, волна 2C #307): `no-explicit-any` + три no-restricted-правила; пресетов нет, `rules-of-hooks` **не проверяется вообще** (плагин не установлен).

## 1. typescript-eslint type-checked (фронт)

Пробный прогон: текущий конфиг + `strictTypeChecked` + `stylisticTypeChecked` (scoped на `**/*.{ts,tsx}` с `projectService: true`) = **1022 ошибки в 247 из 473 файлов**. Разбивка по зонам: app-код 853, `shared/api/generated.ts` 140 (все — `consistent-indexed-object-style`), тесты/e2e 29.

### 1.1. Корректностное ядро (ловит ошибки агента до ревью)

| Правило | Ошибок | Природа по факту |
|---|---|---|
| `no-floating-promises` | 131 | 103 в `features/*/api/hooks.ts` — почти все `queryClient.invalidateQueries()` в `onSuccess` (canonical react-query, лечение — префикс `void`); остальные ~28 — реальные плавающие вызовы в виджетах, ровно запах «Floating promise in a handler» из рубрики CODING_STANDARDS |
| `no-misused-promises` | 36 | async-функции в `onPress`/`onClick` с void-сигнатурой — unhandled rejection в хендлере; реальный класс, фикс — catch/обёртка |
| `no-unnecessary-condition` | 63 | всегда-истинные проверки, `??` на непустом типе; смесь реальных находок и мест, где тип объявлен шире фактического — правка кодом |
| `no-unsafe-assignment`/`-member-access`/`-argument` | 34 | **не HeroUI**: все трассируются в svg-иконки `@/shared/assets/icons` — для `*.svg` нет ambient-декларации (`declare module '*.svg'`), импорты = `any`. Один файл `svg.d.ts` гасит всё семейство |
| `no-unnecessary-type-assertion` | 11 | авто-фикс `--fix` |
| `no-non-null-assertion` | 6 | `!`-assertion; в strict-пресете, брать |
| `no-deprecated` | 13 | использование API, помеченных `@deprecated` (включая типы из next/react) |
| `no-base-to-string` | 4 | потенциальный `[object Object]` при стиринговании |
| `require-await` | 2 | async без await |

### 1.2. Шум, лечимый конфигом (без подавлений в коде)

| Правило | Дефолт | С опцией | Остаток |
|---|---|---|---|
| `no-confusing-void-expression` | 161 | `ignoreArrowShorthand: true` | **1** (`TariffChangeForm.tsx:155`, реальная находка) |
| `restrict-template-expressions` | 91 (все — интерполяция `number`) | `allowNumber: true` | **0** |

Числа — из отдельных пробных прогонов с этими опциями. Для ru-RU продукта потеря `allowNumber` не критична: деньги и так обязаны идти через `formatMoneyKopecks` (правило №8 рубрики), а «голые» числа в строках — осознанное решение.

### 1.3. Stylistic-часть пресетов — не брать целиком

`consistent-type-definitions` 240 (interface↔type), `consistent-indexed-object-style` 140 (все в generated-файле), `array-type` 31, `prefer-nullish-coalescing` 21 (пограничное: фикс реален и дёшев, кандидат отдельным правилом), `no-invalid-void-type` 18, прочее <10. В агентном контексте почти нулевой выигрыш: правила не ловят ошибки, только унифицируют стиль. Решение #295 «полные пресеты не включаем» подтверждается — но теперь с фактом: пресет **не монолит**, ядро (1.1) стоит взять поимённо.

### 1.4. Правила вне пресетов (пробный прогон №2)

| Правило | Конфиг | Ошибок | Вердикт |
|---|---|---|---|
| `explicit-function-return-type` | `allowExpressions + allowIIFEs` | **172** | **Подтверждение отклонения #295**: даже смягчённый вариант — 172 аннотации в app-коде; выигрыш в агентном контексте не отличается от tsc-вывода типов. Не брать |
| `consistent-type-imports` | дефолт | **25** | брать, `--fix` чинит автоматически; дубль на уровне tsc — `verbatimModuleSyntax` (см. §5) |
| `switch-exhaustiveness-check` | дефолт | **1** (`ToastBody.tsx:32`) | брать — бесплатно; ловит забытый кейс при расширении union-статусов (частая агентская ошибка при добавлении значения enum'а на бэкенде) |

В typescript-eslint 8.62/8.67 правило не депрекировано (проверено `meta.deprecated` в установленном пакете).

### 1.5. react-hooks/exhaustive-deps: warn → error

Текущий прогон: **0 предупреждений**. Флип в error бесплатен сегодня и превращает будущие пропуски депсов из «заметки» в блокер pre-commit. `recommended-latest` (дельта к `recommended` = только `void-use-memo`) не включать — сам README плагина называет его «bleeding edge experimental».

## 2. eslint-plugin-react-hooks v6/v7 и eslint-plugin-react-compiler

Хронология по первоисточникам (GitHub Releases facebook/react):

- v6.0.0 (окт 2025) — «mistakenly released and immediately deprecated and untagged on npm»; **v6.1.0** — первый официальный 6.x: «Flat config is now the default recommended preset», compiler-правила внутри.
- **eslint-plugin-react-compiler — deprecated**, правила слиты в react-hooks v6.1+ (npm warn deprecated, react issue #31481; трекинг expo #44237). react.dev: «React Compiler diagnostics are automatically surfaced by this ESLint plugin, and can be used even if your app hasn't adopted the compiler yet».
- v7.1.1 (2026-04-17) — актуальная: ESLint v10, «skipping compilation for non-React files», улучшенные `set-state-in-effect`/`refs`.

Выводы для нас:

- **Отдельный `eslint-plugin-react-compiler` не ставить никогда** — deprecated и дублирует то, что уже включено у нас транзитивно.
- Фронт: ничего делать не нужно — react-hooks 7.1.1 `recommended` уже активен через eslint-config-next 16.3.1; единственное действие — `exhaustive-deps: error` (§1.5).
- **Админка: добавить `eslint-plugin-react-hooks` (devDependency)** — сегодня `rules-of-hooks` там не проверяется вовсе, а это правило-краш (условный вызов хука = runtime-ошибка). Единственная новая зависимость во всём каталоге.

## 3. jsx-a11y

Пробный прогон полного пресета `flatConfigs.recommended` (~30 правил) на фронте: **9 ошибок**, все — в наших собственных компонентах, ни одной в HeroUI:

| Правило | Ошибок | Где |
|---|---|---|
| `click-events-have-key-events` | 4 | `shared/ui/select/Select.tsx` (собственный div-селект) |
| `anchor-is-valid` | 2 | `features/.../PhoneStep.tsx` |
| `no-autofocus` | 2 | `widgets/.../PhoneChangeForm.tsx` (OTP-инпут; если автофокус осознанный UX — это кандидат на конфиг-опцию, не подавление) |
| `no-static-element-interactions` | 1 | `CategorySelect.tsx` |

**Вердикт: брать `recommended` целиком** — цена 9 фиксов (клавиатурная поддержка двух кастомных контролов ui-kit — реальная a11y-работа), и далее гейт для агента: новый интерактивный div без клавиатуры блокируется. В админку jsx-a11y не добавлять: MUI-компоненты react-admin дают семантику сами, своих JSX-примитивов у админки почти нет (32 файла, экраны на готовых контролах).

## 4. no-restricted-syntax для доменных инвариантов

### 4.1. Деньги-копейки: что изменилось с #295

#295 отклонил стат-линт денег как «инструментально ненадёжный: деньги отличимы от int64 только по контексту». AST-неоднозначность **не изменилась** — изменились рамка и данные:

- рамка #326 — zero-tolerance с полномочиями убрать подавления;
- измерена фактическая картина: денежных сайтов с сырой арифметикой `/100`/`*100` **10** (три пары parse/format в `OperationEditForm`, `RecurringOperationEditPage`, `LeaseEditForm`, `LeaseCreateWizard` + сам `format-money.ts`); **процентных** сайтов-омонимов **3** (`LeaseInfo.tsx:93,107` — `* 100` для fill-процентов, `ProfitReport.tsx:208` — `(income_kopecks/maxIncome)* 100`); `.toFixed` вне форм — 4 файла (3 денежных + 1 не-денежный в property-attributes — форматирование площади).

Отсюда два честных пути (выбор за картой #326):

1. **Двухшаговый гейт (рекомендую)**: сначала код-подготовка — извлечь пару хелперов в `shared/lib/format-money.ts` (например `kopecksToInputString`/`inputToKopecks`, у них другая семантика, чем у `formatMoneyKopecks`: без ₽ и группировки разрядов) и перевести 3 процентных сайта на явный `percent()`-хелпер; затем `no-restricted-syntax`: запрет `BinaryExpression[operator in ['/','*']][right Literal 100]` и `MemberExpression[property='toFixed']` вне `shared/lib/format-money.ts`. После подготовки у селектора **0 ложных срабатываний по построению** (allowlist файловый, а не семантический) — возражение #295 снимается: линт больше не обязан понимать, деньги это или проценты.
2. Оставить прозой (как #295) — цена: агент пишет `(kopecks / 100).toFixed(2)` в новой форме мимо ревью.

Путь 1 — это ~13 правок кода + 2 селектора; «formatMoneyKopecks-обязательность» для display-строк закрывается тем же селектором `.toFixed`.

### 4.2. Опасные браузерные API

Сегодня в обоих приложениях **0 вхождений** `dangerouslySetInnerHTML`/`innerHTML`/`eval`/`new Function`/`document.write`/`insertAdjacentHTML` (react-markdown без rehype-raw рендерит в React-элементы). Гейт `no-restricted-syntax` + `no-restricted-globals` на этот список — нулевой цены, защищает от XSS-регрессии агента. Брать.

## 5. tsconfig сверх текущего (оба strict)

Пробные прогоны `tsc --noEmit` через `/tmp/tsconfig.*-trial.json` (extends текущих конфигов приложений):

| Опция | Фронт (ошибок) | Админка | Вердикт |
|---|---|---|---|
| база | 0 | 0 | — |
| `noUncheckedIndexedAccess` | **35** (21×TS2322, 7×TS2532, 5×TS2345, 2×TS2538) | **6** (5 в `propertyAttributes.test.ts`, 1 в generated) | **брать**: цена — точечные `??`/проверки на индексном доступе; выигрыш — `arr[i]` больше не врёт про `undefined`, класс агентских NPE на границах массивов |
| `exactOptionalPropertyTypes` | **+393** (428 суммарно) | **+1** | **не брать**: 359×TS2375 — передача `prop: string \| undefined` в `prop?: string`. Это системный конфликт с сигнатурами React-компонентов (включая HeroUI `CardRootProps.className`, наши ui-kit враперы) и внешних библиотек; лечится только массовыми `?: \| undefined` в чужих типажах. Цена/выигрыш безнадёжны для нас |
| `noPropertyAccessFromIndexSignature` | **1675** | — | не брать |
| `verbatimModuleSyntax` | **2** | **2** (`authProvider`/`dataProvider` type-only импорты) | брать: tsc-уровневый дубль `consistent-type-imports`, чинит разом и будущий код |
| `noImplicitOverride` | 1 | 0 | брать, бесплатно |
| `noUnusedLocals`/`noUnusedParameters` | 0 | уже вкл. | брать во фронте: 0 ошибок сейчас, дубль eslint-`no-unused-vars` |
| `noImplicitReturns` | 2 | 0 | брать, почти бесплатно |

## 6. Админка: добор от минимального конфига #307

Пробные прогоны наложением на текущий конфиг:

**Шаг 1 — `recommendedTypeChecked`: 56 ошибок.** По правилам: `no-misused-promises` 12 (все — async `onSubmit`/`onClick` в MUI/react-admin формах: `LoginPage`, `tariffs`, `userSubscription`, `subscriptionPayments` — реальные unhandled rejections), `no-unsafe-*` 28 (нетипизированный JSON в `dataProvider.ts` — 16 ошибок в одном файле), `no-unnecessary-type-assertion` 5, `require-await` 4, `no-base-to-string` 3 (**реальный bug-класс** `[object Object]` при стиринговании значений в dataProvider/`userSubscription`), `no-unused-vars` 3 (`_params`), `restrict-template-expressions` 1. **Вердикт: брать целиком** — сигнал/шум высокий, все находки содержательные.

**Шаг 2 — `strictTypeChecked` + `stylisticTypeChecked`: 177**, с теми же React-friendly опциями (`ignoreArrowShorthand`, `allowNumber`) — **80**. Дельта шага 2 сверх шага 1: `prefer-nullish-coalescing` 12, `no-unnecessary-condition` 3, `consistent-type-definitions` 3, `no-unnecessary-type-conversion` 3, `no-deprecated` 2, `no-unnecessary-type-parameters` 1. **Вердикт: шаг 1 сейчас, шаг 2 — по желанию после шага 1** (дельта мала и неоднородна; берётся поимённо `prefer-nullish-coalescing` при желании).

**Шаг 3 — `eslint-plugin-react-hooks` recommended** (новая devDependency, §2) — против текущего кода админки expected 0 (react-admin хуки не пишет своих), гейт — на будущий код.

## 7. Шум на HeroUI v3 / react-admin 5 / CSS Modules

- **HeroUI v3 (3.2.1): 0 замечаний**, атрибутируемых типам HeroUI, во всех type-checked прогонах. Семейство `no-unsafe-*` целиком из наших svg-импортов без декларации (§1.1) — фикс одним `svg.d.ts`, не конфигом и не подавлением. Единственное трение с HeroUI — `exactOptionalPropertyTypes` (TS2375 на `className?: string`) — ещё один аргумент против этой опции, а не против HeroUI.
- **CSS Modules**: ни одной находки во всех прогонах (`styles` из `*.module.css` типизируется штатно).
- **react-admin/MUI**: `no-misused-promises` на async `onSubmit` — не шум, а реальный класс (см. §6); лечение кодом (catch), конфиг-послабление `checksVoidReturn.attributes: false` из доков typescript-eslint не применять — оно глушит как раз тот случай, который ловим.
- **generated-код**: `consistent-indexed-object-style` 140 в `shared/api/generated.ts` — стилистику на generated-файлы не гонять (files-scoped блок конфига), correctness-правила оставить.

## 8. Итог: волны зажатия

**Волна A — бесплатно или почти (0–2 фикса, конфиг + 1 новый файл):**
`react-hooks/exhaustive-deps: error`; `switch-exhaustiveness-check`; `consistent-type-imports`; tsconfig: `verbatimModuleSyntax` (2+2), `noImplicitOverride` (1+0), `noUnusedLocals/Parameters` (0), `noImplicitReturns` (2+0); запрет опасных браузерных API (§4.2); `shared/assets/svg.d.ts` (гасит 34 `no-unsafe-*`).

**Волна B — средняя цена, ядро ценности:**
фронт — type-checked ядро поимённо: `no-floating-promises` (131: ~103 фикса — префикс `void` на `invalidateQueries`, ~28 — реальные catch), `no-misused-promises` (36 фиксов хендлеров), `no-unsafe-*` (после svg.d.ts ≈ 0), `no-unnecessary-type-assertion` (11, авто-фикс), `no-non-null-assertion` (6), `no-confusing-void-expression` c `ignoreArrowShorthand` (1), `restrict-template-expressions` c `allowNumber` (0), `no-base-to-string` (4), `require-await` (2); jsx-a11y `recommended` (9); `noUncheckedIndexedAccess` (35/6); админка — `recommendedTypeChecked` (56) + `eslint-plugin-react-hooks`.

**Волна C — решения карты #326:**
деньги-гейт двухшагово (§4.1, путь 1 рекомендован); `no-unnecessary-condition` (63+3, правка кодом); `prefer-nullish-coalescing` (21+12); админка strict-дельта.

**Не брать (подтверждения #295 свежими данными):** `explicit-function-return-type` (172 даже с allowExpressions/allowIIFEs — «шум > выигрыш» подтверждён числом); `exactOptionalPropertyTypes` (393, системный конфликт с React-пропами); `noPropertyAccessFromIndexSignature` (1675); stylistic-пресеты целиком; `eslint-plugin-react-compiler` (deprecated, слит в react-hooks v6.1+); `recommended-latest` react-hooks (bleeding edge); jsx-a11y в админку; `checksVoidReturn.attributes:false`.

**Причина пересмотра #295 (фиксация):** карта #326 дала zero-tolerance полномочия и сняла «подавления в коде» как доступное лечение; фактически изменилось три вещи — (1) измеренная цена: то, что выглядело дорогим, оказалось дешёвым (type-checked ядро с React-friendly опциями сходится к ~170 реальным фиксам на 473 файлов, а не к тысячам), и наоборот (`exactOptionalPropertyTypes` — 393); (2) react-hooks v7.1.1 с compiler-правилами уже включён транзитивно с eslint-config-next 16.3.1 — факта, которого #295 не знал; (3) обнаружен типизационный пробел `*.svg`, порождавший «шум», который раньше списали бы на строгий линт. Отказ #295 от стат-линта денег пересмотрен ограниченно: не «умный» семантический линт, а файловый allowlist после код-подготовки (§4.1).
