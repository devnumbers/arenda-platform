# Дизайн: возврат платежей в статусе `pending`

## Проблема

Админский эндпоинт возврата отклонял платежи в статусе `pending` с ошибкой:

```json
{
  "detail": "payment cannot be refunded in its current status",
  "status": 409
}
```

При этом в Т-Кассе операция уже успешна, но webhook не дошёл, и платёж завис локально.

## Официальная документация Т-Кассы

Метод `Cancel` (`POST /v2/Cancel`) отменяет платёж:

- При отмене операции в статусе `NEW` поле `Amount` игнорируется, отмена идёт на полную сумму.
- В ответе может вернуться `REFUNDED`, `PARTIAL_REFUNDED`, `REVERSED`, `PARTIAL_REVERSED`.

`REVERSED` означает отмену до завершения операции (например, для `NEW`).

## Решение

Разрешить возврат/отмену платежей в статусе `pending` через тот же админский эндпоинт.

- Доменный метод `MarkRefunded` теперь принимает переход из `pending`.
- Сервис `RefundPayment` разрешает начальный статус `pending` и `succeeded`.
- В адаптере Т-Кассы для ответа `Cancel` добавлена отдельная маппинг-функция `mapCancelStatus`, которая `REVERSED`/`PARTIAL_REVERSED` приводит к `refunded` / `partial_refunded`.
- Общая `mapStatus` (для webhook/status) не меняется: `REVERSED` по-прежнему маппится в `failed`.
- Отмена pending-платежа записывается как `refunded` (без нового статуса `canceled`).
- Логика даунгрейда подписки на `basic` остаётся прежней.

## Изменённые файлы

- `apps/backend/internal/billing/domain/subscription_payment.go`
- `apps/backend/internal/billing/adapters/postgres/subscription_payment_repository.go`
- `apps/backend/internal/billing/application/service.go`
- `apps/backend/internal/billing/adapters/payment/tkassa/tkassa.go`
- `apps/backend/internal/billing/domain/subscription_payment_test.go`
- `apps/backend/internal/billing/application/service_test.go`
- `apps/backend/internal/platform/httpapi/subscription_handlers_test.go`
- `apps/backend/internal/billing/adapters/payment/tkassa/tkassa_test.go`
