# Обработка возвратов платежей Т-Кассы

## Контекст

Возврат подписок сейчас инициируется в личном кабинете Т-Банка. Т-Касса присылает webhook-уведомление, в котором поле `Status` может принимать значения:

- `REFUNDED` — полный возврат;
- `PARTIAL_REFUNDED` — частичный возврат;
- `REVERSED` — отмена авторизации (двухстадийка);
- `CANCELED` — отмена незавершённого платежа.

Бэкенд на текущий момент не обрабатывает эти статусы: завершённые платежи (`succeeded`/`failed`) игнорируются в `HandleWebhook`, а доменная модель не содержит статусов возврата.

## Цель

При получении webhook'а о возврате:

- корректно отразить статус платежа в БД;
- немедленно отменить подписку и перевести пользователя на базовый тариф;
- поддержать полный и частичный возвраты;
- не добавлять собственный API для инициации возврата (пока достаточно обработки уведомлений).

## Решение

### Доменная модель

Добавляются статусы:

- `PaymentStatusRefunded`
- `PaymentStatusPartialRefunded`

Добавляется поле `RefundedAmountKopecks *int64` в `SubscriptionPayment`.

Добавляется метод:

```go
func (p *SubscriptionPayment) MarkRefunded(amountKopecks int64, now time.Time) error
```

- допустим только из статуса `succeeded`;
- при `amountKopecks == AmountKopecks` переводит в `PaymentStatusRefunded`;
- при меньшей сумме — в `PaymentStatusPartialRefunded` и сохраняет `RefundedAmountKopecks`.

`IsFinalized()` расширяется, чтобы включить новые статусы.

### Адаптер Т-Кассы

`ParseWebhook` распознаёт статусы:

- `REFUNDED` → `domain.PaymentStatusRefunded`;
- `PARTIAL_REFUNDED` → `domain.PaymentStatusPartialRefunded`;
- `REVERSED` / `CANCELED` → обрабатываются как финальные неуспешные состояния (для одностадийки это неактуально, но обработка остаётся для надёжности).

В `WebhookPayload` передаётся сумма возврата из поля `Amount`.

### Сервисный слой

В `HandleWebhook` убирается ранний выход для уже завершённых платежей, если пришёл refund-статус.

Добавляется ветка обработки:

1. `MarkRefunded` платежа.
2. Загрузка подписки и базового тарифа.
3. Перевод подписки на базовый тариф: `TariffID = basic`, `Status = active`, `ValidUntil = nil`, `AutoRenewEnabled = false`, очистка `Pending*`.
4. Вызов `PropertyArchiver.ArchiveExcessProperties` для архивации избыточных объектов.
5. Сохранение подписки.

### База данных

Миграция добавляет столбец:

```sql
ALTER TABLE subscription_payments ADD COLUMN refunded_amount_kopecks BIGINT;
```

Статусы платежей хранятся как строки, дополнительных изменений не требуется.

## Тестирование

- Домен: переход `succeeded → refunded`, `succeeded → partial_refunded`, ошибка при попытке перевести `pending`.
- Сервис: refund-webhook для `succeeded`-платежа проверяет downgrade подписки и архивацию свойств.
- Адаптер: парсинг `PARTIAL_REFUNDED` с суммой.
- Линтер / vet.

## Ручная проверка

1. Оформить подписку, убедиться, что платёж `succeeded`.
2. Сделать возврат в личном кабинете Т-Банка.
3. Дождаться webhook'а `REFUNDED`.
4. Проверить, что `/subscription` возвращает базовый тариф, а `subscription_payments.status = refunded`.
