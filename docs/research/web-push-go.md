# Web Push: протокол и реализация на Go

Дата: 2026-08-10
Вход для: тикет https://github.com/devnumbers/arenda-platform/issues/174 (Research: Web Push протокол и реализация на Go) — подготовка к превращению кабинета в installable PWA с Web Push.

Файл содержит фактуру по первоисточникам (RFC 8030/8291/8292, MDN, web.dev, WebKit/Apple docs, GitHub-репозитории). Рекомендация по библиотеке — в отдельной секции в конце. Push-кода в репо нет (проверено grep'ом по `webpush|VAPID|PushSubscription`); существующий контекст — `apps/backend/internal/notifications` (email/sms/reminders).

## 1. Протокол

### 1.1 VAPID (RFC 8292) — идентификация application server'а

- **Ключи**: пара ECDSA на кривой P-256 (MUST, RFC 8292 §2). Публичный ключ — 65 байт в uncompressed-форме X9.62 (начинается с `0x04`), кодируется base64url. Генерация одноразовая на приложение/окружение; в Go — `elliptic.P256()` / `ecdh.P256()` + base64url RawURLEncoding (пример — `GenerateVAPIDKeys` в webpush-go).
- **JWT (ES256)** в заголовке `Authorization: vapid t=<JWT>,k=<base64url-публичный-ключ>` (RFC 8292 §2, §3). Claims:
  - `aud` (MUST) — Unicode-сериализация **origin'а push-ресурса** (схема + хост endpoint'а, без пути). Токен привязан к push-сервису и переиспользуется для всех endpoint'ов того же origin'а.
  - `exp` (MUST) — не более чем на 24 часа вперёд от момента запроса (MUST NOT, §2). Практика — ставить ~12 ч, чтобы не упираться в clock skew (web.dev).
  - `sub` (MAY по RFC, фактически ожидается) — контакт: `mailto:` или `https:` URI (§2.1). Autopush: если VAPID передан, `sub` обязателен. Apple: ошибка `BadJwtToken`, если `sub` не URL/mailto.
- **Ошибки валидации**: push-сервис MAY отклонять запрос с 403 при невалидной подписи/claims (§2). Для restricted-подписки (см. ниже): 401 при отсутствии auth, 403 при невалидной (§4.2).
- **Subscription restriction — ключевой механизм** (§4): браузер передаёт публичный VAPID-ключ (`applicationServerKey` из `pushManager.subscribe()`) при создании подписки, push-сервис привязывает его к endpoint'у. С этого момента сообщения на этот endpoint принимаются только с JWT, подписанным соответствующим приватным ключом. Apple явно: ключ в запросе должен совпадать с ключом подписки, иначе `VapidPkHashMismatch` (403).
- **Ротация (критично, гипотеза из тикета подтверждена)**: подписки жёстко привязаны к `applicationServerKey`. RFC 8292 §4.2: при замене ключа подписи сервер «needs to request the creation of a new subscription by the user agent that is restricted to the updated key» — т.е. **смена VAPID-ключей ломает все живые подписки** (отправка начнёт получать 403), нужен переподписной флоу на клиентах. Сервер обязан помнить, с каким ключом создана каждая подписка. RFC §5 рекомендует *плавную* миграцию на новый ключ: резкая смена ломает linkability репутации, и push-сервис может классифицировать запросы с новым ключом как abusive.
- **Реюз токена поощряется** (§5): push-сервис кеширует результат проверки подписи; серверу не нужно подписывать JWT на каждое сообщение. Apple: не обновлять JWT чаще одного раза в час.
- Ключ для подписи VAPID и ephemeral-ключ ECDH для шифрования payload — обязаны быть разными; сервис SHOULD отклонять 400, если совпадают (§3.2).

### 1.2 Шифрование payload (RFC 8291, aes128gcm)

- **Входные данные** (на подписку, генерируются браузером, приезжают в `PushSubscription`): `keys.p256dh` — публичный ECDH-ключ UA (P-256, uncompressed), `keys.auth` — симметричный auth-secret, 16 байт криптостойкой случайности (§3.2). Оба — base64url в JSON.
- **На каждое сообщение** сервер генерирует: ephemeral ECDH-пару P-256 и 16-байтный salt (§2, §3.4). Ephemeral-ключ можно выбросить после шифрования.
- **Деривация ключей** (HKDF-SHA-256, псевдокод §3.4 — совпадает побайтово с кодом webpush-go):
  1. `ecdh_secret = ECDH(as_private, ua_public)`
  2. `PRK_key = HMAC(auth_secret, ecdh_secret)`; `key_info = "WebPush: info" || 0x00 || ua_public || as_public`; `IKM = HMAC(PRK_key, key_info || 0x01)`
  3. `PRK = HMAC(salt, IKM)`; `CEK = HMAC(PRK, "Content-Encoding: aes128gcm" || 0x00 || 0x01)[0..15]`; `NONCE = HMAC(PRK, "Content-Encoding: nonce" || 0x00 || 0x01)[0..11]`
  4. AES-128-GCM: plaintext + байт-разделитель padding `0x02` + padding. Тело запроса = [salt(16) || rs(4) || idlen(1) || as_public(65)] (заголовок RFC 8188, 86 байт) + ciphertext (вкл. 16-байтный GCM-тег). Тестовые вектора — Appendix A RFC 8291.
