# План: исправление ошибок backend по результатам код-ревью

## Контекст

По результатам код-ревью (`docs/reviews/2026-06-20-backend-code-review.md`) выявлено 10 Critical, 22 Important и 23 Minor issue. Пользователь выбрал исправление **Critical + Important** через subagent-driven development с spec/code review после каждой задачи.

Ключевые риски, которые нужно устранить в первую очередь:

1. Внешний HTTP-вызов провайдера выполняется внутри PostgreSQL-транзакции.
2. GET-эндпоинты мутируют состояние (аренды и регулярные операции).
3. Отсутствуют DB-ограничения на «одна открытая аренда на объект» и лимит активных объектов.
4. `ArchiveExcessProperties` работает в отдельной транзакции от billing-изменений.
5. Soft-deleted операции видны запросам дедупликации дат.
6. Cancelled-подписки не даунгрейдятся на базовый тариф.
7. Security: fake-SMS логирует коды, отсутствует HSTS, plaintext-телефоны, не валидируется `X-Request-ID`, `RealIP` доверяет `X-Forwarded-For`.
8. Observability/performance: не логируются успешные запросы, readonly middleware ходит в БД на каждый write, rate limiter глобально сериализует запросы, не ждутся воркеры при shutdown.

## Подходы к организации работы

### Option A: Sequential foundational-first (рекомендуется)

Выполнять задачи последовательно: сначала транзакции/констрейнты/инварианты (фундамент), потом application layer (write-on-read), потом security, потом observability/performance, потом рефакторинг. После каждой задачи — spec-review и code-review. Parallel execution не используется, чтобы избежать merge-конфликтов.

- **Плюсы:** минимум конфликтов, каждая следующая задача стоит на надёжном фундаменте, проще откатить.
- **Минусы:** суммарное время выше, чем при параллельной работе.

### Option B: Theme-based parallel swarms

Разбить агентов на 3–4 параллельных swarm'а по темам (data-consistency, security, observability/perf, refactoring). Каждый swarm работает над своими файлами.

- **Плюсы:** быстрее по календарному времени.
- **Минусы:** высокий риск конфликтов в общих файлах (`service.go`, `server.go`, `config.go`); сложнее интегрировать review; subagent-driven-development skill прямо запрещает dispatch нескольких implementation subagents in parallel из-за конфликтов.

### Option C: Hybrid — sequential critical + parallel independent

Первые 4–5 фундаментальных задачи выполняются последовательно. Оставшиеся независимые темы (security headers, logging, config) — параллельно.

- **Плюсы:** баланс скорости и безопасности.
- **Минусы:** нужен тщательный анализ зависимостей перед parallel step; сложнее контролировать.

## Рекомендация

**Option A — Sequential foundational-first.** Риски data-consistency и architecture tightly coupled: нельзя безопасно менять `PropertyArchiver` до исправления billing transaction boundaries, нельзя добавлять constraints без учёта write-on-read. Последовательный порядок позволяет после каждой задачи прогонять полный `go test ./...` и `make backend-lint`, что снижает регрессии.

## Порядок задач

### Task 1: Billing transaction boundaries — external provider calls outside tx

**Цель:** внешние HTTP-вызовы провайдера не должны выполняться внутри PostgreSQL-транзакции; добавить `defer rollback` для `markTx`.

**Files:**
- `internal/billing/application/service.go:776–893` (`renewSubscription`)
- `internal/billing/application/service.go:261–355` (`changeTariffUpgrade` — `markTx` leak)

**Steps:**
1. TDD: написать тест, который фиксирует, что `provider.Charge` вызывается после `tx.Commit` (mock beginner + mock provider).
2. Рефакторинг `renewSubscription`:
   - Внутри tx создать pending payment и зафиксировать `tx.Commit`.
   - Вне tx вызвать `provider.Charge`.
   - Открыть новую tx для обновления статуса платежа и подписки.
   - Обработать `Pending` статус: сохранить `provider_payment_id` вне основной tx, если он пришёл синхронно.
3. В `changeTariffUpgrade` добавить `defer markTx.Rollback(ctx)` после `Begin`.
4. Запустить `go test ./internal/billing/...` и `make backend-lint`.
5. Spec-review + code-review.

**Риски:** нужно сохранить атомарность перехода в grace/succeeded; webhook-обработчик уже существует для асинхронных статусов.

---

### Task 2: Property archiver participates in billing transaction

**Цель:** `ArchiveExcessProperties` должен выполняться в той же транзакции, что и изменение подписки.

**Files:**
- `internal/billing/application/ports.go:73`
- `internal/billing/application/service.go:936–1045` (`applySuccessfulRenewal`, `applyFreeRenewalOrDowngrade`, `expireNonRenewingSubscription`)
- `internal/properties/application/service.go:288–317`
- `internal/properties/application/ports.go` (если есть)

