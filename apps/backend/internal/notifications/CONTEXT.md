# Notifications

Контекст уведомлений: хранимая лента (карта #734, модель — решение #737), каналы доставки (email, Web Push), push-подписки и события подписки. Прямой grace-канал (issue #253) доставляет биллинговые события немедленно; издатели каталога (#740, #748–#752) пишут ленту и ставят доставки. Per-event-type × per-channel настройки (ADR 0030) сняты вместе с таблицей (миграция 000131): категория «Тариф» честно всегда включена, новый контракт настроек — email per-account + push per-device (ADR 0056, #743).

## Language

### Feed

**Категория / Notification Category**:
Группа per-channel настроек и иконка. Шесть значений: Аренда (`rental`), Платежи и операции (`payments_operations`), Задачи (`tasks`), Совместный доступ (`shared_access`) — настраиваемые; Тариф (`tariff`), Системные (`system`) — служебные, всегда включены, вне экрана настроек. Enum `notification_category`.
_Avoid_: тип события, канал

**Тип события / Notification Event Type**:
Конкретное событие ленты; принадлежит категории (`EventType.FeedCategory`). Каталог v1 — 15 типов (решение #737): `rental_completed`; `payment_due`, `payment_overdue`; `task_overdue`; `property_invitation`, `invitation_accepted`, `access_revoked`, `access_paused`, `access_resumed`, `member_left`; `subscription_payment_failed`, `subscription_payment_reminder`, `subscription_payment_succeeded`, `subscription_plan_changed`; `system_maintenance`. Хранится в enum `notification_event_type`.
_Avoid_: категория

Мёртвые значения `notification_event_type` (`free_reminder`, `operation_due`, `operation_overdue`, `lease_expiring`, `lease_requires_action`) и `notification_target_type` (`free`, `operation`, `recurring_operation`, `lease`) остаются в PostgreSQL навсегда (прецедент #277) — домен и контракты о них не знают. Легаси-значение `subscription_grace` обслуживает прямой grace-канал до унификации доставки (#740/#741); в ленте его нет.

**Уведомление / Notification**:
Строка ленты одного получателя: снимок текста (заголовок, тело, лейбл контекста — имя объекта/тарифа или «Системные уведомления»), payload-ссылки, личные флаги (прочитано `read_at`, удалено `deleted_at`). Одно событие = по строке на каждого получателя (fan-out); переписанный шаблон не трогает хранимые строки. Таблица `notifications`; момент события — UTC instant (`created_at`), группировка «Сегодня/Вчера» — дело экрана (#744).
_Avoid_: пуш, письмо, сообщение

**Лента / Notification Feed**:
Хранимый список уведомлений пользователя, читается newest-first keyset-пагинацией (канон #597). Настройки глушат только email/push — строка ленты пишется всегда.
_Avoid_: история, журнал

**Получатель / Recipient**:
Пользователь, которому адресована строка. Объектные события — владелец + все активные участники (включая «Просмотр»); доступ — адресат по событию; Тариф — владелец; Системные — все пользователи. Инициатор не получает уведомление о собственном действии (actor-skip); один пользователь — одна строка, дублирования на устройства/роли нет.

**Действие / Action**:
Кнопка перехода на экран сущности, никогда не мутация. Вычисляется при чтении (#743) по живому состоянию сущности и правам читателя; возможные действия за событием закрепляет каталог (`ActionsForEvent`). Состояние ушло — кнопок нет; сущность удалена — уведомление остаётся без действий; роль «Просмотр» получает уведомления без кнопок-мутаций.
_Avoid_: кнопка-действие (выполняющая мутацию из ленты)

**Дедуп-ключ / Dedup Key**:
Пара (получатель, `dedup_key`), где ключ = (тип события, сущность, фаза/дата); уникальный индекс делает инвариант прочным: повтор с тем же ключом строку не создаёт. Форматы ключей — словарь издателей (#740, #748–#752).

**Счётчик непрочитанных / Unread Counter**:
Число непрочитанных неудалённых строк пользователя. Снижается кликом по уведомлению, открытием страницы и «Прочитать все»; удаление непрочитанного снижает его; пуш и email на читаемость не влияют.

### Channels

**Channel / Канал доставки**:
Транспорт доставки уведомления. Значения: `email`, `push`. Хранится в `notification_channel` enum.
_Avoid_: тип доставки

**PushSubscription / Push-подписка**:
Подписка устройства/браузера на Web Push-уведомления. Содержит endpoint URL (push-сервис), ключи шифрования (`p256dh`, `auth`). Один пользователь = N устройств. Таблица `push_subscriptions`. Endpoint — естественный ключ и секрет (RFC 8030 §8.3).
_Avoid_: устройство, девайс

**PushSender / Отправитель пушей**:
Порт `application.PushSender`: шифрует payload (RFC 8291), подписывает VAPID JWT (RFC 8292), отправляет POST на endpoint. Маппит коды ответа push-сервиса в доменные ошибки: `ErrSubscriptionGone` (404/410), `ErrRateLimited` (429), `ErrPushPayloadTooLarge` (413).

**PushPayload / Payload пуша**:
JSON-тело, отправляемое в service worker: `title`, `body`, `tag` (для замещения/группировки), `url` (навигация по тапу), `eventType`. Ограничение 3993 байта (практический потолок RFC 8030 после aes128gcm overhead).

**VAPID / Voluntary Application Server Identification**:
RFC 8292. P-256 ключ pair для идентификации application server'а перед push-сервисом. Публичный ключ отдаётся фронту через `GET /push/vapid-public-key`; приватный ключ и subject (`mailto:`/`https:` URI) — серверный секрет. Ротация ломает все подписки — не ротировать без миграции.

### Direct notifications

**Прямое уведомление / Direct Notification**:
Сообщение, доставляемое одному пользователю немедленно, вне какого-либо жизненного цикла: без таблицы отправок, без клейма и без повторов. Отправитель публикует событие один раз — `DirectNotificationService` доставляет оба канала один раз. Grace — служебная категория «Тариф», всегда включена (ADR 0056): настройки каналы не глушат. Оба канала best-effort: сбой канала логируется и никогда не валит другой канал и не влияет на переход, вызвавший событие. Текущий отправитель — grace-события billing (issue #253): «вход в grace» доставляется немедленно, «grace истекает» — по расписанию grace-воркера billing.

### Push as best-effort side channel

Push delivery is best-effort: push failures (429, 5xx, no subscription) are logged but never block email delivery. Dead subscriptions (404/410, `ErrSubscriptionGone`) are deleted on the spot.

### Push delivery observability

`webpush.Metrics` exposes a single counter `notifications.push.dispatched` with an `outcome` attribute (`sent`/`gone`/`rate_limited`/`failed`). The cleanup rate of dead subscriptions — the volume of subscriptions the direct-notification service deletes because the push service returned 404/410 (`ErrSubscriptionGone`) — is observable as `notifications.push.dispatched{outcome=gone}` in Uptrace. The metric is recorded by the adapter before the domain error is returned, so every gone outcome is counted even if the DB delete fails. To monitor cleanup on prod, alert on a sustained non-zero `gone` rate (indicates subscription churn — devices uninstalled, ITP purges, OS token expiry).

### Declarative Web Push — decision (iOS)

**Decision: classic service-worker Web Push only for v1; Declarative Web Push (iOS/iPadOS 18.4+) deferred.** All notifications carry a visible notification (`showNotification` mandatory — iOS revokes the subscription on silent push), so the declarative format's main draw — silent/navigate-only messages without a `push` handler — adds nothing the v1 notifications need. The classic SW-push path (`public/sw.js` push-handler, research `docs/research/ios-pwa-push.md` §1.4) works on Android Chrome and iOS 16.4+ installed PWA alike. Declarative Web Push would primarily help if (a) we shipped silent/technical pushes, or (b) ITP purges of SW registrations started killing subscriptions at scale (the declarative format decouples the subscription from the SW). Neither applies to v1. Re-evaluate when a silent-push use case lands.

## ADRs

- ADR 0056 — хранимая лента и per-category настройки: категория × канал вместо типа события × канал; email per-account, push per-device (мастер + категории на подписке); лента пишется всегда; старая таблица дропнута без переноса (миграция 000131). Supersedes ADR 0030 и ADR 0022.
- ADR 0030 — per-channel notification preferences (email/push independent). Superseded by ADR 0056 (таблица `user_notification_channel_preferences` дропнута миграцией 000131, opt-out'ы сброшены — решение #738). Supersedes point 1 of ADR 0022.
- ADR 0022 — per-event-type notification preferences. Superseded by ADR 0056 (модель per event type ушла целиком).
- ADR 0046 — удаление аренд и Операций: reminders-механика удалена, контекст стал grace-only. Supersedes ADR 0029 (rental super-context).
