# PWA и Web Push на iOS/iPadOS

Дата: 2026-08-10
Вход для: тикет https://github.com/devnumbers/arenda-platform/issues/175
Контекст: кабинет (Next.js 16, `apps/frontend`, общий домен за Caddy, HTTPS есть) превращаем в installable PWA с Web Push-уведомлениями. Документ — фактурная база для спеки реализации по iOS/iPadOS; рекомендации сведены к минимуму, в основном первоисточная фактура с версиями.

Охват версий: iOS/iPadOS 16.4 (27.03.2023) → 18.x → 26 (сентябрь 2025). По каждому факту указана версия, где известна. Спорное помечено «⚠».

## Проверка «якорей» из тикета

Все пять якорей подтверждены первоисточниками:

1. Web Push на iOS появился в 16.4 только для установленных на экран «Домой» веб-приложений — ✅ WebKit blog 13878.
2. `Notification.requestPermission` требует пользовательского жеста — ✅ WebKit blog 13878 + Apple Developer doc.
3. Февраль 2024 — объявление об удалении Home Screen web apps в ЕС в iOS 17.4; март 2024 — отмена — ✅ страница Apple «Update on apps distributed in the EU» (дословная цитата ниже) + пресса.
4. Vibration API на iOS не поддерживается — ✅ firt.dev + MDN.
5. Silent push недоступен — каждый пуш обязан показывать уведомление — ✅ WebKit «Meet Web Push» + Apple Developer doc («Safari revokes the push notification permission»).

---

## 1. Требования для Web Push

### 1.1. Базовое правило (iOS/iPadOS 16.4+, март 2023)

