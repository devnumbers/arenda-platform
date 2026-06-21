# Повторное код-ревью бэкенда

**Дата:** 2026-06-20  
**Коммит:** `6651cb0fd4fd2d86d7d34ec78664cc98613f1ed0`  
**Ревизуемый отчёт:** `docs/reviews/2026-06-20-backend-code-review.md`  
**Ревьюеры:** 5 агентов по направлениям + верификация orchestrator

---

## Итог

Команда закрыла **все 10 критических** и **подавляющее большинство важных** проблем первого ревью. Ключевые архитектурные улучшения: внешние вызовы провайдеров вынесены за пределы транзакций, read-ручки больше не мутируют состояние, добавлены DB-констрейнты (`idx_leases_one_open_per_property`, `chk_recurring_operations_payment_day`, partial indexes), настраиваемые таймауты и пул БД, шардированный rate limiter.

Однако **исправления в платёжном потоке внесли 3 новые критические ошибки**, связанные с рассогласованием состояния между провайдером и БД. Также остаются неисправленными несколько важных проблем первого отчёта (N+1, утечка `instrumentedRow`, подготовленные запросы, advisory lock на весь tick) и появились новые race-условия в `UpdateRecurringOperation` и `UpdateProperty`.

**Счётчик:**
- Предыдущие критические: 10 FIXED
- Предыдущие важные: ~16 FIXED, ~6 NOT FIXED / PARTIALLY FIXED
- Новые критические: 3
- Новые важные: 6
- Новые minor / остатки: 8+

---

## Базовые проверки

```bash
cd apps/backend
git rev-parse HEAD
# 6651cb0fd4fd2d86d7d34ec78664cc98613f1ed0
git status --short
# clean
go test ./...
# ok
go vet ./...
# ok
cd ../.. && make backend-lint
# 0 issues
```

---

## Статус проблем первого отчёта

### Critical — все закрыты

| # | Проблема | Статус | Где исправлено |
|---|----------|--------|----------------|
| C1 | Внешний вызов провайдера внутри транзакции | FIXED | `internal/billing/application/service.go:836` commit до `provider.Charge` |
| C2 | Read-ручки мутировали статус аренды | FIXED | `internal/leases/application/service.go:226-229` — статус считается в памяти |
| C3 | Read-ручки расширяли horizon recurring-операций | FIXED | Генерация только в Create/Update/Resume/CreateLease |
| C4 | Нет DB-констрейнта «одна открытая аренда на объект» | FIXED | `000027_leases_one_open_per_property.up.sql` + `ErrOpenLeaseExists` |
| C5 | Гонка при проверке лимита объектов по подписке | FIXED | `SELECT FOR UPDATE` в `subscription_limiter.go:33-38` |
| C6 | Архивация объектов не атомарна с изменением подписки | FIXED | `PropertyArchiver` принимает `transaction.Tx`, вызов внутри billing-транзакции |
| C7 | Утечка транзакции при ошибке инициализации провайдера | FIXED | `defer tx.Rollback(ctx)` сразу после `Begin` |
| C8 | Soft-deleted операции блокировали повторное использование дат | FIXED | `AND deleted_at IS NULL` в `ListOperationDates*` |
| C9 | Cancelled-подписки не даунгрейдились на basic | FIXED | `expireNonRenewingSubscription` разрешает `cancelled` |
| C10 | Fake SMS логировал коды вне local/dev | FIXED | Запрет `SMS_SENDER=fake` при `APP_ENV != local/dev` |

### Important — закрыты / остались

