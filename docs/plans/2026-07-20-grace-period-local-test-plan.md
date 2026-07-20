# Локальное E2E-тестирование льготного периода (grace period) подписки

Дата: 2026-07-20. Статус: выполнено, все сценарии пройдены.

## Контекст

- Локальный backend с `PAYMENT_PROVIDER=fake`: детерминированный отказ — карта с токеном `fake_fail_grace` → Charge failed.
- Проверка: локальная БД + логи backend + скриншоты `/profile/tariff` в headless-chromium.

## Механика (верифицировано)

- Grace = 7 дней (`domain/subscription.go:26`). `EnterGrace`: `status=grace`, `valid_until = max(valid_until, now+7d)`.
- Истечение: `ProcessExpiredGrace` (billing worker, тикает при старте backend) → downgrade в `basic`, `valid_until=NULL`, `auto_renew=false` + архивация объектов сверх лимита.

## Сетап

- SQL: подписка pro/active/auto_renew с `valid_until` в прошлом + активная fake-карта с токеном `fake_fail_grace` (токен зашифрован ключом приложения через временный go-хелпер, удалён после теста).
- `active_payment_method_id` обязателен — без него renewal идёт по ветке «grace без платежа».

## Сценарий A — grace → продление → active (ПРОШЁЛ)

- Тик worker'а → Charge `fake_fail_grace` → payment `failed`, подписка `grace`, `valid_until` = +7 дней от тика.
- Фронт: чип «Льготный период», баннер «Подписка закончится 27.07.2026», «Автопродление: Включено». CTA/пояснения про фейл продления нет.
- Продление: `POST /subscription/change` pro→business + confirm (fake) → payment `succeeded`, подписка `active/business`, `valid_until` +1 мес ОТ МОМЕНТА ОПЛАТЫ. При смене тарифа остаток grace сгорает — by design: `ApplyTariffChange` считает от now; `ApplyRenewal` для того же тарифа считал бы от `max(valid_until, now)`.

## Сценарий B — grace → истечение → basic (ПРОШЁЛ)

- Повторный grace (Charge failed), затем `valid_until` в прошлое + рестарт backend → `ProcessExpiredGrace` (лог «downgraded expired grace subscriptions count=1») → `active/basic`, `valid_until=NULL`, `auto_renew=false`.
- 1 объект пользователя (лимит basic=1) не архивирован.
- Фронт: «Активна», базовый тариф, баннеров нет.

## Дополнительно подтверждено

- Ветка «grace без платежа»: нет `active_payment_method_id` → grace без создания платежа.

## Находки

Подтверждены тестом, если не указано иное; фиксы — по отдельной команде.

1. `error_code=NULL` в платеже при синхронном failed-Charge (`renewal_service.go:477` — `providerErrorCode(nil)` при `chargeErr==nil`).
2. Нет пути «продлить тот же тариф» из grace: `ErrAlreadyOnTariff` в API, кнопка текущего тарифа disabled в UI. Реально продлить можно только сменой тарифа.
3. Фронт в grace: нет ни объяснения, что автопродление не прошло, ни CTA «оплатить снова» — только чип и общий баннер; «Автопродление: Включено» вводит в заблуждение.
4. Латентный баг (из код-ресёрча, в тесте не гоняли): запланированный downgrade из grace никогда не применится — `ScheduleDowngrade` разрешает grace (`subscription.go:158`), `ApplyScheduledDowngrade` требует active (`subscription.go:251`).
5. Нет теста «grace → покупка upgrade → active».
6. Поведение: каждый успешный платёж создаёт и активирует новую карту. При тесте «хорошая» карта заменила фейл-карту, и повторный renewal прошёл успешно — учитывать при будущих тестах.

## Чистка

- Тестовые fake-карты удалены, `active_payment_method_id=NULL`.
- Backend возвращён в `make backend-run`, аккаунт в исходном состоянии (basic).
