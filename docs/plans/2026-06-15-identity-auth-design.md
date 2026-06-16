# Дизайн: регистрация / вход по SMS и базовая подписка

## Контекст

Первый вертикальный срез MVP — аутентификация собственника. После этого среза пользователь может запросить SMS-код, подтвердить номер телефона, получить серверную сессию и увидеть свой профиль. Если аккаунта ещё нет, система создаёт собственника с базовой подпиской автоматически.

## Принятые решения

| Решение | Обоснование |
|---------|-------------|
| Первый срез — только identity | Блокирует все остальные эндпоинты, соответствует основному пути MVP. |
| Cookie-based opaque sessions | ADR 0004, минимум состояния на клиенте, отзыв сессии на сервере. |
| Fake SMS sender с портом | Быстрый старт, точка расширения под реального провайдера. |
| Таблицы `tariffs` и `user_subscriptions` сразу | Честная доменная модель, не ломается при добавлении платных тарифов. |
| Только роль `owner` в первом срезе | Поле `role` оставляем в схеме, но API и сервис создают только собственника. |
| OpenAPI-first | Правило `apps/backend/AGENTS.md`, защищает контракт. |
| 6-значный код, 5 минут | Баланс безопасности и удобства. |
| Ручное тестирование через Postman | Осознанный компромисс скорости в первом срезе. |

## Bounded contexts

### `identity`

Отвечает за пользователя, код подтверждения, попытки ввода и сессию.

**Domain:**
- `Phone` — нормализация и валидация российского номера.
- `SMSCode` — 6 цифр, `expires_at`, флаг `used`.
- `Session` — opaque token, `expires_at`.
- `User` — id, phone, role.
- `LoginAttemptWindow` — лимиты: 1 запрос кода в минуту, 5 попыток ввода за 30 минут.

**Application:**
- `AuthService`:
  - `SendCode(ctx, phone) error`
  - `VerifyCode(ctx, phone, code) (Session, User, error)`

**Ports:**
- `UserRepository`, `SMSCodeRepository`, `AttemptRepository`, `SessionRepository`
- `Sender` — отправка SMS.
- `Clock` — время для тестируемости (но автотесты вне среза).

**Adapters:**
- Postgres-репозитории через `sqlc`.
- `FakeSender` — логирует код и телефон.

### `billing`

Тарифы и подписки.

**Domain:**
- `Tariff` — имя, лимит активных объектов, цены.
- `Subscription` — пользователь, тариф, источник, статус, срок действия.
- `SubscriptionSource` — `paid` (собственник) / `service` (админ, вне среза).

**Application:**
- `BillingService.CreateDefaultSubscriptionForOwner(ctx, userID) error`

**Adapters:**
- Postgres-репозиторий, seed тарифа `Базовый` в миграции.

### `platform`

HTTP-сервер, конфиг, подключение к БД, observability, middleware.

**Компоненты:**
- `config.Config` — env-конфигурация.
- `database` — `pgxpool` и миграции через `golang-migrate`.
- `httpapi` — generated server, маппинг DTO, cookie middleware, request ID.

## API

- `POST /auth/phone/send` — запрос кода.
- `POST /auth/phone/verify` — проверка кода, создание/вход пользователя, установка cookie.
- `POST /auth/logout` — удаление сессии и cookie.
- `GET /me` — текущий пользователь.

## Схема БД

Таблицы первой миграции:
- `users`
- `sms_codes`
- `login_attempts`
- `sessions`
- `tariffs`
- `user_subscriptions`

Seed: тариф `Базовый` (лимит 1, цена 0).

## Что вне среза

- Админ, служебная подписка.
- Реальный SMS-провайдер.
- Платежи, смена тарифа.
- Автотесты.
- Объекты, аренды, операции.

## Риски

- Без автотестов регрессии откладываются. Принимаем для скорости первого среза.
- OpenAPI-first требует дополнительного шага генерации, но защищает контракт.