**Steps:**
1. Изменить интерфейс `PropertyArchiver` на `ArchiveExcessProperties(ctx context.Context, tx transaction.Tx, ownerID uuid.UUID, limit int) error`.
2. Реализовать `PropertyService.ArchiveExcessProperties` через `s.repo.WithTx(tx)` и `s.billingLifecycle.WithTx(tx)`, без собственного `Begin`.
3. Обновить все вызовы в billing service: передать `tx`.
4. Добавить тест на rollback: billing tx откатывается, свойства остаются незаархивированными.
5. Запустить тесты billing + properties.
6. Spec-review + code-review.

**Риски:** `ArchiveProperty` внутри `ArchiveExcessProperties` сейчас делает много проверок (occupancy, suspend billing); их нужно выполнять в tx-контексте.

---

### Task 3: Cancelled subscriptions downgrade to basic

**Цель:** expired cancelled subscriptions должны даунгрейдиться на базовый тариф.

**Files:**
- `internal/billing/application/service.go:1017–1045` (`expireNonRenewingSubscription`)
- `internal/billing/application/service.go:750–773` (`ProcessRenewals` loop for expired cancelled)

**Steps:**
1. TDD: тест с подпиской в статусе `cancelled`, `valid_until` в прошлом; ожидаем `active` + базовый тариф + `valid_until=nil`.
2. Убрать проверку `sub.Status == Active` в `expireNonRenewingSubscription` для cancelled batch, либо ввести отдельную функцию `expireCancelledSubscription`.
3. Убедиться, что `AutoRenewEnabled=false` для cancelled корректно обрабатывается.
4. Запустить billing tests.
5. Spec-review + code-review.

---

### Task 4: Remove write-on-read from leases

**Цель:** `ListLeases` и `GetLease` не должны писать в БД.

**Files:**
- `internal/leases/application/service.go:201–247`
- `internal/platform/httpapi/lease_handlers.go` (где используется результат)

**Steps:**
1. TDD: тест `ListLeases` не вызывает `leases.Update`.
2. Удалить `recalculateStatus` как write-операцию. Вместо этого:
   - В domain `Lease` добавить метод `EffectiveStatus(now time.Time) LeaseStatus`, который вычисляет статус in-memory.
   - Использовать его в `ListLeases`/`GetLease`.
3. Добавить фоновую задачу/воркер, которая периодически материализует статусы, если это нужно для индексов/запросов. Либо оставить только `UpdateLease` и scheduler как точки материализации.
4. Обновить тесты, которые полагаются на persisted status после GET.
5. Запустить leases tests.
6. Spec-review + code-review.

**Риски:** UI может зависеть от persisted status; нужно проверить, где ещё используется `lease.Status` из БД.

---

### Task 5: Remove write-on-read from recurring operations

**Цель:** `ListRecurringOperationsByProperty` и `GetRecurringOperation` не должны расширять горизонт операций.

**Files:**
- `internal/leases/application/recurring_operation_service.go:170–217`, `:642–644`

**Steps:**
1. TDD: тест, что list/get не вызывают `generateOperations`.
2. Удалить `extendHorizon` из read use cases.
3. Перенести горизонт в `CreateRecurringOperation`, `UpdateRecurringOperation`, `ResumeRecurringOperation`.
4. Исправить логику `extendHorizon`: генерировать до `min(now+12 months, EndDate)`, а не возвращать `nil` рано.
5. Запустить leases tests.
6. Spec-review + code-review.

---

### Task 6: Database constraints and race conditions

**Цель:** БД должна гарантировать «одна открытая аренда на объект» и предотвращать превышение лимита активных объектов; фильтровать soft-deleted операции.

**Files:**
- `db/migrations/000003_leases.up.sql`
- `db/migrations/000006_operation_soft_delete.up.sql`
- `db/migrations/000003_leases.up.sql` (`payment_day` CHECK)
- `db/queries/operations.sql:19`, `:23`, `:39`, `:44`, `:57`, `:66`
- `internal/leases/application/service.go:87–102` (`CreateLease`)
- `internal/properties/application/service.go:91–104`, `:341–354`
- `internal/leases/adapters/postgres/repository.go` (`BulkCreate`, `UpdateStatusByPropertyID`)

