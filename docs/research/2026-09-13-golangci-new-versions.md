# golangci-lint после пина v2.12.2: v2.13.0–v2.13.2 — что изменилось и как апгрейдиться

Research-тикет [#637](https://github.com/devnumbers/arenda-platform/issues/637), карта №636 «Усиление линтеров».
Дата: 2026-09-13. Пин на момент research: v2.12.2 (корневой `Makefile`, `GOLANGCI_LINT_VERSION`; заморожен 2026-08-18).
База сравнения: `docs/research/2026-08-18-golangci-lint-catalog.md` (research #321) и действующий `apps/backend/.golangci.yml`.

## Метод

- Первоисточники: релизы `github.com/golangci/golangci-lint` (GitHub API, теги v2.13.0/v2.13.1/v2.13.2), официальный changelog https://golangci-lint.run/docs/product/changelog/ , diff канонического `.golangci.reference.yml` между v2.12.2 и v2.13.2 (GitHub compare), релизы апстрим-библиотек линтеров, исходники golangci-lint v2.13.2 (`pkg/golinters/perfsprint`, `pkg/goformatters/gofumpt`) и gofumpt v0.11.0.
- Смоук-валидация (2026-09-13): `cd apps/backend && GOLANGCI_LINT_CACHE=<per-checkout> go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2 run --build-tags=integration ./internal/identity/...` на действующем конфиге без правок. Полный модуль не прогонялся (вводная для апгрейд-тикета).
- `package`/`go.mod` не менялись, ничего не коммитилось.

---

## Блок 1. Версии и совместимость

| Версия | Дата релиза | Суть |
|---|---|---|
| v2.12.2 | 2026-05-06 | Текущий пин (заморожен 2026-08-18 — v2.13.0 вышел на день позже). |
| v2.13.0 | 2026-08-19 | Основной релиз: поддержка go1.27; **exhaustruct деприкнут, заменён `exhaustruct_v5`**; новые опции у 9 линтеров; в `modernize` 7 новых анализаторов, `fmtappendf` удалён, `waitgroup` переименован в `waitgroupgo`; всплеск апстрим-бамперов (errcheck 1.10→1.20, gosec 2.26.1→2.28.0, gofumpt 0.9.2→0.11.0, staticcheck →0.8.0). |
| v2.13.1 | 2026-08-20 | Только бамперы багфиксов: staticcheck 0.8.0-rc→0.8.0, ginkgolinter 0.24.0, wsl 5.9.0, gofmt. |
| v2.13.2 | 2026-08-27 | Багфиксы: iface 1.5.1, staticcheck 0.8.1 (фиксы FP SA4023), unparam (фикс паники на type params), canonicalheader переехал на форк, `fix: decrease cache entropy` (дешевле кэш). |

Источники: [releases](https://github.com/golangci/golangci-lint/releases), [changelog](https://golangci-lint.run/docs/product/changelog/) (разделы 2.13.0/2.13.1/2.13.2).

**Актуальная стабильная на 13.09.2026: v2.13.2** (пререлизов нет; проверено списком релизов GitHub API).

### Breaking changes формата v2

**Нет.** Официальный changelog не помечает ни одного breaking change в v2.13.0–v2.13.2; все поля `linters.settings` / `exclusions` / `formatters` из нашего конфига принимаются v2.13.2 без ошибок схемы (подтверждено смоук-прогоном: конфиг валиден, только warning'и, см. Блок 4). Формальные изменения, задевающие конфиг-словарь:

| Изменение | Касается нас? |
|---|---|
| `exhaustruct` деприкнут → используйте `exhaustruct_v5` (новый линтер, новая секция настроек) | Нет (мы его не включали — но именно поэтому см. Блок 3). |
| `modernize`: анализатор `fmtappendf` удалён, `waitgroup` → `waitgroupgo`, +7 новых | Частично: наш `disable: [forvar, omitzero]` остаётся валидным (оба имени живы в v2.13.2, проверено по `.golangci.reference.yml@v2.13.2`). |
| `formatters.settings.gofumpt.extra-rules` деприкнут (работает, но warning) — замена `extra: {group-params, clothe-returns, balance-calls}` | **Да, обязательная правка при апгрейде** — см. Блок 5. |

### Требования к Go

- golangci-lint v2.13.x: `go 1.26.0` в собственном `go.mod` («minimum Go version must always be latest-1») — совместимо с нашим стеком (`apps/backend/go.mod` = go 1.26.5, локальный тулчейн 1.26.6). Источник: `go.mod` репозитория golangci-lint на теге v2.13.2.
- Поддержка go1.27 добавлена в v2.13.0 (changelog, «go1.27 support») — нам пока не требуется.

---

## Блок 2. Новое у уже включённых линтеров (включённых у нас ~67)

### 2.1. Новые проверки, которые заработают сами (без правок конфига)

| Линтер | Что нового | Оценка для стека (Go 1.26, chi, sqlc/pgx, slog, OTel) |
|---|---|---|
| staticcheck | Новый чек **SA9010** «Returned function should be called in defer» (`defer foo()` при `foo() func()`) — staticcheck 2026.2 (в golangci как v0.8.0). У нас `checks: ["all"]` — активен сразу. | Баг-класс: неотложенный `defer Unlock()/Close()`-в-стиле. Наш baseline чист — реальных случаев, вероятно, нет; прогон на identity их не нашёл. Источник: [staticcheck 2026.2](https://github.com/dominikh/go-tools/releases/tag/2026.2). |
| modernize | 7 новых анализаторов: `atomictypes`, `embedlit`, `errorsastype` (errors.As → errors.AsType[T]), `importcomment`, `reflecttypeassert`, `slicesbackward`, `slicesclip` — все активны по умолчанию. `errorsastype` опирается на Go 1.26 API `errors.AsType[T]` — на нашем go 1.26.5 это живая рекомендация. | Гигиена с autofix. Возможны единичные новые находки по модулю (на identity — 0). Источники: diff `.golangci.reference.yml`, [changelog 2.13.0](https://golangci-lint.run/docs/product/changelog/). |
| unparam | Пин на HEAD вырос дважды; новое поведение: «see the bodies of methods on unexported types», «see through defer stores and loads when detecting forwarded returns», фикс паники на type params. | Расширение анализа → возможны новые находки «результат всегда X» в неэкспортированных типах (адаптеры). Источники: [commits](https://github.com/mvdan/unparam/commits) (2fa3d84, 430936f, 3570034). |
| recvcheck | v0.3.0: дефолтное исключение сменено — теперь по умолчанию исключаются методы `Unmarshal`, а не `Marshal`. | Смещение счётчика возможно в обе стороны; baseline у нас 0. Источник: [recvcheck v0.3.0](https://github.com/raeperd/recvcheck/releases/tag/v0.3.0). |
| gosec | 2.27.x/2.28.0: **фикс FP G115** — guard-паттерны `min(v, max)`/`max(v, min)` теперь распознаются; G118 FP fix (cancel в slice/map); G404 расширен (больше weak-random функций); G101 ловит AWS temporary keys. | Прямо в наш G115-класс (бывшие ~40 nolint → хелпер `ToInt32Clamped`): guard-признание стало строже — наши if-гварды остаются валидными, min/max-варианты больше не требуют nolint. G404-расширение может зацепить retry в tkassa (там nolint G404 по каталогу #321). Источники: [gosec v2.28.0](https://github.com/securego/gosec/releases/tag/v2.28.0), [v2.27.0](https://github.com/securego/gosec/releases/tag/v2.27.0). |
| errcheck | 1.10.0 → **1.20.0**: исключения для generic-функций (type parameters); дефолтные исключения расширены (crypto/sha3.Read/Write). | Возможность точнее вести `exclude-functions` (наши три Builder-метода не generic). Дефолтные исключения нам не релевантны — `disable-default-exclusions: true`. Источник: [errcheck v1.20.0](https://github.com/kisielk/errcheck/releases/tag/v1.20.0). |
| gofumpt (formatter) | 0.9.2 → **0.11.0**: база — gofmt go1.26. Новые дефолтные правила: снятие лишних скобок (`f((3))`), пустая строка между top-level декларациями; правило балансировки скобок многострочных вызовов переехало в extra `balance_calls` и по умолчанию выключено; фикс «второго прогона». | **Реальный diff**: смоук нашёл 1 файл (`internal/identity/adapters/postgres/phone_crypto_test.go` — пустые строки между однострочными методами). Чинится `golangci-lint fmt`. Источники: [gofumpt v0.10.0](https://github.com/mvdan/gofumpt/releases/tag/v0.10.0), [v0.11.0](https://github.com/mvdan/gofumpt/releases/tag/v0.11.0). |

### 2.2. Новые опции (включаются сознательно, сейчас не нужны)

| Линтер | Новая опция | Комментарий |
|---|---|---|
| goconst (включён) | `exclude-types` (Assignment/Binary/Case/Return/Call/CompositeLit), `ignore-map-keys` | Тюнинг шума, если пойдёт волна goconst-находок. Источник: diff reference-конфига, [changelog](https://golangci-lint.run/docs/product/changelog/). |
| iface (включён) | Новый анализатор `unusedmethod` (методы интерфейса, не используемые в пакете определения) | **Осторожно: FP-риск на DDD-портах** — методы порта дёргаются адаптерами из других пакетов. Не включать без trial-прогона. |
| dupword (включён) | `skip-raw-strings` | Полезно, если появятся FP на сырых строках (SQL-тексты). |
| fatcontext (включён) | `check-loops`, `check-function-literals` | Возможность отключения; нам не нужна — оба детекта нужны и baseline 0. |
| gomoddirectives (включён) | `replace-allow-all`, поле `IgnoreForbidden` | Расширение; дефолты нас устраивают. |
| loggercheck и остальные | — | Изменений не заявлено. |

### 2.3. Новый линтер каталога: `exhaustruct_v5`

Единственное пополнение каталога в v2.13.x — **замена** exhaustruct на переработанный `exhaustruct_v5` (апстрим `dev.gaijin.team/go/exhaustruct/v5`, репозиторий [GaijinEntertainment/go-exhaustruct](https://github.com/GaijinEntertainment/go-exhaustruct)). Подробно — в Блоке 3.

---

## Блок 3. «Осознанно НЕ включаем»: изменился ли статус excluded

Правило вердикта прежнее: отклонение снимается, если ушла **причина** (линтер переписан, появились опции, позволяющие выразить нашу политику, FP исправлены). Вводная для research-тикета «Пересмотр excluded-списка»; финальное решение — за владельцем.

| Линтер (счётчик v2.12.2) | Обновления в v2.13.x | Статус причины отклонения |
|---|---|---|
| **exhaustruct (1705)** | **Полная переработка v5** ([v5.0.0](https://github.com/GaijinEntertainment/go-exhaustruct/releases/tag/v5.0.0), v5.1.0, v5.2.0): режим **explicit-mode** (проверять только типы, помеченные `//exhaustruct:enforce` или `enforce-patterns`), `ignore-patterns`/`optional-patterns` **с точностью до поля** (`Type#Field`), директивы комментариев вместо struct-тегов, пачка фиксов на embedded-полях и кэше, `allow-empty-*`-семейство. В golangci доступен как отдельный линтер `exhaustruct_v5` со своей секцией настроек; старый деприкнут. | **Причина существенно ослаблена — кандидат №1 на пересмотр.** Прежнее «1705 шума: частичная инициализация sqlc-параметров и доменов — идиома» била по blanket-политике; explicit-mode даёт opt-in (точечно включать проверку на избранных типах, например конфиги/деньги-структуры), optional/ignore-patterns гасят sqlc-шум. Замер шума при explicit-mode — предмет trial-прогона. |
| nonamedreturns (11) | v1.0.7/v1.0.8 + опция `allow-unused-named-returns` (именованный результат разрешён, если на него не ссылаются в теле). Наши 11 — defer-идиома метрик tkassa: **ссылаются** в defer. | Причина сохраняется (опция наш класс не спасает). |
| wsl_v5 (5448) | 5.9.0: один FP-фикс (labelled statements в cuddle-checks). | Причина сохраняется (стиль пустых строк, баг-класса нет). |
| noinlineerr (1426) | v1.0.6: поддержка `if err = doSomething(); err != nil` (reassign-ветка) — покрытие расширилось, философия та же. | Причина сохраняется (запрет inline-err противоречит идиоме проекта). |
| funcorder (85) | Добавлено поле конфигурации `Function` (#6627) — механика, политика не менялась. | Причина сохраняется (стиль без баг-класса). |
| nlreturn (2486), varnamelen (1044), mnd (131), embeddedstructfieldcheck (49), err113 (282), testpackage (143), gochecknoglobals (67), ireturn (142), interfacebloat (9), tagliatelle (50), funlen (37), nestif (19) | Обновлений нет. | Причины сохраняются. |
| govet shadow (341), fieldalignment (269) | Обновлений анализаторов, снимающих причину, нет. | Причины сохраняются. |
| gocritic performance-тег (hugeParam 381 + rangeValCopy 221) | go-critic 0.14.4: только «update goStdlib to go1.27», новых проверок/изменений тегов нет (v0.15.0 существует вне golangci — придёт позже). | Причина сохраняется. |
| Стек-нерелевантные (arangolint, clickhouselint, ginkgolinter, promlinter, protogetter, zerologlint, unqueryvet) | Точечные бамперы; наш стек не появился. | Причины сохраняются (x/exp в go.mod по-прежнему нет — `exptostd` ждёт). |
| gomodguard_v2 (0) | Обновлений нет. | Причина сохраняется (дубль depguard no-orm). |
| — (сноска perfsprint) | Проверено в исходнике v2.13.2: `pkg/golinters/perfsprint/perfsprint.go` по-прежнему хардкодит `"fiximports": false`. | Сноска в хвосте конфига остаётся актуальной. |

---

## Блок 4. Смоук-прогон v2.13.2 (конфиг без правок)

Команда: `cd apps/backend && GOLANGCI_LINT_CACHE=$PWD/../../.golangci-lint-cache go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2 run --build-tags=integration ./internal/identity/...` (кэш обязательный, пер-чекаут).

Результат:

- Схема конфига принята без ошибок — миграция конфига **не блокирует** апгрейд.
- 2 предупреждения: `gofumpt: 'extra-rules' is deprecated, please use 'extra.group-params' instead`.
- 1 новая находка (formatter): `internal/identity/adapters/postgres/phone_crypto_test.go:135` — gofumpt 0.11 требует пустую строку между однострочными методами (`fmt -d` подтверждает: только пустые строки, правка механическая).
- `nolintlint`/`warn-unused` и forbidigo-исключения отработали штатно (Skipped-предупреждения по правилам с 0 срабатываний — нормальное поведение warn-unused).
- Полный модуль не прогонялся: перед флипом пина нужен полный `run` + `fmt` (см. Блок 5).

---

## Блок 5. Рекомендация: целевой пин v2.13.2 и чек-лист апгрейда

**Пин: `GOLANGCI_LINT_VERSION := v2.13.2`** — последний стабильный; несёт свежие багфиксы линтеров (SA4023 FP, паника unparam, iface) и фикс «cache entropy». Промежуточные v2.13.0/v2.13.1 брать нет смысла.

Порядок (один тикет, без смешивания с включением новых линтеров):

1. **Makefile**: `GOLANGCI_LINT_VERSION := v2.13.2` → `make versions-sync` → закоммитить штампованные файлы (конвенция корневого `AGENTS.md`: версии пинятся только в Makefile).
2. **gofumpt-миграция в `apps/backend/.golangci.yml`** (до первого прогона):
   ```yaml
   formatters:
     settings:
       gofumpt:
         extra:
           group-params: true
           clothe-returns: true
   ```
   Почему обязательно, а не косметика: в gofumpt v0.11.0 `ExtraRules` реализован как `opts.Extra.Set("true")` — то есть включает **все три** extra-правила, включая новый `balance_calls` (спорное правило, которое gofumpt v0.11.0 сознательно сделал opt-in). Старый `extra-rules: true` в v0.9.2 означал только group-params + clothe-returns; без миграции апгрейд молча меняет политику форматирования. Источники: `format/format.go@v0.11.0` (строки 163, `Extra.Set`), warning адаптера `pkg/goformatters/gofumpt/gofumpt.go@v2.13.2`.
3. `golangci-lint fmt` по модулю (gofumpt 0.11: пустые строки между top-level декларациями, снятие лишних скобок) — минимум 1 известный файл, полный объём узнаем прогоном.
4. Полный `make backend-lint`-эквивалент (`run --build-tags=integration ./...`) на v2.13.2: собрать счётчики новых находок от SA9010, новых modernize-анализаторов, unparam HEAD, recvcheck-смены дефолта, G404-расширения (tkassa retry). По опыту identity-прогона ожидание — единицы, не волна.
5. Новые линтеры/опции (exhaustruct_v5 trial, iface unusedmethod) в этот тикет **не тащить** — отдельные решения: exhaustruct_v5 — в пересмотр excluded-списка (Блок 3), unusedmethod — trial с прицелом на FP DDD-портов.

---

## Источники

- Релизы: https://github.com/golangci/golangci-lint/releases (v2.13.0 — 2026-08-19, v2.13.1 — 2026-08-20, v2.13.2 — 2026-08-27)
- Официальный changelog: https://golangci-lint.run/docs/product/changelog/ (разделы 2.13.0/2.13.1/2.13.2, получено 2026-09-13)
- Diff `.golangci.reference.yml` v2.12.2…v2.13.2: GitHub compare API `repos/golangci/golangci-lint/compare/v2.12.2...v2.13.2`
- `go.mod` golangci-lint v2.13.2 (минимум Go 1.26.0): GitHub contents API, ref v2.13.2
- gofumpt: [v0.10.0](https://github.com/mvdan/gofumpt/releases/tag/v0.10.0), [v0.11.0](https://github.com/mvdan/gofumpt/releases/tag/v0.11.0), `format/format.go@v0.11.0`
- staticcheck: [2026.2](https://github.com/dominikh/go-tools/releases/tag/2026.2) (= v0.8.0, SA9010, go1.27), [2026.2.1](https://github.com/dominikh/go-tools/releases/tag/2026.2.1) (= v0.8.1, фиксы FP SA4023)
- gosec: [v2.27.0](https://github.com/securego/gosec/releases/tag/v2.27.0) (G118 FP), [v2.28.0](https://github.com/securego/gosec/releases/tag/v2.28.0) (G115 min/max FP, G404, G101)
- errcheck: [v1.20.0](https://github.com/kisielk/errcheck/releases/tag/v1.20.0)
- exhaustruct v5: [GaijinEntertainment/go-exhaustruct v5.0.0](https://github.com/GaijinEntertainment/go-exhaustruct/releases/tag/v5.0.0), [v5.1.0](https://github.com/GaijinEntertainment/go-exhaustruct/releases/tag/v5.1.0), [v5.2.0](https://github.com/GaijinEntertainment/go-exhaustruct/releases/tag/v5.2.0)
- wsl: [v5.9.0](https://github.com/bombsimon/wsl/releases/tag/v5.9.0); noinlineerr: [v1.0.6](https://github.com/AlwxSin/noinlineerr/releases/tag/v1.0.6); recvcheck: [v0.3.0](https://github.com/raeperd/recvcheck/releases/tag/v0.3.0); unparam: https://github.com/mvdan/unparam/commits (2fa3d84)
- Исходники golangci-lint v2.13.2: `pkg/golinters/perfsprint/perfsprint.go` (`fiximports: false`), `pkg/goformatters/gofumpt/gofumpt.go` (warning деприкации)
- Смоук-прогон: 2026-09-13, golangci-lint v2.13.2 на `./internal/identity/...` (детали в Блоке 4)

## Открытые вопросы

1. Смоук показал 1 gofumpt-находку только на identity — фактический объём `fmt`-диффа по модулю узнается на шаге 3 чек-листа (в этот research не входил).
2. exhaustruct_v5 explicit-mode: замер шума на нашем модуле — вводная для тикета «Пересмотр excluded-списка»; замер не делался.
3. iface `unusedmethod`: гипотеза о FP на DDD-портах требует trial-прогона, прежде чем что-то решать.
