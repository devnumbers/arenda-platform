# Фикс периода продления подписки (current_period): дизайн и верификация

Дата: 2026-07-21. Статус: реализовано, проверено локальным E2E и регрессионными тестами.

## Контекст

Продолжение `2026-07-20-grace-fixes-design.md`: при E2E-проверке жизненного цикла подписки обнаружен баг в определении периода продления. Этот документ фиксирует: проблему → решение → проверку.

## Баг — период продления и `currentPeriod` выводились из последнего платежа

- Проблема: период автопродления и поле `currentPeriod` в API подписки вычислялись из **последнего succeeded-платежа** (`application/renewal_service.go` — `resolveRenewalTariffAndAmount`, `application/subscription_service.go` — `GetSubscription`). Но запланированный downgrade применяется **без платежа** (ADR 0008 §3): после его применения последний succeeded-платёж по-прежнему ссылается на период старого тарифа, и система продолжала считать период устаревшим (stale period).
- Воспроизведено 2026-07-21: подписка business с запланированным downgrade на pro/year после применения downgrade продлилась как pro/**month** — списание 490 ₽ и +1 месяц вместо 4400 ₽ и +1 года.

## Фикс — `current_period` как состояние подписки

- Решение: период подписки хранится в `user_subscriptions.current_period` (nullable, `'month'|'year'`) — это состояние подписки, а не вывод из истории платежей.
- Миграция `000084_user_subscriptions_current_period`: добавляет колонку с CHECK-ограничением и бэкфиллит её `DISTINCT ON (subscription_id)` по последнему succeeded-платежу (эквивалент старого поведения для существующих данных).
- Домен (`domain/subscription.go`) устанавливает `CurrentPeriod` в `ApplyTariffChange`, `ApplyRenewal`, `ApplyScheduledDowngrade` и очищает в `DowngradeToBasic` (путь grace-expiry/истечения cancelled → бесплатный basic периода не имеет). `ReconstituteSubscription` валидирует значение.
- Чтение: `GetSubscription` (`application/subscription_service.go`) отдаёт `sub.CurrentPeriod` в API-вью, `resolveRenewalTariffAndAmount` (`application/renewal_service.go`) берёт период из подписки; `nil` → дефолт `month`. Запрос последнего платежа из обоих мест убран.
- Замечание по дизайну: `ApplyScheduledDowngrade` при downgrade на платный тариф выставляет `current_period='month'` — это существующий дизайн бесплатного месячного цикла 0 ₽ после применения downgrade; не путать со сбросом в `NULL` в `DowngradeToBasic` на пути истечения grace.

## E2E-верификация (локально, `PAYMENT_PROVIDER=fake`)

Целевой баг:

- business → запланирован downgrade на pro/year → применение → в API `currentPeriod="year"`, на `/profile/tariff` отображается годовая цена (4 400 ₽ /год);
- истечение периода → автопродление: платёж `pro/year 440000 коп succeeded`, `valid_until` +1 год.

Регрессионная матрица (все сценарии пройдены):

- basic → pro/month, продление 49000 коп;
- pro → business/month, продление 99000 коп;
- неуспешное продление → grace (`error_code` сохранён) → продление того же тарифа → `active`, `current_period` сохранён, `valid_until` от момента оплаты;
- истечение grace → basic + автоархивация (аренды `completed`) + `current_period IS NULL`;
- отмена автопродления → истечение → basic;
- запланированный downgrade business→basic → применение (через `ApplyScheduledDowngrade`, `current_period='month'` — см. замечание по дизайну выше).

## Регрессия

- `go test ./internal/...` — полностью зелёные.
- Контракт фронтенда не менялся: `priceDisplay` на `/profile/tariff` уже использует `currentPeriod` из API.

## Деплой

Миграции не запускаются при старте бэкенда: на каждом окружении требуется `make migrate-up` (шаг деплоя).