**Steps:**
1. Миграция: добавить partial unique index `idx_leases_one_open_per_property`.
2. Миграция: добавить `CHECK (payment_day BETWEEN 1 AND 31)`.
3. Миграция: добавить partial indexes для `operations` с `deleted_at IS NULL`.
4. Обновить sqlc-запросы: добавить `AND deleted_at IS NULL` где нужно; убрать/переименовать hard-delete запросы, которые не учитывают soft-delete.
5. Перегенерировать sqlc-код.
6. В `CreateLease` перенести проверку `HasOpenLease` внутрь транзакции; обрабатывать unique violation.
7. В `CreateProperty`/`UnarchiveProperty` использовать `SELECT FOR UPDATE` на subscription/user row внутри tx.
8. Добавить интеграционные тесты на concurrent `CreateLease` и `CreateProperty`.
9. Запустить все тесты + lint.
10. Spec-review + code-review.

**Риски:** Миграции могут падать на существующих данных, если уже есть дубликаты открытых аренд.

---

### Task 7: Security fixes

**Цель:** устранить critical и important security issues.

**Files:**
- `cmd/api/main.go:139–143` (fake SMS env guard)
- `internal/identity/adapters/sms/fake.go:21` (logging)
- `internal/platform/httpapi/server.go:46–54` (HSTS)
- `internal/platform/httpapi/server.go:61` (RealIP / trusted proxy)
- `internal/platform/httpapi/request_id.go:33–41` (validation)
- `internal/platform/httpapi/session.go:120–125` (clear stale cookie)
- `internal/platform/httpapi/session.go:52` (MaxAge truncation)
- `internal/platform/encryption/aes.go:22–25` + `cmd/api/main.go:91–93` (no-op encryptor guard)
- `internal/platform/config/config.go:56–58` (APP_ENV default)
- `internal/platform/httpapi/problem.go:24–28` + handlers (error message leakage)
- `db/migrations/000001_init_schema.up.sql` (phone encryption — optional, important)
- `internal/notifications/adapters/postgres/repository.go:440–449` (SMS audit plaintext — optional)

**Steps:**
1. TDD для каждого пункта (где применимо):
   - fake SMS rejected in staging.
   - request ID invalid → generated.
   - stale cookie cleared.
2. Реализовать исправления.
3. Добавить HSTS header при `CookieSecure=true`.
4. Валидация `X-Request-ID`: UUID или base58, max length 64.
5. Очистка stale cookie при expired/not found.
6. Guard: no-op encryptor + real provider → panic/fatal.
7. Default `APP_ENV` to `production` или fail-fast.
8. Запустить httpapi + identity tests.
9. Spec-review + code-review.

**Риски:** Шифрование телефонов — ломающее изменение для existing data; может быть вынесено в отдельную задачу.

---

### Task 8: Observability and performance fixes

**Цель:** исправить логирование, кэширование subscription status, rate limiter, graceful shutdown, cleaner batching, session lookup.

**Files:**
- `internal/platform/httpapi/server.go:42–43` (`logSuccessfulRequests`)
- `internal/platform/httpapi/readonly_middleware.go:69–78`, `:96` (subscription caching, clock injection)
- `internal/platform/httpapi/ratelimit.go` (mutex sharding)
- `internal/platform/httpapi/session.go:95–127` (path-aware session lookup)
- `cmd/api/main.go:226–230` (worker shutdown)
- `internal/platform/cleaner/cleaner.go:48–61` (batching, clock)
- `internal/platform/config/config.go:108–113` (pool tuning env vars)
- `internal/platform/database/database.go:38–63` (statement_timeout, idle_in_transaction_session_timeout)

**Steps:**
1. Сделать `logSuccessfulRequests` env-configurable (`LOG_SUCCESSFUL_REQUESTS`), default `true`.
2. Добавить `SubscriptionStatus` / `CanMutateData` в request context после `GetMe`; `readonly` middleware читает из context.
3. Внедрить `clock.Clock` в `readonly` middleware.
4. Shard rate limiter по хешу ключа (или `sync.Map`).
5. Сделать session middleware path-aware: пропускать `/auth`, `/webhooks`, `/internal/perf/*`.
6. Graceful shutdown для workers через `sync.WaitGroup` / errgroup.
7. Batched cleaner: `DELETE ... LIMIT 1000` в цикле.
8. Добавить env vars для pool tuning и PostgreSQL timeouts.
9. Запустить platform tests.
10. Spec-review + code-review.

---

### Task 9: Go idioms and perf tooling refactoring

**Цель:** устранить important Go-level issues и улучшить maintainability perf-кода.

**Files:**
- `cmd/perfvegeta/main.go` (split into packages)
- `cmd/perfseed/seed.go` (relative path, TRUNCATE RESTART IDENTITY)
- `internal/platform/database/postgres/transaction.go:18` (`NewBeginner` variadic logger)
- `internal/leases/adapters/postgres/repository.go:587–606` (`BulkCreate` with `pgx.CopyFrom`)
- `internal/billing/adapters/postgres/subscription_repository.go:33–93` (`WithTx` dead code)
- `internal/billing/adapters/postgres/subscription_repository.go:292–311` (pgconv)
- `internal/billing/adapters/postgres/payment_method_repository.go:90–91` (pgerrcode)
- `internal/platform/httpapi/property_handlers.go:208` (`ptrString`)
- `cmd/api/main.go:82` (shadowing logger)
- `internal/platform/httpapi/context.go` (context keys sentinel types)