- **Один record**: сообщение MUST шифроваться единственной записью; `rs` > plaintext + 1 + padding + 16 (§4).
- **Лимит размера (~4 КБ)** — первоисточник: RFC 8030 §7.2: push-сервис MUST NOT отвечать 413 на тело ≤ 4096 байт (т.е. 4096 — гарантированный минимум поддержки). RFC 8291 §4: за вычетом 86 байт заголовка, минимум 1 байта padding и 16 байт тега — **максимум 3993 байта plaintext**. Ответ при превышении — 413. Фактические лимиты сервисов: autopush — 4028 байт (errno 104), Apple — «over the limit of 4 KB» (`PayloadTooLarge`), bridged-подписки autopush через FCM — всего 2744 байта (base64-транскрипция). Практический потолок — держать payload заметно меньше 3993 байт.
- **aes128gcm — единственный легальный content-encoding** для push-сообщений (§4: MUST NOT использовать другие; сжатие запрещено — риск утечки). По спецификации Push API UA MUST поддерживать `aes128gcm` и MAY поддерживать кодировки из прежних версий спецификации (MDN `PushManager.supportedContentEncodings` — обычно возвращает ровно `["aes128gcm"]`).
- **Устаревший `aesgcm`** (draft-ietf-webpush-encryption, заголовки `Encryption: salt=...`, `Crypto-Key: dh=...`, HKDF-info `Content-Encoding: auth\0`): существовал в черновиках 2016–2017, статья web.dev по протоколу описывает именно его (статья 2016 года, устарела в этой части). Текущий статус поддержки `aesgcm` в FCM/autopush/APNs: **Непроверено** — спецификация делает его опциональным для UA; для нового кода использовать только `aes128gcm`.

Источники:
- https://www.rfc-editor.org/rfc/rfc8292 (§2, §2.1, §3.2, §4, §4.2, §5)
- https://www.rfc-editor.org/rfc/rfc8291 (§2, §3.2–3.4, §4, §5, Appendix A)
- https://www.rfc-editor.org/rfc/rfc8030 (§7.2)
- https://developer.mozilla.org/en-US/docs/Web/API/PushManager/supportedContentEncodings
- https://web.dev/articles/push-notifications-web-push-protocol (описание legacy-схемы)
- https://mozilla-services.github.io/autopush-rs/http.html, https://mozilla-services.github.io/autopush-rs/errors.html
- https://developer.apple.com/documentation/usernotifications/sending-web-push-notifications-in-web-apps-and-browsers

## 2. Push-сервисы: браузер → сервис → endpoint

Все сервисы реализуют один протокол (RFC 8030) — серверный код единый, endpoint берётся из подписки, его host ни в чём хардкодить нельзя (autopush прямо: «endpoint is opaque, мы можем менять URL в любой момент»).

| Браузер | Push-сервис | Типичный endpoint | Источники |
|---|---|---|---|
| Chrome / Edge / Opera / Samsung Internet | FCM (Firebase Cloud Messaging) | `https://fcm.googleapis.com/fcm/send/...` (бывает `/wp/...`) | web-push-libs README, pushpad.xyz, github.com/andreinwald/webpush-ios-example |
| Firefox (desktop/Android) | Mozilla autopush | `https://updates.push.services.mozilla.com/wpush/...` | mozilla-services.github.io/autopush-rs |
| Safari (macOS 13+, iOS/iPadOS 16.4+ PWA) | APNs | `https://web.push.apple.com/...` (любой поддомен `*.push.apple.com`) | WebKit blog 12945/13878, Apple Developer doc |

Quirks по сервисам:

- **FCM (Chrome)**: legacy `gcm_sender_id` в manifest.json нужен был только Chrome ≤ 51 (и старым Opera/Samsung) — с Chrome 52 работает VAPID, сейчас `gcm_sender_id` не нужен (web-push-libs README, таблица поддержки). Chrome исторически принимал до-стандартную схему авторизации `Authorization: WebPush <JWT>` + `Crypto-Key: p256ecdsa=<ключ>` (описана в статье web.dev по протоколу; поддерживается и сейчас — в webpush-go это опция `AuthScheme: WebPush`, добавлена в 2026 по issue «Need support for chrome?»). Стандартная схема RFC 8292 `vapid t=...,k=...` — работает везде, включая Chrome.
- **Apple (Safari)**: Web Push появился в Safari 16 на macOS Ventura 13 (WWDC22, июнь 2022); на iOS/iPadOS — с 16.4 (март 2023) и **только для web-приложений, установленных на домашний экран** (в браузере Safari на iOS подписки нет). Apple Developer Program не нужен. Требования: запрос подписки только по явному user gesture; `userVisibleOnly: true` обязателен; **уведомление обязано быть показано на каждый push — иначе Safari отзывает разрешение** («Violations of the userVisibleOnly promise will result in a push subscription being revoked»). Если сеть сервера фильтрует исходящие — открыть `https://*.push.apple.com`. Протоколы: HTTP/1.1 (pipelining ≤ 100 неподтверждённых запросов) и HTTP/2 (не превышать SETTINGS_MAX_CONCURRENT_STREAMS). TTL — хранение недоставленных до 30 дней, число хранимых сообщений ограничено. В ответе — заголовок `apns-id` и JSON с `reason` при ошибках: `BadTtl`, `BadUrgency`, `BadWebPushRequest`, `BadWebPushTopic`, `VapidPkHashMismatch`, `BadAuthorizationHeader`, `BadJwtToken`, `BadVapidPublicKey`, `PayloadTooLarge`, `TooManyRequests` («too many consecutive requests to the same device token»), `ServiceUnavailable`, `Shutdown`. Urgency `high` — «attempt to deliver immediately».
- **Mozilla autopush (Firefox)**: VAPID **опционален** (но если ключ передан при регистрации — endpoint становится restricted, и `sub` в JWT обязателен). Возвращает только 201 (push receipts из RFC 8030 §5.1 не поддерживает: «Autopush cannot support the Push Message Receipt at this time»). TTL валиден 0…2592000 (≈30 суток, errno 112). Topic — ≤32 байта ASCII alphanumeric. 413 при >4028 байт (errno 104). Bridged-подписки (через FCM/APNs-мосты) — payload ограничен 2744 байтами.
- **Обязательность VAPID в целом**: Chrome/Edge требуют `applicationServerKey` уже на этапе `subscribe()` (MDN: «required in some browsers like Chrome and Edge»); на Firefox опционально; у Apple фактически обязателен (сверка ключа подписки, `VapidPkHashMismatch`). Вывод: VAPID используем всегда — единый код.

Источники:
- https://webkit.org/blog/12945/meet-web-push/ (Safari 16 / macOS Ventura, *.push.apple.com, revoke)
- https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/ (iOS 16.4, Home Screen only)
- https://developer.apple.com/documentation/usernotifications/sending-web-push-notifications-in-web-apps-and-browsers (reason-коды, pipelining, TTL 30 дней, Urgency)
- https://mozilla-services.github.io/autopush-rs/http.html, https://mozilla-services.github.io/autopush-rs/errors.html
- https://raw.githubusercontent.com/web-push-libs/web-push/master/README.md (gcm_sender_id, поддержка браузеров)
- https://developer.mozilla.org/en-US/docs/Web/API/PushManager/subscribe
- https://web.dev/articles/push-notifications-web-push-protocol (legacy-схема WebPush/Crypto-Key)
- https://github.com/SherClockHolmes/webpush-go (README, PR #84, issues #82/#72)

## 3. Go-библиотеки

| Библиотека | ★ / форки | Активность | Статус | Заметки |
|---|---|---|---|---|
| `github.com/SherClockHolmes/webpush-go` | 448 / 86 | последний коммит 2026-04-22, релиз v1.4.0 (2025-01-02) | **не archived**, MIT | Де-факто стандарт: 644 go.mod-файлов на GitHub ссылаются на него (code search). RFC 8291 aes128gcm (код совпадает с псевдокодом RFC), RFC 8292 обе auth-схемы, Topic/Urgency/TTL, `VapidExpiration` default 12 ч, `MaxRecordSize = 4096` + `ErrMaxPadExceeded`. Тесты есть (`vapid_test.go`, `webpush_test.go`). |
| `github.com/marknefedov/go-webpush/v2` | 3 / — | коммиты 2026-07 | активный форк | Клиентский API, multi-record, batch sending, receipt metadata. Крошечное комьюнити. |
| `github.com/gootsolution/pushbell` | 2 / — | 2026-08 | новый | Зависит от fasthttp — чужеродно для проекта (stdlib net/http). |
| `github.com/zhengkai/webpush-go`, `daaku/webpush`, `wuc656/webpush-go`, `imjasonh/webpush` | ≤2 | 2019–2025 | штучные/заброшенные | Форки/учебные реализации. |
| `github.com/pennersr/shove` | 285 / — | 2026-08 | активный | Не библиотека, а standalone push-**сервис** (APNs/FCM/WebPush/Telegram/Email). Вне нашей модели. |
| `web-push-libs/web-push` (npm) | 3533 | 2026-08 | активный | Не Go, но эталонная реализация для сверки поведения. |