| # | Проблема | Статус | Комментарий |
|---|----------|--------|-------------|
| I11 | `WithTx` dead-code branch (`invalidSubscriptionRepository`) | NOT FIXED | Распространилась на другие billing-репозитории |
| I12 | `payment_day` без DB-level CHECK | FIXED | `CHECK (payment_day BETWEEN 1 AND 31)` |
| I13 | Soft-deleted операции без partial indexes | FIXED | `000029_operations_partial_indexes.up.sql` |
| I14 | Hard-delete запросы игнорируют soft-deletion | FIXED | Все DELETE фильтруют `deleted_at IS NULL` |
| I15 | `instrumentedRow` утечка соединения | NOT FIXED | `release()` вызывается только в `Scan` |
| I16 | Lost-update в lease/operation updates | PARTIALLY FIXED | Lease и operation теперь `FOR UPDATE`, но `UpdateRecurringOperation` и `UpdateProperty` — нет |
| I17 | `sqlc` не использует prepared queries | NOT FIXED | `emit_prepared_queries: false` |
| I18 | Нет statement/idle-transaction таймаутов | FIXED | `ConnConfig.RuntimeParams` |
| I19 | Pool tuning hard-coded | FIXED | env-переменные |
| I20 | `idx_leases_open_past_end` не partial | FIXED | Partial index по статусам |
| I21 | `logSuccessfulRequests` hardcoded false | FIXED | env `LOG_SUCCESSFUL_REQUESTS` |
| I22 | Подписка запрашивалась на каждом mutating-запросе | FIXED | Кеширование в контексте |
| I23 | Rate limiter сериализовал все запросы | FIXED | 256 шардов |
| I24 | Workers не ждали shutdown | FIXED | `sync.WaitGroup` |
| I25 | Cleaner делал unbounded deletes | FIXED | Batch size 1000 |
| I26 | Session lookup на каждом запросе | FIXED | Пропуск публичных путей |
| I27 | Billing worker держит advisory lock весь tick | NOT FIXED | `pg_try_advisory_lock` на весь `tick()` |
| I28 | No-op encryptor с пустым ключом | FIXED | Fail-fast при пустом ключе + реальный провайдер |
| I29 | HSTS header отсутствовал | FIXED | `Strict-Transport-Security` |
| I30-I31 | Телефоны и SMS audit в plaintext | DEFERRED | Документировано в ADR-0009 |
| I32 | `APP_ENV` defaults to `local` | FIXED | Required + allow-list |
| I33 | `X-Request-ID` не валидировался | FIXED | Length + regex |
| I34 | `RealIP` слепо доверяет `X-Forwarded-For` | DEFERRED | TODO в коде |
| I35-I36 | 4xx и `internalError` отдавали/логировали raw ошибки | FIXED | `UserFacingDetail` + `sanitizeError` |

---

## Новые находки

### Critical

#### CR-1. `recoverRenewalFailure` может перевести подписку в grace после фактически успешного списания
**Файлы:** `internal/billing/application/service.go:846-854`, `recoverRenewalFailure:959-988`

Если `provider.Charge` возвращает ошибку (например, HTTP timeout), но на стороне провайдера списание фактически прошло, функция recovery:
1. Помечает платёж `failed` только если он ещё `pending`;
2. **Безусловно** переводит подписку в `grace`.

Это приводит к рассогласованию: деньги списаны, а в системе подписка в grace и платёж failed. Возможен повторный списание при следующем цикле.

**Исправление:** в recovery повторно читать платёж под блокировкой; если статус уже `succeeded` (или провайдер подтверждает успех), не переводить подписку в grace. Использовать идемпотентный запрос статуса к провайдеру.

#### CR-2. Rollback успешного списания при ошибке post-charge обработки
**Файлы:** `internal/billing/application/service.go:911-915`, `applySuccessfulRenewal:1048-1051`

`applySuccessfulRenewal` выполняется внутри `resultTx`. Если `ArchiveExcessProperties` или `ApplyTariffChange` упадёт, вся транзакция откатится, включая `MarkSucceeded` и обновление подписки. Но деньги уже списаны провайдером.

**Исправление:** либо вынести архивацию в отдельную компенсирующую операцию после коммита платежа, либо не откатывать успешный платёж при ошибках downstream.

#### CR-3. `RebuildSchedule` удаляет исторические операции
**Файлы:** `internal/leases/application/rent_service.go:140-142`, `db/queries/operations.sql:69-73`

`DeleteUneditedOperationsByLease` удаляет **все** `is_exception = false` операции по lease без ограничения по дате. При изменении `StartDate` аренды уничтожаются исторические записи о прошлых платежах.

**Исправление:** ограничить удаление операциями с `operation_date >= min(старая_start_date, новая_start_date)`; прошлые exception и закрытые периоды сохранять.

---

### Important

#### IM-1. Lost-update в `UpdateRecurringOperation`
**Файлы:** `internal/leases/application/recurring_operation_service.go:211-217, 265`

Сущность загружается вне транзакции, мутируется в памяти, затем обновляется внутри транзакции. Два конкурентных запроса могут молча перезаписать изменения друг друга.

**Исправление:** начинать транзакцию до загрузки и использовать `GetByIDAndOwnerForUpdate`.

#### IM-2. Lost-update в `UpdateProperty`
**Файлы:** `internal/properties/application/service.go:163-218`

Аналогично: загрузка без транзакции и блокировки, затем `Update`.

**Исправление:** обернуть в транзакцию с `SELECT FOR UPDATE`.

#### IM-3. Upgrade flow разбит на несколько транзакций
**Файлы:** `internal/billing/application/service.go:253-337`