**Steps:**
1. Разбить `cmd/perfvegeta/main.go` на модули: `cmd`, `vegeta`, `report`, `diagnostics`, `fixtures`.
2. Исправить shadowing `errors` в perfvegeta.
3. `NewBeginner` → один `*slog.Logger`.
4. `BulkCreate` → `pgx.CopyFrom`.
5. Упростить `WithTx` в subscription repository.
6. Заменить magic string `"23505"` на `pgerrcode.UniqueViolation`.
7. Использовать `pgconv.UUIDFromPgtype` в subscription repository.
8. Исправить `ptrString`, shadowing `logger`, context keys.
9. Запустить все тесты.
10. Spec-review + code-review.

---

### Task 10: Minor fixes and final cleanup

**Цель:** устранить оставшиеся minor issues, не вошедшие в предыдущие задачи.

**Files:**
- `internal/platform/cleaner/cleaner.go:49` (clock injection)
- `internal/platform/httpapi/auth_handlers.go:57–66` (log level)
- `internal/platform/httpapi/problem.go:40` (encoder error handling)
- `internal/platform/httpapi/logging.go:110–113` (unmatched route label)
- `internal/billing/application/service.go:76` (`ListTariffs` ordering in SQL)
- `internal/leases/application/ports.go:42–43` (`GetByLease` vs `GetByLeaseID`)
- `internal/notifications/application/ports.go:25–27` (`MarkSent` vs `MarkReminderSent`)
- `internal/notifications/adapters/postgres/repository.go:462–467` (duplicate error constraint check)
- `internal/identity/application/service.go:73` (redundant rollback)
- `db/migrations/000010_leases_past_end_index.up.sql` (partial index)
- `apps/backend/AGENTS.md:28` (bounded contexts list)

**Steps:**
1. Пробежаться по minor issues, сгруппированным по файлам.
2. Для каждого: либо TDD-тест, либо минимальное исправление.
3. Обновить `AGENTS.md` список bounded contexts.
4. Запустить полный `go test ./...`, `go vet ./...`, `make backend-lint`.
5. Spec-review + code-review.

---

## Verification after each task

После каждой задачи обязательно:

```bash
cd apps/backend
go test ./...
go vet ./...
make backend-lint
```

Если какой-либо из шагов падает — исправлять до получения ✅ от code reviewer.

## Review workflow per task

Для каждой задачи:

1. **Implementer subagent** (coder):
   - Получает полный текст задачи + контекст.
   - Задаёт уточняющие вопросы (если нужно).
   - Пишет тесты first, затем код.
   - Самостоятельно запускает `go test`, `go vet`, lint.
   - Делает commit.

2. **Spec reviewer subagent** (coder):
   - Проверяет соответствие задаче: все ли требования выполнены, ничего лишнего.
   - Возвращает ✅ или список недочётов.

3. **Code quality reviewer subagent** (coder):
   - Проверяет Go idioms, Clean Architecture, DDD layers, PostgreSQL practices.
   - Возвращает ✅ или замечания Critical/Important/Minor.

4. **Orchestrator** (я):
   - Если review не ✅, возвращает implementer'у на доработку.
   - После ✅ переходит к следующей задаче.

## Final integration

После Task 10:

1. Полный regression test:
   ```bash
   cd apps/backend
   go test ./...
   go vet ./...
   make backend-lint
   ```
2. Final code review subagent для всего diff.
3. Обновить `CHANGELOG.md` под текущую дату.
4. Обновить `docs/reviews/2026-06-20-backend-code-review.md`: отметить исправленные issue.

## Required skills during implementation

- `$go` — для всех Go-задач.
- `$postgresql-best-practices` — для миграций, sqlc, индексов, транзакций.
- `$test-driven-development` — каждая задача начинается с падающего теста.
- `$requesting-code-review` — после каждой задачи.
- `$verification-before-completion` — перед каждым переходом к следующей задаче.
- `$subagent-driven-development` — основной workflow исполнения.

## Execution choice

После approval плана два варианта:

1. **Subagent-driven in this session** — последовательно dispatch'у implementer/reviewer subagents, остаёмся в текущей сессии.
2. **Parallel executing-plans session** — переносим план в отдельную сессию с `superpowers:executing-plans` для батчевого выполнения.

Рекомендуется **Option 1** из-за tightly coupled задач и необходимости review после каждого шага.