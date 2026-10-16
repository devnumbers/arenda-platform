# golangci-lint v2.12.2: полный каталог против текущего конфига — что и как зажать

Research-тикет [#321](https://github.com/devnumbers/arenda-platform/issues/321), карта №319 (режим качества Go-бекенда).
Дата: 2026-08-18. Версия линтера: v2.12.2 (пин в корневом Makefile, `GOLANGCI_LINT_VERSION`).

## Метод

- Baseline: текущий `.golangci.yml` по `apps/backend` с `--build-tags=integration` — **0 нарушений** (точка отсчёта чистая).
- Пробные прогоны временными конфигами в `.tmp/lint-trials/config.trial*.yml` (копия текущего конфига + добавленное); реальный `.golangci.yml` не менялся.
- Команда прогона: `cd apps/backend && go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run --config ../../.tmp/lint-trials/config.X.yml --build-tags=integration --output.text.print-issued-lines=false ./...`
- `issues.max-issues-per-linter: 0` в конфиге — счётчики полные, не обрезаны.
- Полный каталог линтеров и настроек — официальная документация https://golangci-lint.run/docs/linters/ и канонический `.golangci.reference.yml` из репозитория golangci-lint (все дефолты и опции настроек; получен 2026-08-18). Поведение gosec-правил проверено эмпирическим probe-файлом (удалён после прогона).
- 8 батчей: все не-включённые линтеры разом (trial1), ужесточения настроек (trial2), gocritic полные теги (trial3), lll@140 (trial4/5), errcheck/sloglint максимальные (trial6), gofumpt extra-rules (trial7), пороги сложности (trial8).

Политика карты: нулевой допуск исключений — код чиним, а не глушим; прежние отклонения пересматриваются.

---

## Блок 1. Не включённые линтеры (полный каталог v2.12.2 против enable)

Из ~103 линтеров каталога 67 уже включены (62 в enable + 5 standard). Ниже — измеренные счётчики всех остальных (прогон по всему `apps/backend`, v2.12.2).

### 1.1. Включить немедленно — 0 нарушений, чистая профилактика

| Линтер | Счётчик | Обоснование |
|---|---|---|
| `gochecksumtype` | 0 | Exhaustiveness-проверки Go sum types (sealed interfaces, Go 1.25+). Страховка на будущее, бесплатная. [Каталог](https://golangci-lint.run/docs/linters/) |
| `testableexamples` | 0 | Example-функций в модуле нет; правило начнёт работать при их появлении. Прежнее отклонение «нет Example» пересмотрено: 0-стоимость включения. |

### 1.2. Включить после микроремедиации — счётчик ≤ 6, дешёвый фикс

| Линтер | Счётчик | Что чинить |
|---|---|---|
| `godox` | 1 | `TODO: re-add SMS dispatch` в `platform/scheduler/reminder_worker.go:235` → перенести в GitHub Issue. |
| `intrange` | 1 | Один цикл `for i := 0; i < n; i++`. ВАЖНО: modernize (rangeint) его НЕ ловит — счётчик modernize = 0, значит это не дубль, а слепая зона. Прежнее отклонение «дубль modernize» пересмотрено. |
| `gocyclo` | 2 (порог 30) | `platform/httpsupport/problem.go:62` UserFacingDetail (39) — рефакторинг switch→map; `billing/adapters/payment/fake/fake_test.go:308` (32). |
| `iface` | 2 | Два идентичных интерфейса `ExpiredSessionDeleter`/`ExpiredLoginCodeDeleter` в `identity/adapters/scheduler/cleaner.go:21,27`. **Пересмотр отклонения**: прежние «13 FP на DDD-портах» устарели — код почищен, осталось 2 реальных дубля. |
| `dogsled` | 4 | 3+ подчёркивания в присваивании — тривиальный рефакторинг. |

### 1.3. Включить со средней ремедиацией — реальный сигнал, чинится кодом

| Линтер/настройка | Счётчик | Оценка фикса |
|---|---|---|
| `gocritic` + `importShadow` | 20 | Затенение имён пакетов (`clock`, `url`) локальными переменными — реальный источник багов. Дешёвый фикс (переименование локалей). |
| `gocritic` + `paramTypeCombine` | 16 | `func(_ context.Context, _ uuid.UUID, _ uuid.UUID)` → `(_, _ uuid.UUID)`. Механика. |
| `gocritic` + `unnamedResult` | 12 | Именовать результаты там, где типы одинаковые/неочевидные. |
| `dupl` threshold 150 (сейчас 200) | 16 (= 8 пар) | Реальные кластеры: `platform/scheduler/billing_worker.go` ↔ `payment_reconciliation_worker.go`, `admin/adapters/http/admin_handlers.go`, `leases/adapters/http/operation_handlers.go`, `properties/adapters/http/property_handlers.go` (4 пары в проде) + 4 пары в тестах. Рефакторинг дублирования. |
| `revive` + `unhandled-error` | 30 | 17 из них — `strings.Builder.Write*` (документированный always-nil error; исключить через `exclude-functions`), 12 `fmt.Fprintf` (10 — реальный сигнал в `platform/mailer/smtp/smtp.go`: игнорирование ошибок записи в SMTP-сессии) + 1 `io.Writer.Write`. |
| `sloglint` `static-msg` | 6 | 6 сообщений через переменные (billing/workers, identity/scheduler, database/instrumentation) → константы. Прежний вердикт «одна нефиксибельна» не подтвердился. |

### 1.4. Включить большой ремедиацией — решение о сроках за баром №323

| Линтер | Счётчик | Комментарий |
|---|---|---|
| `errcheck` check-blank + check-type-assertions + disable-default-exclusions | 248 (137 тесты / 111 прод) | Реальный баг-класс: `_ = tx.Rollback`, `resp.Body.Close`, `w.Write`, blank `uuid.NewV7()`. Прежнее отклонение check-blank «212 шумовых» пересматривается политикой: шум чинится кодом (обработка или осознанный хелпер must*). |
| `goconst` | 314 (56 прод / 258 тесты) | Пересмотр прежнего отклонения: при нулевой политике повторяющиеся строки выносятся в константы. Механическая волна. |
| `godot` `scope: all` + `capital: true` | 395 (314 capital / 81 period; 143 прод / 252 тесты) | Гигиена комментариев. Механика. |
| `staticcheck` вернуть ST1000 | 199 | Package comments. Механика (дописать doc-комментарии пакетам). Прочие исключения ST1016/ST1020-1022 — 0 дополнительных нарушений: вернуть в `checks: ["all"]` можно бесплатно. |
| `cyclop` max-complexity 15 | 57 | Техдолг-функции (Load, HandleWebhook, invitation_service) — карта ремедиации №325. |
| `gocognit` min-complexity 25 | 64 | То же. |
| `lll` line-length 140 | 623 | При 120 — 1399; при 140 — 623. Гигиена длинных строк, поэтапно. |
| `paralleltest` | 888 (billing 384, identity 136, leases 112, properties 99, notifications 71, access 48, platform 33, shared 5) | Добавление `t.Parallel()`. Большая волна с рисками (общие фикстуры, testcontainers); включать поэтапно с юнит-тестов. |

### 1.5. Не включать — отклонение сохраняется при новой политике (аргументированный пересмотр)

Правило пересмотра: отклонение сохраняется не потому, что «шумно», а потому что (а) линтер борется с идиомами Go/проекта без баг-класса, (б) инструмент структурно несовместим с DDD-архитектурой, (в) нет объекта проверки в стеке.

| Линтер | Счётчик | Причина сохранения отклонения |
|---|---|---|
| `wsl_v5` | 5448 | Стиль пустых строк. Нулевой баг-класс. |
| `nlreturn` | 2486 | Пустая строка перед return. Чистый стиль. |
| `exhaustruct` | 1705 | Частичная инициализация composite literals — идиоматичный Go (sqlc-параметры, домены). Принуждение к полному списку полей контрпродуктивно. |
| `noinlineerr` | 1426 | Прямо запрещает `if err := ...; err != nil` — базовая идиома проекта и Go. |
| `varnamelen` | 1044 | Короткие локальные имена (i, tx, id) идиоматичны в малых областях видимости. |
| `err113` | 282 | Запрещает `errors.New`/`fmt.Errorf` в телах функций — паттерн доменных ошибок проекта (динамические сообщения конфигурации). |
| `testpackage` | 143 | White-box доменные тесты — осознанный выбор; перевод в `_test` — большая механика с риском ломки инкапсуляционных проверок. |
| `ireturn` | 142 | FP на DDD-портах: возврат интерфейсов — суть hexagonal, не дефект. |
| `mnd` | 131 | Числа в тестах/SQL-лимитах — данные, а не магия. |
| `funcorder` | 85 | Порядок exported/unexported — стиль без баг-класса. |
| `tagliatelle` | 50 | Внешний API-контракт (T-Bank PascalCase + openapi смешанный) не подчиняется внутренней конвенции. |
| `gochecknoglobals` | 67 | Package-level таблицы (sortFields-мапы), OTel tracer, regexp.Must — идиома; тестовые фикстуры — данные. |
| `embeddedstructfieldcheck` | 49 | Чистый стиль без баг-класса. |
| `funlen` | 37, `nestif` 19, `gocognit`@30 43, `cyclop`@10 236 | Метрики — тема рефакторинга №325, не CI-гейт в лоб (при порогах 15/25 см. 1.4). |
| `interfacebloat` | 9 | Осознанные порты 12-31 метода; дробление синтетично. |
| `nonamedreturns` | 11 | Named return — идиома defer+метрики (tkassa). Не шум, а язык. |
| `gomodguard_v2` | 0 | Дублирует depguard `no-orm` (запреты gorm.io/entgo.io/xorm.io уже в deny). |
| `maintidx` 0, `decorder` 0, `grouper` 0, `importas` 0 | 0 | Нет правила для enforce / нечего группировать. |
| `arangolint`, `clickhouselint`, `ginkgolinter`, `promlinter`, `protogetter`, `zerologlint`, `unqueryvet`, `exptostd` | 0 | Нет соответствующего стека в модуле (Arango/ClickHouse/ginkgo/prometheus/protobuf/zerolog/`SELECT *`/x-exp). exptostd — включить при появлении golang.org/x/exp в go.mod. |

Проверено прогонами: все «нулевые» реально дают 0 на текущем коде (не выключены конфигом).

---

## Блок 2. Настройки существующих линтеров

### 2.1. Ужесточить немедленно — 0 нарушений (бесплатно)

| Настройка | Значение | Ссылка |
|---|---|---|
| `sloglint.no-global` | `"all"` | 0 нарушений (прошлый «1 сигнал» уже не воспроизводится). [sloglint](https://github.com/go-simpler/sloglint#no-global-logger) |
| `sloglint.msg-style` | `"lowercased"` | 0 нарушений — конвенция уже соблюдается. |
| `govet` + `stdversion` | enable | 0 нарушений при go 1.26. |
| `errchkjson.check-error-free-encoding` | `true` | 0 нарушений. |
| `formatters.settings.gofumpt.extra-rules` | `true` | 0 diff (проверено `golangci-lint fmt -d`). |
| `staticcheck.checks` | `["all"]` без `-ST1016,-ST1020,-ST1021,-ST1022` | Возврат этих проверок — 0 дополнительных нарушений (только ST1000 даёт сигнал, см. 1.4). |

### 2.2. Ужесточить с малой ремедиацией

- `dupl.threshold`: 200 → **150** (16 нарушений = 8 пар, см. 1.3).
- `sloglint.static-msg: true` (6).
- `revive` + правило `unhandled-error` (30, из них 17 — структурный FP `strings.Builder.Write*`, гасится `exclude-functions: [strings.Builder.WriteString, strings.Builder.WriteByte, strings.Builder.WriteRune]` — документированный always-nil error).
- `gocritic.enabled-checks` + `importShadow`, `paramTypeCombine`, `unnamedResult` (20/16/12).

### 2.3. Ужесточить большой ремедиацией (решение бара №323)

- `errcheck`: `check-blank: true`, `check-type-assertions: true`, `disable-default-exclusions: true` → 248. exclude-functions для Builder-методов — только в pointer-форме `(*strings.Builder).WriteString`: форма без `(*...)` не матчится и под disable-default-exclusions молча перестаёт исключать (обнаружено волной #354); для revive unhandled-error форма проверяется при его включении.
- `godot`: `scope: all`, `capital: true` → 395.
- `govet` + `shadow` → **341**: сохраняю рекомендацию «не включать»: `if err := ...; err != nil` — фундаментальная идиома; shadow-находки в 95% легитимные затенения в ветках обработки. Это не глушение шума, а выбор идиомы.
- `govet` + `fieldalignment` → **269**: не включать — переупаковка полей ради байтов противоречит читаемости DDD-доменов (не embedded).
- `gocritic` теги целиком: `performance` (hugeParam 381 + rangeValCopy 221 = 91% из 662) — не включать: передача sqlc-структур и доменных агрегатов по значению — осознанная семантика; переход на указатели меняет API-контракты ради микрокопий. Пересмотр возможен отдельной волной рефакторинга. `style`-тег целиком не включать — курируемые чеки точечнее (см. 2.2).

### 2.4. nolintlint: «полный запрет nolint»

Механизма полного запрета `//nolint` в самом golangci-lint нет (nolintlint проверяет только формат: объяснение, конкретность, неиспользуемость — всё уже включено в максимальном режиме). Ноль-политика по nolint реализуется вне конфига:

1. Ремедиация всех 54 директив (классификация в блоке 3).
2. Гейт «0 nolint» отдельной проверкой в CI (grep-скрипт в `tools/`, аналогично migration-lint) — предложение для бара №323, не для `.golangci.yml`.

### 2.5. gosec

Фильтров severity/confidence в конфиге нет — весь gosec активен (правильно). Правило-специфика по probe-прогону:

- **G115** (int-конверт) гасится кодом: `if v > math.MaxInt32 { ... } if v < math.MinInt32 { ... }` перед `int32(v)` — gosec замолкает. `min(v, math.MaxInt32)` НЕ распознаётся как guard (проверено эмпирически).
- **G404**: флагает и math/rand, и math/rand/v2 — единственный фикс кодом это crypto/rand.
- **G124** (cookie): флагает даже `Secure: true, HttpOnly: true, SameSite: Lax` — правило фактически требует Strict; конфликтует с ADR 0018 (SameSite=Lax — продуктовое решение). Нужен выбор: gosec `excludes: ["G124"]` с ADR-обоснованием (сигнал правила в наших условиях нулевой) или пересмотр ADR 0018 на Strict (вне скоупа этого research).
- **G120**: `ParseMultipartForm` флагается всегда, даже с константным лимитом. Фикс кодом — переход на `r.MultipartReader()` (стриминг, честно лучше по памяти).
- **G101**: триггерится именем переменной (`...Secret`) — фикс переименованием.

---

## Блок 3. Классификация 54 nolint-директив и 3 path-исключений forbidigo

Распределение: `gosec` 45, `sloglint` 6, `staticcheck` 1, `contextcheck` 1, `containedctx` 1.

### 3.1. Чинится кодом — 49 директив

| Группа | Кол-во | Места | Фикс |
|---|---|---|---|
| gosec **G115** (int→int32/byte конверты для sqlc-параметров; pagination/limit/batch/payment-day) | ~40 | `leases/adapters/postgres/repository.go` (17), `notifications/adapters/postgres/repository.go` + `free_reminder_repository.go` (18), `billing/adapters/postgres/{payment,tariff,subscription}_repository.go` (6), `identity/adapters/postgres/attempt_repository.go` (2), `notifications/adapters/webpush/crypto.go:189` (1) | Общий guard-хелпер `shared.ToInt32Clamped(v)` c if-границами (probe: G115 гаснет) — один хелпер закрывает всю группу. |
| sloglint (context:all) | 6 | `cmd/api/main.go:30,390` (2) — лог до инициализации: фикс `context.Background()`; `platform/logger/context_test.go:25,71,95` (3) — тест обёртки: передать ctx; `platform/httpsupport/problem.go:164` (1) — `WriteProblem` без ctx в сигнатуре: добавить ctx (вызовы имеют `r.Context()`). | Прямой фикс кода. |
| gosec **G404** | 1 | `billing/adapters/payment/tkassa/provider.go:143` — full-jitter retry. | crypto/rand-хелпер (rand/v2 НЕ решает — флагается тоже). |
| gosec **G101** | 1 | `notifications/adapters/webpush/crypto_test.go:84` — RFC 8291 test vector. | Переименовать `rfc8291ECDHSecret` → без «Secret» в имени. |
| gosec **G120** | 1 | `properties/adapters/http/property_handlers.go:460` — ParseMultipartForm(6<<20). | Переписать на `r.MultipartReader()` (стриминг). |
| gosec **G124** | 2 | `platform/httpsupport/session.go:26,40` — динамический Secure по APP_ENV. | Прямого фикса кодом нет (см. 2.5): **переконфигурация** — `gosec.excludes: [G124]` с ADR-обоснованием «SameSite=Lax по ADR 0018, HttpOnly всегда, Secure по APP_ENV; правило требует Strict». Единственная группа на исключение правила вместо nolint. |
| contextcheck | 1 | `cmd/api/wire/properties.go:43` — NewS3Storage с ctx.Background внутри. | Добавить ctx-параметр в сигнатуру конструктора (комментарий признаёт: «signature unchanged from original» — волонтёрное решение). |
| containedctx | 1 | `cmd/api/wire/wire.go:67` — поле `Platform.Ctx`. | Убрать поле: метод `Run(ctx)` / `Cancel` через отдельный вызов. Средняя цена, композиционный корень один. |

Итого «чинится кодом»: 47; «переконфигурацией»: 2 (G124, при выборе excludes-пути).

### 3.2. Легитимно с ADR-обоснованием — 1 директива

| Место | Правило | Обоснование |
|---|---|---|
| `notifications/adapters/webpush/vapid.go:101` — `elliptic.Curve.ScalarBaseMult` | staticcheck SA1019 (deprecated) | Единственный stdlib-путь P-256 scalar→point для восстановления `ecdsa.PrivateKey` (ES256 для Web Push VAPID); `crypto/ecdh` не даёт обратного преобразования. Альтернатива — полный переход sender-ключа на ecdh с собственным ES256-подписыванием (средняя цена). Оставить как ADR-случай или завести тикет на ecdh-переход. |

### 3.3. Три path-исключения forbidigo в конфиге

| Исключение | Вердикт |
|---|---|
| `path-except: internal/(identity\|billing)/application` для Begin (ADR 0033) | **Переконфигурация по плану ремедиации**, не навсегда: правило сужается по мере миграции контекстов на UoW (карта №319, Notes — fate решается в плане ремедиации №325 вместе с до-миграцией). После миграции последнего контекста — исключение удалить. |
| `internal/identity/application/.*_test\.go` (Begin) | Аналогично: тест-фикстуры зовут `transaction.Beginner.Begin` напрямую. Либо переводить тесты на runInTx-хелпер, либо после полного UoW-покрытия признать легитимным в ADR 0033 (фикстурам нужен контроль транзакции). Рекомендация: оставить до ремедиации, затем переоценить. |
| `internal/billing/application/.*_test\.go` (Begin) | То же. |

---

## Черновик целевого конфига

ЭТАП 1 — включить сразу (после микроремедиации ≤ 6 правок; всё остальное уже 0): godox, intrange, gocyclo, iface, dogsled, gochecksumtype, testableexamples; настройки 2.1 + 2.2 (dupl 150, sloglint static-msg, revive unhandled-error, gocritic +3 чека) — суммарная ремедиация ~90 правок.
ЭТАП 2 — после ремедиационных волн (бар №323 + план №325): errcheck full, goconst, godot strict, ST1000, cyclop@15/gocognit@25, lll@140, paralleltest.
В черновике этап 2 закомментирован меткой `# REMEDIATION:` — включается по готовности кода.

```yaml
# Конфигурация golangci-lint v2. Версия пинится в Makefile (GOLANGCI_LINT_VERSION).
# Принцип: курируемый строгий набор — баг-класс + гигиена под стек проекта.
# Черновик целевого состояния (research #321, 2026-08-18): ЭТАП 1 готов к включению
# после микроремедиации; блоки с "# REMEDIATION:" включаются после волн ремедиации
# (бар №323, план №325). Список осознанных отклонений — в конце файла.
version: "2"

run:
  timeout: 5m

linters:
  default: standard
  enable:
    # --- баги и корректность ---
    - asasalint
    - bidichk
    - bodyclose
    - contextcheck
    - copyloopvar
    - dupword
    - durationcheck
    - errchkjson
    - errname
    - errorlint
    - exhaustive
    - fatcontext
    - forcetypeassert
    - gocheckcompilerdirectives
    - gocritic
    - gosec
    - loggercheck
    - makezero
    - mirror
    - musttag
    - nilerr
    - nilnesserr
    - nilnil
    - noctx
    - nosprintfhostport
    - reassign
    - rowserrcheck
    - spancheck
    - sqlclosecheck
    - tparallel
    - unparam
    - wastedassign
    # --- современный Go и гигиена ---
    - asciicheck
    - canonicalheader
    - containedctx
    - godoclint
    - godot
    - gomoddirectives
    - goprintffuncname
    - gosmopolitan
    - inamedparam
    - iotamixing
    - misspell
    - modernize
    - nakedret
    - perfsprint
    - prealloc
    - thelper
    - unconvert
    - usetesting
    - usestdlibvars
    - whitespace
    # --- архитектура и процесс ---
    - depguard
    - dupl
    - forbidigo
    - gochecknoinits
    - nolintlint
    - predeclared
    - recvcheck
    - revive
    - sloglint
    - testifylint
    # --- ЭТАП 1 (research #321): новые линтеры ---
    - dogsled            # 4 фиксa
    - gochecksumtype     # 0 нарушений; страховка sealed-интерфейсов
    - godox              # 1 TODO → issue tracker
    - gocyclo            # 2 (порог 30): problem.go, fake_test.go
    - iface              # 2: дубль-интерфейсы identity/scheduler
    - intrange           # 1 цикл; modernize его не ловит (не дубль)
    - testableexamples   # 0; профилактика Example-функций
    # REMEDIATION (этап 2, бар №323):
    # - goconst          # 314 (56 прод): вынос констант
    # - cyclop           # 57 при max-complexity 15
    # - gocognit         # 64 при min-complexity 25
    # - lll              # 623 при line-length 140
    # - paralleltest     # 888: t.Parallel() поэтапно с юнит-тестов
  settings:
    depguard:
      rules:
        domain-clean:
          list-mode: lax
          files:
            - "**/internal/**/domain/*.go"
            - "!**/*_test.go"
          deny:
            - pkg: database/sql
              desc: domain must stay persistence agnostic
            - pkg: net/http
              desc: domain must stay transport agnostic
            - pkg: github.com/aws/aws-sdk-go-v2
              desc: domain must stay external-service agnostic
            - pkg: github.com/jackc/pgx
              desc: domain must stay persistence agnostic
            - pkg: github.com/nambers/arenda-planform/apps/backend/internal/platform
              desc: domain must not depend on platform infrastructure
            - pkg: github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters
              desc: domain must not depend on adapters
            - pkg: github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters
              desc: domain must not depend on adapters
            - pkg: github.com/nambers/arenda-planform/apps/backend/internal/properties/adapters
              desc: domain must not depend on adapters
            - pkg: github.com/nambers/arenda-planform/apps/backend/internal/leases/adapters
              desc: domain must not depend on adapters
            - pkg: github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters
              desc: domain must not depend on adapters
            - pkg: github.com/nambers/arenda-planform/apps/backend/internal/audit/adapters
              desc: domain must not depend on adapters
            - pkg: github.com/nambers/arenda-planform/apps/backend/internal/admin/adapters
              desc: domain must not depend on adapters
            - pkg: github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi
              desc: domain must not import generated OpenAPI types
            - pkg: github.com/nambers/arenda-planform/apps/backend/internal/platform/config
              desc: domain must not read infrastructure configuration
        application-clean:
          list-mode: lax
          files:
            - "**/internal/**/application/*.go"
            - "!**/*_test.go"
          allow:
            - github.com/nambers/arenda-planform/apps/backend/internal/platform/requestctx
          deny:
            - pkg: database/sql
              desc: application layer must use ports instead of concrete persistence
            - pkg: net/http
              desc: application layer must stay transport agnostic
            - pkg: github.com/aws/aws-sdk-go-v2
              desc: application layer must use ports instead of concrete external clients
            - pkg: github.com/jackc/pgx
              desc: application layer must use ports instead of concrete persistence
            - pkg: github.com/nambers/arenda-planform/apps/backend/internal/platform
              desc: application layer must not depend on platform infrastructure
            - pkg: github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters
              desc: application layer must not depend on adapters
            - pkg: github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters
              desc: application layer must not depend on adapters
            - pkg: github.com/nambers/arenda-planform/apps/backend/internal/properties/adapters
              desc: application layer must not depend on adapters
            - pkg: github.com/nambers/arenda-planform/apps/backend/internal/leases/adapters
              desc: application layer must not depend on adapters
            - pkg: github.com/nambers/arenda-planform/apps/backend/internal/notifications/adapters
              desc: application layer must not depend on adapters
            - pkg: github.com/nambers/arenda-planform/apps/backend/internal/audit/adapters
              desc: application layer must not depend on adapters
            - pkg: github.com/nambers/arenda-planform/apps/backend/internal/admin/adapters
              desc: application layer must not depend on adapters
            - pkg: github.com/nambers/arenda-planform/apps/backend/internal/platform/openapi
              desc: application layer must not import generated OpenAPI types
            - pkg: github.com/nambers/arenda-planform/apps/backend/internal/platform/config
              desc: application layer must not read infrastructure configuration
        platform-clean:
          list-mode: lax
          files:
            - "**/internal/platform/**/*.go"
            - "!**/*_test.go"
          allow:
            - github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/http
            - github.com/nambers/arenda-planform/apps/backend/internal/identity/application
          deny:
            - pkg: github.com/nambers/arenda-planform/apps/backend/internal/identity
              desc: platform infrastructure must not depend on a bounded context; use shared/actor
        no-orm:
          list-mode: lax
          deny:
            - pkg: gorm.io
              desc: persistence is explicit SQL through sqlc; ORM libraries are banned
            - pkg: entgo.io
              desc: persistence is explicit SQL through sqlc; ORM libraries are banned
            - pkg: xorm.io
              desc: persistence is explicit SQL through sqlc; ORM libraries are banned
        no-minio:
          list-mode: lax
          deny:
            - pkg: github.com/minio
              desc: S3-compatible storage access goes through the storage port; MinIO is not a dependency
    errcheck:
      # REMEDIATION (этап 2): 248 нарушений (tx.Rollback, Body.Close, w.Write, blank uuid.NewV7)
      # check-blank: true
      # check-type-assertions: true
      # disable-default-exclusions: true
      exclude-functions:
        # pointer-receiver форма обязательна: без `(*...)` запись не матчится,
        # и под disable-default-exclusions Builder-находки вернутся (волна #354)
        - (*strings.Builder).WriteString
        - (*strings.Builder).WriteByte
        - (*strings.Builder).WriteRune
    govet:
      enable:
        - nilness
        - stdversion        # 0 нарушений при go 1.26
    exhaustive:
      default-signifies-exhaustive: true
    staticcheck:
      # REMEDIATION (этап 2): вернуть ST1000 (199 package comments); прочие ST — уже 0
      # checks: ["all", "-ST1000"]
      checks: ["all", "-ST1000", "-ST1016", "-ST1020", "-ST1021", "-ST1022"]
    spancheck:
      checks: ["end", "set-status", "record-error"]
    loggercheck:
      require-string-key: true
      no-printf-like: true
    sloglint:
      context: all
      no-global: "all"       # 0 нарушений
      msg-style: "lowercased" # 0 нарушений — конвенция соблюдается
      static-msg: true        # ЭТАП 1: 6 констант
    errchkjson:
      check-error-free-encoding: true   # 0 нарушений
    gocritic:
      enabled-checks:
        - appendCombine
        - badLock
        - badRegexp
        - badSorting
        - badSyncOnceFunc
        - deferInLoop
        - dupOption
        - dynamicFmtString
        - emptyFallthrough
        - evalOrder
        - externalErrorReassign
        - filepathJoin
        - nilValReturn
        - regexpPattern
        - returnAfterHttpError
        - sloppyReassign
        - sortSlice
        - sqlQuery
        - truncateCmp
        - weakCond
        - sprintfQuotedString
        # ЭТАП 1 (research #321): +3 чека из style-тега (пересмотр раунда 2)
        - importShadow        # 20: затенение пакетов clock/url
        - paramTypeCombine    # 16
        - unnamedResult       # 12
    dupl:
      threshold: 150          # ЭТАП 1: 8 реальных пар (scheduler-воркеры, HTTP-хендлеры)
    forbidigo:
      analyze-types: true
      forbid:
        - pattern: ^(fmt\.Print(|f|ln)|print|println)$
        - pattern: ^log\.
          msg: use log/slog instead of stdlib log
        - pattern: '\.Begin$'
          msg: "use runInTx/UoW instead of manual Begin (ADR 0033)"
        - pattern: '^(github\.com/google/)?uuid\.New(|V4|String|Random|RandomFromReader)$'
          msg: "identifiers are UUIDv7 only (ADR 0019): use uuid.NewV7() or uuid.Must(uuid.NewV7())"
    modernize:
      disable:
        - forvar
        - omitzero
    revive:
      rules:
        - name: exported
          disabled: true
        - name: package-comments
          disabled: true
        - name: error-strings
          disabled: true
        - name: errorf
          disabled: true
        - name: use-any
        - name: early-return
        - name: unhandled-error    # ЭТАП 1: 13 реальных (smtp.go Fprintf); Builder-методы
          exclude-functions:        # исключены в errcheck.exclude-functions (always-nil error)
          - strings.Builder.WriteString
          - strings.Builder.WriteByte
          - strings.Builder.WriteRune
    godot:
      # REMEDIATION (этап 2): scope all + capital (395: 314 capital / 81 period)
      scope: declarations
    nolintlint:
      allow-unused: false
      require-explanation: true
      require-specific: true
    gosec:
      excludes:
        - G124   # cookie-атрибуты: правило требует SameSite=Strict, ADR 0018 фиксирует Lax
                 # (Secure по APP_ENV, HttpOnly всегда); сигнал правила в этих условиях нулевой.
  exclusions:
    generated: strict
    warn-unused: true
    rules:
      - linters: [forbidigo]
        path-except: internal/(identity|billing)/application
        text: 'use runInTx/UoW instead of manual Begin'
      - linters: [forbidigo]
        path: internal/identity/application/.*_test\.go
        text: 'use runInTx/UoW instead of manual Begin'
      - linters: [forbidigo]
        path: internal/billing/application/.*_test\.go
        text: 'use runInTx/UoW instead of manual Begin'

formatters:
  enable:
    - gofumpt
    - gci
  settings:
    gofumpt:
      extra-rules: true   # 0 diff — бесплатно

issues:
  max-issues-per-linter: 0
  max-same-issues: 0

# Осознанно НЕ включаем (пересмотрено research #321, 2026-08-18; счётчики измерены v2.12.2):
# - Стиль без баг-класса: wsl_v5 (5448), nlreturn (2486), funcorder (85),
#   embeddedstructfieldcheck (49), varnamelen (1044), mnd (131), dogsled→включён.
# - Против идиом Go/проекта: noinlineerr (1426 — запрет if err :=), exhaustruct (1705 —
#   частичная инициализация), nonamedreturns (11 — defer+named return), err113 (282 —
#   доменные errors.New), testpackage (143 — white-box доменные тесты), gochecknoglobals
#   (67 — package-level таблицы/tracer), ireturn (142 — FP на DDD-портах), interfacebloat
#   (9 — осознанные порты).
# - Внешний контракт: tagliatelle (50 — T-Bank PascalCase/openapi смешанный).
# - Метрики: funlen 37 / nestif 19 — тема рефакторинга №325; cyclop/gocognit/lll — этап 2.
# - govet shadow (341 — идиома if err :=) и fieldalignment (269 — упаковка против
#   читаемости DDD-доменов); gocritic performance-тег (hugeParam 381 + rangeValCopy 221 —
#   семантика передачи sqlc-структур/доменов по значению).
# - Стек-нерелевантные (0): arangolint, clickhouselint, ginkgolinter, promlinter,
#   protogetter, zerologlint, unqueryvet; exptostd — при появлении x/exp.
# - Дубль: gomodguard_v2 (0 — депguard no-orm уже запрещает ORM), intrange→включён
#   (modernize не покрывает найденный случай).
```

## Источники

- Каталог линтеров v2: https://golangci-lint.run/docs/linters/ (получено 2026-08-18)
- Канонические дефолты и опции всех настроек: `.golangci.reference.yml` из https://github.com/golangci/golangci-lint (main)
- sloglint-опции: https://github.com/go-simpler/sloglint
- gosec-правила и поведение G101/G115/G120/G124/G404: https://github.com/securego/gosec + эмпирический probe-прогон v2.12.2 (временный файл, удалён)
- Прежние отклонения: комментарии «Раунд 1/2» в `.golangci.yml`; прогоны прошлых дат сравнивались с актуальными счётчиками этого research

## Открытые вопросы для бара №323

1. G124: принять `gosec.excludes` с ADR или пересмотреть ADR 0018 на SameSite=Strict?
2. vapid.go SA1019: ADR-легитимация единственной nolint или тикет на ecdh-переход?
3. Приоритеты и порядок этапа 2 (errcheck 248 / goconst 314 / ST1000 199 / godot 395 / complexity 57+64 / lll 623 / paralleltest 888) — что раньше.
4. Гейт «0 nolint» как CI-скрипт (golangci-lint не умеет запрещать nolint целиком).
