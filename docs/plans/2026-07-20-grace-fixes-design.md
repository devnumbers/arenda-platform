# Фиксы льготного периода (grace period): дизайн и верификация

Дата: 2026-07-20. Статус: реализовано, проверено локальным E2E и регрессионными тестами.

## Контекст

Продолжение `2026-07-20-grace-period-local-test-plan.md`: по результатам локального E2E-теста grace-периода подтверждены 5 проблем (находки 1–4 + поведение автоархива). Этот документ фиксирует по каждой: проблему → решение → проверку.

## Фикс 1 — продление из grace считается от момента оплаты

- Проблема: `ApplyRenewal` наращивал `valid_until` от `max(valid_until, now)`. В grace `valid_until` — это конец 7-дневного льготного периода, т.е. неоплаченное время: платёж в первый день grace «дарил» пользователю оставшиеся 6 дней сверх оплаченного периода.
- Решение: стекинг (продление от текущего `valid_until`) сохранён только для `status=active`; в grace и после истечения продление стартует от `now` (`domain/subscription.go`, `ApplyRenewal`). Grace — не оплаченное время и не должно поддаваться.
- Проверка: E2E с fake-провайдером — оплата во время grace → `valid_until` = момент оплаты + 1 месяц.

## Фикс 2 — `error_code` сохраняется при неуспешном продлении

- Проблема: при синхронном failed-Charge и при failed-ответе GetState (поллинг статуса) платёж помечался failed с `error_code=NULL` (`renewal_service.go` — `providerErrorCode(nil)` при `chargeErr==nil`).
- Решение: `ChargeResult.ErrorCode` и `PaymentStatusResult.ErrorCode` добавлены в порты (`application/ports.go`); tkassa заполняет из `resp.ErrorCode` (когда статус failed и код != "0"), fake — `defaultErrorCode` (`"fake_error"`). Хелпер `providerResultErrorCode(resultCode, cause)` (`application/helpers.go`) берёт код из результата, а при его отсутствии — из ошибки-причины. Все три точки `renewal_service.go` (sync charge, poll, recovery) используют его.
- Проверка: E2E — неуспешный платёж продления имеет `error_code='fake_error'`.

## Фикс 3 — продление того же тарифа из grace

- Проблема: из grace нельзя было продлить текущий тариф: API отвечал `ErrAlreadyOnTariff`, кнопка текущего тарифа в UI была disabled. Фактический путь «оплатить снова» отсутствовал — только смена тарифа.
- Решение: `ChangeTariff` (`application/subscription_service.go`) разрешает same-tariff при `IsInGrace(now)`: платёж идёт по upgrade-ветке, webhook применяет его через `ApplyRenewal`. Для `active` same-tariff по-прежнему `ErrAlreadyOnTariff`; для истёкшего grace (`status=grace`, но окно вышло) — `ErrInvalidSubscriptionState`.
- Фронт: на `/profile/tariff` в grace кнопка текущего тарифа становится «Продлить» (primary, `TariffChangeForm.tsx`), добавлен warning-баннер «Автопродление не прошло. Продлите подписку — доступ сохранится до …» со ссылкой «Продлить» (`TariffOverview.tsx`).
- Проверка: E2E — продление того же тарифа из grace проходит, подписка `active`, `valid_until` = оплата + 1 мес.

## Фикс 4 — запрет запланированного downgrade из grace

- Проблема (латентный баг): `ScheduleDowngrade` разрешал `status=grace`, а `ApplyScheduledDowngrade` требует `active` — выбор пользователя молча терялся, downgrade никогда бы не применился.
- Решение: `ScheduleDowngrade` (`domain/subscription.go`) принимает только `status=active`; из grace — `ErrInvalidSubscriptionState`. Grace — транзитное состояние ≤ 7 дней, планировать отложенные изменения в него бессмысленно.
- Проверка: E2E — попытка запланировать downgrade в grace → 409.

## Фикс 5 — тарифный автоархив принудительно завершает открытые аренды

- Проблема: `ArchiveExcessProperties` пропускал объекты с открытой арендой — лимит тарифа фактически не соблюдался.
- Решение: новый метод `CompleteOpenLeases` в `leases/adapters/postgres/property_billing_lifecycle.go`: завершает все открытые аренды объекта с теми же побочными эффектами, что у ручного `CompleteLease` (будущие неотредактированные операции удаляются, регулярные операции приостанавливаются, напоминания по аренде и регулярной операции отменяются), + аудит `ActionLeaseCompleted` с `trigger=billing_limit`. В `archivePropertyInTx` добавлен флаг `forceCompleteLeases`: тарифный автоархив (`ArchiveExcessProperties`) вызывает с `true`, ручной `ArchiveProperty` — с `false` и по-прежнему отклоняет занятые объекты `ErrPropertyHasOpenLease`. Возврат из архива остаётся ручным (`UnarchiveProperty`, с проверкой лимита); авто-восстановления при upgrade нет.
- Проверка: E2E — автоархив 2 объектов, один с открытой арендой → объект архивирован, аренда `completed`; повторный upgrade не восстанавливает архив; ручной возврат из архива работает.

## E2E-верификация (локально, `PAYMENT_PROVIDER=fake`)

Пройдены сценарии:

- вход в grace через failed-Charge;
- продление того же тарифа из grace → `active`, `valid_until` = оплата + 1 мес;
- запланированный downgrade в grace → 409;
- автоархив 2 объектов сверх лимита, включая объект с открытой арендой → архив + аренда `completed`;
- re-upgrade не восстанавливает архивные объекты;
- ручной возврат из архива (unarchive) успешен.

## Регрессия

- `go test ./internal/billing/... ./internal/leases/... ./internal/properties/...` — полностью зелёные.
- Фронт: lint + build зелёные.
