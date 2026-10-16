# apps/backend/CODING_STANDARDS.md

How Go code in this backend is written and reviewed. Read before implementing or reviewing a backend change; the Standards axis of `/code-review` diffs against this file.

Not duplicated here — single sources of truth elsewhere:

- Tool-enforced invariants (money kopecks, UUIDv7, layer imports, no ORM, no stdlib log): `AGENTS.md` (same directory) + `.golangci.yml` + `make migrations-lint`. If lint catches it, review does not re-report it.
- Domain language: per-context `GLOSSARY.md` (index in `GLOSSARY-MAP.md`). Decisions: `docs/adr/`.
- Logging, tracing, span, and PII conventions: `docs/backend-observability.md` (slog `*Context` variants are enforced by `sloglint`).

## Architecture inside a bounded context

Canonical layout of a context (`internal/<context>/`):

- `domain/` — types and rules in domain language. Pure Go: no I/O, no clocks, no infrastructure (depguard `domain-clean` enforces imports; it cannot enforce that logic actually lives here — see review rubric).
- `application/` — one service per cohesive area, one method per use case. The service owns orchestration, the transaction boundary, and actor/scope threading (see `AGENTS.md`).
- `adapters/http`, `adapters/postgres`, … — transport and persistence. OpenAPI DTOs map to domain/application models here and nowhere else.

Rules that no linter can check:

- **Ports are declared by consumers, not providers.** The application package declares the interface it needs (repository, storage, clock); the adapter implements it. Do not declare an interface next to an implementation "just in case" — grow ports from a real second implementation or test fake.
- **Time arrives through the `clock.Clock` port** (`internal/shared/clock`), injected into services. Domain and application code never call `time.Now()` themselves; they receive the instant. Tests fake the clock (`fakeClock` precedent in identity).
- **Transactions go through `internal/transaction`** — `UoW.Do(ctx, work)` or `runInTx` (ADR 0033). Manual `Begin` in production code is blocked module-wide by forbidigo; the only exempt production path is `internal/platform` (the transaction layer itself), test fixtures are exempt per ADR 0033 («Test fixtures»). Audit records join the business operation's transaction (ADR 0020).
- **Shared kernel before new packages**: `internal/shared/` already carries `actor`, `clock`, `policy`, `pgerr`, `sanitize`, `timeutil`, `tzresolver`. Check these before inventing a parallel helper.

Adding a new bounded context — checklist:

1. `internal/<context>/{domain,application,adapters}` + `GLOSSARY.md` (via `/domain-modeling`).
2. The clean-architecture depguards (`domain-clean`, `application-clean`) key on file globs (`**/internal/**/domain|application/*.go`), so the base denies — `database/sql`, `net/http`, `pgx`, `aws-sdk-go-v2`, `internal/platform`, `platform/config`, `platform/openapi` — cover a new context automatically. The adapters ban is not automatic: both rules run `list-mode: lax` over explicit per-context deny entries, so a new context's domain/application can import `adapters/` (its own or another context's) without lint failing. Add `internal/<context>/adapters` to the deny list of **both** rules in `.golangci.yml`:

   ```yaml
   - pkg: github.com/nambers/arenda-planform/apps/backend/internal/<context>/adapters
     desc: domain must not depend on adapters
   ```

   Under `application-clean` the same `pkg` entry carries `desc: application layer must not depend on adapters`.
3. Decide the transaction story: UoW from the start — a production `Begin` outside `internal/platform` fails `make backend-lint` with no migration-ticket escape hatch (test fixtures are exempt, ADR 0033).

## Errors

