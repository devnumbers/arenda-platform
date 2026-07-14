# Дизайн: ссылка «Вернуться к оплате» для pending-платежа

Дата: 2026-07-14. Статус: утверждён пользователем.

## Контекст

Пока платёж подписки в статусе pending, `/profile/tariff` блокирует смену тарифа и показывает баннер «Ожидаем оплату». Если пользователь закрыл вкладку банковской формы, вернуться к оплате было нельзя — приходилось ждать истечения платежа (15 минут, ADR 0017), после чего воркер переводит его в failed.

## Решение

1. **Backend**: отдаём уже сохранённый `PaymentURL` (колонка `subscription_payments.payment_url`, записывается после tkassa Init) наружу:
   - `paymentUrl` (nullable string) добавлен в схему `SubscriptionPayment` в `apps/backend/api/openapi/openapi.yaml`;
   - маппинг — одно поле в `subscriptionPaymentResponse` (`apps/backend/internal/platform/httpapi/subscription_handlers.go:507`).
2. **Frontend**: регенерирован `shared/api/generated.ts`; тип `SubscriptionPayment` (`entities/billing/model/types.ts`) и `mapSubscriptionPaymentResponse` расширены `paymentUrl: string | null`.
3. **UI**: ссылка «Вернуться к оплате» (`target="_blank" rel="noopener noreferrer"`) в трёх местах — баннер `TariffOverview.tsx`, баннер `TariffChangeForm.tsx`, карточка `PaymentDetail.tsx`.

## TTL-правило

Ссылка показывается только пока платёж свежий: возраст < `PAYMENT_STALE_MS` (15 минут, `features/billing/api/hooks`), что совпадает со временем жизни банковской формы (RedirectDueDate = 15 мин, ADR 0017). После этого остаётся текст устаревшего баннера, а воркер переводит платёж в failed.

## Альтернативы

- **Возобновление через идемпотентный `POST /subscription/change`** — отклонено: POST с overview-страницы семантически странный, а возвращается всё равно тот же сохранённый URL.
- **Показывать ссылку всё время, пока pending** — отклонено: через 15 минут банковская форма уже мертва.

## Проверка

- Backend-тесты проходят; frontend lint/build проходят.
- E2E (headless chromium против локального backend с реальным тестовым терминалом tkassa): ссылка присутствует на всех трёх страницах с корректными href/target/rel; ответ API содержит `paymentUrl`.
