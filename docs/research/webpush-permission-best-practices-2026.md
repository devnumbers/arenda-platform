# Web Push: permission flow — best practices 2025/2026

Дата: 2026-10-01
Вход для: research-тикет [#1026](https://github.com/devnumbers/arenda-platform/issues/1026) карты пушей [#1024](https://github.com/devnumbers/arenda-platform/issues/1024). Флоу: системный промпт новому юзеру сразу при первом входе; управление пушами только на экране настроек; мастер-тумблер per-device; выключение = жёсткая отписка (`pushManager.unsubscribe()` + DELETE на бекенде); denied → красная инструкция; iOS без установленного PWA → красный текст снизу.

Метод: первоисточники — MDN, web.dev / developer.chrome.com / blog.google / blog.chromium.org, спецификация W3C Push API, WebKit/Apple docs; вторичные — для контекста, помечены. Спорное/неподтверждённое помечено «⚠». Протокольная фактура (RFC, payload, TTL, VAPID, Go-реализация) и iOS-фактура уже собраны и здесь не дублируются: [web-push-go.md](web-push-go.md), [ios-pwa-push.md](ios-pwa-push.md), [pwa-manifest-installability.md](pwa-manifest-installability.md). Этот документ — про permission-флоу и его поведение в браузерах по состоянию на октябрь 2026.

---

## 1. Состояния permission и базовая механика

- `Notification.permission` ∈ `granted | denied | default`; `default` = «решение неизвестно, приложение ведёт себя как при denied» — уведомления не показываются, но запросить разрешение можно. Свойство — на **origin** (не на сайт/аккаунт), только secure context, доступно и в воркерах. [MDN Notification.permission](https://developer.mozilla.org/en-US/docs/Web/API/Notification/permission_static)
- `Notification.requestPermission()` — Promise-форма (callback-форма deprecated). Современная практика: сначала проверить `Notification.permission`, вызывать `requestPermission()` только при `default` — «если пользователь отказал, не нужно больше его беспокоить». [MDN requestPermission](https://developer.mozilla.org/en-US/docs/Web/API/Notification/requestPermission_static)
- **Повторный вызов**: при `default` — окно может показаться снова (см. оговорки Chrome в §3); при `denied` — окно не показывается никогда, promise возвращает `denied`; программно переспросить нельзя, разблокировка — только руками пользователя в настройках браузера/ОС. [MDN requestPermission](https://developer.mozilla.org/en-US/docs/Web/API/Notification/requestPermission_static), [web.dev — subscribing a user](https://web.dev/articles/push-notifications-subscribing-a-user)
- **Cross-origin iframe**: Chrome и Firefox больше не позволяют запрашивать notification-permission из cross-origin iframe («with other browsers to follow»). Для нас неактуально (первый-party фронт), но запрещает любые схемы «попросить чужой виджет». [MDN requestPermission](https://developer.mozilla.org/en-US/docs/Web/API/Notification/requestPermission_static)
- **Сайд-эффект subscribe()**: `pushManager.subscribe()` при `default` сам вызывает permission-промпт. Чтобы держать контроль над UX, явно вызывать `requestPermission()` первым и подписываться только при `granted`. [web.dev — subscribing a user](https://web.dev/articles/push-notifications-subscribing-a-user)
- Современный способ читать состояние и **реагировать на изменения**: `navigator.permissions.query({ name: 'notifications' })` → `permissionStatus.state` + `onchange`. Нужно обязательно (§3.3). [MDN requestPermission](https://developer.mozilla.org/en-US/docs/Web/API/Notification/requestPermission_static), [developer.chrome.com — Chrome 155](https://developer.chrome.com/blog/notification-prompts-android)

## 2. User activation (жест): требования по браузерам

Сводка на октябрь 2026 — когда системное окно реально появится, если `requestPermission()` вызван **без** пользовательского жеста:

| Браузер | Промпт без жеста | Что происходит | Источник |
|---|---|---|---|
| Chrome desktop | Да, технически возможен (жёсткого запрета не задокументировано) | Окно может показаться, но см. suppression в §3; Chromium прямо требует «only in response to a user action» | [Chromium issue 41037127](https://issues.chromium.org/41037127), [chromium.org design doc](https://www.chromium.org/developers/design-documents/desktop-notifications/api-specification) |
| Chrome / Android (со 155, июль 2026) | Неблокирующий промпт; без решения таймаутится в `default` | Новая модель UX, см. §3.3 | [developer.chrome.com](https://developer.chrome.com/blog/notification-prompts-android) |
| Firefox 72+ (янв 2020) | **Нет** | Promise rejected: «The Notification permission may only be requested from inside a short running user-generated event handler»; подавленные запросы — иконка в address bar | [Mozilla Hacks](https://hacks.mozilla.org/2019/11/upcoming-notification-permission-changes-in-firefox-72), [OneSignal SDK #540](https://github.com/OneSignal/OneSignal-Website-SDK/issues/540) |
| Safari macOS 16.1+ | Нет | Только по жесту | [Apple Developer](https://developer.apple.com/documentation/usernotifications/sending-web-push-notifications-in-web-apps-and-browsers) |
| iOS/iPadOS (PWA, 16.4+) | **Нет** | Только по прямому жесту в установленном PWA; вне PWA пуши недоступны вовсе | [WebKit blog 13878](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/), [Apple Developer](https://developer.apple.com/documentation/usernotifications/sending-web-push-notifications-in-web-apps-and-browsers) |

Детали и оговорки:

- **Apple требует вызывать подписку немедленно из обработчика жеста** («call the push subscription method immediately from the gesture's event handler») — асинхронные пробросы (`await` чего-либо до вызова) могут потерять контекст жеста. [developer.apple.com](https://developer.apple.com/documentation/usernotifications/sending-web-push-notifications-in-web-apps-and-browsers)
- **Chrome desktop**: жёсткого «отказать без activation» в задокументированных изменениях нет — Lighthouse до сих пор ловит «Requests the notification permission on page load» как best-practice-нарушение (то есть вызов на load всё ещё возможен и детектится), а телеметрия web.dev показывает, что 77% desktop permission-промптов появляются без сигнала намерения. Но: [Lighthouse notification-on-start](https://developer.chrome.com/docs/lighthouse/best-practices/notification-on-start), [web.dev permissions-best-practices](https://web.dev/articles/permissions-best-practices), [Chromium issue 41037127](https://issues.chromium.org/41037127). ⚠ Точная граница принуждения не оформлена как shipped-change; в пререндеренных (не активированных) документах `requestPermission()` и конструктор `Notification()` просто не работают (Chrome 138+). [MDN Speculation Rules API](https://developer.mozilla.org/en-US/docs/Web/API/Speculation_Rules_API)
- **Transient activation не переживает навигацию**: жест кнопки «Войти» живёт в контексте текущей страницы и расходуется; после редиректа на пост-логинный экран активации нет. Следствие: «автоматический промпт сразу после логина» во всех браузерах = запрос без жеста. ⚠ Поведение «промпт отменяется при навигации, если успели вызвать до неё» — общепризнанное практикой, но первоисточником не закреплено.

## 3. Chrome: suppression, новая UX-модель Android, автоотзыв

### 3.1. Quieter / predictive permission UI (с Chrome 80, 2020)

Chrome заменяет модальный промпт «тихим» UI (иконка в address bar, окно появляется только после клика по ней) для: пользователей, которые обычно отклоняют permission-запросы, и сайтов с низкой долей согласий; сайты с навязчивыми/обманными промптами блокируются целиком. Механика прогнозная и недокументированная в деталях: промпт может **не показаться вовсе**, permission остаётся `default`. [blog.chromium.org (Chrome 80)](https://blog.chromium.org/2020/01/introducing-quieter-permission-ui-for.html), [support.google.com](https://support.google.com/chrome/answer/3220216), условия quieter UI — ⚠ по вендорским разборам ([AWeber](https://docs.aweber.com/web-push-notifications/web-push-notifications/what-is-the-quiet-permission-ui-on-chrome-and-fire), [Insider One](https://insiderone.com/quieter-permission-ui-for-web-push)).

### 3.2. Автоотзыв разрешений (октябрь 2025)

Chrome автоматически **отбирает** notification permission у сайтов с очень низкой вовлечённостью при высоком объёме уведомлений; **установленные web apps исключены** из отзыва. Пользователя уведомляют; вернуть можно через Safety Check или повторным включением на сайте. Статистика Google: меньше 1% всех пушей получают любое взаимодействие. [blog.google/chromium/automatic-notification-permission](https://blog.google/chromium/automatic-notification-permission)

Следствие: `granted` — не навсегда; при каждом старте приложения проверять актуальное состояние permission и живость подписки, а не кэшировать «включённые пуши» во фронтее. ⚠ Возвращается ли permission после автоотзыва в `default` (т.е. можно ли снова показать промпт) — прямо не указано; по тексту («visiting the site and re-enabling») — да.

### 3.3. Chrome 155 на Android: неблокирующий промпт (июль 2026)

Существенное изменение модели, влияющее на наш код напрямую ([developer.chrome.com — Lighter notification prompts on Android](https://developer.chrome.com/blog/notification-prompts-android)):

- Модальный промпт заменён неблокирующим; если пользователь не решил — запрос **таймаутится, permission остаётся `default`**.
- В Site Settings появляется отдельная точка входа для подписки на уведомления — пользователь, пропустивший промпт, может включить позже в любой момент.
- **Обязательный паттерн для сайтов**: не трактовать `default`-результат `await requestPermission()` как финальный отказ и слушать смену состояния:

```js
navigator.permissions.query({ name: 'notifications' }).then((permissionStatus) => {
  permissionStatus.onchange = () => {
    if (permissionStatus.state === 'granted') subscribeToPushNotifications();
  };
});
```

Без `onchange` мы пропустим «поздний оптин» из Site Settings и никогда не подпишем такого пользователя.

### 3.4. Что реально рекомендует web.dev (vs решение владельца)

Канон [web.dev/articles/permissions-best-practices](https://web.dev/articles/permissions-best-practices):

- Никогда не запрашивать permission при загрузке страницы или без пользовательского взаимодействия — по телеметрии Chrome, 77% desktop permission-промптов показываются без сигнала намерения, и соглашаются лишь на 12% из них; после взаимодействия — 30%.
- Просить **в контексте**, когда пользователь понимает, зачем (подписаться на тип уведомления → промпт).
- Pre-prompt (собственное окно-объяснение перед системным) повышает конверсию.
- Избегать попадания в blocked-состояние: его трудно реверсить, поэтому не запрашивать, когда согласие маловероятно.

Решение владельца (промпт сразу при первом входе, без in-app диалога) прямо противоречит этой рекомендации. Владелец риск осознал; технические последствия — в §8.

## 4. iOS / iPadOS (кратко; полная фактура — [ios-pwa-push.md](ios-pwa-push.md))

- Пуши — **только в Home Screen web apps**, iOS/iPadOS 16.4+; в Safari-вкладке и сторонних браузерах подписки нет. iOS 26 (сентябрь 2025) смягчил установку («every site can be a web app», манифест не обязателен), но добавление на экран «Домой» по-прежнему условие доступа к пушам. [WebKit blog 13878](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/), [WebKit blog 17333](https://webkit.org/blog/17333/webkit-features-in-safari-26-0/)
- `requestPermission` — **только по жесту**; iOS показывает системный промпт как у нативного приложения. После отказа — только Настройки → Уведомления. [WebKit blog 13878](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/)
- **Каждый пуш обязан показывать уведомление**, иначе Safari отзывает подписку (silent-push penalty; к Declarative Web Push с 18.4 не применяется). [WebKit blog 12945](https://webkit.org/blog/12945/meet-web-push/), [blog 16535](https://webkit.org/blog/16535/meet-declarative-web-push/)
- Для «красного текста снизу»: детект `iOS && !standalone` (`navigator.standalone` / `matchMedia('(display-mode: standalone)')`) и инструкция Share → «На экран "Домой"». С 16.4 установка возможна и из сторонних браузеров (Share menu), инструкцию стоит формулировать нейтрально к браузеру. [ios-pwa-push.md §2.3](ios-pwa-push.md)

## 5. Firefox, Edge, Samsung Internet

- **Firefox 72+**: без жеста — rejected promise (точный текст ошибки в §2); подавленный запрос — иконка в address bar; пользователь управляет per-site в about:preferences#privacy → Permissions → Notifications. После deny программно не переспросить. Push работает в обычном окне (в т.ч. Android) без установки PWA. [Mozilla Hacks](https://hacks.mozilla.org/2019/11/upcoming-notification-permission-changes-in-firefox-72), [MDN](https://developer.mozilla.org/en-US/docs/Web/API/Notification/requestPermission_static), [firt.dev](https://firt.dev/notes/pwa-ios/)
- **Edge**: Chromium — семантика Push API/Notifications та же; UI промптов свои. Chrome-специфичные UX-слои (quieter UI, автоотзыв окт. 2025, неблокирующий промпт Chrome 155) — это фичи Google Chrome; их наличие в Edge/Samsung Internet не подтверждено ⚠ — код обязателен к поведению «окно не показалось/permission остался default» в любом браузере.
- **Samsung Internet**: Push API поддерживается с версии 4.0 (Chromium); встречаются кейсы, когда системные Android-настройки уведомлений для браузера глушат показ — диагностировать через Settings → Apps → [браузер] → Notifications. [developer.samsung.com](https://developer.samsung.com/browser/android/web-developer-guide.html), ⚠ вторичные ([Modern Web Weekly](https://modernwebweekly.substack.com/p/when-push-notifications-dont-show))
- **Вывод по флоу**: отдельные ветки для Edge/Samsung в спеке не нужны — достаточно универсальной обработки трёх состояний и отсутствия окна.

## 6. Гигиена подписок: unsubscribe, 404/410, переподписка

Протокол и серверная часть детально — [web-push-go.md §4–5](web-push-go.md); здесь клиентская семантика по [спецификации Push API](https://w3c.github.io/push-api/):

- `pushManager.unsubscribe()` **деактивирует подписку**: UA обязан отправить запрос на деактивацию push-сервису («UA MUST NOT deliver any further push messages for the push subscription»); при сетевой ошибке UA обязан ретраить деактивацию разумное время. Обещание резолвится `false`, если подписка уже деактивирована. Endpoint деактивированной подписки **не переиспользуется** для новой. Разрегистрация service worker'а деактивирует подписку без window-accessible scope. [W3C Push API](https://w3c.github.io/push-api/)
- Серверная гигиена: при отправке 404/410 — подписку удалить из БД (410 — «gone», включая случай, когда клиент сам вызвал `unsubscribe()`); «вечная» на бумаге подписка (`expirationTime: null`) на практике эвиктится браузером (долго не было пушей, неиспользование). [web-push-go.md §4](web-push-go.md), [web.dev](https://web.dev/articles/push-notifications-subscribing-a-user)
- **Переподписка**: событие `pushsubscriptionchange` поддерживается плохо (Chrome только с 138; на iOS не поддерживается) — страховать re-subscribe при старте приложения + сверкой endpoint/ключа. Классический web.dev-паттерн «re-subscribe при каждом визите» **конфликтует с мастер-тумблером** — см. §8. [web-push-go.md §7](web-push-go.md)
- Payload ≤ 3993 байт (aes128gcm), TTL/urgency/topic, 429 + `Retry-After` — без изменений: [web-push-go.md §1.2, §5](web-push-go.md).

## 7. VAPID: subject и ротация

- `sub` в VAPID JWT — контакт `mailto:` или `https:` (RFC 8292 §2.1); Mozilla autopush требует `sub` при наличии VAPID, Apple отвечает `BadJwtToken` на кривой формат. **Ротация ключей ломает все живые подписки** (подписка привязана к `applicationServerKey`; отправка с новым ключом получает 403) — менять только при компрометации, плавно (RFC 8292 §5). Детали: [web-push-go.md §1.1](web-push-go.md).
- Публичный ключ фронту отдавать runtime-endpoint'ом (`GET /push/vapid-public-key`), не зашивать в бандл: рассинхрон ключа с БД подписок фатален. [web-push-go.md §6](web-push-go.md)

## 8. Что это значит для спеки Рентли

Прямые следствия для принятого флоу, по пунктам решения владельца.

### 8.1. Системный промпт новому юзеру сразу при первом входе

- **Жест потерян**: после редиректа логина активации нет (§2). Реальный расклад: Chrome — окно возможно, но с конверсией ~12% и риском прогностического suppression (§3.1); Firefox — rejected promise (обязателен try/catch); iOS — окно не покажется, и на iOS это вообще вне PWA невозможно (§4).
- Спека должна задать: авто-вызов выполняется **один раз** (флаг «уже просили» per-устройство), оборачивается в try/catch, ошибка не показывается пользователю.
- Результат `default` (включая таймаут Chrome 155 и suppression) — **не ошибка и не отказ**: тумблер остаётся выключенным, повторная попытка — только из жеста.
- Обязателен `permissions.query({name:'notifications'}).onchange` (§3.3) — иначе теряем «поздний оптин» из Site Settings.
- Каждый показанный и отклонённый промпт снижает site acceptance rate → Chrome переводит сайт в quieter UI **для всех будущих запросов**, включая жестовые. Это главная системная цена авто-промпта: хуже не только этот пользователь, но и вся следующая воронка.
- Компромисс, который стоит предложить владельцу без ломки решения: авто-вызов оставить, но дублировать жестовым путём — кнопка/тумблер на экране настроек как надёжный основной сценарий (повторный запрос из жеста при `default` разрешён всегда, §8.2).

### 8.2. Повторный запрос при default из жеста кнопки

- Разрешён во всех браузерах; окно может не появиться (Chrome quieter/predictive; Chrome 155 — неблокирующий с таймаутом в `default`). Обрабатывать все три исхода; `granted` → сразу `pushManager.subscribe()`; `default` → оставляем тумблер выключенным + `onchange` ловит поздний оптин; `denied` → §8.3.
- Вызов — немедленно из обработчика клика (требование Apple: без `await`-прокидок до вызова; у нас `await` на `requestPermission()` до `subscribe()` допустим). [developer.apple.com](https://developer.apple.com/documentation/usernotifications/sending-web-push-notifications-in-web-apps-and-browsers)

### 8.3. denied → красный текст-инструкция

- Переспросить программно невозможно (§1). Инструкция по браузерам: Chrome — иконка в address bar / Site Settings → Notifications; iOS PWA — Настройки → Уведомления → [приложение]; Firefox — about:preferences → Permissions → Notifications; диплинка в настройки из веба не существует.
- Состояние проверять через `navigator.permissions.query` и перерисовывать UI по `onchange`: пользователь разблокировал в соседней вкладке/настройках → через `onchange` состояние станет `granted` и можно подписать без перезагрузки страницы.
- `Notification.permission` синхронен и origin-scoped — обновление из другой вкладки тоже видно, но `onchange`-механизм надёжнее и обязателен в силу Chrome 155 в любом случае.

### 8.4. iOS без установленного PWA → красный текст снизу

- Детект: `/iphone|ipad|ipod/` (или `navigator.platform`/UA-data) && `!navigator.standalone` && `!matchMedia('(display-mode: standalone)').matches`. Текст — инструкция «Поделиться → На экран "Домой"».
- После установки промпт запрашиваем жестом при первом запуске PWA (первоисточник требований — §4). Манифест у нас есть (`display: standalone`), значит добавленный сайт откроется как web app и пуши доступны: [pwa-manifest-installability.md §2](pwa-manifest-installability.md).

### 8.5. Выключение мастера = жёсткая отписка

- Последовательность: `DELETE /push/subscriptions` (бекенд) + `pushManager.unsubscribe()` (деактивация endpoint у push-сервиса, §6). Порядок некритичен; `unsubscribe()` может не успеть офлайн — страховка: сервер сам чистит подписки по 404/410, а при следующем включении `subscribe()` создаст новую подписку (endpoint не переиспользуется, upsert по endpoint это покрывает).
- **Ключевой грабль**: стандартный паттерн web.dev «re-subscribe при каждом старте» молча включит пуши обратно пользователю, который выключил мастер. Спека обязана хранить **намерение** пользователя (выключено/включено — локально и на бекенде per-device) и никогда не переподписывать автоматически при выключенном мастере. Авто-восстановление (`subscribe()` при старте) допустимо только при включённом мастере **и** отсутствии действующей подписки.
- Автоотзыв Chrome (§3.2) — ещё одна причина проверять состояние при старте, а не доверять кэшу: мастер «включён», а подписки/permission уже нет → тихо привести UI в соответствие (тумблер виден включённым только при живой подписке).

### 8.6. Общее

- Тумблер per-device соответствует протоколу: подписка — на браузер/устройство (`push_subscriptions` per endpoint), fan-out по всем устройствам owner'а — модель бекенда из [web-push-go.md §6](web-push-go.md) совпадает.
- Все обращения к permission/подписке — только после feature detection (`'serviceWorker' in navigator && 'PushManager' in window`) и только в secure context.

## Открытые/спорные моменты

1. ⚠ **Жёсткое требование user activation в Chrome**: issue 41037127 существует, формального «shipped»-анонса нет; Chrome может в любом релизе закрыть окно без жеста. Флоу проектировать так, будто запрос без жеста **не работает** (это же и есть best practice) — тогда любое ужесточение Chrome ничего не ломает.
2. ⚠ **Условия quieter/predictive UI** — вторичные источники; первичный пост 2020 доступен только метаданными. Факт suppression и сохранения `default` подтверждён направлением Chrome 155/автоотзыва.
3. ⚠ **Состояние permission после автоотзыва Chrome** (окт. 2025): `default` или `denied` — не указано; из текста («re-enable by visiting the site») следует `default`.
4. ⚠ **Chrome Android < 155 без жеста** — поведенческих первоисточников не найдено; моделированием «default без окна» покрыто.
5. ⚠ **Наличие Chrome-фич (quieter UI, автоотзыв, неблокирующий промпт) в Edge/Samsung Internet** — не проверено; универсальная обработка состояний делает это неважным для кода.
6. **Промпт отменяется навигацией** (если вызывать в обработчике кнопки логина перед redirect) — общепризнанное практикой поведение, первоисточник не найден.

## Источники

Первоисточники:
- MDN: [Notification.permission](https://developer.mozilla.org/en-US/docs/Web/API/Notification/permission_static) · [Notification.requestPermission](https://developer.mozilla.org/en-US/docs/Web/API/Notification/requestPermission_static) · [Speculation Rules API (activation-ограничения, Chrome 138)](https://developer.mozilla.org/en-US/docs/Web/API/Speculation_Rules_API)
- W3C: [Push API (unsubscribe/деактивация, lifetime)](https://w3c.github.io/push-api/)
- Chrome: [Lighter notification prompts on Android (Chrome 155, 07.2026)](https://developer.chrome.com/blog/notification-prompts-android) · [Chromium blog: quieter permission UI (Chrome 80)](https://blog.chromium.org/2020/01/introducing-quieter-permission-ui-for.html) · [blog.google: automatic notification permission revocation (10.2025)](https://blog.google/chromium/automatic-notification-permission) · [Chromium issue 41037127](https://issues.chromium.org/41037127) · [Desktop notifications API design doc](https://www.chromium.org/developers/design-documents/desktop-notifications/api-specification) · [Lighthouse: notification-on-start](https://developer.chrome.com/docs/lighthouse/best-practices/notification-on-start) · [support.google.com: notifications](https://support.google.com/chrome/answer/3220216)
- web.dev: [Web permissions best practices (телеметрия 77%/12%/30%)](https://web.dev/articles/permissions-best-practices) · [Subscribing a user](https://web.dev/articles/push-notifications-subscribing-a-user)
- WebKit/Apple: [Web Push for Web Apps on iOS and iPadOS](https://webkit.org/blog/13878/web-push-for-web-apps-on-ios-and-ipados/) · [WebKit Features in Safari 26.0](https://webkit.org/blog/17333/webkit-features-in-safari-26-0/) · [Meet Declarative Web Push](https://webkit.org/blog/16535/meet-declarative-web-push/) · [Apple: Sending web push notifications](https://developer.apple.com/documentation/usernotifications/sending-web-push-notifications-in-web-apps-and-browsers)
- Mozilla: [Firefox 72 notification permission changes](https://hacks.mozilla.org/2019/11/upcoming-notification-permission-changes-in-firefox-72)
- Samsung: [Web Development Guide](https://developer.samsung.com/browser/android/web-developer-guide.html)

Вторичные (контекст): [OneSignal SDK #540 (текст ошибки Firefox)](https://github.com/OneSignal/OneSignal-Website-SDK/issues/540) · [AWeber: quiet permission UI](https://docs.aweber.com/web-push-notifications/web-push-notifications/what-is-the-quiet-permission-ui-on-chrome-and-fire) · [Insider One: quieter permission UI](https://insiderone.com/quieter-permission-ui-for-web-push) · [Modern Web Weekly: debugging Samsung delivery](https://modernwebweekly.substack.com/p/when-push-notifications-dont-show)

Внутренние (фактура, не дублируется здесь): [web-push-go.md](web-push-go.md) — протокол, сервер, VAPID, retry; [ios-pwa-push.md](ios-pwa-push.md) — iOS-платформа; [pwa-manifest-installability.md](pwa-manifest-installability.md) — манифест и installability.