Детали по SherClockHolmes/webpush-go (прочитаны `vapid.go`, `webpush.go`, README, issues/PR):
- API: `webpush.SendNotification([]byte, *Subscription, *Options)` → `*http.Response`. `Subscription{Endpoint, Keys{Auth, P256dh}}` — ровно JSON подписки с фронта. `Options{Subscriber, VAPIDPublicKey, VAPIDPrivateKey, TTL, Topic, Urgency, RecordSize, HTTPClient, AuthScheme, VapidExpiration}`.
- Ответ push-сервиса **не интерпретируется** — код статуса (404/410/413/429) разбирает вызывающий код. Retry-логики внутри нет.
- Использует deprecated `crypto/elliptic` API (`elliptic.GenerateKey`, `ScalarMult`) вместо `crypto/ecdh` — PR #60 «Use crypto/ecdh» открыт с 2023-11. Работает, но на Go 1.26 — deprecated-API (go vet не ругает, статический анализ может).
- `go.mod`: `go 1.13`, зависимости `golang-jwt/jwt/v5`, `golang.org/x/crypto` — лёгкие.
- Открытые issue-риски: #81 «Apple Devices cant receive the webpush notification» (2025-04, без ответа), #66 «MS Edge rejects push messages» (2024-02), #47 «Wrong Subscription.Keys.Auth value does not return an error» (2022-07). Поддержка — медленная, но пульс есть (PR #84 слит в апреле 2026).

**Plan B — своя реализация поверх stdlib** (оценка объёма, проверено по наличию пакетов на Go 1.26 проекта):
- Всё необходимое есть в stdlib: `crypto/ecdh` (P-256 ECDH), `crypto/ecdsa` (ES256), `crypto/aes` + `crypto/cipher` (GCM), **`crypto/hkdf` (в stdlib с Go 1.24 — проверено `go doc crypto/hkdf`)**, `crypto/sha256`/`hmac`, `encoding/base64`. JWT ES256 — ~50 строк (header.payload в base64url + ECDSA-подпись + конвертация ASN.1 → R‖S 64 байта); в go.mod бэкенда JWT-библиотеки сейчас нет (есть `golang.org/x/crypto v0.53.0`).
- Объём: шифрование RFC 8291 ≈ 150–200 строк (есть эталонный псевдокод §3.4 и тест-вектора Appendix A), VAPID ≈ 60–100 строк, отправка/заголовки ≈ 100 строк + тесты. Реалистично, но дублирует готовое.

Источники:
- https://github.com/SherClockHolmes/webpush-go (+ `gh api` метаданные: stars/archived/commits/issues/PR #84/#60, go.mod, vapid.go, webpush.go)
- https://pkg.go.dev/search?q=webpush
- GitHub search `webpush language:go` (через `gh api search/repositories`)
- https://github.com/web-push-libs/web-push
- `go doc crypto/hkdf` локально (Go 1.26)

## 4. Жизненный цикл подписки