- Web Push работает **только в Home Screen web apps** — веб-приложениях, добавленных на экран «Домой» («Add to Home Screen», A2HS). Стек: Push API + Notifications API + Service Worker; транспорт — тот же **APNs**, что и у нативных приложений. Членство в Apple Developer Program не требуется; серверу нужен доступ к `https://*.push.apple.com` (SNI обязателен, HTTP/1.1 и HTTP/2). [webkit.org/blog/13878](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/), [developer.apple.com — Sending web push notifications](https://developer.apple.com/documentation/usernotifications/sending-web-push-notifications-in-web-apps-and-browsers)
- Запрос разрешения возможен **только в ответ на прямое действие пользователя** (тап по кнопке «Подписаться» внутри установленного PWA); iOS показывает системный промт, как для нативного приложения. Разрешения управляются пользователем в Настройки → Уведомления по каждому веб-приложению отдельно. Уведомления приходят на экран блокировки, в Центр уведомлений и на спаренные Apple Watch; интегрированы с Focus (фокусирование). [webkit.org/blog/13878](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/)
- Тот же код, что и для Chrome/Firefox (VAPID, стандартные RFC 8030/8291/8292), работает без изменений — WebKit сам просит использовать feature detection вместо browser detection. [webkit.org/blog/13878](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/), [webkit.org/blog/12945 «Meet Web Push»](https://webkit.org/blog/12945/meet-web-push/)

### 1.2. Где Web Push НЕ работает на iOS (16.4–26)

- **Safari-браузер (вкладка)** — подписка недоступна (в отличие от macOS, где в Safari 16.1+ push работает для обычных сайтов без установки).
- **Сторонние браузеры (Chrome/Firefox/Edge на iOS)** — push недоступен в браузере; доступен только в PWA, установленном из этого браузера.
- **WKWebView / in-app browsers** — недоступен.

Источники: таблица совместимости [firt.dev/notes/pwa-ios](https://firt.dev/notes/pwa-ios/) («Web Push ✅ 16.4 only for installed PWAs — not available in Safari or other browsers»); сводка Firtman (16.4 beta 1): «Not available in Safari / in third-party browsers / in Web Views; available on PWAs installed via A2HS from Safari share menu or third-party browsers' share menu» — цитировано по [webventures.rejh.nl](https://webventures.rejh.nl/blog/2023/ios-web-push-requires-install/).

### 1.3. Сторонние браузеры и DMA (ЕС)

- iOS/iPadOS **16.4**: сторонние браузеры **могут** предлагать «Add to Home Screen» из Share menu (нужен managed entitlement `com.apple.developer.web-browser`, WKWebView в `activityItems`, HTTP(S)-документ; Shared iPad исключён). Независимо от того, какой браузер установил PWA, оно открывается как веб-приложение при наличии манифеста с `display: standalone|fullscreen` (до iOS 26 — см. 1.5). Закладки без манифеста с 16.4 открываются в браузере по умолчанию. [webkit.org/blog/13878](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/)
- Опт-in конкретных браузеров: firt.dev фиксирует «16.4, if opted-in by the browser» [firt.dev/notes/pwa-ios](https://firt.dev/notes/pwa-ios/); OneSignal документирует установку из Safari/Chrome/Edge на iOS через Share → Add to Home Screen ([documentation.onesignal.com](https://documentation.onesignal.com/docs/en/web-push-for-ios), вендорский источник).
- **DMA-сага (ЕС)**: в бете iOS 17.4 (январь–февраль 2024) Apple убрала Home Screen web apps для пользователей ЕС. 15.02.2024 подтверждено ([9to5mac](https://9to5mac.com/2024/02/15/ios-17-4-web-apps-european-union/)). **1 марта 2024 Apple отменила решение**. Дословно со страницы Apple:
  > «UPDATE: Previously, Apple announced plans to remove the Home Screen web apps capability in the EU as part of our efforts to comply with the DMA… We have received requests to continue to offer support for Home Screen web apps in iOS and iPadOS, therefore we will continue to offer the existing Home Screen web apps capability in the EU. This support means **Home Screen web apps continue to be built directly on WebKit and its security architecture**, and align with the security and privacy model for native apps on iOS and iPadOS.»
  [developer.apple.com/support/dma-and-apps-in-the-eu](https://developer.apple.com/support/dma-and-apps-in-the-eu/)
- **Альтернативные движки (не WebKit)** в ЕС разрешены с iOS 17.4 / iPadOS 18 (авторизация Apple, критерии безопасности). Но Home Screen web apps в любом случае исполняются на WebKit (цитата выше). Первоисточного подтверждения, что крупный браузер реально поставляет альтернативный движок в ЕС, не найдено — ⚠ считать: Chrome/Firefox на iOS остаются на WebKit, PWA-логика едина. [developer.apple.com/support/dma-and-apps-in-the-eu](https://developer.apple.com/support/dma-and-apps-in-the-eu/)

### 1.4. Declarative Web Push (iOS/iPadOS 18.4, весна 2025)

Safari 18.4 принёс **Declarative Web Push** — новый способ доставки пушей без обязательного service worker:

- Появился `window.pushManager` — подписка без регистрации SW: `await window.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey })`. Если SW существует, подписка общая; **удаление SW-регистрации (в т.ч. ITP) больше не убивает подписку**.
- Тело пуша — стандартизованный JSON: `{"web_push": 8030, "notification": {"title": "…", "body": "…", "navigate": "https://…", "silent": false, "app_badge": "1", "lang", "dir"}}`. `web_push: 8030` — магическое значение-оптин; `title` обязателен; `navigate` обязателен (URL, открываемый по тапу); `app_badge` обновляет бейдж на Home Screen web apps.
- Если SW установлен, `push`-event диспатчится как раньше и может **заменить** «proposed notification»; при сбое/таймауте JS показывается декларативный fallback — «silent push penalty» (отзыв подписки) к декларативным сообщениям не применяется.
- Обратная совместимость: старые браузеры обработают тот же JSON через SW.
- Тестирование: iOS 18.4/iPadOS 18.4; на macOS пришло в Safari 18.5 (macOS 15.5, май 2025).

Источники: [webkit.org/blog/16535 «Meet Declarative Web Push»](https://webkit.org/blog/16535/meet-declarative-web-push/), [webkit.org/blog/16574 «WebKit Features in Safari 18.4»](https://webkit.org/blog/16574/webkit-features-in-safari-18-4/), [webkit.org/blog/16923 «WebKit Features in Safari 18.5»](https://webkit.org/blog/16923/webkit-features-in-safari-18-5/).

### 1.5. iOS/iPadOS 26 (сентябрь 2025)

- **«Every site can be a web app»**: любой сайт, добавленный на экран «Домой», по умолчанию открывается как веб-приложение, манифест не требуется («zero requirements for installability in Safari»). В шаге добавления есть переключатель «Open as Web App» (можно сохранить как закладку даже при наличии манифеста). Манифест по-прежнему даёт иконки/имя/поведение, но не является условием. Следствие для пушей: push-доступен любому сайту, добавленному на Home Screen. [webkit.org/blog/17333 «WebKit Features in Safari 26.0»](https://webkit.org/blog/17333/webkit-features-in-safari-26-0/)
- Для разработки: автоматический запуск/пауза инспекции Service Worker'ов в Web Inspector (удобно отлаживать push-обработчики). Тот же пост.

---

## 2. Флоу установки

### 2.1. Программного install API нет

- `beforeinstallprompt` — **не поддерживается** на iOS/iPadOS (все версии, включая 26). События `appinstalled` — тоже нет. Установка только жестом пользователя: **Share → «Add to Home Screen» → (имя) → «Add»**. Источники: [firt.dev/notes/pwa-ios](https://firt.dev/notes/pwa-ios/) (оба — ❌), [MDN BeforeInstallPromptEvent](https://developer.mozilla.org/en-US/docs/Web/API/BeforeInstallPromptEvent) (Limited availability, non-standard; Chrome/Android/Edge/Samsung Internet), [openpwa.net](https://openpwa.net/reference/installation/ios-add-to-home-screen).
- Установить можно из Safari и (с 16.4) из сторонних браузеров, оптинутых на A2HS. iOS 17 добавила A2HS из **Safari View Controller** (SFSafariViewController): если у сайта манифест с `display: standalone|fullscreen`, откроется как Home Screen web app. [webkit.org/blog/14445 «Safari 17.0»](https://webkit.org/blog/14445/webkit-features-in-safari-17-0/)
- Иконка: манифест `icons` — с iOS 15.4; `apple-touch-icon` в `<head>` **имеет приоритет** над манифестом; если иконок нет, с 16.4 генерируется монограмма (первая буква имени + цвет сайта; раньше — скриншот). SVG и maskable не поддерживаются. [webkit.org/blog/13878](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/), [firt.dev/notes/pwa-ios](https://firt.dev/notes/pwa-ios/)
- Сплэш-скрин: только через `<link rel="apple-touch-startup-image">` (причём работает при наличии meta `apple-mobile-web-app-capable`); манифест `background_color` iOS для сплэша не использует. [firt.dev/notes/pwa-ios](https://firt.dev/notes/pwa-ios/) ⚠ openpwa.net утверждает, что iOS генерирует сплэш из `background_color`+иконки манифеста — источники расходятся, надёжнее legacy-теги.
- Manifest `id` (16.4+): iOS конкатенирует `id` с именем, данным пользователем, — идентичность установки; используется для синхронизации Focus-настроек между устройствами и поддержки нескольких установок одного PWA. [webkit.org/blog/13878](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/)

### 2.2. Детект standalone-режима

- `window.navigator.standalone` — нестандартное iOS-свойство (boolean, read-only, с iOS 2.0), `true` когда страница запущена из Home Screen. [Apple — Configuring Web Applications](https://developer.apple.com/library/archive/documentation/AppleApplications/Reference/SafariWebContent/ConfiguringWebApplications/ConfiguringWebApplications.html)
- Медиазапрос `matchMedia('(display-mode: standalone)')` — стандартная замена с iOS 11.3 (firt.dev). [MDN display-mode](https://developer.mozilla.org/en-US/docs/Web/CSS/@media/display-mode), [firt.dev/notes/pwa-ios](https://firt.dev/notes/pwa-ios/)
- Манифест `display`: `browser`/`standalone` поддерживаются; `minimal-ui` → фолбэк в `browser`, `fullscreen` → фолбэк в `standalone`. Единственный путь к «полному» фулскрину — legacy meta `apple-mobile-web-app-status-bar-style: black-translucent`. [firt.dev/notes/pwa-ios](https://firt.dev/notes/pwa-ios/)
- Meta-теги: `apple-mobile-web-app-capable` (с 11.3 опционален — заменён `display: standalone`), `apple-mobile-web-app-title` (устарел), `apple-mobile-web-app-status-bar-style` (устарел с 15.0 → `theme-color` meta). [firt.dev/notes/pwa-ios](https://firt.dev/notes/pwa-ios/), [Apple — Configuring Web Applications](https://developer.apple.com/library/archive/documentation/AppleApplications/Reference/SafariWebContent/ConfiguringWebApplications/ConfiguringWebApplications.html)

### 2.3. Когда показывать инструкцию «Поделиться → На экран "Домой"»

- Типовой детект: `const isIos = /iphone|ipad|ipod/i.test(navigator.userAgent)`, `const isStandalone = navigator.standalone === true` → показывать кастомную инструкцию только если iOS && !standalone. Поскольку с 16.4 установка возможна из разных браузеров, инструкцию стоит адаптировать под браузер (иконка Share расположена по-разному). [openpwa.net](https://openpwa.net/reference/installation/ios-add-to-home-screen)
- Триггеры/частота: не на первом визите (показ после 2–3 визитов — openpwa); не прерывать пользовательский флоу (CTA ниже формы логина); давать dismiss и запоминать его; повторный показ только при смене отношения пользователя с продуктом (логин, покупка). [web.dev — Patterns for promoting PWA installation](https://web.dev/articles/promote-install) (рекомендации написаны под `beforeinstallprompt`-браузеры; для iOS применимы только UX-части, события не будет никогда).
- Проверять `display-mode`/standalone при каждом показе промо: после установки инструкция должна исчезнуть (точной нотификации об установке нет — `appinstalled` не поддерживается).

---

## 3. Запрос разрешения на пуши внутри установленного PWA

- **Только по жесту.** WebKit: «as long as that request is in response to direct user interaction». Apple doc уточняет: предоставьте кнопку; **вызывайте subscribe немедленно из обработчика жеста** («call the push subscription method immediately from the gesture's event handler code») — асинхронные пробросы могут потерять контекст жеста. [webkit.org/blog/13878](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/), [developer.apple.com](https://developer.apple.com/documentation/usernotifications/sending-web-push-notifications-in-web-apps-and-browsers)
- Состояния `Notification.permission`: `granted` / `denied` / `default` (неизвестно; приложение ведёт себя как при `denied`). [MDN Notification.permission](https://developer.mozilla.org/en-US/docs/Web/API/Notification/permission_static)
- Промт — системный, как у нативного приложения; после разрешения приложение появляется в Настройки → Уведомления с отдельными тумблерами (баннеры, звук(?), бейджи). [webkit.org/blog/13878](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/)
- **После отказа переспросить программно нельзя** — повторный `requestPermission()` не покажет промт (стандартное поведение Notifications API; MDN прямо советует «не беспокоить пользователя»). Путь восстановления — только руками пользователя в Настройки → Уведомления → [веб-приложение]; диплинка в настройки из веба нет. [MDN requestPermission](https://developer.mozilla.org/en-US/docs/Web/API/Notification/requestPermission_static), [webkit.org/blog/13878](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/)
- Подписка: `registration.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey })` → `PushSubscription` с `endpoint` и ключами `p256dh`/`auth`; endpoint+ключи хранятся на сервере привязанными к аккаунту. Endpoint — секрет (capability URL). [developer.apple.com](https://developer.apple.com/documentation/usernotifications/sending-web-push-notifications-in-web-apps-and-browsers), [MDN Push API](https://developer.mozilla.org/en-US/docs/Web/API/Push_API)
- `userVisibleOnly: true` **обязателен** на WebKit (см. 4.2).
- Подписка может протухнуть: push-сервис отвечает **410** («device token has expired») — такую подписку надо удалять на сервере и пересоздавать при следующем визите. [developer.apple.com](https://developer.apple.com/documentation/usernotifications/sending-web-push-notifications-in-web-apps-and-browsers)

---

## 4. Ограничения и известные баги

### 4.1. Бейджи (Badging API)

- iOS/iPadOS **16.4+**, только Home Screen web apps: `navigator.setAppBadge(n)` / `navigator.clearAppBadge()`; `setAppBadge(0)` = сброс. [webkit.org/blog/13878](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/), [MDN setAppBadge](https://developer.mozilla.org/en-US/docs/Web/API/Navigator/setAppBadge)
- Вызов работает **даже до выдачи разрешения** — пока приложение открыто на переднем плане или обрабатывает push в фоне; **отображение** бейджа на иконке включается автоматически вместе с разрешением на уведомления (т.е. отдельного запроса нет). Дальше пользователь управляет бейджем в Настройки → Уведомления. [webkit.org/blog/13878](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/), подтверждено [developer.apple.com](https://developer.apple.com/documentation/usernotifications/sending-web-push-notifications-in-web-apps-and-browsers) («Users can configure badging permissions … in iOS 16.4 or later») и [webkit.org/blog/13966](https://webkit.org/blog/13966/webkit-features-in-safari-16-4/) («Permission … is automatically granted when a user gives permission for notifications»).
- ⚠ Расхождение со спекой: MDN указывает `NotAllowedError`, если `PermissionStatus` не `granted` — WebKit вместо ошибки молча меняет счётчик; практически: оборачивать вызов в try/catch и feature-detection (`'setAppBadge' in navigator`).
- Declarative Web Push (18.4+): бейдж обновляется полем `"app_badge"` прямо в payload без JS. [webkit.org/blog/16535](https://webkit.org/blog/16535/meet-declarative-web-push/)

### 4.2. Silent push — нет

- WebKit **требует** `userVisibleOnly: true` и выполнение обещания: каждый входящий push обязан завершиться `registration.showNotification(...)`. Нарушение → **отзыв push-подписки**. Баги в SW, сеть или состояние устройства, помешавшие вовремя показать уведомление, приводили к отзыву даже без вины разработчика. [webkit.org/blog/12945](https://webkit.org/blog/12945/meet-web-push/) («Violations of the userVisibleOnly promise will result in a push subscription being revoked»), [developer.apple.com](https://developer.apple.com/documentation/usernotifications/sending-web-push-notifications-in-web-apps-and-browsers) («Safari doesn't support invisible push notifications… If you don't, Safari revokes the push notification permission for your site»), механика penalty и её снятие для декларативных сообщений — [webkit.org/blog/16535](https://webkit.org/blog/16535/meet-declarative-web-push/).
- Следствие для спеки: любой «технический» пуш (синк, инвалидция) на iOS должен нести видимое уведомление, либо использовать Declarative Web Push с `navigate`+`title`.

### 4.3. Звук и вибрация

- **Вибрация**: Vibration API (`navigator.vibrate`) на iOS/iPadOS не поддерживается (все версии). [firt.dev/notes/pwa-ios](https://firt.dev/notes/pwa-ios/) (❌), [MDN Vibration API](https://developer.mozilla.org/en-US/docs/Web/API/Vibration_API) (compat: Safari iOS — нет).
- **Звук**: в iOS 16.4 web push-уведомления приходили **тихо** — в настройках уведомлений веб-приложения не было тумблера «Звуки» (Stack Overflow [#75910612](https://stackoverflow.com/questions/75910612/no-sound-for-pwa-web-app-notifications-in-ios-16-4), аналогично [dev59](https://dev59.com/0F_rs4gBPY-HTNNji3o2) — «no sound, vibration, haptics or screen wake»). ⚠ Оба источника вторичные; прямого заявления Apple нет.
- Safari 17.0: «Fixed passing `NotificationOptions.silent`» и «Fixed Notifications API to default `silent` to the platform convention» — т.е. семантика `silent` приведена к платформенной. [webkit.org/blog/14445](https://webkit.org/blog/14445/webkit-features-in-safari-17-0/)
- В Declarative Web Push (18.4+) JSON поддерживает `"silent": false` — формальный рычаг есть. [webkit.org/blog/16535](https://webkit.org/blog/16535/meet-declarative-web-push/)
- Итоговый статус звука на iOS 18/26 первоисточником **не подтверждён** — см. «Открытые моменты».

### 4.4. Хранилище и eviction

- **ITP 7-day cap** (все script-writeable хранилища: IndexedDB, LocalStorage, SessionStorage, Cache API, **Service Worker registrations**, media keys) удаляются после 7 дней без пользовательского взаимодействия с сайтом **в браузерном контексте**. [webkit.org/tracking-prevention](https://webkit.org/tracking-prevention/)
- **Исключение: домен Home Screen web apps** — «The first-party domain of home screen web applications is exempt from ITP's 7-day cap on all script-writeable storage… In addition, the website data of home screen web applications is kept **isolated from Safari**». Т.е. для установленного PWA авто-эвикшена ITP нет, и данные PWA не разделяются с Safari. [webkit.org/tracking-prevention](https://webkit.org/tracking-prevention/) ⚠ openpwa.net утверждает обратное («web clip shares storage with the installing browser») — вторичный источник, противоречит официальному документу WebKit и firt.dev («Storage shared with Browser: ❌»); доверять WebKit.
- **Квота**: до iOS 17 — стартовый лимит 1 ГБ на origin (превышение = ошибка записи в Home Screen web apps / промт в Safari). С **iOS 17/Safari 17.0** квота вычисляется от общего объёма диска. [webkit.org/blog/14445](https://webkit.org/blog/14445/webkit-features-in-safari-17-0/)
- `navigator.storage.persist()` — поддерживается с 15.2 (firt.dev); `StorageManager.estimate` — quota API с 17 (firt.dev).
- Связка с пушами: удаление SW-регистрации ITP'шкой раньше убивало подписку — Declarative Web Push (18.4+) решает (подписка не привязана к SW). [webkit.org/blog/16535](https://webkit.org/blog/16535/meet-declarative-web-push/)
- Удаление иконки PWA с Home Screen удаляет его данные (web clip = приложение, не закладка, если был манифест). [MDN — Installing and uninstalling web apps](https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Guides/Installing)

### 4.5. Надёжность доставки, APNs-лимиты (серверная сторона)

Из [developer.apple.com — Sending web push notifications](https://developer.apple.com/documentation/usernotifications/sending-web-push-notifications-in-web-apps-and-browsers):

- Payload: максимум **4 КБ** (`413` / reason `PayloadTooLarge`).
- Заголовки: `TTL` (хранение недоставленного ≤ 30 дней; количество хранимых офлайн-уведомлений ограничено), `Authorization` (VAPID JWT; не обновлять чаще раза в час; exp ≤ 1 суток в будущем; ключ = ключу из `PushManager.subscribe`), `Content-Encoding`, `Topic` (≤ 32 символов, коалесцирование), `Urgency` (`very-low|low|normal|high`; `high` = попытка немедленной доставки).
- Транспорт: HTTP/1.1 pipelining ≤ 100 неподтверждённых запросов; HTTP/2 — не превышать `SETTINGS_MAX_CONCURRENT_STREAMS`; TLS SNI обязателен.
- Коды ответов: `201` OK; `400/403/404/405`; **`410` — токен протух (удалить подписку)**; `413`; `429` (TooManyRequests — много подряд на один device token); `500/503`. Reasons: `BadTtl`, `BadUrgency`, `BadWebPushRequest`, `BadWebPushTopic`, `VapidPkHashMismatch`, `IdleTimeout`, `BadAuthorizationHeader`, `BadJwtToken`, `BadVapidPublicKey`, `PayloadTooLarge`, `TooManyRequests`, `Shutdown`.
- Доставка не требует запущенного приложения: системный демон (`webpushd` на macOS; на iOS — системная интеграция APNs) будит SW. [webkit.org/blog/12945](https://webkit.org/blog/12945/meet-web-push/)
- Уведомления дедуплицируются/настраиваются Focus'ом; Focus-режимы синхронизируются между устройствами пользователя через Manifest ID + имя установки. [webkit.org/blog/13878](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/)

### 4.6. Зафиксированные баги и фиксы по версиям

- iOS 16.4 (GA): «тихие» уведомления без тумблера звука (⚠ вторичные источники, см. 4.3).
- **Safari 17.0**: «Fixed Web Push notifications not working in some cases by running the service worker before firing the activate event»; фиксы `NotificationOptions.silent` (см. 4.3). [webkit.org/blog/14445](https://webkit.org/blog/14445/webkit-features-in-safari-17-0/)
- **Safari 16.5**: фикс «Untitled» label на кнопке возврата в предыдущее приложение при открытии веб-приложения по ссылке. [webkit.org/blog/14154](https://webkit.org/blog/14154/webkit-features-in-safari-16-5/)
- ⚠ Уведомление показывается, даже когда PWA на переднем плане (iOS 17.2.1, [firebase-js-sdk#8002](https://github.com/firebase/firebase-js-sdk/issues/8002)) — один источник, баг-трекер.
- ⚠ Единичные жалобы на «случайное исчезновение» подписок на iOS 17–18 ([webscraft.org](https://webscraft.org/blog/pwa-pushspovischennya-na-ios-u-2026-scho-realno-pratsyuye?lang=en), вторичный обзор без первоисточника) — считать неподтверждённым; рекомендованная практика восстановления через 410 и re-subscribe при визите покрывает.

### 4.7. Ключевые отличия от Android (Chrome)

- Chrome Android: push работает **в браузере без установки**; на iOS — только установленный PWA. [webventures.rejh.nl](https://webventures.rejh.nl/blog/2023/ios-web-push-requires-install/), [firt.dev/notes/pwa-ios](https://firt.dev/notes/pwa-ios/)
- Установка: Android — WebAPK + `beforeinstallprompt` + mini-infobar; iOS — ручной жест через Share sheet, никакого API. [MDN BeforeInstallPromptEvent](https://developer.mozilla.org/en-US/docs/Web/API/BeforeInstallPromptEvent), [web.dev](https://web.dev/articles/promote-install)
- Background Sync / Periodic Background Sync / Background Fetch на iOS — ❌ (у Chrome Android есть). [firt.dev/notes/pwa-ios](https://firt.dev/notes/pwa-ios/)
- Vibration API: Android ✅, iOS ❌. [MDN Vibration API](https://developer.mozilla.org/en-US/docs/Web/API/Vibration_API)
- Link capturing (открытие ссылок в установленном PWA): iOS ❌ — «Only a push message can open an installed PWA»; внешние ссылки открываются в браузере/in-app browser. [firt.dev/notes/pwa-ios](https://firt.dev/notes/pwa-ios/) (на macOS Sequoia 15 для веб-приложений в Dock link capturing добавили — [webkit.org/blog/15865](https://webkit.org/blog/15865/webkit-features-in-safari-18-0/), на iOS нет).
- Манифест на iOS игнорирует: `shortcuts`, `screenshots`, `orientation`, `background_color`, `display_override`, `share_target`, `protocol_handlers`, маскабл/SVG-иконки. [firt.dev/notes/pwa-ios](https://firt.dev/notes/pwa-ios/)
- Хранилище установленного PWA на iOS изолировано от браузера и не подпадает под ITP-эвикшн (см. 4.4) — на Android WebAPK тоже изолирован, но там нет аналога ITP 7-day cap в принципе.

---

## 5. Что из «нативного ощущения» недостижимо на iOS и типовые компенсации

| Возможность | Статус на iOS/iPadOS (версия) | Компенсация |
|---|---|---|
| Install prompt API (`beforeinstallprompt`) | ❌ (все версии, включая 26) | Кастомная инструкция «Share → Add to Home Screen» с детектом `navigator.standalone`/`display-mode`, показ после 2–3 визитов, dismiss-логика |
| Push в браузере без установки | ❌ (16.4–26) | Воронка «установи → включи уведомления»; на macOS установка не нужна — разные UX-ветки по платформам |
| Background Sync | ❌ | Синк при открытии приложения; пуш как триггер «зайти» (но пуш обязан быть видимым) |
| Periodic Background Sync | ❌ | То же; Declarative Web Push (18.4+) для регулярных уведомлений без SW |
| Background Fetch | ❌ | Предзагрузка при запуске; агрессивный кэш |
| Silent/данные-only push | ❌ — каждый пуш видимый, иначе отзыв подписки | Declarative Web Push; объединение событий через `Topic`; `Urgency` |
| Vibration API | ❌ | Звук/баннер ОС (на что способно платформенное уведомление) |
| Звук уведомления | ⚠ исторически тихие на 16.4; `silent`-семантика с 17.0; точный статус на 18/26 не подтверждён | Бейдж + баннер; не закладываться на звук в критичных сценариях |
| Lock screen / Notification Center / Apple Watch / Focus | ✅ (16.4+) — как у нативных приложений | — |
| Link capturing (ссылки → PWA) | ❌ | Push с `navigate`/глубокими ссылками внутрь scope; универсальные ссылки невозможны для PWA |
| Splash screen из манифеста | ❌ (`background_color` игнорируется) | `apple-touch-startup-image` (+ `apple-mobile-web-app-capable`) |
| Маскабл/SVG иконки | ❌ | PNG `apple-touch-icon` 180×180 (приоритет над манифестом) |
| `appinstalled` событие | ❌ | Эвристика: смена `display-mode` на standalone при следующем запуске |
| Несколько установок одного PWA | ✅ (16.4+, Manifest ID + имя) | Учитывать при проектировании подписок: подписка привязана к установке |
| Screen Wake Lock, Screen Orientation (partial), Web Codecs, User Activation | ✅ с 16.4 | — |

Источники таблицы: [firt.dev/notes/pwa-ios](https://firt.dev/notes/pwa-ios/), [webkit.org/blog/13878](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/), [webkit.org/blog/13966](https://webkit.org/blog/13966/webkit-features-in-safari-16-4/), [webkit.org/blog/16535](https://webkit.org/blog/16535/meet-declarative-web-push/), [MDN Installing](https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Guides/Installing).

---

## Открытые/спорные моменты

1. **Звук в пушах на актуальных iOS (18.x/26)** — не подтверждён первоисточником. Подтверждено: 16.4 — тихие (⚠ вторичные), 17.0 — фиксы `silent`, 18.4 — поле `"silent"` в Declarative JSON. Нужен эксперимент на реальном устройстве или явное заявление Apple.
2. **Альтернативные движки в ЕС**: фреймворки авторизации есть с iOS 17.4/iPadOS 18, но shipped-статус не-WebKit браузеров первоисточником не подтверждён. Гарантия: Home Screen web apps исполняются на WebKit независимо от движка устанавливающего браузера (заявление Apple).
3. **Расхождение openpwa.net vs WebKit/firt.dev** по разделяемому хранилищу (web clip + браузер) и по сплэшу из `background_color` — принята версия WebKit/firt.dev; openpwa помечен как недостоверный в этих пунктах.
4. **«Случайное исчезновение» подписок на iOS 17–18** — ⚠ не подтверждено вторым независимым источником (только обзорная статья без первоисточника). Митигация через обработку 410 и re-subscribe.
5. **Показ уведомления при открытом PWA** (форграунд) — ⚠ один источник (GitHub issue firebase-js-sdk#8002, iOS 17.2.1).
6. **Декларативный `app_badge` без разрешения на уведомления** — поведение не описано; бейдж на иконке исторически требует разрешения уведомлений.
7. **iPadOS-специфика DMA**: iPadOS получила EU-изменения с 18.0 (не 17.4); на поведение PWA/пушей это не влияет, но для точности хронологии.

## Источники

Первоисточники:
- WebKit blog: [Web Push for Web Apps on iOS and iPadOS (16.02.2023)](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/) · [WebKit Features in Safari 16.4](https://webkit.org/blog/13966/webkit-features-in-safari-16-4/) · [Meet Web Push (06.2022)](https://webkit.org/blog/12945/meet-web-push/) · [Safari 16.5](https://webkit.org/blog/14154/webkit-features-in-safari-16-5/) · [Safari 17.0](https://webkit.org/blog/14445/webkit-features-in-safari-17-0/) · [Safari 17.4](https://webkit.org/blog/15068/webkit-features-in-safari-17-4/) · [Safari 18.0](https://webkit.org/blog/15865/webkit-features-in-safari-18-0/) · [Meet Declarative Web Push (05.2025)](https://webkit.org/blog/16535/meet-declarative-web-push/) · [Safari 18.4](https://webkit.org/blog/16574/webkit-features-in-safari-18-4/) · [Safari 26.0](https://webkit.org/blog/17333/webkit-features-in-safari-26-0/)
- WebKit: [Tracking Prevention in WebKit (ITP)](https://webkit.org/tracking-prevention/)
- Apple Developer: [Sending web push notifications in web apps and browsers](https://developer.apple.com/documentation/usernotifications/sending-web-push-notifications-in-web-apps-and-browsers) · [Update on apps distributed in the EU (DMA)](https://developer.apple.com/support/dma-and-apps-in-the-eu/) · [Configuring Web Applications (Safari Web Content)](https://developer.apple.com/library/archive/documentation/AppleApplications/Reference/SafariWebContent/ConfiguringWebApplications/ConfiguringWebApplications.html)
- MDN: [Push API](https://developer.mozilla.org/en-US/docs/Web/API/Push_API) · [PushManager](https://developer.mozilla.org/en-US/docs/Web/API/PushManager) · [Notification.permission](https://developer.mozilla.org/en-US/docs/Web/API/Notification/permission_static) · [Notification.requestPermission](https://developer.mozilla.org/en-US/docs/Web/API/Notification/requestPermission_static) · [setAppBadge](https://developer.mozilla.org/en-US/docs/Web/API/Navigator/setAppBadge) · [BeforeInstallPromptEvent](https://developer.mozilla.org/en-US/docs/Web/API/BeforeInstallPromptEvent) · [display-mode](https://developer.mozilla.org/en-US/docs/Web/CSS/@media/display-mode) · [Vibration API](https://developer.mozilla.org/en-US/docs/Web/API/Vibration_API) · [Installing and uninstalling web apps](https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Guides/Installing)
- web.dev: [Patterns for promoting PWA installation](https://web.dev/articles/promote-install)

Вторичные (использованы для подтверждения/контекста):
- [firt.dev — iOS PWA Compatibility](https://firt.dev/notes/pwa-ios/) · [webventures.rejh.nl — Web Push on iOS requires installing the web app](https://webventures.rejh.nl/blog/2023/ios-web-push-requires-install/) · [openpwa.net — iOS Add to Home Screen](https://openpwa.net/reference/installation/ios-add-to-home-screen) · [9to5mac — iOS 17.4 removes Home Screen web apps in the EU](https://9to5mac.com/2024/02/15/ios-17-4-web-apps-european-union/) · [AppleInsider — Apple reverses course (01.03.2024)](https://appleinsider.com/articles/24/03/01/apple-reverses-course-on-death-of-progressive-web-apps-in-eu) · [Stack Overflow #75910612 (звук)](https://stackoverflow.com/questions/75910612/no-sound-for-pwa-web-app-notifications-in-ios-16-4) · [firebase-js-sdk#8002 (форграунд)](https://github.com/firebase/firebase-js-sdk/issues/8002) · [OneSignal — Web Push for iOS](https://documentation.onesignal.com/docs/en/web-push-for-ios)