После `provider.Init` идут отдельные транзакции для сохранения `provider_payment_id` и payment method. Crash между `Init` и follow-up транзакциями оставляет pending payment без `provider_payment_id`, а у провайдера — сессия/токен, не привязанные к БД.

**Исправление:** сохранять `provider_payment_id` и token в одной транзакции сразу после `Init`; рассмотреть outbox/saga для идемпотентности.

#### IM-4. Логирование сырых ошибок внешних провайдеров
**Файлы:** `internal/billing/application/service.go:850-853`, `internal/platform/scheduler/reminder_worker.go:201`

В production-логи попадают сырые `err.Error()` от платёжного и SMS провайдеров. Ошибки могут содержать PCI-данные (masked PAN, token hints) или PII.

**Исправление:** применять `sanitizeError` (или эквивалент) перед логированием любых ошибок от внешних провайдеров.

#### IM-5. Утечка соединения `instrumentedRow`
**Файлы:** `internal/platform/database/instrumentation.go:265-282`

`release()` вызывается только в `Scan`. Если caller получает `QueryRow` и не вызывает `Scan` (panic, early return, misuse), соединение pgx-pool утекает до таймаута.

**Исправление:** задокументировать контракт и/или добавить fallback release (например, через `runtime.SetFinalizer` или обёртку с `Close`).

#### IM-6. Billing worker удерживает advisory lock на весь tick
**Файлы:** `internal/platform/scheduler/billing_worker.go:70-89`

`pg_try_advisory_lock` захватывается на все 3 подпроцесса (`ProcessScheduledChanges`, `ProcessRenewals`, `ProcessExpiredGrace`). Долгий tick блокирует другие инстансы.

**Исправление:** освобождать lock после захвата батча, либо полагаться на `FOR UPDATE SKIP LOCKED` внутри worker-запросов.

---

### Minor / оставшиеся проблемы

1. **Domain layer импортирует shared utilities** — `internal/leases/domain/lease.go:9`, `internal/identity/domain/phone.go` импортируют `internal/shared/timeutil` / `internal/shared/phone`. Нарушение DDD-границы.
2. **Сгенерированный SQLC остаётся в `internal/generated/postgres`** вместо `internal/platform/generated/postgres`.
3. **Защитные `invalid*Repository`/`invalid*Limiter` в `WithTx`** остались и размножились (`billing/adapters/postgres/*`). В Go это программистская ошибка — достаточно type assertion / panic.
4. **`sqlc.yaml` не использует prepared queries** (`emit_prepared_queries: false`) — недорогой perf-win.
5. **N+1 в `ListLeases`** — `leaseResponse` всё ещё вызывает `tenantContactSvc.GetTenantContact` на каждую аренду (`internal/platform/httpapi/lease_handlers.go:362-387`).
6. **`UpdateStatusByPropertyID` на hand-written SQL** — `internal/leases/adapters/postgres/repository.go:526-533`.
7. **Tariff cache без TTL** — `internal/billing/adapters/postgres/tariff_repository.go:18-25`.
8. **Magic string в `payment_method_repository.go:181`** вместо `pgerrcode.ForeignKeyViolation`.

---

## Рекомендации по приоритетам

### P0 — исправить до продакшена
1. CR-1: не переводить в grace при recovery, если платёж мог успеть пройти.
2. CR-2: не откатывать успешный платёж при ошибках post-charge.
3. CR-3: не удалять исторические операции в `RebuildSchedule`.

### P1 — закрыть в ближайшем спринте
4. IM-1 / IM-2: lost-update в `UpdateRecurringOperation` и `UpdateProperty`.
5. IM-3: атомарность upgrade flow.
6. IM-4: sanitize ошибок провайдеров в логах.
7. IM-5: защита от утечки `instrumentedRow`.

### P2 — технический долг
8. IM-6: гранулярность advisory lock.
9. Minor: prepared queries, N+1, TTL tariff cache, DDD imports, generated location.

---

## Наблюдения

- Паттерн `sanitizeError` в `internal/platform/httpapi/problem.go` — хорошее решение, но он остался только в HTTP-слое. Его стоит вынести в observability-пакет и применять в сервисах и воркерах.
- Платёжный поток (`renewSubscription`, `changeTariffUpgrade`) стал сильно transaction-choreographed. Рассмотрите выделение saga/outbox-helper для повышения auditability и тестируемости.
- Тестовое покрытие новых критических путей недостаточно: нет тестов на recovery после timeout провайдера, на rollback post-charge, на конкурентные update property/recurring operation.
- Отложенные риски (plaintext phones, trusted proxy) корректно документированы в ADR/TODO, поэтому в этом ревью не повторно оспариваются.
