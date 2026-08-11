# Notifications

Напоминания о важных датах арендной недвижимости и их доставка по каналам (email, Web Push). Контекст живёт внутри супер-контекста Rental (ADR 0029) — напоминания привязаны к объектам, операциям и арендам.

## Language

### Reminder

**Reminder / Напоминание**:
Конкретное запланированное уведомление о дате (предстоящая операция, просрочка, окончание аренды, свободное напоминание). Lifecycle: `pending → sending → sent` (или `skipped`/`cancelled`/`failed`). Хранится в `reminders`.
_Avoid_: уведомление (notification — это канал-агностичное сообщение, reminder — конкретный экземпляр)

**EventType / Тип события**:
Категория напоминания. Значения: `operation_due` (предстоящая операция), `operation_overdue` (просроченная операция), `lease_expiring` (аренда заканчивается), `lease_requires_action` (аренда требует действия), `free_reminder` (свободное напоминание).
_Avoid_: категория

**TargetType / Тип цели**:
Сущность, к которой привязано напоминание: `operation`, `recurring_operation`, `lease`, `free`. CHECK exactly-one-target в таблице `reminders`.

**ReminderStatus / Статус напоминания**:
- `pending` — запланировано, ждёт времени отправки.
- `sending` — клеймлено воркером, в обработке.
- `sent` — доставлено (email и/или push).
- `skipped` — терминальный: получатель отозвал разрешение для event type, отправки не было.
- `cancelled` — отменено (цель удалена или заменена).
- `failed` — терминальный после исчерпания попыток.

**FreeReminder / Свободное напоминание**:
Пользовательский шаблон напоминания, не привязанный к операции или аренде. Может быть разовым (`once`) или периодическим (`daily`/`weekly`/`monthly`/`yearly`). Хранится в `free_reminders`; конкретные экземпляры материализуются в `reminders` с `target_type = 'free'`.
_Avoid_: пользовательское напоминание

### Channels

**Channel / Канал доставки**:
Транспорт доставки напоминания. Значения: `email`, `push`. Хранится в `notification_channel` enum.
_Avoid_: тип доставки

**NotificationChannelPreference / Per-channel настройка**:
Разрешение пользователю отправлять напоминания данного event type по данному каналу. Opt-out модель: отсутствующая строка = разрешено. Таблица `user_notification_channel_preferences(user_id, event_type, channel, allowed)` (ADR 0030).
_Avoid_: prefs, настройки уведомлений

### Web Push

**PushSubscription / Push-подписка**:
Подписка устройства/браузера на Web Push-уведомления. Содержит endpoint URL (push-сервис), ключи шифрования (`p256dh`, `auth`). Один пользователь = N устройств. Таблица `push_subscriptions`. Endpoint — естественный ключ и секрет (RFC 8030 §8.3).
_Avoid_: устройство, девайс

**PushSender / Отправитель пушей**:
Порт `application.PushSender`: шифрует payload (RFC 8291), подписывает VAPID JWT (RFC 8292), отправляет POST на endpoint. Маппит коды ответа push-сервиса в доменные ошибки: `ErrSubscriptionGone` (404/410), `ErrRateLimited` (429), `ErrPushPayloadTooLarge` (413). Реализация адаптера — отдельный тикет (своя реализация на stdlib).

**PushPayload / Payload пуша**:
JSON-тело, отправляемое в service worker: `title`, `body`, `tag` (для замещения/группировки), `url` (навигация по тапу), `eventType`. Ограничение 3993 байта (практический потолок RFC 8030 после aes128gcm overhead).

**VAPID / Voluntary Application Server Identification**:
RFC 8292. P-256 ключ pair для идентификации application server'а перед push-сервисом. Публичный ключ отдаётся фронту через `GET /push/vapid-public-key`; приватный ключ и subject (`mailto:`/`https:` URI) — серверный секрет. Ротация ломает все подписки — не ротировать без миграции.

**sent_push_reminders / Audit-таблица пушей**:
Запись об успешно отправленном пуше per (reminder, recipient). Дедупликация: при повторной dispatch проверка `IsPushReminderSent` предотвращает дубль. Симметрична `sent_email_reminders`.

## Architecture

### Multi-channel dispatch (single worker)

Reminder delivery uses a **single worker, multi-channel dispatch** pattern: the `ReminderWorker` claims a reminder (`MarkReminderSending`, `FOR UPDATE SKIP LOCKED`) and within its per-recipient fan-out loop dispatches both email and push independently. One lifecycle, one claim, one finalize decision. Per-channel audit tables (`sent_email_reminders`, `sent_push_reminders`) provide per-recipient deduplication so retries do not duplicate either channel.

This is a deliberate departure from spec #178's original «parallel sender» design (two independent workers, two fan-out points). The single-worker approach was chosen because it eliminates claim races and keeps the lifecycle coherent — the two-system alternative required a separate `push_deliveries` claim table and its own retry semantics, which was overengineering for a polling-DB single-instance deployment.

### Fan-out

A property reminder is delivered to the owner and to every active (non-suspended) member of the property (`MemberRecipientAdapter.ListActiveRecipientIDs`). Each recipient is checked against their own per-channel preferences independently (`IsChannelAllowed(type, ChannelEmail)`, `IsChannelAllowed(type, ChannelPush)`).

### Push as best-effort side channel

Push delivery is best-effort: push failures (429, 5xx, no subscription) are logged but never block email delivery or the reminder lifecycle. When push succeeds for at least one recipient but email does not, the reminder is finalized as `sent` (push deduplication via `sent_push_reminders` prevents redelivery). The authoritative finalize decision is driven by email outcome; push is additive.

### Push delivery observability

`webpush.Metrics` exposes a single counter `notifications.push.dispatched` with an `outcome` attribute (`sent`/`gone`/`rate_limited`/`failed`). The cleanup rate of dead subscriptions — the volume of subscriptions the worker deletes because the push service returned 404/410 (`ErrSubscriptionGone`) — is observable as `notifications.push.dispatched{outcome=gone}` in Uptrace. The deletion itself happens in `ReminderWorker.dispatchPush` on the `ErrSubscriptionGone` branch; the metric is recorded by the adapter before the domain error is returned, so every gone outcome is counted even if the DB delete fails. To monitor cleanup on prod, alert on a sustained non-zero `gone` rate (indicates subscription churn — devices uninstalled, ITP purges, OS token expiry).

### Declarative Web Push — decision (iOS)

**Decision: classic service-worker Web Push only for v1; Declarative Web Push (iOS/iPadOS 18.4+) deferred.** All reminders carry a visible notification (`showNotification` mandatory — iOS revokes the subscription on silent push), so the declarative format's main draw — silent/navigate-only messages without a `push` handler — adds nothing the v1 reminders need. The classic SW-push path (`public/sw.js` push-handler, research `docs/research/ios-pwa-push.md` §1.4) works on Android Chrome and iOS 16.4+ installed PWA alike. Declarative Web Push would primarily help if (a) we shipped silent/technical pushes, or (b) ITP purges of SW registrations started killing subscriptions at scale (the declarative format decouples the subscription from the SW). Neither applies to v1. Re-evaluate when a silent-push use case lands.

## ADRs

- ADR 0030 — per-channel notification preferences (email/push independent). Supersedes point 1 of ADR 0022; the expand→contract transition is complete (legacy `user_notification_preferences` table dropped in migration `000102`).
- ADR 0022 — per-event-type notification preferences. Points 2–6 still apply, generalised to (event type, channel); point 1 superseded by ADR 0030.
- ADR 0029 — rental super-context (notifications + properties + leases tightly coupled).
