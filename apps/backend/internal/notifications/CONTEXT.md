# Notifications

Каналы доставки уведомлений (email, Web Push), per-channel настройки (ADR 0030) и события подписки. Контекст grace-only (тикет #438, ADR 0046): домен знает единственный тип события — `subscription_grace`; механика напоминаний (reminders) удалена вместе с контекстом аренд.

## Language

### Events

**EventType / Тип события**:
Категория уведомления. Единственное активное значение: `subscription_grace` (grace-события подписки: неудачное списание и истечение льготного периода, issue #253).
_Avoid_: категория

Мёртвые enum-значения типов БД: `notification_event_type` навсегда содержит `free_reminder`, `operation_due`, `operation_overdue`, `lease_expiring`, `lease_requires_action`, а `notification_target_type` — `free`, `operation`, `recurring_operation`, `lease` (значения удалённых напоминаний: свободные — спека #380, арендные — тикет #438; выпиливать их из PostgreSQL нельзя — прецедент #277). Домен и контракты о них не знают; дроп-миграции 000108 и 000114 стёрли данные и схему (таблицы `reminders`, `free_reminders`, `sent_email_reminders`, `sent_push_reminders`), хранимые per-channel строки с мёртвыми event type игнорируются при чтении настроек.

### Channels

**Channel / Канал доставки**:
Транспорт доставки уведомления. Значения: `email`, `push`. Хранится в `notification_channel` enum.
_Avoid_: тип доставки

**NotificationChannelPreference / Per-channel настройка**:
Разрешение пользователю отправлять уведомления данного event type по данному каналу. Opt-out модель: отсутствующая строка = разрешено. Таблица `user_notification_channel_preferences(user_id, event_type, channel, allowed)` (ADR 0030).
_Avoid_: prefs, настройки уведомлений

### Web Push

**PushSubscription / Push-подписка**:
Подписка устройства/браузера на Web Push-уведомления. Содержит endpoint URL (push-сервис), ключи шифрования (`p256dh`, `auth`). Один пользователь = N устройств. Таблица `push_subscriptions`. Endpoint — естественный ключ и секрет (RFC 8030 §8.3).
_Avoid_: устройство, девайс

**PushSender / Отправитель пушей**:
Порт `application.PushSender`: шифрует payload (RFC 8291), подписывает VAPID JWT (RFC 8292), отправляет POST на endpoint. Маппит коды ответа push-сервиса в доменные ошибки: `ErrSubscriptionGone` (404/410), `ErrRateLimited` (429), `ErrPushPayloadTooLarge` (413).

**PushPayload / Payload пуша**:
JSON-тело, отправляемое в service worker: `title`, `body`, `tag` (для замещения/группировки), `url` (навигация по тапу), `eventType`. Ограничение 3993 байта (практический потолок RFC 8030 после aes128gcm overhead).

**VAPID / Voluntary Application Server Identification**:
RFC 8292. P-256 ключ pair для идентификации application server'а перед push-сервисом. Публичный ключ отдаётся фронту через `GET /push/vapid-public-key`; приватный ключ и subject (`mailto:`/`https:` URI) — серверный секрет. Ротация ломает все подписки — не ротировать без миграции.

## Architecture

### Direct notifications

**Прямое уведомление / Direct Notification**:
Сообщение, доставляемое одному пользователю немедленно, вне какого-либо жизненного цикла: без таблицы отправок, без клейма и без повторов. Отправитель публикует событие один раз — `DirectNotificationService` доставляет оба канала один раз. Оба канала уважают per-channel предпочтения пользователя (ADR 0030); оба best-effort: сбой канала логируется и никогда не валит другой канал и не влияет на переход, вызвавший событие. Текущий отправитель — grace-события billing (issue #253): «вход в grace» доставляется немедленно, «grace истекает» — по расписанию grace-воркера billing.

### Push as best-effort side channel

Push delivery is best-effort: push failures (429, 5xx, no subscription) are logged but never block email delivery. Dead subscriptions (404/410, `ErrSubscriptionGone`) are deleted on the spot.

### Push delivery observability

`webpush.Metrics` exposes a single counter `notifications.push.dispatched` with an `outcome` attribute (`sent`/`gone`/`rate_limited`/`failed`). The cleanup rate of dead subscriptions — the volume of subscriptions the direct-notification service deletes because the push service returned 404/410 (`ErrSubscriptionGone`) — is observable as `notifications.push.dispatched{outcome=gone}` in Uptrace. The metric is recorded by the adapter before the domain error is returned, so every gone outcome is counted even if the DB delete fails. To monitor cleanup on prod, alert on a sustained non-zero `gone` rate (indicates subscription churn — devices uninstalled, ITP purges, OS token expiry).

### Declarative Web Push — decision (iOS)

**Decision: classic service-worker Web Push only for v1; Declarative Web Push (iOS/iPadOS 18.4+) deferred.** All notifications carry a visible notification (`showNotification` mandatory — iOS revokes the subscription on silent push), so the declarative format's main draw — silent/navigate-only messages without a `push` handler — adds nothing the v1 notifications need. The classic SW-push path (`public/sw.js` push-handler, research `docs/research/ios-pwa-push.md` §1.4) works on Android Chrome and iOS 16.4+ installed PWA alike. Declarative Web Push would primarily help if (a) we shipped silent/technical pushes, or (b) ITP purges of SW registrations started killing subscriptions at scale (the declarative format decouples the subscription from the SW). Neither applies to v1. Re-evaluate when a silent-push use case lands.

## ADRs

- ADR 0030 — per-channel notification preferences (email/push independent). Supersedes point 1 of ADR 0022; the expand→contract transition is complete (legacy `user_notification_preferences` table dropped in migration `000102`).
- ADR 0022 — per-event-type notification preferences. Points 2–6 still apply, generalised to (event type, channel); point 1 superseded by ADR 0030.
- ADR 0046 — удаление аренд и Операций: reminders-механика удалена, контекст стал grace-only. Supersedes ADR 0029 (rental super-context).
