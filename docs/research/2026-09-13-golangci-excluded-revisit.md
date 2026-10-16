# Пересмотр «осознанно НЕ включаем»: свежие advisory-замеры на golangci-lint v2.13.2

Research-тикет [#638](https://github.com/devnumbers/arenda-platform/issues/638), карта №636 «Усиление линтеров».
Дата: 2026-09-13. Версия линтера: **v2.13.2** (целевой пин из research #637, `docs/research/2026-09-13-golangci-new-versions.md`).
База сравнения: `docs/research/2026-08-18-golangci-lint-catalog.md` (замеры 18.08 на v2.12.2) и хвост `apps/backend/.golangci.yml`.

**Решение по каждому пункту — за владельцем (grilling-тикет карты).** Ниже — цифры и рекомендация-кандидат.

---

## Метод

- Бинарь: `cd apps/backend && go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2 run …` — модуль скачался штатно, фолбэк на v2.12.2 не понадобился (`--version` подтверждает 2.13.2, go1.26.6).
- **Кэш**: `GOLANGCI_LINT_CACHE=$PWD/../.golangci-lint-cache` в каждом прогоне (глобальный кэш машины отравлен чужими ворктри).
- Конфиг: `.golangci.yml` не менялся. Для каждого прогона — временный конфиг `.golangci-advisory-*.yml` (копия основного + добавление; все прошли `golangci-lint config verify` на схеме v2.13.2); после замеров удалены.
- Команда прогона: `run --config .golangci-advisory-X.yml --build-tags=integration --output.json.path /tmp/advisory-X.json ./...` (в v2 флаг `--out-format` удалён — вывод файлов настраивается через `--output.<format>.path`, см. `golangci-lint run --help` v2.13.2). Счётчики и примеры агрегированы из JSON (`FromLinter`/`Pos`/`Text`); `issues.max-issues-per-linter: 0` и `max-same-issues: 0` унаследованы из основного конфига — счётчики полные.
- Батчи: 17 «просто включаемых» линтеров одним прогоном (у каждого свой счётчик); govet shadow+fieldalignment одним прогоном (раздел по префиксу текста `shadow:` / `fieldalignment:`); nonamedreturns с опцией — отдельно; gocritic-теги — 4 отдельных прогона (`linters.settings.gocritic.enabled-tags`, ключ подтверждён по `.golangci.reference.yml` v2.13.2).
- Режимы настроечных линтеров — по методике 18.08:
  - **exhaustruct_v5** — без `enforce-patterns` (все дефолты): это Implicit Mode, «By default, **all** struct literals are checked» (README go-exhaustruct v5.0.3) — сопоставимо со старым blanket-замером exhaustruct=1705.
  - **gocritic-теги** — `enabled-tags: [<tag>]` при уже включённых `enabled-checks` (те в baseline дают 0, на счётчик не влияют).
  - Пороги funlen/nestif/mnd/interfacebloat/ireturn/tagliatelle — дефолтные (как в trial-конфигах 18.08).
- **Baseline-прогон основного конфига на v2.13.2: 34 находки** — 26 gofumpt, 6 modernize (`errorsastype`), 2 staticcheck (SA1019 на деприкации Go 1.26 в vapid_test.go). Это дрейф v2.13-стека (материал чек-листа апгрейда #637, Блок 5), на измеряемые линтеры не влияет — ни один из них в baseline не участвует.
- Сопоставимость с 18.08: код за 4 недели дрейфовал (после каталога #321 приземлились волны сетки #325 — тикеты #360–#376, включая paralleltest 994→0 и структурную волну), поэтому часть дельт — рост/очистка кода, а не смена поведения линтеров. Версия-зависимые изменения среди измеряемых минимальны (wsl 5.9.0 — один FP-фикс; exhaustruct→exhaustruct_v5 — единственная замена движка, см. research #637).

---

## Основная таблица (полный модуль `./...`, `--build-tags=integration`, v2.13.2)

| Линтер | 18.08 (v2.12.2) | 13.09 (v2.13.2) | прод/тесты | Вердикт-кандидат |
|---|---|---|---|---|
| **gocritic style-тег целиком** | 50* | **8** | 3/5 | **включать** — микроремедиация 8 правок (кандидат №1) |
| **nestif** | 19 | **5** | 4/1 | **включать** — 5 микроремедиаций (кандидат №2) |
| **interfacebloat** | 9 | **6** | 6/0 | **включать с настройкой** `max: 18` → 0 текущих, страховка от вырождения портов (кандидат №3) |
| exhaustruct_v5 (implicit) | 1705 (exhaustruct) | 2103 | 386/1717 | blanket — **оставить отклонённым**; explicit-mode (`//exhaustruct:enforce` / `enforce-patterns`) — отдельное opt-in-решение, не blanket |
| nonamedreturns | 11 | 60 | 39/21 | **оставить отклонённым**; новая опция `allow-unused-named-returns` наш класс не спасает (56, см. ниже) |
| govet shadow | 341 | **251** | 65/186 | оставить отклонённым (идиома `if err :=`; счётчик упал, характер прежний) |
| govet fieldalignment | 269 | 328 | 159/169 | оставить отклонённым (переупаковка против читаемости доменов) |
| wsl_v5 | 5448 | 7302 | 1948/5354 | оставить отклонённым (стиль пустых строк, баг-класса нет; вырос вместе с тестовой базой) |
| nlreturn | 2486 | 2760 | 1814/946 | оставить отклонённым (чистый стиль) |
| noinlineerr | 1426 | 1776 | 585/1191 | оставить отклонённым (прямо запрещает `if err := …; err != nil` — идиому проекта) |
| varnamelen | 1044 | 1504 | 413/1091 | оставить отклонённым (идиомы chi: `w`, `r`, `p`, `h` — 484 срабатывания только по `h`) |
| err113 | 282 | 395 | 207/188 | оставить отклонённым (динамические доменные ошибки — паттерн проекта) |
| testpackage | 143 | 175 | 0/175 | оставить отклонённым (white-box доменные тесты — осознанный выбор) |
| ireturn | 142 | 142 | 59/83 | оставить отклонённым (FP на DDD-портах: 117 из 142 — `WithTx`/`Begin` UoW-паттерна) |
| funcorder | 85 | 120 | 114/6 | оставить отклонённым (порядок exported/unexported — стиль; 29 из 120 — placements sqlc-хелпера `q`) |
| mnd | 131 | **113** | 113/0 | оставить отклонённым (числа-данные: дефолты конфига `cmd/api/main.go`, SQL-лимиты) |
| funlen | 37 | 96 | 20/76 | оставить отклонённым (метрики; в проде — `Wire*`-функции и `run`; cyclop/gocognit уже в гейте) |
| embeddedstructfieldcheck | 49 | 70 | 33/37 | оставить отклонённым (чистый стиль; но из style-группы — второй по дешевизне кандидат, если владелец захочет) |
| gochecknoglobals | 67 | 70 | 32/38 | оставить отклонённым (package-level таблицы/tracer, тестовые фикстуры — данные) |
| tagliatelle | 50 | 51 | 51/0 | оставить отклонённым (внешний контракт: T-Bank PascalCase в `tkassa/client.go`, snake_case в fake/openapi) |
| gocritic performance-тег | 662 | 678 | 418/260 | оставить отклонённым (hugeParam 500 + rangeValCopy 175 = 99%; семантика передачи sqlc-структур/доменов по значению) |
| gocritic opinionated-тег | 48* | 4 | 2/2 | отдельно не нужен — полностью покрывается style-тегом (мульти-теговые чеки, см. раздел gocritic) |
| gocritic experimental-тег | 18* | 8 | 3/5 | отдельно не нужен — идентично style-прогону, покрывается style-тегом |

\* Для style/opinionated/experimental каталог 18.08 чисел не даёт; 50/48/18 — из более раннего (недатированного) раунда, живущего комментарием в `.golangci.yml` (строка «Теги НЕ включать: … дали 50/367/48/18»), измеренного на старом коде до волн сетки #325. Сравнивать их с сегодняшними 8/4/8 напрямую нельзя, но порядок снижения согласуется с дрейфом кода. Для performance каталог 18.08 даёт 662 (hugeParam 381 + rangeValCopy 221) — сегодня 678 (hugeParam 500 + rangeValCopy 175 + stringXbytes 2 + rangeExprCopy 1).

---

## Типичные примеры (файл:строка)

**gocritic style-тег (8, все перечислены)** — 3 прод:
- `internal/identity/application/login_code_service.go:342` — ptrToRefParam: `outErr` лучше не-указателем;
- `internal/notifications/adapters/webpush/sender.go:195` — emptyStringTest: `len(tag) == 0` → `tag == ""`;
- `internal/properties/adapters/http/property_handlers.go:676` — ptrToRefParam: `*map[string]any` → `map[string]any`;
- + 5 в тестах: httpNoBody (`retry_transport_test.go:62`, `removed_routes_test.go:49,70` — `http.NoBody` вместо nil), nestingReduce (`stores_test.go:563`, `push_subscription_handlers_test.go:32`).

**nestif (5)**: `internal/billing/adapters/payment/tkassa/provider.go:722` (complexity 10 — парсинг JSON-ответа T-Bank), `internal/billing/application/workers.go:570` (6), `internal/identity/application/session_service.go:82` (6), `internal/contacts/application/service.go:253` (5), `contract_test.go:148` (5).

**interfacebloat (6)**: `internal/access/application/ports.go:18` (18 методов), `internal/properties/application/ports.go:168` (15), `internal/payments/application/ports.go:72,112` (12+12), `internal/billing/application/provider.go:341` (11) — крупнейший порт за историю замеров сократился с ~31 до 18 методов.

**nonamedreturns (60; в проде 39)**: `internal/access/application/service.go:170` (`created domain.Membership`), `internal/admin/application/service.go:54` (`normLimit int`), `internal/tasks/adapters/postgres/task_store.go:203` (`rows`), `internal/billing/adapters/payment/fake/fake.go:230` (`res`, серия по fake-адаптерам). Прежняя «defer-идиома метрик tkassa» больше не доминирует — характер сместился к именованным результатам сборки (`res`/`rows`/`fields`) и `err`-паттерну в хелперах/фейках.

**exhaustruct_v5 (2103, прод 386)**: `cmd/api/main.go:30` (`slog.HandlerOptions` без AddSource/ReplaceAttr), `cmd/api/main.go:231` (`http.Server`), `cmd/api/wire/billing_test.go:39` (`openapi.MeResponse`), массово `application.Config`/`domain.Property` — тот же характер, что 18.08: частичная инициализация идиоматична.

**govet shadow (251, прод 65)**: `cmd/api/wire/wire.go:140` (`err` shadow line 96), основная масса — легитимные затенения `err` в ветках обработки integration-тестов. **fieldalignment (328, прод 159)**: `cmd/api/wire/billing.go:29` (`Billing` 224 → 144 pointer-bytes) и доменные/фикстурные структуры.

**Остальные** (шум того же характера, что 18.08): wsl_v5 — «missing whitespace above this line» (6060 из 7302); nlreturn — «return with no blank line before»; noinlineerr — «avoid inline error handling using `if err := …`»; varnamelen — `p`/`h`/`w`/`r` в chi-хендлерах; err113 — `fmt.Errorf` с динамическим сообщением (`cmd/api/main.go:300` и далее); testpackage — «package should be `*_test`» (175, все тесты); mnd — «Magic number: 5/10/30/120, in <assign>» (`cmd/api/main.go:234-237` — дефолты конфига); funlen — `WireIdentity` 97>60 строк, `run` 52>40 стейтментов; funcorder — «unexported method "q" … should be placed after the exported method»; gochecknoglobals — `likePatternEscaper`, `integrationBaseTime`; tagliatelle — json(camel) в `billing/adapters/payment/fake/fake.go:641-645` и `tkassa/client.go:56`.

---

## Раздел gocritic-тегов

Прогоны: `linters.settings.gocritic.enabled-tags: [<tag>]` поверх действующего `enabled-checks` (ключ и механика подтверждены по `.golangci.reference.yml` v2.13.2; в golangci v2 теги включаются только через `enabled-tags` — в `enabled-checks` перечисляются имена чеков). Пороги дефолтные (hugeParam sizeThreshold=80, rangeValCopy=128 — исходники go-critic v0.14.4, bundled в v2.13.2).

| Тег | Счётчик | Прод/тесты | Разбивка по чекам |
|---|---|---|---|
| performance | 678 | 418/260 | hugeParam 500, rangeValCopy 175, stringXbytes 2, rangeExprCopy 1 |
| **style** | **8** | 3/5 | httpNoBody 3, nestingReduce 2, ptrToRefParam 2, emptyStringTest 1 |
| opinionated | 4 | 2/2 | nestingReduce 2, ptrToRefParam 2 |
| experimental | 8 | 3/5 | httpNoBody 3, nestingReduce 2, ptrToRefParam 2, emptyStringTest 1 |

Ключевой факт — **мульти-теговость чеков** (проверено по `info.Tags` в исходниках go-critic v0.14.4, `checkers/*_checker.go` и `checkers/rulesdata`):

- `nestingReduce`, `ptrToRefParam` → `[style, opinionated, experimental]`;
- `httpNoBody`, `emptyStringTest` → `[style, experimental]`.

Следствия:

1. **style-тег покрывает всё**, что дают opinionated (4) и experimental (8): отдельное включение этих тегов не добавит ни одной находки сверх style. opinionated/experimental как отдельные кандидаты снимаются с повестки.
2. Уже включённые курируемые чеки style-происхождения (`importShadow` [style, opinionated], `paramTypeCombine`, `unnamedResult`) в baseline дают 0 — включение style-тега с ними не конфликтует.
3. Причина «style-тег целиком — курируемые чеки точечнее» (каталог 18.08) опирается на цену всего тега; при сегодняшних 8 находках (3 прод) цена микроскопическая, а выигрыш — будущие чеки тега бесплатно. Вероятная причина снижения с 50 — чистка кода волнами сетки #325 и/или отличие методики раннего раунда; в обоих случаях текущее число — то, по чему решать.

---

## nonamedreturns и allow-unused-named-returns (новое в v2.13)

Опция `linters.settings.nonamedreturns.allow-unused-named-returns: true` (ключ подтверждён по `.golangci.reference.yml` v2.13.2: «Allow named returns in the signature but report them if referenced in the body or used by a naked return», default false).

- Дефолтный режим (сопоставим с замером 18.08): **60** (39 прод / 21 тесты) против 11 на 18.08 — объём вырос в 5 раз, характер сместился: не «defer-метрики tkassa», а именованные результаты сборки (`res`, `rows`, `fields`, `created`, `normLimit`) и `err`-паттерн в фейках/хелперах.
- С опцией: **56** (36 прод). Гранулярность репорта меняется (репортятся только «используемые» и naked-return случаи, при этом по несколько на функцию), поэтому 60→56 — не «минус 4», а другая метрика; практически весь объём остаётся.
- Вывод: опция снимает только «мёртвые» имена; наш реальный класс (используемые именованные возвраты) не спасает. Причина отклонения актуальна и усилилась.

---

## Методологические заметки

1. Версия бинаря — v2.13.2 (скачана штатно, без сетевых фолбэков); сравнение с 18.08 — v2.12.2. Между ними поведение измеряемых линтеров почти не менялось (research #637): единственные двигатели — wsl 5.9.0 (FP-фикс, шум не снижает), exhaustruct→exhaustruct_v5 (замер выше — уже v5). Дельты счётчиков в основном объясняются кодом, а не версий.
2. В golangci v2 вывод в файл — `--output.json.path` (флаг `--out-format` удалён); подсчёт — по JSON, чтобы не зависеть от парсинга текста.
3. Счётчики с дефолтным `--uniq-by-line` (true): несколько находок на одной строке схлопываются в одну (затрагивает в основном nonamedreturns на сигнатурах с двумя именованными результатами; та же оговорка относится и к замерам 18.08 — методика идентична).
4. Temp-конфиги `.golangci-advisory-*.yml` создавались в `apps/backend` (пути exclusions в конфиге отсчитываются от каталога конфига), после замеров удалены; `.golangci.yml` не менялся; ничего не закоммичено.
5. Baseline-дрейф v2.13.2 (26 gofumpt + 6 modernize + 2 staticcheck SA1019) — вводная для чек-листа апгрейда (#637, Блок 5), сюда не входит.

## Источники

- `.golangci.reference.yml` тега v2.13.2 (локальная копия в модульном кэше): секции `gocritic` (`enabled-tags`, строка 1284), `nonamedreturns` (`allow-unused-named-returns`, строка 2442), `exhaustruct_v5` (`enforce-patterns`/`ignore-patterns`/`optional-patterns`, строка 571).
- Документация v2.13.2: https://golangci-lint.run/docs/linters/ (exhaustruct_v5, nonamedreturns, gocritic), https://golangci-lint.run/usage/configuration/ (output-флаги v2).
- exhaustruct v5 Implicit Mode: README `dev.gaijin.team/go/exhaustruct/v5@v5.0.3` («By default, **all** struct literals are checked»).
- Теги gocritic-чеков: исходники `github.com/go-critic/go-critic@v0.14.4` (`checkers/nestingReduce_checker.go`, `ptrToRefParam_checker.go`, `hugeParam_checker.go`, `checkers/rulesdata/rulesdata.go`).
- Замеры: 8 прогонов 2026-09-13 (baseline + группа A + govet + nonamedreturns + 4 gocritic-тега), сырые JSON — `/tmp/advisory-*.json` (вне репо).
