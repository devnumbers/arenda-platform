# Notifications

Контекст уведомлений: хранимая лента (карта #734, модель — решение #737), каналы доставки (email, Web Push), push-подписки и события подписки. Доставка по каналам идёт через очередь River и dispatch-пайплайн (#740, ADR 0057): издатели пишут ленту и ставят джобы через `Publisher` — grace (#741) и каталог (#748–#752). Чтение ленты и настройки — REST (#743): keyset-страница, счётчик непрочитанных, прочтение/удаление, страница уведомления с живыми действиями; матрица настроек — email per-account, push per-device (ADR 0056, решение #738, миграция 000133). Per-event-type × per-channel настройки (ADR 0030) сняты вместе с таблицей (миграция 000131): категория «Тариф» честно всегда включена.

## Language

### Feed

**Категория / Notification Category**:
Группа per-channel настроек и иконка. Шесть значений: Аренда (`rental`), Платежи и операции (`payments_operations`), Задачи (`tasks`), Совместный доступ (`shared_access`) — настраиваемые; Тариф (`tariff`), Системные (`system`) — служебные, всегда включены, вне экрана настроек. Enum `notification_category`.
_Avoid_: тип события, канал

**Тип события / Notification Event Type**:
Конкретное событие ленты; принадлежит категории (`EventType.FeedCategory`). Каталог — 17 типов: 15 из решения #737 плюс grace-пара `subscription_grace_entered` / `subscription_grace_expiring` (#741). Изданы к настоящему моменту: grace-пара (#741) и `rental_completed` — «Аренда завершена» (#748, скан по поясу собственника). Хранится в enum `notification_event_type`.
_Avoid_: категория

Мёртвые значения `notification_event_type` (`free_reminder`, `operation_due`, `operation_overdue`, `lease_expiring`, `lease_requires_action`, а с #741 и `subscription_grace` — значение прямого канала, снесённого #741) и `notification_target_type` (`free`, `operation`, `recurring_operation`, `lease`) остаются в PostgreSQL навсегда (прецедент #277) — домен и контракты о них не знают.

**Уведомление / Notification**:
Строка ленты одного получателя: снимок текста (заголовок, тело, лейбл контекста — имя объекта/тарифа или «Системные уведомления»), payload-ссылки, личные флаги (прочитано `read_at`, удалено `deleted_at`). Одно событие = по строке на каждого получателя (fan-out); переписанный шаблон не трогает хранимые строки. Таблица `notifications`; момент события — UTC instant (`created_at`), группировка «Сегодня/Вчера» — дело экрана (#744).
_Avoid_: пуш, письмо, сообщение

**Снимок сущности / EntityRef**:
Payload-ссылка с карточными строками: `id` (переход) + `name` (карточка живёт даже после переименования/удаления сущности) + необязательные строки карточки — `address` у объекта, `email` у приглашающего (решение владельца 19.09.2026, #745). Оба — снимки момента публикации; строки, которых в снимке нет, карточка просто не рисует. Словарь издателей — #748–#752 (адрес из `properties.address`, email из профиля актора на момент события).

**Лента / Notification Feed**:
Хранимый список уведомлений пользователя, читается newest-first keyset-пагинацией (канон #597). Настройки глушат только email/push — строка ленты пишется всегда.
_Avoid_: история, журнал

**Получатель / Recipient**:
Пользователь, которому адресована строка. Объектные события — владелец + все активные участники (включая «Просмотр»); доступ — адресат по событию; Тариф — владелец; Системные — все пользователи. Инициатор не получает уведомление о собственном действии (actor-skip); один пользователь — одна строка, дублирования на устройства/роли нет.

**Действие / Action**:
Кнопка перехода на экран сущности, никогда не мутация. Вычисляется при чтении страницы уведомления (`GET /notifications/{id}`, #743) по живому состоянию сущности и правам читателя; возможные действия за событием закрепляет каталог (`ActionsForEvent`). Состояние ушло — кнопок нет; сущность удалена — уведомление остаётся без действий; роль «Просмотр» получает уведомления без кнопок-мутаций.
_Avoid_: кнопка-действие (выполняющая мутацию из ленты)

**Дедуп-ключ / Dedup Key**:
Пара (получатель, `dedup_key`), где ключ = (тип события, сущность, фаза/дата); уникальный индекс делает инвариант прочным: повтор с тем же ключом строку не создаёт. Словарь издателей: grace (#741) — `subscription_grace_entered:<subscription_id>:<YYYY-MM-DD конца окна>` (без даты, когда окно без известного конца); скан аренды (#748) — `rental_completed:<rental_id>:<plannedEndDate YYYY-MM-DD>` — продление аренды меняет дату, значит ключ: повторное «Ожидает действия» уведомляет заново; остальные издатели — #749–#752.

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

### Delivery pipeline

**Пайплайн доставки / Delivery Pipeline** (#740, ADR 0057):
Путь уведомления от издателя до каналов: издатель после коммита своего бизнес-транзакции (канон grace_events, post-commit) вызывает `Publisher.Publish` → в одной транзакции: строка ленты на получателя (in-app доставлено) + джобы `deliver_email`/`deliver_push` (River `InsertTx`). Строка и джобы атомарны: джоба без строки и строка без доставки не существуют. Джобы несут только id — контент читается из закоммиченной строки при доставке. Строго после коммита пайплайн толкает в стрим `notification.created` + `notification.unread_count` (#742, ADR 0058) — best-effort, на доставку каналов не влияет.

**Скан завершения аренды / Rental-Completed Scan** (#748):
Издатель каталога `rental_completed` — не хук перехода, а часовой зональный скан (канон тиков ADR 0048 p.3): один «сегодня» на пояс собственника объекта, по всем зонам; цели — незавершённые аренды с плановым окончанием строго раньше «сегодня» на неархивном объекте (первый проход после полуночи пояса = «на следующий день после plannedEndDate», решение #737 тип №1; архив вне скана — канон тиков active/maintenance: мутации аренды архивного объекта отвергаются, кнопки были бы тупиковыми). Идемпотентность несёт дедуп-ключ, а не состояние прохода: повторный проход — пусто, продление (новая дата окончания) — новый ключ и повторное уведомление. Получатели — владелец + активные участники («Просмотр» включён); инициатора у системного события нет (actor-skip не срабатывает). Пейлоад — снимок объекта (id, имя, адрес) и id аренды; кнопки «Продлить»/«Завершить» вычисляются при чтении (#743), а диплинк пуша/письма/тоста строится из этих id. Отказы изолированы по зоне и по аренде — сломанная цель не останавливает проход.

**Очередь / Delivery Queue**:
River (Postgres-очередь, ADR 0057) в процессе API; очереди `notifications_email` и `notifications_push` с независимыми потолками воркеров. Ретраи — дефолтная лестница River (`attempts^4 ± 10%`), бюджет 8 попыток на канал; исчерпание → `discarded` (запросable, retention 7 дней). Дедуп джоб: unique по args (= id уведомления) во всех нетерминальных состояниях — у одного уведомления максимум одна джоба канала «в полёте». Доставка at-least-once: дубликат письма/пуша лучше потери.

**Доставка в момент доставки / Delivery-time resolution** (#740):
Джоба сама в момент доставки проверяет всё, что может устареть: контакт получателя (нет верифицированного email — leg пропускается), живой список push-подписок (мертвые 404/410 удаляются на месте) и матрицу настроек категория × канал — email-джоба спрашивает account-матрицу (`DeliverySettings`), push-джоба фильтрует подписки по их собственному состоянию (`Accepts`: мастер + категория; #743).

**Настройки / Settings** (#743, решение #738, ADR 0056):
Матрица категория × канал в двух местах: **email — на аккаунте** (таблица `notification_email_preferences`, одна строка на пользователя, четыре настраиваемые категории; нет строки = включено; `GET/PUT /notification-preferences` — полная замена) и **push — на устройстве** (мастер `enabled` + те же четыре флага прямо на push-подписке; `GET/PUT /push/subscriptions/preferences` по endpoint URL; `POST /push/subscriptions` несёт желаемое состояние). Выключение мастера — флаг: подписка и категории сохраняются, dispatch пропускает устройство. Тариф и Системные всегда включены, в настройках не хранятся. Лента настройками не глушится — пишется всегда.
_Avoid_: opt-out таблица (ADR 0030), per-event-type настройки

**Бюджет провайдера / Provider Budget**:
Глобальный email-лимит (`NOTIFICATIONS_EMAIL_PROVIDER_PER_MINUTE`, токен-бакет) списывается внутри email-джобы перед отправкой; исчерпание — `JobSnooze` (попытка не тратится). Потолок одновременных SMTP-вызовов — `MaxWorkers` очереди. Per-user бюджеты (5/час и пр., ADR 0013) остаются в точках HTTP-лимитов — в пайплайне их нет.

**Метрики доставки / Delivery Metrics**:
`notifications.email.dispatched{outcome=sent|skipped|rate_limited|failed|cancelled}` и `notifications.push.dispatched{outcome}` (канон webpush) + спаны/метрики River через `otelriver` → Uptrace. Алерты: устойчивый рост `rate_limited` — узкий бюджет провайдера; всплеск `discarded` в `river_job` — сломанный воркер.

### Push as best-effort side channel

Push delivery is best-effort: push failures (429, 5xx, no subscription) are logged but never block email delivery. Dead subscriptions (404/410, `ErrSubscriptionGone`) are deleted on the spot.

### Push delivery observability

`webpush.Metrics` exposes a single counter `notifications.push.dispatched` with an `outcome` attribute (`sent`/`gone`/`rate_limited`/`failed`). The cleanup rate of dead subscriptions — the volume of subscriptions the push delivery worker deletes because the push service returned 404/410 (`ErrSubscriptionGone`) — is observable as `notifications.push.dispatched{outcome=gone}` in Uptrace. The metric is recorded by the adapter before the domain error is returned, so every gone outcome is counted even if the DB delete fails. To monitor cleanup on prod, alert on a sustained non-zero `gone` rate (indicates subscription churn — devices uninstalled, ITP purges, OS token expiry).

### Declarative Web Push — decision (iOS)

**Decision: classic service-worker Web Push only for v1; Declarative Web Push (iOS/iPadOS 18.4+) deferred.** All notifications carry a visible notification (`showNotification` mandatory — iOS revokes the subscription on silent push), so the declarative format's main draw — silent/navigate-only messages without a `push` handler — adds nothing the v1 notifications need. The classic SW-push path (`public/sw.js` push-handler, research `docs/research/ios-pwa-push.md` §1.4) works on Android Chrome and iOS 16.4+ installed PWA alike. Declarative Web Push would primarily help if (a) we shipped silent/technical pushes, or (b) ITP purges of SW registrations started killing subscriptions at scale (the declarative format decouples the subscription from the SW). Neither applies to v1. Re-evaluate when a silent-push use case lands.

## Stream

**Стрим / Event Stream** (#742, ADR 0058):
Общий пользовательский SSE-стрим `GET /notifications/stream` — живой слой поверх ленты: тосты, бейдж непрочитанных, будущие потребители (карта #714). Авторизация — та же cookie-сессия (401 problem+json до старта стрима); транспорт однонаправленный, любые мутации — только через обычный API. Кадры best-effort: состояние клиент дочитывает своими запросами, лента остаётся системой записи.
_Avoid_: пуш (это другой канал), WebSocket, «realtime-состояние» (стрим не несёт состояния)

**Хаб / Hub**:
`internal/platform/sse.Hub` — реестр открытых соединений в памяти процесса. Лимит 8 соединений на пользователя (вкладки/устройства, старейший вытесняется), буфер 32 кадра, переполнение = медленный потребитель: соединение закрывается, клиент переподключается и перечитывает. Публикация никогда не блокирует издателя. На остановке процесса хаб закрывается первым — graceful shutdown не ждёт долгоживущие стримы.

**Конверт / Envelope**:
JSON внутри кадра: `{v, occurredAt, payload}` — версия схемы, момент события (RFC3339 UTC), типизированная нагрузка из id и дисплейных полей. Имя события — стабильное грубое: `connected` (старт), `notification.created` (тост: id, категория, заголовок, тело, deeplink), `notification.unread_count` (бейдж). Новые имена добавляются без ломки старых клиентов (нет слушателя — кадр игнорируется); ломающее изменение payload — подъём `v`.

**Курсор / Last-Event-ID**:
Заголовок переподключения с монотонным id кадра (последовательность хаба). В v1 курсор логируется, replay не реализован: клиент на каждом открытии (включая переподключение) перечитывает живое через react-query. Ring-buffer replay — заготовленный v2.

### Stream observability

`sse.connections` (gauge) — открытые соединения прямо сейчас, нагрузочная граница транспорта; `sse.frames.dropped` — вытеснения медленных потребителей, устойчивый ненулевой темп = клиенты не успевают или соединения зависают. Метрики nil-safe: без OTel-провайдера (локально, тесты) инструмент — no-op.

## ADRs

- ADR 0058 — общий пользовательский SSE-стрим: in-memory хаб, WriteTimeout: 0 с переносом защиты в read-таймауты и heartbeat, конверт v1, publication-хук пост-коммит, LISTEN/NOTIFY как мост на N реплик. Реализация #742.
- ADR 0057 — очередь доставки уведомлений: River в процессе API, транзакционная постановка в транзакции публикации, at-least-once, rivermigrate отдельной цепочкой. Реализация #740.
- ADR 0056 — хранимая лента и per-category настройки: категория × канал вместо типа события × канал; email per-account, push per-device (мастер + категории на подписке); лента пишется всегда; старая таблица дропнута без переноса (миграция 000131, настройки — миграция 000133). Supersedes ADR 0030 и ADR 0022. Реализация настроек и REST ленты — #743.
- ADR 0030 — per-channel notification preferences (email/push independent). Superseded by ADR 0056 (таблица `user_notification_channel_preferences` дропнута миграцией 000131, opt-out'ы сброшены — решение #738). Supersedes point 1 of ADR 0022.
- ADR 0022 — per-event-type notification preferences. Superseded by ADR 0056 (модель per event type ушла целиком).
- ADR 0046 — удаление аренд и Операций: reminders-механика удалена, контекст стал grace-only. Supersedes ADR 0029 (rental super-context).