- Sentinel errors per context live in `<context>/application/errors.go` (`Err` prefix; `identity/domain/errors.go` is the precedent for rules that are domain invariants).
- Return sentinels **unwrapped** from domain and application code. Wrap with `%w` only when adding information a caller needs (which operation, which id) — never re-wrap a sentinel into an opaque error on a path where the caller must match it.
- Matching happens with `errors.Is` **at the transport edge only** — `writeBillingError` in `billing/adapters/http` and `httpsupport.UserFacingDetail` are the patterns: one place per edge maps application errors to problem details and to the user-facing Russian message. Layer-to-layer `errors.Is` switching in the middle of the stack is a smell.
- Error text is a lowercase, unpunctuated continuation of the context (`staticcheck` ST1005 enforces form; the policy here is content: what failed + the identifying value).
- **Handle an error once.** One error, one handler: log it, map it to a response, or wrap-and-return it — never two on the same path. A returned error is the caller's to handle; logging it locally and returning it too produces double reporting and log spam (`revive unhandled-error` catches the statement-position calls; the "once" discipline above it is review's to check). The sanctioned discards are explicit: `_, _ = fmt.Fprintf(os.Stderr, ...)` in test teardown and `hash.Hash.Write` (documented never-fail) — an intentional discard says so with a comment.

## HTTP and contract conventions

