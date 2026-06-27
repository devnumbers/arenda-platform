# Дизайн: админский эндпоинт списка платежей

## Цель

Дать администратору возможность просматривать все подписочные платежи, чтобы находить конкретный платёж для ручного возврата.

## Эндпоинт

```text
GET /admin/subscription/payments?status={status}&limit={limit}&offset={offset}
```

- Доступен только пользователям с ролью `admin` через `AdminOnlyMiddleware`.
- Query-параметры:
  - `status` — опциональный фильтр по статусу платежа: `pending`, `succeeded`, `failed`, `refunded`, `partial_refunded`.
  - `limit` — количество записей на странице, по умолчанию `20`, максимум `100`.
  - `offset` — смещение, по умолчанию `0`.
- Сортировка по `created_at DESC` (новые платежи первыми).

## Ответ

```json
{
  "items": [
    {
      "id": "...",
      "tariff": { ... },
      "period": "year",
      "amountKopecks": 100000,
      "status": "succeeded",
      "provider": "tkassa",
      "userId": "...",
      "userPhone": "+79150380663",
      "createdAt": "2026-06-25T..."
    }
  ],
  "total": 42
}
```

## Слои

- **HTTP:** новый метод `ListAdminSubscriptionPayments` в `SubscriptionHandlers` (`apps/backend/internal/platform/httpapi/subscription_handlers.go`).
- **Application:** новый метод `ListAllPayments` в `BillingService` (`apps/backend/internal/billing/application/service.go`).
- **Repository:** новый метод `ListAll` в `SubscriptionPaymentRepository` (`apps/backend/internal/billing/adapters/postgres/subscription_payment_repository.go`).
- **SQL:** две sqlc-запроси в `apps/backend/db/queries/billing.sql`:
  - `ListSubscriptionPaymentsAdmin` — список с `JOIN users` для получения телефона.
  - `CountSubscriptionPaymentsAdmin` — общее количество с фильтром по статусу.
- **OpenAPI:** добавить `GET /admin/subscription/payments`, схемы `AdminSubscriptionPayment` и `AdminSubscriptionPaymentsResponse`.

## Ограничения первой версии

- Пагинация `limit`/`offset`.
- Фильтр только по `status`.
- В ответе добавляются только `userId` и `userPhone`.
