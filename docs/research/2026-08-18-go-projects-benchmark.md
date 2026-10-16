# Внешний бенчмарк: правила и структура эталонных Go-проектов против наших

- Тикет: [#320](https://github.com/devnumbers/arenda-platform/issues/320), карта: [#319](https://github.com/devnumbers/arenda-platform/issues/319)
- Дата: 2026-08-18. Все факты — из первоисточников (официальные доки и репозитории), ссылки у каждого утверждения.
- Наш свод для сравнения: `apps/backend/AGENTS.md`, `apps/backend/CODING_STANDARDS.md`, `.golangci.yml` (корень), фактическая структура `apps/backend/internal/*`.

## 1. Наш свод в двух словах (база сравнения)

- DDD modular monolith: `internal/<context>/{domain,application,adapters}` (контексты: access, admin, audit, billing, identity, leases, notifications, popups, properties + `platform/*` инфраструктура, `shared/*` shared kernel, `transaction` UoW). Направление зависимостей adapters → application → domain, залинчено depguard-правилами `domain-clean` / `application-clean` / `platform-clean` (`.golangci.yml`).
- Контракт-first OpenAPI + sqlc; freshness-гейты генерации (`backend-openapi-check`, `backend-sqlc-check`, `attributes-check`); миграции — golang-migrate + squawk + кастомный `tools/migration-lint` (деньги BIGINT-копейки, запрет float, UUIDv7 без DB DEFAULT).
- golangci-lint v2: `default: standard` + ~45 включённых линтеров (gosec, errorlint, exhaustive, spancheck, sloglint, forbidigo UUIDv7/Begin/slog, nolintlint strict, depguard и т. д.), формatters gofumpt + gci. Политика исключений — точечные rules с ADR-обоснованием, `warn-unused`.
- Прозрачно зафиксированные конвенции без линтера: порты объявляет потребитель; время через `clock.Clock`; транзакции через UoW (ADR 0033); sentinel-ошибки, `errors.Is` только на transport edge; каждый горутин имеет владельца; fakes over mocks; table-driven + `t.Parallel` + testify.

## 2. Таблицы «у них vs у нас»

### 2.1. Google Go Style Guide + Go Code Review Comments

Источники: [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments), [Style Decisions](https://google.github.io/styleguide/go/decisions), [Best Practices](https://google.github.io/styleguide/go/best-practices).

| Область | У них | У нас | Дельта |
|---|---|---|---|
| Формат | gofmt/goimports обязательны ([CRC](https://go.dev/wiki/CodeReviewComments)) | gofumpt (строже gofmt) + gci | мы сильнее |
| Пакеты | нижний регистр, без `util/common/misc`; пакет не «мусорка» ([decisions](https://google.github.io/styleguide/go/decisions)) | то же + depguard-границы слоёв | parity+, у нас гейт |
| Ошибки | `error` последним; строки lowercase без точки; in-band errors запрещены; не сравнивать ошибки по строке в тестах (`errors.Is`/`cmpopts.EquateErrors`); wrap `%w` если caller смотрит цепочку, `%v` для аннотации на границах; «just return err» против обёрток-пустышек ([decisions](https://google.github.io/styleguide/go/decisions), [best practices](https://google.github.io/styleguide/go/best-practices)) | sentinel per context, `errors.Is` только на edge, ST1005 + контент-политика в CODING_STANDARDS | parity; у нас строже про место матчинга |
| Интерфейсы | создавать только при реальной нужде, объявляет потребитель, «accept interfaces, return concrete» ([CRC](https://go.dev/wiki/CodeReviewComments)) | то же правилом в CODING_STANDARDS (ports by consumer) | parity (проза, не линтер) |
| Context | первый параметр, не в структурe, без данных в ctx ([CRC](https://go.dev/wiki/CodeReviewComments)) | то же + `containedctx`, `contextcheck`, `fatcontext`, `noctx` | мы сильнее (линтер) |
| Горутины | «make it clear when or whether they exit», синхронные функции предпочтительнее ([CRC](https://go.dev/wiki/CodeReviewComments)) | правило владельца горутины в CODING_STANDARDS + contextcheck + `-race` | parity+; goleak нет (см. кандидаты) |
| Паники | не для обычных ошибок ([decisions](https://google.github.io/styleguide/go/decisions)) | идиома; `uuid.Must(...)` точечно по ADR 0019 | parity |
| Тесты | assertion-библиотеки «not idiomatic», `cmp`/`cmp.Diff`, `t.Error` > `t.Fatal`, сравнение целых структур, сабтесты, helpers с `t.Helper` ([best practices](https://google.github.io/styleguide/go/best-practices)) | testify (require/assert) + testifylint, table-driven + `t.Parallel` | осознанное расхождение: Google против assert-библиотек, у нас testify легализован и залинчен |
| Именование | MixedCaps, initialisms (`appID`), без Get-префикса, Must*, receiver короткий ([decisions](https://google.github.io/styleguide/go/decisions)) | ST1003 initialisms включён; остальное — идиома без гейта | почти parity |
| Тенирование | предупреждение про `:=` в новой области (классический ctx-баг) ([best practices](https://google.github.io/styleguide/go/best-practices)) | govet shadow осознанно выключен (139 шумовых) | осознанное расхождение (зафиксировано в `.golangci.yml`) |
| Доки | doc-комментарории на все экспортируемые; сигнальные комментарии для неочевидного ([decisions](https://google.github.io/styleguide/go/decisions)) | ST1000/ST1020-22 выключены (шум), но `godoclint` + `godot` включены | частично; их норма жёстче ours по объёму доков, но у них гигантский легаси-объём |

### 2.2. Uber Go Style Guide

Источник: [github.com/uber-go/guide/blob/master/style.md](https://github.com/uber-go/guide/blob/master/style.md) (+ [пример `.golangci.yml` репозитория](https://github.com/uber-go/guide/blob/master/.golangci.yml)).

| Область | У них | У нас | Дельта |
|---|---|---|---|
| Ошибки | таблица решений (static/matching → `var Err...`; dynamic/matching → тип с суффиксом `Error`); `%w` как дефолт; «handle once» — не логировать и не возвращать одновременно; без нагромождения «failed to» | sentinels в `<ctx>/application/errors.go`, wrap-политика в CODING_STANDARDS; про «handle once» не сказано | parity; «handle once» — кандидат |
| Интерфейсы | static-assert компиляции `var _ Iface = (*T)(nil)` | паттерн де-факто используется (напр. `identity/adapters/events/dispatcher.go`), но не записан в CODING_STANDARDS | кандидат (проза) |
| Горутины | нет fire-and-forget: предсказуемая остановка + способ ждать (WaitGroup/done); `go.uber.org/goleak` в тестах; Stop/Shutdown методы | правило владельца + rubric; goleak нет | goleak — кандидат |
| Каналы | «size of one or unbuffered», буфер требует обоснования | «Mystery buffer size» в рубрике CODING_STANDARDS | parity (правило есть, линтера нет ни у кого) |
| Глобальное состояние | избегать мутабельных глобалов; DI вместо глобалов (`time.Now` полем структуры!) | `clock.Clock` порт — та же идея, формальнее | мы не слабее |
| init/exit | избегать `init()`; `os.Exit`/`log.Fatal` только в `main` (паттерн `run() error`) | `gochecknoinits` есть; exit-политика не записана и не линтуется (фактически чисто: в `internal` нет `os.Exit`/`log.Fatal`) | кандидат |
| Время | всегда `time`-пакет; `AddDate` vs `Add(24h)`; RFC 3339 наружу; единицы в именах полей | `gosmopolitan` (time.Local-ловушки), UTC-конвенция, `timeutil`/`tzresolver` | мы сильнее (линтер) |
| Embedding | не встраивать типы в публичные структуры | нет правила | микрокандидат |
| Тесты | table-driven, `give`/`want` префиксы, без условной логики в таблицах; `tt := tt` для Parallel | table-driven + testify; конвенция имён полей таблиц не записана | parity; микрокандидат |
| Линтеры | минимум: errcheck, goimports, revive, govet, staticcheck; раннер golangci-lint | standard + 45 поверх | мы сильно шире |

### 2.3. go-kratos (фреймворк + kratos-layout)

Источники: [go-kratos/kratos-layout README](https://github.com/go-kratos/kratos-layout/blob/main/README.md), [`.golangci.yml` go-kratos/kratos](https://github.com/go-kratos/kratos/blob/main/.golangci.yml).

| Область | У них | У нас | Дельта |
|---|---|---|---|
| Слои | `internal/{server,service,biz,data}` + `api/` (proto-first), Wire DI; поток service → biz → data; **biz владеет интерфейсами репозиториев** (инверсия зависимостей по договору, не линтером) | `internal/<ctx>/{domain,application,adapters}`, интерфейсы объявляет application, инверсия залинчена depguard | parity по идее; у нас гейт |
| API | proto-first: `google.api.http`, AIP-пагинация (`page_size`/`page_token`), FieldMask для partial update, OpenAPI генерится из proto | OpenAPI-first (openapi.yaml → oapi-codegen), freshness-гейт | parity по принципу contract-first, другой носитель |
| Ошибки | определены в biz-слое; подробных конвенций в layout нет | sentinel + edge-маппинг в problem details | наш свод подробнее |
| Линтеры | bodyclose, dogsled, durationcheck, errcheck, goconst, **gocyclo (50)**, govet (+shadow), lll (160), misspell, **mnd**, prealloc, revive, staticcheck, unconvert, unused, wastedassign, whitespace; формatters gofmt+gofumpt+goimports; presets-исключения | шире по баг-классу; gocyclo/mnd/lll у нас осознанно отклонены (метрики/стиль), shadow выключен | сравнимо; их gocyclo=50 — очень мягкий |
| CI | make api/config/all (генерация), `go test ./...` | то же + freshness-гейты + testcontainers-integration | мы сильнее |

### 2.4. go-zero

Источники: [Project Structure](https://go-zero.dev/concepts/project-structure/), [Design Principles](https://go-zero.dev/concepts/design-principles/), [Error Handling](https://go-zero.dev/guides/http/server/error/), [Code Style](https://go-zero.dev/community/code-style/), CI: [`reviewdog.yml`](https://github.com/zeromicro/go-zero/blob/master/.github/workflows/reviewdog.yml), [`go.yml`](https://github.com/zeromicro/go-zero/blob/master/.github/workflows/go.yml).

| Область | У них | У нас | Дельта |
|---|---|---|---|
| Слои | api-сервис: `internal/{config,handler,logic,svc,middleware,types}`; rpc: `internal/{config,server,logic,svc}`; «handler должен только декодировать запрос, вызвать один метод logic и закодировать ответ»; **один файл logic на use case**; `svc.ServiceContext` переносит все зависимости в logic | application: «одна services per cohesive area, один метод на use case»; DI вручную; хендлеры тонкие | parity по сути; у нас слойная граница залинчена |
| Зависимости | через ServiceContext (сервис-локатор по сути) | конструкторы + depguard | мы строже по границам |
| Ошибки | logic возвращает error → handler `httpx.Error` → глобальный `httpx.SetErrorHandler` маппит в code/msg; sentinel-ошибки на уровне пакета; `fmt.Errorf` с `%w` | sentinel + один edge-маппер (аналог их глобального handler) | parity идея; у нас фиксация в CODING_STANDARDS |
| Тесты | table-driven; mockgen или рукописные стабы; **цель 80% покрытия новых пакетов**; `go test -race` | table-driven + fakes; coverage-таргета нет; `-race` в CI | coverage-таргет — отклонённый кандидат (шум) |
| Стиль | gofmt/goimports; комментарии «почему», а не «что»; именования по Java-стилю запрещены | gofumpt + godoclint + godot | parity+ у нас |
| CI | staticcheck через reviewdog с **отключёнными SA1019/SA1029/SA5008**; `go vet -stdmethods=false`; проверка `go mod tidy`; CodeQL; codecov | golangci-lint как единый гейт, без отключений staticcheck | мы сильнее (не глушим проверки) |

### 2.5. Three Dots Labs: wild-workouts + «Go With The Domain»

Источники: [ThreeDotsLabs/wild-workouts-go-ddd-example](https://github.com/ThreeDotsLabs/wild-workouts-go-ddd-example) (структура, [`.golangci.yml` сервиса](https://github.com/ThreeDotsLabs/wild-workouts-go-ddd-example/blob/master/internal/trainings/.golangci.yml)), книга [Go With The Domain](https://threedots.tech/go-with-the-domain/), статья [Combining DDD, CQRS and Clean Architecture](https://threedots.tech/post/ddd-cqrs-clean-architecture-combined/).

| Область | У них | У нас | Дельта |
|---|---|---|---|
| Слои | на сервис: `app/{command,query}` (CQRS), `domain/<aggregate>`, `adapters/{mysql,...}`, `ports/http`, `service` (wiring); `internal/common` (httperr, decorator, logs, metrics) | `domain/application/adapters` + `shared` + `platform` | parity; CQRS-расщепление app у нас не принято (сервис = класс с методами-use case) |
| Домен | приватные поля; конструктор валидирует все инварианты (невалидный объект несоздаваем); без сеттеров; поведение методами; «доменные функции без объектов», когда хватает функции; отдельные типы для значений с инвариантами | домен чистый (depguard), но правило «конструктор валидирует, невалидное несоздаваемо» в CODING_STANDARDS не записано | кандидат (проза) |
| Транзакции | **репозиторий сам открывает транзакцию БД** и выполняет переданную updateFn внутри (Firestore RunTransaction / MySQL); приложение о транзакции не знает; альтернатива — domain-интерфейс Tx в data-слое | UoW `transaction.UoW.Do(ctx, work)` в application (ADR 0033); ручной Begin запрещён forbidigo в мигрировавших контекстах | два разных решения одной проблемы; у нас формализовано и залинчено |
| Ошибки | slug errors: домен возвращает ошибки со «слагом» для показа пользователю; `httperr.RespondWithSlugError` на HTTP-краю | sentinel + problem details + `httpsupport.UserFacingDetail` | parity по размещению (edge); механика другая |
| Observability | **decorator-обёртки command/query handlers** для логов/метрик (сквозные, вне бизнес-кода) | slog/OTel конвенции (`docs/backend-observability.md`), spancheck | parity по цели; у нас гейт |
| Тесты | интеграционные на реальной БД (docker-compose), unit домена без моков | testcontainers-интеграция + fakes | parity |
| Линтеры | v1-набор: errcheck/gosimple/govet/staticcheck + asciicheck, bodyclose, exhaustive, exportloopref, **gocognit, nestif, goheader**, gosec, nakedret, noctx, rowserrcheck, sqlclosecheck, unparam… | v2, шире и кураторнее; gocognit/nestif осознанно отклонены | мы сильнее |

### 2.6. Grafana Loki

Источники: [grafana/loki](https://github.com/grafana/loki) (структура `pkg/*`), [`.golangci.yml`](https://github.com/grafana/loki/blob/main/.golangci.yml).

| Область | У них | У нас | Дельта |
|---|---|---|---|
| Структура | package-by-feature: `pkg/{distributor,ingester,compactor,engine,logql,logproto,...}`, `cmd/`, `integration/` (отдельные integration-тесты с build-tag), `vendor/` | слоёный DDD по контекстам | разная парадигма; их подход не переносим (у нас бизнес-домен, у них инфра-система) |
| Линтеры | 11: copyloopvar, depguard (одно правило), errcheck, gochecksumtype, goconst (5+), govet, ineffassign, misspell, revive, staticcheck, unconvert; unparam выключен; **много exclusion-правил по текстам** под легаси; `issues.fix: true` | ~50 линтеров, exclusion — точечные rules с обоснованием | мы сильнее и по набору, и по гигиене исключений |
| Тесты | integration-сьют с build-tag `integration` прямо в основном конфиге линтера | отдельная integration-джоба `-tags=integration -race` | parity по смыслу |

### 2.7. Tailscale

Источники: [tailscale/tailscale](https://github.com/tailscale/tailscale), [`.golangci.yml`](https://github.com/tailscale/tailscale/blob/main/.golangci.yml).

| Область | У них | У нас | Дельта |
|---|---|---|---|
| Структура | package-by-feature в корне (`ipn`, `control`, `derp`, `disco`, `wgengine`, `tailcfg`, …) | слоёный DDD | разная парадигма |
| Линтеры | минимум по числу, максимум кастомизации: `default: none` + bidichk, govet (**~30 analyzers перечислено явно**, свои printf-функции), importas (**no-unaliased** + обязательный alias `gliderssh`), misspell, revive (точечные правила: atomic, defer-immediate-recover, duplicated-imports, errorf, string-of-int, time-equal) | govet default + nilness; importas не нужен (алиасов нет) | их приём «явный список govet-анализаторов + свои printf-функции» — кандидат на рассмотрение в тикете №321 |
| Дисциплина | комментарии в конфиге: «Matches what we use in corp» — конфиг синхронизирован с закрытым корп-репо | наш конфиг самодостаточен | — |

### 2.8. Kubernetes

Источники: [`hack/golangci.yaml.in` (шаблон)](https://github.com/kubernetes/kubernetes/blob/master/hack/golangci.yaml.in), сгенерированные [`hack/golangci.yaml`](https://github.com/kubernetes/kubernetes/blob/master/hack/golangci.yaml) / [`hack/golangci-hints.yaml`](https://github.com/kubernetes/kubernetes/blob/master/hack/golangci-hints.yaml), скрипты [`hack/verify-golangci-lint.sh`](https://github.com/kubernetes/kubernetes/blob/master/hack/verify-golangci-lint.sh) и [`hack/verify-golangci-lint-pr-hints.sh`](https://github.com/kubernetes/kubernetes/blob/master/hack/verify-golangci-lint-pr-hints.sh).

| Область | У них | У нас | Дельта |
|---|---|---|---|
| Модель линтинга | **три конфигурации из одного шаблона**: base (весь существующий код проходит), hints (+errorlint, gomega BeTrueBecause — решает разработчик/ревьюер), strict; PR-режим проверяет только новые строки PR (`verify-golangci-lint-pr-hints.sh -n`) | один строгий конфиг на всё; политика нулевого допуска (карта №319) | осознанное расхождение: k8s — миллионы строк легаси, мы выбрали жёсткий режим; их модель — референс, если когда-нибудь понадобится транзит |
| Кастомные линтеры | свои плагины: `logcheck` (структурированные логи), `sorted` (feature gates отсортированы), `kubeapilinter` (API-конвенции) | свои гейты вне golangci: migration-lint (squawk + domain rules), freshness-чеки генерации | parity по приёму «домен-специфичные свои проверки» |
| depguard | `k8s.io/utils/pointer` → ptr; **go-cmp только в тестах**; `html/template` только в тестах | контекстные границы слоёв | их приём «тестовые библиотеки только в тестах» — кандидат |
| forbidigo | md5 запрещён; AnnotatedEventf; managedfields; featuregate.Add; ginkgo Report*; в hints — BeTrue/BeFalse → Because-варианты | UUIDv7, Begin/UoW, slog, fmt.Print | parity по приёму; наборы разные |
| Тест-линтеры | testifylint, ginkgolinter | testifylint | parity |
| gocritic | в base — точечно выключенные проверки с ссылками на issues | курируемый enabled-checks | parity по философии «курируем, не всё подряд» |
| exclusion-гигиена | каждое исключение с комментарием-ссылкой на issue | каждое с ADR-обоснованием + `warn-unused` | parity+, у нас `warn-unused` есть всегда |

## 3. Кандидаты на принятие

Формат: практика → чем у нас не покрыто → цена (S = часы, M = дни, L = недели).

### Высокая ценность / низкая цена

1. **`go.uber.org/goleak` в TestMain долгоживущих компонент** (Uber: «Use go.uber.org/goleak to check for leaks» — [style.md, Goroutines](https://github.com/uber-go/guide/blob/master/style.md)). У нас правило владельца горутины есть (рубрика CODING_STANDARDS), но выхода за пределы ревью нет: утечка в `platform/scheduler`/http-server не падает тестом. Цена S: зависимость + `defer goleak.VerifyNone(t)` в `TestMain` одного-двух тестовых бинарников.
2. **«Handle errors once» — логировать ИЛИ возвращать, не оба** (Uber — [style.md, Errors](https://github.com/uber-go/guide/blob/master/style.md)). Не покрыто ничем (проза). Цена S: одна строка-правило в CODING_STANDARDS «Errors».
3. **Static-assert имплементации порта: `var _ Port = (*Adapter)(nil)`** (Uber — [style.md, Verify Interface Compliance](https://github.com/uber-go/guide/blob/master/style.md)). Не покрыто ничём формально; паттерн уже де-факто живёт в репо (`identity/adapters/events/dispatcher.go`, `access/adapters/postgres/slot_ports.go` и др.). Цена S: закрепить конвенцией в CODING_STANDARDS и требовать в рубрике; для адаптеров без ассерта — механическое добавление.
4. **Exit-политика: `os.Exit`/`log.Fatal` только в `cmd/`** (Uber — [style.md, Exit in Main](https://github.com/uber-go/guide/blob/master/style.md)). Сегодня не покрыто ни правилом, ни линтером; код фактически чист (проверено grep — нарушений нет). Цена S: forbidigo-паттерн `os\.Exit|log\.Fatal` с path-except `cmd/` + строка в AGENTS.md.
5. **Домен-правило Three Dots Labs: конструктор валидирует все инварианты, невалидный объект несоздаваем; без сеттеров** ([Combining DDD, CQRS and Clean Architecture](https://threedots.tech/post/ddd-cqrs-clean-architecture-combined/)). У нас depguard держит чистоту импортов, но чистоту модели — только ревью. Цена S: секция «Domain» в CODING_STANDARDS (2–3 правила: приватные поля мутации только через поведение, конструктор с полной валидацией, отдельный тип для значений с инвариантами).

### Средняя ценность / средняя цена

6. **depguard «тестовые библиотеки только в тестах»** (K8s: go-cmp и html/template только в `$test` — [`hack/golangci.yaml.in`](https://github.com/kubernetes/kubernetes/blob/master/hack/golangci.yaml.in)). У нас testify вне тестов сегодня не встречается, но гейта нет. Цена S: правило depguard `tests-only-libraries` (deny `stretchr/testify` вне `*_test.go`). Дешёвая страховка тренда.
7. **Явный список govet-анализаторов + свои printf-функции** (Tailscale — [`.golangci.yml`](https://github.com/tailscale/tailscale/blob/main/.golangci.yml)). У нас govet default + nilness; явно выписанный список делает конфиг воспроизводимым при смене мажорной версии golangci (дефолты меняются). Цена S–M: перечислить и зафиксировать; вопрос тикета №321, здесь только фиксация факта.
8. **«Please keep alphabetized» в конфиге линтера** (K8s — комментарии в [`hack/golangci.yaml.in`](https://github.com/kubernetes/kubernetes/blob/master/hack/golangci.yaml.in)). Мелочь для reviewability конфига. Цена S (один комментарий-конвенция).

### Рассмотрено и отклонено (с причиной)

- **PR-hints / new-from-rev модель K8s** (строже только для новых строк) — противоречит принятой политике нулевого допуска (карта №319: nolint и исключения убираются, код чинится). Ценность появляется только при несоразмерном легаси, которого у нас нет.
- **coverage-таргет 80% новых пакетов** (go-zero Code Style) — метрика поощряет антуражные тесты; карта №319 уже имеет «метрический трек приёмки» как открытый вопрос бара (№323), решение там, а не здесь.
- **gocyclo/gocognit/nestif/lll/mnd** (kratos, wild-workouts) — уже прогнаны и осознанно отклонены в нашем `.golangci.yml` (150+ срабатываний на известном техдолге; тема ремедиационного плана №325, не CI-гейта).
- **Closure-in-repository транзакции Three Dots** — конкурирующее решение с нашим UoW (ADR 0033); миграция смысла не имеет, у нас залинчено forbidigo.
- **assertion-библиотеки под запрет (Google)** — у нас testify принят и залинчен testifylint; откат ломает весь тестовый корпус.
- **importas с no-unaliased (Tailscale)** — в модуле нет алиасов для enforce (зафиксировано в нашем конфиге).
- **Wire DI (kratos), ServiceContext (go-zero)** — кодогенерация/сервис-локатор против наших явных конструкторов; ADR не требуется, depguard-границы важнее.

## 4. Где наш свод сильнее эталонов (не сломать при синхронизации)

1. **Границы слоёв залинчены depguard** с перечислением контекстов (`domain-clean`/`application-clean`/`platform-clean`) — ни один из изученных эталонов не имеет автоматического гейта на направление зависимостей между слоями (kratos и Three Dots держат инверсию договором/ревью; Loki/Tailscale вообще не слоёные).
2. **Доменные гейты вне Go-линтера**: деньги BIGINT-копейки с тотальным запретом float в миграциях, UUIDv7 app-side (forbidigo + migration-lint), no-ORM, no-MinIO, squawk lock-safety — уникально против всех эталонов (ближайший аналог — кастомные плагины K8s под их домен).
3. **Гигиена исключений**: точечные rules с ADR-обоснованием + `warn-unused: true` + `nolintlint` strict (`require-explanation`, `require-specific`, `allow-unused: false`) против exclusion-heavy подхода Loki/K8s (десятки глобальных text-исключений под легаси).
4. **Observability-гейты**: spancheck со всеми тремя проверками, sloglint `context: all`, forbidigo на stdlib log — ни у кого из эталонов наблюдаемость не залинчена (у Three Dots — декораторы вручную, у K8s — свой logcheck, но только структурированность).
5. **Freshness-гейты кодогенерации** (`backend-openapi-check`, `backend-sqlc-check`, `attributes-check`, `backend-tkassa-spec-check`) — контракт-first с защитой от рассинхрона; у kratos генерация есть, гейта свежести нет.
6. **Набор линтеров**: standard + ~45 (баг-класс: errorlint, nilnesserr, musttag, usetesting, gosmopolitan…) против 11 у Loki, 17 у kratos, 5 у Tailscale, ~15 у K8s-base. Ближайший по плотности — K8s, и тот с выключенными категориями.
7. **Форматтер gofumpt** (строже gofmt) — из фреймворков только kratos; Google/Uber/Loki/Tailscale на gofmt/goimports.
8. **Тест-инфраструктура**: testcontainers-интеграция на реальном PostgreSQL 18 + `-race` в CI как обязательный контур — у эталонов либо docker-compose руками (Three Dots), либо integration-build-tag (Loki), либо нет отдельного integration-гейта.

## 5. Куда дальше

- Кандидаты 1–5 (goleak, handle-once, static-assert, exit-политика, домен-конструкторы) — готовый вход для тикета-бара №323; каждый кандидат самостоятелен, взаимных блокировок нет.
- Кандидат 6 (test-only depguard) и 7 (govet-список) — вход в каталог тикета №321.
- Метрические вопросы (coverage, сложность) остаются открытому вопросу карты (№319 «Not yet specified»).

## 6. Использованные первоисточники

- Google: [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments), [Go Style Guide — Decisions](https://google.github.io/styleguide/go/decisions), [Best Practices](https://google.github.io/styleguide/go/best-practices).
- Uber: [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md), [пример конфига](https://github.com/uber-go/guide/blob/master/.golangci.yml).
- Kratos: [kratos-layout README](https://github.com/go-kratos/kratos-layout/blob/main/README.md), [kratos .golangci.yml](https://github.com/go-kratos/kratos/blob/main/.golangci.yml).
- go-zero: [Project Structure](https://go-zero.dev/concepts/project-structure/), [Design Principles](https://go-zero.dev/concepts/design-principles/), [Error Handling](https://go-zero.dev/guides/http/server/error/), [Code Style](https://go-zero.dev/community/code-style/), [reviewdog.yml](https://github.com/zeromicro/go-zero/blob/master/.github/workflows/reviewdog.yml), [go.yml](https://github.com/zeromicro/go-zero/blob/master/.github/workflows/go.yml).
- Three Dots Labs: [wild-workouts-go-ddd-example](https://github.com/ThreeDotsLabs/wild-workouts-go-ddd-example), [Go With The Domain](https://threedots.tech/go-with-the-domain/), [Combining DDD, CQRS and Clean Architecture](https://threedots.tech/post/ddd-cqrs-clean-architecture-combined/).
- Production: [grafana/loki](https://github.com/grafana/loki) (+ [конфиг](https://github.com/grafana/loki/blob/main/.golangci.yml)), [tailscale/tailscale](https://github.com/tailscale/tailscale) (+ [конфиг](https://github.com/tailscale/tailscale/blob/main/.golangci.yml)), [kubernetes/kubernetes](https://github.com/kubernetes/kubernetes) ([golangci.yaml.in](https://github.com/kubernetes/kubernetes/blob/master/hack/golangci.yaml.in), [verify-golangci-lint-pr-hints.sh](https://github.com/kubernetes/kubernetes/blob/master/hack/verify-golangci-lint-pr-hints.sh)).
