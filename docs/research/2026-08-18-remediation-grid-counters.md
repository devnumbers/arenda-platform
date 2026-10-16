# Ремедиационная сетка бекенда: advisory-счётчики «волна × контекст»

Материал решения тикета [#325](https://github.com/devnumbers/arenda-platform/issues/325) (карта №319).
Бар качества ратифицирован [#323](https://github.com/devnumbers/arenda-platform/issues/323), каталог линтеров — research [#321](https://github.com/devnumbers/arenda-platform/issues/321) (`docs/research/2026-08-18-golangci-lint-catalog.md`), контур БД — [#324](https://github.com/devnumbers/arenda-platform/issues/324).

## Метод

Advisory-прогон golangci-lint v2.12.2 (пин Makefile) поверх действующего `.golangci.yml` с включёнными настройками восьми волн этапа 2: errcheck (check-blank + check-type-assertions + disable-default-exclusions, exclude-functions `strings.Builder.Write*`), goconst (дефолты), godot (scope: all, capital: true), staticcheck `["all", "-ST1016", "-ST1020", "-ST1021", "-ST1022"]` (сняли только -ST1000), cyclop@15, gocognit@25, lll@140, paralleltest (дефолты). Прогон с `--build-tags=integration`, как в `make backend-lint`. Одноразовый конфиг — `.tmp/waves-advisory.golangci.yml` (не коммитится); воспроизвести — копия текущего конфига + правки из этого списка.

Расхождение с research #321: paralleltest 994 против 888 — #321 гонял без integration-тега; здесь счётчики соответствуют реальному гейту, сетка использует их. Остальные волны сходятся (±2): errcheck 248 = 248, goconst 314 = 314, godot 393 ≈ 395, ST1000 200 ≈ 199, cyclop 57 = 57, gocognit 64 = 64, lll 605 ≈ 623.

## Матрица «волна × контекст»

Счётчик = количество находок на момент замера (2026-08-18, dev 3966f56).

| Волна | Всего | access | billing | identity | leases | notifications | platform | properties | прочие |
|---|---|---|---|---|---|---|---|---|---|
| errcheck | 248 | 30 | 68 | 20 | 48 | 9 | 54 | 19 | — |
| goconst | 314 | 27 | 139 | 17 | 55 | 10 | 22 | 36 | cmd 6, admin 2 |
| godot strict | 393 | 120 | 73 | 42 | 45 | 57 | 25 | 17 | admin 5, audit 4, cmd 4, shared 1 |
| ST1000 | 200 | — | 40 | 34 | 29 | 25 | 41 | 14 | admin 5, popups 6, shared 6 |
| cyclop@15 | 57 | 5 | 33 | 2 | 7 | 2 | 5 | 3 | — |
| gocognit@25 | 64 | 2 | 21 | 7 | 19 | 2 | 4 | 8 | cmd 1 |
| lll@140 | 605 | 40 | 177 | 31 | 171 | 73 | 45 | 39 | cmd 11, admin 18 |
| paralleltest | 994 | 61 | 443 | 144 | 121 | 79 | 33 | 108 | shared 5 |

Все 200 staticcheck-находок — только ST1000; ST1016/ST1020–1022 подтвержждены чистыми (возврат в `checks: ["all"]` бесплатный).

## Состав структурного свипа (cyclop + gocognit)

Счётчик двухсоставный:

- **Прод-функции** (рефакторинг декомпозиции): `UserFacingDetail` 39, `UnarchiveProperty` 23, `tick` 20, `stringifyValue` 20, `sanitizeSQLOperationName` 19, `handlePropertyError` 19, `activateInvitation` 19, `subscriptionInSelection` 18, `dispatchPushReminder` 18, `renderTenantsSheet` 16, `Update` 16.
- **Тестовые функции billing** — бо́льшая часть счётчика (33 из 57 cyclop): `TestWorkers_*`, `TestSubscriptionService_*`, `TestWebhook_*`, `TestSubscriptionLifecycle_*`, `TestTariffRepository_*`. Декомпозиция таблицами/хелперами, отдельная работа от прод-рефакторинга.

`activateInvitation` уезжает в UoW-миграцию access (переписывается application-слой); прод-список свипа уточняется после закрытия UoW-тикетов.

## Метод тикетов волн

Общие правила для всех тикетов волн сетки #325:

1. **Актуальный счётчик** в начале работы — живой прогон, не замер из этого файла (код двигается): временный конфиг по разделу «Метод» + `cd apps/backend && go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run --config <conf> --build-tags=integration ./internal/<ctx>/...` (для cmd — `./cmd/...`).
2. **Флип-конвенция**: линтер/настройка волны входит в действующий `.golangci.yml`, когда её **глобальный** счётчик по модулю 0. Флип делает тикет, закрывший последнюю ненулевую ячейку волны, — переносом настройки из REMEDIATION-черновика research #321 (для dupl — опустить threshold 150 после закрытия dupl-кластеров).
3. **Классы фиксов** — по каталогу research #321: errcheck — обработка или осознанный must*-хелпер (политика #323 «шум чинится кодом»); goconst — вынос констант; godot — точки и заглавные в комментариях; lll — перенос строк ≤140; paralleltest — `t.Parallel()`, старт с юнит-пакетов, общие фикстуры/testcontainers — по месту.
4. Волна×контекст тикеты четырёх немигрированных контекстов (access, properties, notifications, leases) открываются после их UoW-миграции — не чистить файлы, которые переписываются.

## Источники

- Резолюция #323 — бар: двухэтапный конфиг, политика nolint, архитектурная дельта.
- Research #321 — каталог линтеров, классификация 54 nolint, черновик целевого конфига.
- Резолюция #324 — контур БД (CHECK/FK-ремедиация, конвенции миграций).
- Карта №289 — гильотека инструментов, `docs/agents/tooling.md`.
