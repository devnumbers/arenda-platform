# Ремедиационная сетка фронта и админки: advisory-счётчики «семейство правил × слайс/ресурс»

Материал решения тикета [#332](https://github.com/devnumbers/arenda-platform/issues/332) (карта [#326](https://github.com/devnumbers/arenda-platform/issues/326)).
Бар качества принят [#330](https://github.com/devnumbers/arenda-platform/issues/330); каталог правил и глобальные счётчики — research [#328](https://github.com/devnumbers/arenda-platform/issues/328) (`docs/research/2026-08-18-eslint-ts-strictness.md`). Этот файл добавляет недостающее измерение — разбивку по слайсам FSD (фронт) и ресурсам (админка) — и вход для плана ремедиации [#333](https://github.com/devnumbers/arenda-platform/issues/333).

## Метод

Замер на живом коде 2026-08-18, dev `ee70b19`. Одноразовые advisory-конфиги `apps/*/eslint.advisory.mjs` (действующий конфиг + правила бара #330; не коммитятся, удалены после прогона):

- **Фронт** — поверх действующего конфига (next-пресеты + boundaries): jsx-a11y `recommended` целиком в error (плагин уже зарегистрирован next-пресетами — ре-регистрация запрещена, поднимаются только rules); type-checked блок `projectService: true` с ядром поимённо: `no-floating-promises`, `no-misused-promises`, `no-unsafe-{assignment,member-access,argument,call,return}`, `no-unnecessary-type-assertion`, `no-non-null-assertion`, `no-base-to-string`, `require-await`, `no-deprecated`, `no-confusing-void-expression` (`ignoreArrowShorthand`), `restrict-template-expressions` (`allowNumber`), `no-unnecessary-condition`, `prefer-nullish-coalescing`, `switch-exhaustiveness-check`, `consistent-type-imports`, `react-hooks/exhaustive-deps: error`; один `no-restricted-syntax` с селекторами бара: браузерные запреты (`dangerouslySetInnerHTML`, `document.write/writeln`, `innerHTML`, `new Function`, `eval`) + деньги (`/100`, `*100` обе стороны, `.toFixed`) вне `shared/lib/format-money.ts`. Прогон `npx eslint --config eslint.advisory.mjs -f json`.
- **Админка** — поверх действующего конфига (волна 2C): `recommendedTypeChecked` (scoped на `src/**`), `projectService`, `prefer-nullish-coalescing`, react-hooks `recommended` (плагин 7.1.1 импортирован абсолютным путём из `apps/frontend/node_modules` — devDependency админки ещё не установлена; для замера достаточно), деньги-селекторы те же вне `src/fields.tsx` (URL-литералы и деньги объединены в один `no-restricted-syntax`; в `fields.tsx` абсолютных URL нет — проверено grep).
- **tsconfig-набор бара** — отдельные прогоны `tsc --noEmit` по trial-конфигам (extends действующего + одна опция, `incremental:false`): фронт — `noUncheckedIndexedAccess` (nuia), `verbatimModuleSyntax` (vms), `noImplicitOverride` (nio), `noUnusedLocals/Parameters` (nul), `noImplicitReturns` (nir); админка — nuia/vms/nio/nir. Пути в выводе tsc относительные; для per-slice атрибуции парсить по `^(файл.tsx?)\((line,col)\): error TS` (скобки в сообщениях и в `app/(cabinet)` ломают наивные сплиты).

**Сверка с research #328 — полное совпадение поимённо** (код между замерами не двигался): фронт floating 131, condition 63, misused 36, unsafe 34, type-imports 25, nullish 21, deprecated 13, assertion 11, non-null 6, base-to-string 4, a11y 9, require-await 2, switch 1, void 1; tsc 35/2/1/0/2. Админка пресет 56 (misused 12, unsafe 28, assertion 5, require-await 4, base 3, unused 3, restrict-template 1), nullish 12; tsc 6/2/0/0. Отклонения — два новых факта, см. «Новое сверх #328».

## Фронт: сетка «слайс × семейство»

Семейства: **B-core** — type-checked ядро (floating, misused, unsafe, assertion, non-null, base-to-string, require-await, deprecated, void, restrict-template); **A** — конфигурные ESLint-правила (type-imports, switch; браузерные запреты и exhaustive-deps дали 0); **a11y** — jsx-a11y; **C** — деньги-селекторы, no-unnecessary-condition, prefer-nullish-coalescing; **tsc** — пять tsconfig-опций. `no-unsafe-*` (34) входит в B-core, но по бару гасится файлом `shared/assets/svg.d.ts` волны A — код-фиксов не требует.

| Слайс | Всего | B-core | A | a11y | C | tsc |
|---|---|---|---|---|---|---|
| widgets/property-detail | 43 | 22 | 1 | 0 | 10 | 10 |
| widgets/operations | 41 | 10 | 0 | 3 | 23 | 5 |
| widgets/profile | 41 | 17 | 2 | 2 | 19 | 1 |
| shared | 39 | 9 | 2 | 1 | 13 | 14 |
| features/properties | 36 | 34 | 1 | 0 | 1 | 0 |
| features/operations | 21 | 19 | 1 | 0 | 1 | 0 |
| widgets/leases | 20 | 10 | 0 | 0 | 10 | 0 |
| features/recurring-operations | 17 | 16 | 1 | 0 | 0 | 0 |
| widgets/cabinet-layout | 14 | 13 | 0 | 0 | 1 | 0 |
| widgets/properties | 14 | 8 | 1 | 1 | 2 | 2 |
| features/leases | 13 | 12 | 1 | 0 | 0 | 0 |
| features/billing | 11 | 10 | 1 | 0 | 0 | 0 |
| features/property-attributes | 9 | 5 | 0 | 0 | 2 | 2 |
| widgets/tenants | 9 | 5 | 0 | 0 | 4 | 0 |
| entities/operation | 7 | 5 | 0 | 0 | 2 | 0 |
| features/free-reminders | 7 | 6 | 1 | 0 | 0 | 0 |
| features/push-notifications | 7 | 3 | 1 | 0 | 2 | 1 |
| features/auth | 6 | 3 | 1 | 2 | 0 | 0 |
| entities/billing | 5 | 5 | 0 | 0 | 0 | 0 |
| features/property-contacts | 5 | 4 | 1 | 0 | 0 | 0 |
| корень (конфиги) | 5 | 1 | 1 | 0 | 1 | 2 |
| features/reminders | 4 | 3 | 1 | 0 | 0 | 0 |
| features/tenant-contacts | 4 | 3 | 1 | 0 | 0 | 0 |
| widgets/finance | 4 | 4 | 0 | 0 | 0 | 0 |
| entities/calendar | 3 | 2 | 0 | 0 | 0 | 1 |
| entities/lease | 3 | 0 | 0 | 0 | 3 | 0 |
| features/profile | 3 | 2 | 1 | 0 | 0 | 0 |
| widgets/dashboard | 3 | 1 | 0 | 0 | 2 | 0 |
| widgets/free-reminders | 3 | 1 | 0 | 0 | 2 | 0 |
| widgets/tenant-detail | 3 | 2 | 0 | 0 | 1 | 0 |
| features/notification-preferences | 2 | 1 | 1 | 0 | 0 | 0 |
| features/operation-categories | 2 | 1 | 1 | 0 | 0 | 0 |
| features/popups | 2 | 1 | 1 | 0 | 0 | 0 |
| features/subscription | 2 | 0 | 1 | 0 | 1 | 0 |
| widgets/calendar | 2 | 0 | 0 | 0 | 0 | 2 |
| app | 1 | 0 | 1 | 0 | 0 | 0 |
| features/access | 1 | 0 | 1 | 0 | 0 | 0 |
| features/finance | 1 | 0 | 1 | 0 | 0 | 0 |
| **Итого** | **413** | **238** | **26** | **9** | **100** | **40** |

Ссылочная разбивка tsc-колонки: `shared` 14 = Icon 4, PullToRefresh 3, ToastProvider 2, query-provider 2 (vms), navigation 1, api/errors 1 (nio), ServiceWorkerRegister 1 (nir); `widgets/property-detail` 10 = StatusBadge 6, SharingModal 3, DetailPage 1; `widgets/operations` 5 = DetailPage 4, CreateWizard 1; `widgets/properties` 2 = AddressField + PropertyAddressStep; `корень` 2 = playwright.config.ts; остальные по 1–2.

### Горячие точки и классы фиксов (вход для #333)

- **`no-floating-promises` (131, 35% счётчика)** — почти все в `features/<slice>/api/hooks.ts` (properties 28, operations 19, recurring-operations 16, leases 12, billing 10, free-reminders 6, property-contacts 4, reminders/tenant-contacts по 3, прочие 1–2): преимущественно `queryClient.invalidateQueries()` в `onSuccess`. Механический класс фикса (префикс `void` / явный catch), распределён по 15+ слайсам дёшево.
- **`no-unnecessary-condition` (63)** — widgets/profile 18, widgets/operations 10, widgets/property-detail 8, widgets/tenants 4: всегда-истинные проверки, `??` на непустом типе; правка по месту, местами — сужение объявленного типа.
- **`no-misused-promises` (36)** — async-хендлеры в `onPress`/`onClick`: widgets/profile 11, widgets/property-detail 5, widgets/operations 5, widgets/tenants 4; фикс — catch-обёртка.
- **`no-unsafe-*` (34)** — svg-импорты без ambient-декларации: widgets/cabinet-layout 11, features/properties 6, widgets/property-detail 6, entities/operation 5, shared 3, widgets/operations 3. Один файл `shared/assets/svg.d.ts` волны A закрывает столбец.
- **`consistent-type-imports` (25, авто-`--fix`)** — размазан по 24 слайсам по 1–2.
- **Деньги-селекторы (16)** — денежные арифметика/toFixed: `widgets/leases/ui/LeaseEditForm.tsx:46,58`, `widgets/operations/ui/OperationEditForm.tsx:54,66`, `widgets/operations/ui/RecurringOperationEditPage.tsx:122,134`, `widgets/leases/ui/LeaseCreateWizard.tsx:95,96`, `widgets/operations/model/types.ts:54`; процентные омонимы (кандидаты `percent()`-хелпера): `entities/lease/ui/LeaseInfo.tsx:93,107`, `widgets/operations/ui/ProfitReport.tsx:208`; не-денежной `.toFixed`: `features/property-attributes/lib/format.ts:27` (площадь). Совпадает с инвентарём #328 §4.1.
- **tsc `noUncheckedIndexedAccess` (35)** — widgets/property-detail 10, widgets/operations 5, shared 9 (Icon 4, ToastProvider 2, PullToRefresh 2, navigation 1), widgets/calendar 2, widgets/properties 2, playwright.config 2, остальные по 1. vms 2 (`shared/providers/query-provider.tsx`), nio 1 (`shared/api/errors.ts`), nir 2 (`PullToRefresh.tsx`, `ServiceWorkerRegister.tsx`), nul 0.
- **a11y (9)** — `click-events-have-key-events` 4 (собственный `shared/ui/select/Select.tsx:223`; `widgets/operations/ui/CategorySelect.tsx:169,181`; `widgets/properties/ui/PropertyAddressStep.tsx:141`), `anchor-is-valid` 2 (`features/auth/ui/phone-step/PhoneStep.tsx:65,69`), `no-autofocus` 2 (`widgets/profile/ui/PhoneChangeForm.tsx:129,162`, OTP-фокус — кандидат на осознанную конфиг-опцию), `no-static-element-interactions` 1 (`CategorySelect.tsx:181`).
- **Прочее**: `no-deprecated` 13 (widgets/profile 3, widgets/operations 2, features/auth 2, features/push-notifications 2, widgets/tenants 1, widgets/properties 1, widgets/leases 1, widgets/property-detail 1); `no-unnecessary-type-assertion` 11 (entities/billing 5, widgets/properties 3, entities/calendar 2, тесты 1); `no-non-null-assertion` 6 (4 — тест-файл `features/property-attributes/lib/validate.test.ts`, 2 — формы); `no-base-to-string` 4 (все — `shared/api/client.ts`); `require-await` 2 (next.config.ts, shared); `switch-exhaustiveness-check` 1 (`shared/ui/toast/ToastBody.tsx:32`); `no-confusing-void-expression` 1 (`widgets/profile/ui/TariffChangeForm.tsx:155`).

## Админка: сетка «ресурс × семейство»

| Ресурс/файл | Всего | B-core | react-hooks | C (деньги) | tsc |
|---|---|---|---|---|---|
| src/dataProvider.ts | 22 | 21 (unsafe 8, nullish 5, require-await 3, unused 3, assertion 1, base-to-string 1) | 0 | 0 | 1 (vms) |
| src/authProvider.ts | 12 | 11 (nullish 6, unsafe-assignment 4, require-await 1) | 0 | 0 | 1 (vms) |
| src/userSubscription.tsx | 11 | 11 (unsafe-call 4, misused 4, assertion 2, base 1) | 0 | 0 | 0 |
| src/Dashboard.tsx | 8 | 7 (unsafe-call 4, unsafe-member 3) | 1 (set-state-in-effect) | 0 | 0 |
| src/tariffs.tsx | 6 | 6 (misused 4, unsafe 2) | 0 | 0 | 0 |
| src/subscriptionPayments.tsx | 4 | 4 (unsafe-call 2, misused 2) | 0 | 0 | 0 |
| src/LoginPage.tsx | 2 | 2 (misused 2) | 0 | 0 | 0 |
| src/fields.tsx | 2 | 2 (assertion 1, base-to-string 1) | 0 | 0 | 0 |
| src/lib/propertyAttributes.test.ts | 6 | 1 (assertion 1) | 0 | 0 | 5 (nuia) |
| src/lib/generated/format.ts | 2 | 0 | 0 | 1 (toFixed) | 1 (nuia) |
| src/lib/report-error.ts | 2 | 2 (unsafe 1, nullish 1) | 0 | 0 | 0 |
| src/lib/tariffs.ts | 1 | 1 (restrict-template 1) | 0 | 0 | 0 |
| **Итого** | **78** | **68** | **1** | **1** | **8** |

Классы фиксов: `no-misused-promises` 12 — async `onSubmit`/`onClick` в формах (LoginPage, tariffs, userSubscription, subscriptionPayments) — catch-обёртки; `no-unsafe-*` 28 — нетипизированный JSON в `dataProvider.ts`/`authProvider.ts` — типизация HTTP-границ; `[object Object]`-стиринг 3 (`no-base-to-string`) — реальный bug-класс.

## Zero-tolerance: свежий срез (ручной код)

- `eslint-disable`: фронт **15** (11 `react-hooks/set-state-in-effect` — draft-хуки форм; 2 `exhaustive-deps`; 2 `@next/next/no-img-element` в `PhotoGrid.tsx`), админка **0** — базлайн #330 подтверждён без изменений.
- explicit `any`: фронт **0**, админка **0** в ручном коде (единственный grep-хит — слово «any» в комментарии `features/push-notifications/lib/subscription-sync.ts:22`).
- `@ts-ignore`/`@ts-expect-error`: **0** в ручном коде обоих приложений.

**Новый факт для счётчик-скрипта волны A**: артефакты сборки `.next/types/**` и `.next/dev/types/**` содержат 20 `any` и 102 `@ts-ignore` (их генерирует Next.js), `shared/api/generated.ts` — 1 `any`. Tools-скрипт подавлений обязан скоупиться на ручной код (исключить `.next/`, `node_modules/`, `shared/api/generated*`, `src/lib/generated/`), иначе счётчик всегда ненулевой; это же исключение — files-scoped блок конфига из решения #330.

## Новое сверх #328

1. **Админка react-hooks не 0**: `react-hooks/set-state-in-effect` — 1 находка, `src/Dashboard.tsx:206` (research §6 шаг 3 предполагал 0). Реальный фикс кодом, входит в админ-волну ремедиации.
2. **Деньги-гейт админки задевает generated**: единственный хит селекторов — `.toFixed` в `src/lib/generated/format.ts:27`. Деньги-селекторы админки должны исключать `src/lib/generated/**` (аналог allowlist `fields.tsx`): генерированный код не чинится руками; правка шаблона генератора — вопрос #333.
3. **Локации tsc-находок фронта** (в #328 не атрибутированы): vms — `shared/providers/query-provider.tsx`; nio — `shared/api/errors.ts`; nir — `PullToRefresh.tsx`, `ServiceWorkerRegister.tsx`; полный список nuia — в горячих точках выше.

## Наблюдения для плана ремедиации #333

- Ни один слайс не имеет профиля, требующего переписывания: максимум 43 находки (widgets/property-detail), разложенные на независимые механические классы (10 nuia + 10 floating/misused + 8 condition). Кандидатов «переписывать целиком» по данным сетки не видно — но решение за #333.
- Самые дешёвые массовые классы — `consistent-type-imports` (25) и `no-unnecessary-type-assertion` (11), оба авто-`--fix`; размазаны, отдельных тикетов на слайс не требуют.
- `exhaustive-deps: error` подтверждён бесплатным: новых нарушений 0 (существующие 2 — под `eslint-disable`, чинятся в zero-tolerance-проходе).
- Волна A (svg.d.ts + type-imports `--fix` + tsconfig-набор) снимает ~99 находок сетки без правок логики: 34 unsafe + 25 type-imports + 40 tsc.
- Горячая точка админки — `dataProvider.ts` (22 из 78): типизация JSON-границ разом закрывает большинство unsafe/base-to-string.

## Источники

- Резолюция #330 — принятый бар (волны A/B/C, опции правил, zero-tolerance, приёмка).
- Research #328 — каталог строгости, глобальные счётчики, обоснования опций.
- Карта #326 — Destination и Notes (приоритет качества, политика «чинить по умолчанию»).