- **Структура `PushSubscription`** (MDN): `endpoint` (string, секретный URL), `expirationTime` (`DOMHighResTimeStamp` или `null`), `options` (опции создания), методы `getKey('p256dh'|'auth')` → ArrayBuffer, `toJSON()`, `unsubscribe()` → Promise<boolean>. `JSON.stringify(subscription)` даёт `{endpoint, expirationTime, keys: {p256dh, auth}}` — это и есть тело для отправки на сервер.
- **Смысл ключей**: `p256dh` — публичный ECDH-ключ UA (65 байт uncompressed), `auth` — 16-байтный auth-secret (RFC 8291). Приватные ключи браузер не отдаёт никому.
- **Повторный `subscribe()`** на том же service worker возвращает существующую подписку («A new push subscription is created if the current service worker does not have an existing subscription», MDN) — безопасно дёргать при каждом старте приложения.
- **Истечение/смена**: `expirationTime` почти всегда `null` (формально «вечная»), но на практике браузеры прекращают подписки (давно не было пушей, неиспользуемое приложение, ротация токенов сервиса) — web.dev прямо предупреждает и советует паттерны переподписки (не злоупотребляя). Событие **`pushsubscriptionchange`** в SW: переподписаться с `event.oldSubscription.options` и отправить новую подписку на сервер (MDN). **Поддержка события неполная** (MDN browser-compat-data): Chrome только с v138 (2025!), Safari 16+ (macOS), Firefox 44+ частично (без `oldSubscription`/`newSubscription`, bug 1497429), **Safari iOS — не поддерживается**. Вывод: на событие полагаться нельзя; страховка — re-subscribe при запуске PWA + сверка endpoint'а + серверная чистка по 404/410.
- **404/410 при отправке** → подписку удалить из БД: 404 — subscription expired/invalid (RFC 8030 §7.3; autopush errno 102), 410 — gone, воспроизводится вызовом `unsubscribe()` (web.dev, таблица статусов; autopush errno 103/105/106). После этого ждать, пока клиент переподпишется.
- **TTL** (RFC 8030 §5.2): заголовок **обязателен** (400 при отсутствии), неотрицательное число секунд (парсинг — минимум 31 бит). Сервис MAY урезать TTL и возвращает фактический в ответе (`TTL` response header). `TTL: 0` — доставить немедленно или выбросить, если UA офлайн. autopush: максимум 2592000 (~30 суток); Apple: хранит до 30 дней, количество хранимых сообщений ограничено.
- **Urgency** (§5.3): `very-low | low | normal | high`, default `normal`; несколько значений → 400; UA заголовок не видит. Apple: `high` = попытка немедленной доставки. Использовать для батарейной экономики (напоминания — `normal`, срабатывание по факту просрочки — `high`).
- **Topic** (§5.4): token ≤ 32 символов из URL/filename-safe Base64; сообщение с topic **замещает** недоставленное сообщение с тем же topic на той же подписке (вместе с TTL/Urgency); нарушение формата → 400; UA не видит, не шифруется. autopush: `[A-Za-z0-9]` ≤ 32 байта; Apple: `BadWebPushTopic`. Применение: «у вас N просроченных платежей» — пересылаем с тем же topic, пользователь получает одно актуальное, а не пачку устаревших.

Источники:
- https://developer.mozilla.org/en-US/docs/Web/API/PushSubscription
- https://developer.mozilla.org/en-US/docs/Web/API/PushManager/subscribe
- https://developer.mozilla.org/en-US/docs/Web/API/ServiceWorkerGlobalScope/pushsubscriptionchange_event + mdn/browser-compat-data `api/ServiceWorkerGlobalScope.json`
- https://web.dev/articles/push-notifications-subscribing-a-user (resubscribe pattern)
- https://web.dev/articles/push-notifications-web-push-protocol (таблица статусов 201/400/404/410/413/429)
- https://www.rfc-editor.org/rfc/rfc8030 (§5.2–5.4, §7.2, §7.3)
- https://mozilla-services.github.io/autopush-rs/errors.html

## 5. Надёжность: retry, rate limits, batching

- **Семантика 201**: «accepted for delivery», НЕ «delivered» (RFC 8030 §5). Receipts (`Prefer: respond-async`, §5.1) в протоколе есть, но autopush их не поддерживает, FCM/APNs — тоже фактически нет: считать 201 = «принял push-сервис»; end-to-end подтверждения доставки до устройства нет.
- **Фатальные коды (не ретраить)**: 400 (кривые заголовки/Topic/TTL/Urgency), 401/403 (VAPID — чинить конфиг, не ретраить), 404/410 (подписку удалить), 405, 413 (уменьшить payload).
- **429 Too Many Requests**: RFC 8030 §8.4 — сервис MAY вернуть 429 при превышении rate limit'а и SHOULD приложить `Retry-After` → ретрай только после указанного времени. Apple: `TooManyRequests` — «too many consecutive requests to the same device token» (т.е. лимит в первую очередь per-endpoint).
- **5xx**: autopush 503 несёт errno: 201 — «use exponential back-off for retries», 202 — «immediate retry ok»; 502 — ошибки bridge (errno 900–903). Apple: 500/503 (`ServiceUnavailable`, `Shutdown`). Стратегия: экспоненциальный backoff с джиттером + честь `Retry-After`, если есть; ограничение попыток; DLQ/метрика на неустранимые.
- **Rate limits по сервисам**: конкретных публичных цифр нет. RFC 8030 §8.4: сервисы SHOULD ограничивать rate сообщений на одного UA и MAY отключать подписки, получающие слишком много сообщений. FCM квоты для webpush-endpoint'ов не документированы (**Непроверено**, документация FCM описывает квоты только Admin API). Apple — per-device-token consecutive limit (выше). autopush — 503/errno вместо публичных цифр.
- **Batching**: пакетного endpoint'а в протоколе **нет** — один POST = одно сообщение одной подписке (RFC 8030 §5). Параллелим сами: HTTP/2 — не выше SETTINGS_MAX_CONCURRENT_STREAMS (Apple прямо предписывает), HTTP/1.1 pipelining — ≤100 неподтверждённых запросов (Apple). Практика для нас: worker-pool (errgroup с лимитом), keep-alive `http.Client` на всех.
- **Кеширование VAPID JWT**: `aud` = origin сервиса → один JWT покрывает все endpoint'ы этого сервиса; живёт ≤24 ч; RFC поощряет реюз (кеш валидации на стороне сервиса); Apple — не чаще раза в час. Практика: кешировать JWT per-origin на ~12 ч (запас под clock skew — так же делает webpush-go по умолчанию и советует web.dev). Подпись на каждое сообщение не нужна.
- **Topic как anti-storm**: при массовых событиях (ночной батч напоминаний) topic гасит дубли на стороне сервиса — дешевле, чем доставка пачки.

