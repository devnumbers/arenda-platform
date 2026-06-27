# Синхронизация зависших pending-платежей с Т-Кассой

## Контекст

Webhook от Т-Кассы о финальном статусе платежа иногда не доходит. В результате платёж остаётся в статусе `pending`, хотя в Т-Кассе операция уже завершена — успешно или с ошибкой.

## Цель

Автоматически и вручную проверять статус таких платежей через метод `GetState` Т-Кассы (`POST /v2/GetState`) и приводить локальное состояние в соответствие с реальным.

## Решение

### Репозиторий

Добавлен запрос `ListPendingPayments`:

```sql
-- name: ListPendingPayments :many
SELECT *
FROM subscription_payments
WHERE status = 'pending'
  AND created_at < $1
  AND provider_payment_id IS NOT NULL
  AND provider_payment_id <> ''
ORDER BY created_at ASC
LIMIT $2;
```

Платежи без `provider_payment_id` исключаются, потому что `GetState` требует этот идентификатор.

### Сервисный слой

Добавлены методы `BillingService`:

```go
func (s *BillingService) SyncPendingPayment(ctx context.Context, paymentID uuid.UUID) error
func (s *BillingService) ReconcilePendingPayments(ctx context.Context, now time.Time) (int, error)
```

`SyncPendingPayment`:

1. Загружает платёж и проверяет, что он в статусе `pending` и имеет `provider_payment_id`.
2. Вызывает `provider.Status` (обёртка над `GetState`).
3. В короткой транзакции повторно читает платёж с блокировкой и применяет результат:
   - `succeeded` — формирует синтетический `WebhookPayload` без карточных данных, вызывает `applyPaymentResult`, фиксирует транзакцию, затем `applySubscriptionRenewalAndArchive`.
   - `failed` — вызывает `applyPaymentResult` с `failed`; если платёж относится к текущему тарифу подписки, переводит подписку в grace.
   - `refunded` / `partial_refunded` — вызывает `applyPaymentResult` и `applyRefundToSubscription`, чтобы отметить возврат и перевести подписку на базовый тариф.
   - `pending` — no-op.

Уже финализированные платежи возвращают `ErrInvalidPaymentStatus`, чтобы админский endpoint не маскировал повторные вызовы под `204`.

`ReconcilePendingPayments` выбирает платежи старше `pendingPaymentStalenessThreshold` (5 минут) пачками по `renewalBatchSize` (100) и вызывает для каждого `SyncPendingPayment`. Ошибки логируются, обработка продолжается. Возвращает количество успешно синхронизированных платежей.

### Worker

Добавлен `PaymentReconciliationWorker` в `internal/platform/scheduler`:

- интервал 5 минут (конфигурируется переменной `PAYMENT_RECONCILIATION_WORKER_INTERVAL`);
- использует PostgreSQL advisory lock с уникальным ключом, чтобы одновременно работал только один инстанс;
- логирует количество обработанных платежей и ошибки;
- корректно останавливается при отмене контекста;
- подключён в `cmd/api/main.go`.

### HTTP

Добавлен админский endpoint:

```
POST /admin/subscription/payments/{paymentId}/sync
```

- доступен только пользователям с ролью `admin` (`AdminOnlyMiddleware`);
- вызывает `BillingService.SyncPendingPayment`;
- возвращает `204` при успешной синхронизации;
- возвращает `404`, если платёж не найден;
- возвращает `409`, если платёж не в статусе `pending` или у него нет `provider_payment_id`.

Также добавлены `/admin` и `/subscription/cancel` в exempt-список `readonlyMiddleware`, чтобы админы и пользователи могли совершать эти операции даже при заблокированной подписке.

### Ручная проверка

1. Создать платёж, дождаться его появления в `pending`.
2. Не дожидаясь webhook'а, вызвать `POST /admin/subscription/payments/{paymentId}/sync`.
3. Убедиться, что статус платежа и подписки обновились в соответствии с реальным состоянием в Т-Кассе.

## Ограничения

- `GetState` не возвращает данные карты. Если платёж был совершён с новой карты и webhook пропал, синхронизация отметит платёж успешным и активирует подписку, но не создаст `payment_method`. Для сохранения автопродления всё равно понадобится webhook или ручная привязка карты.
- `GetState` не возвращает сумму возврата. Если платёж был частично возвращён и webhook пропал, синхронизация зафиксирует возврат как полный и переведёт подписку на базовый тариф.