- **PATCH-семантика**: null/отсутствие поля = «не менять»; очистка значения — пустая строка (#593). Nullable-опциональные поля, которые можно выключить, — tri-state: omitted = не менять, null = сбросить, значение = установить (канон endDate) (#824).
- **Тела декодируются строго** (`DisallowUnknownFields`): снос поля контракта — временный shim на переходный период, снимается тикетами клиентов, иначе старые клиенты получают 400 (#1006).
- **Суммы на проводе всегда положительные** (копейки); знак, метка и цвет — UI-конвенция поверх статуса, не поле контракта (#614). Денежные поля — `format: int64`; усекающие `int(...)` конверсии копеек запрещены (#425).
- **Приватность: нет доступа = 404** — неотличимо от несуществующего (правило GitHub для приватных репозиториев); отдельный код только для явно оговорённых состояний (suspended → 403) (#152). Скоуп-авторизация зашивается в SQL-запросы, не фильтруется постфактум (#693).
- **Один доменный инвариант = один 409-код и одна копия текста** во всех контекстах; архивные сущности read-only: мутация → 409, чтение разрешено (#618, #89).
- **Гейты fail-closed**: сбой хранилища подписки/прав — 500, не пропуск запроса (#998). Гейтуемая поверхность = парное изменение: паттерн в `shared/lib/paid-sections.ts` + оверрайды серверных роутов + тест парности.
- **Глобальные ленты и поиск — keyset-курсор** (opaque, стабильный ORDER BY с уникальным тай-брейком id); offset для лент не используется (ADR 0061/0062, #597). Сортировка/фильтрация пагинированных списков — серверная ось в openapi.yaml; клиентская сортировка — только задокументированная деградация (#847).
- **Идемпотентные creation-эндпоинты** принимают опциональный `Idempotency-Key` (бронь ключа до исполнения, TTL 24ч, request_hash); в доках честно: стандарта на заголовок нет, паттерн Stripe (#1121).
- Имена аудит-событий — `<module>.<verb>` по образцу существующих (`auth.phone_changed`), не буква тикета (#721).

## Migrations

- Каждая up-миграция начинается с `SET lock_timeout = '1s'` и `SET statement_timeout` (дефолт 5s; отличное от 5s значение — с комментарием-обоснованием в той же строке); down зеркален; конвенция — с cutoff 000108 (#324; гейт `migration-timeouts` в `make migrations-lint`, реестр: `docs/agents/tooling.md`).
- Новая миграция проходит up→down→up цикл (`TestMigrationsUpDownUpCycle`, #316).
- Destructive-схемные изменения — expand→contract (ADR 0024): авто-откат деплоя БД не откатывает; DROP старой SQL-функции в релизе переименования запрещён — старое имя живёт до contract-релиза с пин-тестом окна (#280, #897).
- Номер миграции сверяется с активными ветками/ворктри до создания; migrations-lint дублей версий не ловит — при коллизии перенумеровывается своя ветка (#1049, #734).
- Мёртвые значения PG-enum не удаляются и не переиспользуются (#728).
- CHECK-констрейнты — для инвариантов данных (неотрицательность, порядок); формат (phone/email/RRULE) валидирует только домен (#225, #277).
- Бизнес-триггеры БД не вводятся, пока писатель один; гварды статусов — в application-слое внутри транзакции с `FOR UPDATE` строки-агрегата (#1048).
- `AT TIME ZONE` в Postgres-запросах не используется: граница суток считается в Go (timeutil-хелперы, пояс владельца) и приходит параметром (#445, #112). «Сегодня» и все календарные вычисления отдаёт сервер (ADR 0053) — клиент TZ-слеп (#528, #586).

## Process lifetime

- **The exit policy: the process exits only in `cmd/`.** `os.Exit`/`log.Fatal*` live in the composition root (startup failures, signal-driven shutdown); everything under `internal/` returns errors upward so deferred cleanup and graceful shutdown stay possible (enforced by forbidigo in `.golangci.yml`). The one adjacent idiom: `TestMain` relies on the Go 1.15+ test wrapper exiting with `m.Run`'s result — use `defer` for teardown, not `os.Exit(code)`.

## Domain constructors and validation

- **Domain constructors validate at creation.** `domain.NewX(...)` either returns a valid aggregate/value or an error — an invalid domain value is unrepresentable. Do not add a `Validate()` method that a caller might forget; when a constructor grows past a field check, decompose it (the `Load` config precedent: per-section loaders), don't grow a branch monster. Mapping of constructor errors to user-facing text still happens at the transport edge only.
- **Ports get static conformance assertions.** Every consumer-declared port gets a compile-time check at the adapter side: `var _ ExpiredDeleter = (*fakeDeleter)(nil)` (see `identity/adapters/scheduler/cleaner_test.go`) — the wiring survives renames without a runtime surprise. One assertion per adapter at the assignment or test site; do not collect them in a central file.

## Concurrency and workers

- **Every goroutine has an owner responsible for its exit.** The owner passes the context that cancels it; "fire-and-forget" goroutines with no cancellation path fail review even when `contextcheck` stays quiet about them. The two long-lived scheduler test binaries (`platform/scheduler`, `identity/adapters/scheduler`) also fail their run on any goroutine that outlives the tests, via `goleak.VerifyTestMain` in `TestMain` (`docs/agents/tooling.md`).
- Periodic work lives in `platform/scheduler` workers; contexts do not hand-roll their own tick loops.
- Prefer ownership and channels over shared memory; a mutex is fine for a cache, not fine around an I/O call (see rubric).
- `make test`'s `-race` integration runs are the backstop, not the design argument.

## Side effects: publication and delivery

- **Публикация — строго после коммита.** Доменные события, кадры realtime и задачи очереди публикуются только после успешного коммита транзакции — при откате публикация структурно недостижима (#284, #829). Джобы несут только идентификаторы; контент читается из закоммиченной строки (#740). Исключение — письма-уведомления о смене контакта в identity (#1258): джоба несёт адресата и момент смены, потому что для смены email адресуемый (старый) адрес исчезает из строки пользователя в момент коммита — после него читать его неоткуда. Публикация best-effort: её падение логируется и не откатывает мутацию; дедуп-ключ и actor-skip (издатель не получает своё событие) — по словарю GLOSSARY.md.
- **Аудит — противоположность публикации**: запись аудита в той же транзакции, что и операция, fail-loud (ADR 0020); контекст содержит имена полей и идентификаторы, PII не пишутся (#89, #156, #694).
- **Одно событие — один канал доставки.** При вводе общего пайплайна (уведомления, письма) прямые пути сносятся — дубль доставки недопустим (#751). Транзакционные письма шлются после коммита, fire-and-forget с логированием; ошибка доставки не ломает операцию (#162).
- **Ретраи провайдеров**: только идемпотентные чтения на 5xx (backoff+jitter); 4xx не ретраются никогда; ретрай мутации — отдельный конфиг-флаг с документированным риском (#423).

## Tests

- Table-driven tests, `t.Parallel()`, testify (`require` for fatal preconditions, `assert` for outcomes; `testifylint` polices usage).
- **Fakes over mocks**: ports get hand-written fakes (`fakeClock`, `mutableClock` in identity); assertion-framework mocks of domain ports are not the pattern here.
- Never `time.Sleep` to synchronize — fake the clock or use a channel.
- Adapters and repositories are covered by integration tests (`-tags=integration`, testcontainers — commands and CI wiring in `AGENTS.md`, Quality Gates).
- Test the behavior through the service; a test that asserts "the mock was called" instead of an outcome tests the mock.

## Review rubric — Go mistakes lint does not catch

Judgement calls for the Standards axis (source: distilled from [100go.co](https://100go.co) filtered against `.golangci.yml`; not violations). Read each as *what it is* → *how to fix*.

- **Ownerless goroutine** — a `go` statement whose exit nobody guarantees (no ctx, no WaitGroup, no done channel). → give it an owner and a cancellation path, or delete it.
- **Sentinel erased** — a sentinel error re-wrapped/converted on a path where the caller matches it with `errors.Is`. → return it as-is, or wrap with `%w` and check the chain still matches.
- **Provider-side interface** — an interface declared beside the only implementation. → move it to the consumer that needs the seam, or drop it.
- **Observable map iteration** — output (response, file, seed) built by ranging a map, order-random. → sort keys explicitly when the output is observable.
- **Mystery buffer size** — `make(chan T, N)` with an unexplained N, or fan-in into a channel nobody bounds. → unbuffered by default; a buffer needs a stated reason (burst/backpressure).
- **Lock across I/O** — a mutex held across a network/DB call. → shrink the critical section to memory, or redesign ownership.
- **`time.After` in a tick loop** — a timer allocated per iteration. → `time.Ticker`, stopped with `defer Stop()`.
- **Boundary without timeout** — an outbound HTTP/provider call on the request's bare ctx. → derive a scoped ctx with timeout at the use-case boundary; cancellation belongs to the owner of the work.
- **Молчаливый фолбэк/нормализация** — некорректный ввод даёт явную доменную ошибку (`ErrInvalidPeriod`), не дефолт; валидная строка хранится и возвращается ровно как прислана (#283, #278). → вернуть ошибку домена, не чинить ввод молча.
- **Вторая копия канона** — повторяющийся маппинг/паттерн (ErrNoRows-маппинг, unique-violation, ILIKE-эскейп, row-mapper) живёт одним хелпером; divergent локальные копии сносятся (#217, #852). → поднять в один канонический хелпер.
- **N+1 в списке** — агрегаты и обогащение списка читаются одним батч-запросом, не per-row gateway-вызовом (#845, #585). → батч-SQL на весь список.
- **Дедлок лечится порядком, не ретраем** — ретрай 40P01 не применяется; первым локом транзакции берётся lock в каноническом порядке (#546). → исправить порядок блокировок.
- **Поиск не по канону** — подстрочный поиск — trgm; FTS добавляется ровно там, где trgm слеп; always-OR предикат (prefix-FTS OR ILIKE-trgm) без mode-переключателей и предсказания интента (#705, #839). → маршрут по форме запроса.
- **Env-ручка без потребности** — доменная константа (каденс тика) живёт в конфиге кода; env — только операционные параметры окружения (#458). → константа в коде, env-переменную убрать.
- **Док-рот** — докстринги-инварианты, GLOSSARY.md, ADR-упоминания, `docs/agents/tooling.md` синхронизируются с фактическим поведением ветки в том же изменении; «инвариантные» формулировки («only X», «единственный источник») лгут первыми (#857, #832). Расхождение доки коду — жёсткая находка, не косметика (#723, #149). → догнать доки тем же коммитом.