Источники:
- https://www.rfc-editor.org/rfc/rfc8030 (§5, §5.1, §8.4)
- https://mozilla-services.github.io/autopush-rs/errors.html (errno 201/202, 502/900–903)
- https://developer.apple.com/documentation/usernotifications/sending-web-push-notifications-in-web-apps-and-browsers (429/500/503, pipelining)
- https://web.dev/articles/push-notifications-web-push-protocol (429 + Retry-After)

## 6. API-дизайн (с привязкой к конвенциям репо)

Репо: contract-first `apps/backend/api/openapi/openapi.yaml` → oapi-codegen; миграции — пары `db/migrations/NNNNNN_name.up/down.sql`; UUIDv7 app-side без DEFAULT (ADR 0019); `timestamptz` + триггер `set_updated_at()`; enum'ы — `TEXT + CHECK`; деньги не нужны. Контекст `internal/notifications` уже имеет `domain/`, `application/` (service, ports), `adapters/{postgres,http,email,sms}` — push встраивается как ещё один канал доставки + новая сущность.

- **Endpoints** (предлагаемая форма, не контракт):
  - `POST /push/subscriptions` — тело = `PushSubscription` JSON как есть (`endpoint`, `expirationTime?`, `keys{p256dh, auth}`); 201/200. Идемпотентность через `UNIQUE(endpoint)` + upsert: повторный `subscribe()` в том же браузере возвращает ту же подписку, реконнекты/переустановки — upsert по endpoint.
  - `DELETE /push/subscriptions` — тело/параметр `endpoint`; 204. Endpoint — естественный ключ, он единственное, что клиент знает наверняка (внутренний UUID можно и не светить).
  - `GET /push/vapid-public-key` — отдаёт публичный VAPID-ключ фронту в рантайме: `{"publicKey": "..."}`.
- **Хранение** — таблица `push_subscriptions` (по образцу `000090_free_reminders_table.up.sql`):
  - `id uuid NOT NULL PRIMARY KEY` (UUIDv7 из приложения, без DEFAULT)
  - `owner_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE`
  - `endpoint text NOT NULL UNIQUE` (уникален глобально — push-сервис выдаёт уникальный URL на подписку)
  - `p256dh text NOT NULL`, `auth text NOT NULL`
  - `expiration_time timestamptz` NULL (из `expirationTime`, обычно NULL)
  - `user_agent text` NULL (диагностика; Спорно: полезность невысока, но дёшево)
  - `created_at`, `updated_at timestamptz NOT NULL DEFAULT now()` + триггер `trg_push_subscriptions_updated_at`
  - индекс `(owner_id)` — выборка всех устройств пользователя при отправке
  - Один пользователь = N устройств/браузеров → уведомление = fan-out по всем строкам owner'а, мёртвые сносятся по 404/410.
  - **Секретность**: endpoint + keys — секрет (владение ими = возможность слать пуши пользователю, RFC 8030 §8.3). Не логировать (см. `docs/backend-observability.md` про PII), HTTPS обязателен (уже есть за Caddy).
- **Слои**: `domain/push_subscription.go` (сущность + валидация длин), `application` — порты `PushSubscriptionRepository`, `PushSender`; `adapters/postgres` (sqlc-запросы в `db/queries/`), `adapters/webpush` (обёртка над библиотекой + маппинг кодов ответа в доменные ошибки Gone/RateLimited/…), `adapters/http` (handlers по сгенерированному интерфейсу).
- **VAPID-ключ фронту — runtime-endpoint vs build-time env**:
  - Runtime (`GET /push/vapid-public-key`): фронт гарантированно в синке с бэкендом — а рассинхрон фатален (subscription restriction: подписка привязана к ключу, 403/`VapidPkHashMismatch`); ротация без перевыкладки фронта; per-окружение ключи из того же конфига бэкенда. Цена — один запрос перед subscribe (кешируется в SW/памяти).
  - Build-time (`NEXT_PUBLIC_*`): проще на старте, но ключ зашит в бандл; ротация = перевыкладка фронта + ручная синхронизация env'ов двух приложений.
  - Вывод: runtime-endpoint. Приватный ключ — env бэкенда (`.env.<env>`, GitHub Environments, по `docs/deployment.md`); публичный — тоже конфиг бэкенда (не генерить на лету: подписки привязаны к ключу).
- **Ротация VAPID на практике**: не менять без нужды; при компрометации — новый ключ в конфиг, клиентский флоу `unsubscribe()` + `subscribe()` с новым ключом (фронт сверяет ключ из runtime-endpoint'а с `subscription.options.applicationServerKey` и переподписывается), старые подписки сносятся по 403 как мёртвые. Плавно (RFC 8292 §5 — резкая смена бьёт по репутации у push-сервисов).

Источники:
- Конвенции: `apps/backend/db/migrations/000090_free_reminders_table.up.sql`, `apps/backend/AGENTS.md`, `docs/adr/0019-uuid-v7-app-generated-ids.md`, дерево `apps/backend/internal/notifications/`
- https://www.rfc-editor.org/rfc/rfc8292 (§4.2, §5)
- https://developer.mozilla.org/en-US/docs/Web/API/PushManager/subscribe

## 7. Фронтенд-сторона

- **Feature detection** (web.dev): `'serviceWorker' in navigator` && `'PushManager' in window`. На iOS Push API доступен только внутри установленной на домашний экран PWA (WebKit) — в браузере Safari iOS подписки нет; значит, UI подписки на iOS показывать только в standalone-режиме (`display-mode: standalone` / `navigator.standalone`).
- **Permission**: `Notification.permission` ∈ `default | granted | denied` (MDN; `default` = решения не было, ведёт себя как denied). Запрос — `Notification.requestPermission()` (Promise-форма; старые браузеры — callback, web.dev даёт обёртку) или сайд-эффект `subscribe()`. **Только по user gesture** (MDN; Firefox блокирует с v72; Apple требует явно). `denied` необратим без ручной разблокировки пользователем → сначала собственный pre-permission экран с объяснением ценности, системный промпт — по клику (web.dev Permission UX).
- **Подписка**: `registration.pushManager.subscribe({userVisibleOnly: true, applicationServerKey})`. `applicationServerKey` — публичный VAPID-ключ как `Uint8Array`/`ArrayBuffer` (Base64url-строка тоже допускается по MDN, но классический надёжный путь — конвертация); обязателен в Chrome/Edge. `userVisibleOnly: true` обязателен фактически везде: Chrome отклоняет promise с ошибкой, Apple отзывает разрешение за «тихие» пуши. Конвертер `urlBase64ToUint8Array` (web.dev): дополнить `=` до кратности 4, заменить `-`→`+`, `_`→`/`, `atob` → `Uint8Array`.
- **Отправка на сервер**: `JSON.stringify(pushSubscription)` → POST (web.dev). MDN советует помнить, что `fetch` из SW не сработает офлайн — переподписку делать устойчивой к ошибкам.
- **SW: событие `push`**: `event.waitUntil(registration.showNotification(title, {body, icon, badge, tag, data}))`. URL для перехода кладём в `data` — доступен в клике как `event.notification.data` (web.dev patterns). Показ обязателен (Chrome иначе показывает системное «This site has been updated in the background», Safari отзывает разрешение). **Исключение**: если окно приложения открыто и в фокусе — можно не показывать нотификацию, а `postMessage` в открытые окна: `clients.matchAll({type:'window', includeUncontrolled:true})` → проверка `windowClient.focused` (web.dev patterns).
- **SW: `notificationclick`**: `event.notification.close()`, затем фокус существующего окна vs открытие нового (web.dev patterns): `clients.matchAll({type:'window', includeUncontrolled:true})` → ищем `windowClient.url === urlToOpen` → `matchingClient.focus()`, иначе `clients.openWindow(url)`. Видны только окна своего origin. `event.waitUntil()` на всю цепочку, иначе SW убьют. Есть ещё `notificationclose` (свайп/крестик) — для аналитики.
- **SW: `pushsubscriptionchange`**: переподписка с `event.oldSubscription.options` + отправка новой подписки на сервер (MDN). Поддержка: Chrome 138+, Safari 16 macOS, Firefox частично (без old/new), Safari iOS — нет → дополнительно: при каждом запуске PWA `getSubscription()` → нет подписки/сменился endpoint/`applicationServerKey` из runtime-endpoint'а ≠ `subscription.options.applicationServerKey` → `subscribe()`/переподписка + sync с сервером.
- **`tag` у нотификации** — клиентский аналог Topic: новая нотификация с тем же `tag` заменяет показанную (web.dev patterns; `registration.getNotifications()` для merge-сценариев).
- **Локальные лимиты**: payload держим маленьким (≤ ~3.9 КБ, см. §1.2) — в пуш идёт только «звонок» (тип события + ссылка), данные приложение подтягивает само после фокуса.

Источники:
- https://web.dev/articles/push-notifications-subscribing-a-user (permission flow, urlBase64ToUint8Array, PushSubscription JSON, resubscribe)
- https://web.dev/articles/push-notifications-common-notification-patterns (focus vs openWindow, data, isClientFocused, tag/merge)
- https://developer.mozilla.org/en-US/docs/Web/API/PushManager/subscribe
- https://developer.mozilla.org/en-US/docs/Web/API/PushSubscription
- https://developer.mozilla.org/en-US/docs/Web/API/Notification/permission_static
- https://developer.mozilla.org/en-US/docs/Web/API/ServiceWorkerGlobalScope/pushsubscriptionchange_event (+ BCD)
- https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/ (Home Screen only, gesture)

## Выбор библиотеки / рекомендация

**Брать `github.com/SherClockHolmes/webpush-go`** (последняя версия v1.4.0+ / master): единственная зрелая Go-реализация (448★, 644 зависимых модуля, MIT, живые коммиты в 2026), покрывает всю нужную тройку RFC (8030 заголовки + 8291 aes128gcm + 8292 обе auth-схемы), Topic/Urgency/TTL из коробки, зависимости лёгкие и уже частично есть в проекте (`x/crypto`). Известные слабости закрыть у себя в адаптере `internal/notifications/adapters/webpush`:
- разбор кодов ответа (201/400/403/404/410/413/429/5xx) и удаление подписок по 404/410 — наша обёртка;
- retry с backoff + честь `Retry-After` — наш sender-worker;
- кеширование VAPID JWT per-origin (библиотека подписывает JWT на каждый вызов; при fan-out — считать один на origin, см. §5);
- deprecated `crypto/elliptic` внутри библиотеки — терпимо (работает на Go 1.26), следить за PR #60 / форками;
- issue #81 (iOS) — проверить на реальном устройстве в рамках имплементации (см. «Открытые вопросы»).

Plan B (своя реализация на stdlib, ~400 строк + тесты по векторам RFC 8291 Appendix A) держать в уме, но не делать: выгоды нет, а риск ошибиться в криптографии есть.

Архитектурно (сверка с контекстом notifications): `push_subscriptions` таблица + доменная сущность; порт `PushSender` в `application`; фоновый sender с очередью/батчингом по owner'у; `GET /push/vapid-public-key`; фронт — модуль подписки в SW с re-subscribe при старте.

## Открытые вопросы

- **Статус legacy `aesgcm`** в текущих FCM/autopush/APNs не проверен до конца (спецификация делает его опциональным; для нового кода неактуально — шлём aes128gcm). Непроверено.
- **Issue #81 webpush-go (iOS/Safari не получают пуши)** — открыт с 2025-04 без ответа; возможные причины в сети наши: legacy auth-схема, несовпадение VAPID-ключа, отсутствие установки PWA. Требует проверки на реальном iOS-устройстве в имплементационном тикете. Непроверено.
- **Rate limits FCM** для webpush-endpoint'ов публично не документированы — узнаем только по 429 в рантайме; мониторить. Непроверено.
- **`pushsubscriptionchange` на iOS** отсутствует, на Chrome — только с v138: окончательный набор триггеров переподписки (стартап PWA + сверка ключа/endpoint'а) закрепить в дизайне фронт-модуля.
- **Поведение `unsubscribe()` на сервере при многопользовательских устройствах** (общий iPad): подписка привязана к браузерному профилю+PWA, а не к user_id — при смене пользователя на устройстве решать политику отвязки (DELETE по endpoint при логауте). Продуктовый вопрос, не протокольный.
