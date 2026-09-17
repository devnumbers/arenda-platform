# Research #725: Лучшие практики безопасности сессий (sliding, ротация, CSRF)

Дата: 2026-09-16. Методология: исследование по первоисточникам (OWASP Cheat Sheets, MDN, PortSwigger Web Security Academy, документация Go, инженерные статьи), каждое утверждение привязано к источнику. Репозиторий не изменялся.

---

## 1. Контекст проекта (проверено по коду 16.09.2026)

| Факт | Где в коде/доках |
|---|---|
| Go-бэкенд `net/http`, PostgreSQL; сессии — server-side opaque-токены (ADR 0004) | `docs/adr/0004-cookie-sessions.md` |
| Токен: 32 байта `crypto/rand`, hex → **256 бит энтропии** | `apps/backend/internal/identity/domain/session.go` (`NewSession`) |
| В БД — **HMAC-SHA256** (ключевой, детерминированный) хеш токена; lookup по хешу как по индексному ключу | `apps/backend/internal/platform/encryption/aes.go` (`hashToken`), `identity/adapters/postgres/session_repository.go` (`GetByTokenHash`) |
| Cookie `__Host-session_id` (prod), `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/` | `apps/backend/internal/platform/httpsupport/session.go`; ADR 0018 |
| Sliding-окно 7 дней + абсолютный кап 30 дней от создания (текущее состояние) | ADR 0011; `SessionBaseTTL`/`SessionMaxTTL` в `session.go` (и зеркально в `identity/domain`) |
| Принятое решение владельца: **pure sliding 7 дней без абсолютного капа** | тикет #725 |
| `POST /auth/logout`, `POST /auth/logout-all`; аудит с IP (fail-open); смена телефона завершает все другие сессии | `identity/adapters/http/auth_handlers.go`; `DeleteByUserIDExcept` в `session_repository.go` |
| Throttle: транспортные in-memory rate-limiter (ADR 0013) + доменные (send-throttle 1 мин; окно попыток 15 неудач / 30 мин на телефон) | `platform/httpsupport/ratelimit.go`; `identity/CONTEXT.md` |
| Вход объединён с регистрацией по phone+email-коду; при входе выдаётся **новая** сессия (новый токен) | ADR 0015; `session_service.go` (`Issue`) |
| Нет state-changing GET-эндпоинтов — правило зафиксировано в ADR 0018 | `docs/adr/0018-samesite-lax.md` |
| Нет CORS вообще — API строго same-origin (SPA Next.js на том же домене) | grep по `Access-Control-Allow|cors` — 0 вхождений |
| Сравнение логин-кода — constant-time (`subtle.ConstantTimeCompare`) | `identity/domain/login_code.go:67` |
| Тело запросов: `DecodeJSONBody` парсит JSON, **не проверяя `Content-Type`** | `platform/httpsupport/request.go` |
| Нет CSRF-токена, нет проверки `Origin`/`Sec-Fetch-Site`, нет `Cache-Control: no-store` на аутентифицированных ответах | grep по всему `internal/` — только `no-store` в health-check |
| Go 1.26.5 (значит доступен `net/http.CrossOriginProtection` из Go 1.25) | `Makefile` (`GO_VERSION := 1.26.5`), `go.mod` |
| Фоновая очистка истёкших сессий батчами | `DeleteExpiredBefore` в `session_repository.go` |

Примечание: в постановке тикета хранение описано как «SHA-256-хеш»; фактически это **HMAC-SHA256 с ключом** (`platform/encryption/port.go`: «returns a deterministic HMAC-SHA256 hash») — это даже сильнее: подделать/подобрать строку без ключа нельзя, а rainbow-таблицы по глобальному SHA-256 неприменимы.

---

## 2. Вопрос 1. Насколько безопасен отказ от абсолютного капа (pure sliding)

### Что говорят первоисточники

**OWASP Session Management Cheat Sheet** требует **оба** таймаута:

> «All sessions should implement an idle or inactivity timeout» и «All sessions should implement an absolute timeout, regardless of session activity».

И прямо описывает именно ту атаку, против которой стоит абсолютный кап:

> «If the attacker is able to hijack a given session, the idle timeout does not limit the attacker's actions, as they can generate activity on the session periodically to keep the session active for longer periods of time».

То есть **pure sliding = вечная сессия для того, кто ей владеет** — легальный пользователь и укравший cookie атакующий неразличимы по паттерну «активность продлевает жизнь». Референсные цифры OWASP (idle 2–5 мин для high-value, 15–30 мин для low-risk; absolute 4–8 часов) — при этом сам же документ оговаривает: «The session expiration timeout values must be set accordingly with the purpose and nature of the web application, and balance security and usability» [1].

**Важная лазейка в самом OWASP для длинных сессий** — renewal timeout (ротация ID):

> «The renewal timeout complements the idle and absolute timeouts, specially when the absolute timeout value extends significantly over time (e.g. it is an application requirement to keep the user sessions open for long periods of time)» [1].

То есть OWASP сам признаёт сценарий «сессии живут долго по требованиям продукта» и в этом случае предписывает компенсацию — **периодическую ротацию идентификатора сессии**.

**Консенсус инженерных источников 2023–2026** (WorkOS, LoginRadius, Descope): sliding без абсолютного капа в общем случае не считается лучшей практикой; рекомендована комбинация idle + absolute. WorkOS: «Even if a session is active, it should eventually expire after a defined period» [8][9][10].

**Контраргумент в пользу продукта** (тоже от OWASP, в разделе Reauthentication): жёсткие таймауты сами по себе не панацея — OWASP ссылается на Tailscale «Why Frequent Reauthentication Can Be a UX Pitfall»: частый принудительный ре-логин повышает риск (пользователи ослабляют пароли, выбирают «запомнить» в браузере, обходят политики) [1, раздел Reauthentication; 11]. Для record-keeping-приложения (ведение учёта, без движения денег; единственная платёжная операция — подписка SaaS через T-Kassa, ADR 0036) риск-профиль заметно ниже банковского, и OWASP явно допускает «application requirement to keep the user sessions open for long periods of time» при компенсирующих мерах [1].

### Вывод для проекта

Отказ от капа **приемлем как продуктовое решение при обязательном наборе компенсаций**, потому что единственную функцию капа («украденный cookie умрёт не позже N дней даже при активности атакующего») берут на себя другие контролы:

1. **Серверная ревокабельность** — есть (logout, logout-all, ревокация при смене телефона).
2. **Список устройств с ревокацией** — планируется; OWASP прямо рекомендует давать пользователю «the ability to check the presence of details of their active sessions, monitor concurrent logons and remotely terminate any session» [1]. Это главный пользовательский контур обнаружения.
3. **Idle-лимит 7 дней** — и есть «короткое» окно неактивности: неактивная сессия умирает за 7 дней. Атакующий вынужден генерировать активность — что оставляет след в `LastUsedAt`, аудите (IP) и в будущем списке устройств (второй IP/UA в списке = видимый пользователю артефакт).
4. **Step-up реаутентификация на чувствительных операциях** — частично уже есть (смена телефона требует кода на email; смена email сбрасывает подтверждение). См. раздел 5.
5. **Аудит + мониторинг** — аудит с IP есть; мониторинг аномалий — опционально (раздел 7).

Риск, который осознанно принимается: сессия, украденная **до** внедрения списка устройств или **незамеченная** в нём, живёт бесконечно, пока атакующий поддерживает активность. Это платформа для ведения учёта: максимум ущерба от долгоживущей сессии — порча/удаление записей учёта (восстановимо из аудита и резервных копий) и продление/отмена чужой подписки владельца. Кап 30 дней этот ущерб не ограничивал бы содержательно — только по времени.

---

## 3. Вопрос 2. CSRF для SPA + cookie-сессий: достаточен ли SameSite=Lax

### Что говорят первоисточники

**OWASP CSRF Prevention Cheat Sheet**: «SameSite is useful as a defense-in-depth control but it does not replace a proper CSRF defense in most deployments» [2]. Задокументированные отказы SameSite-защиты: state-changing GET (safe-методы всегда носят cookie — «the single most common way SameSite-based defenses fail in practice»); site-скоуп вместо origin-скоупа (поддомены); навигационные трюки; старые/встроенные браузеры. OWASP требует: «Implement at least one mitigation from Defense in Depth Mitigations section» — то есть **помимо** токенов/в дополнение к SameSite нужен ещё минимум один слой: верификация Origin/Referer или Fetch Metadata [2].

**PortSwigger (Web Security Academy)**: «The SameSite attribute... If set to Lax, the browser will only send the cookie in requests that fulfil both criteria: the request uses a safe top-level method (GET); the request results from a top-level navigation» и перечисляет обходы: «attacks using GET requests; attacks using on-site gadgets; attacks using vulnerable sibling domains» [4]. Strict рекомендован для cookies, сопровождающих state-changing действия [4].

**Chrome «Lax + POST»**: Google (web.dev) описывает поведение по умолчанию «Lax + POST»: куки младше **2 минут** отправляются и в top-level **POST**-навигациях (2-минутная льгота) [6]. Это узкое окно, релевантное в основном сразу после логина (login CSRF через кросс-сайтовую форму).

**Fetch Metadata**: OWASP: `Sec-Fetch-Site` — «the primary signal for CSRF protection»; политика — «Treat cross-site as untrusted for state-changing actions»; но поскольку старые браузеры заголовок не шлют, «a fallback to standard origin verification headers is a mandatory requirement» [2]. Origin/Referer надёжны, потому что это forbidden headers (не подделываются JS), но сравнивать надо **весь origin** («make sure you are matching against the entire origin»), а при отсутствии обоих заголовков «we recommend blocking» [2]. Покрытие `Sec-Fetch-Site`: «available in all browsers since 2023» (документация Go [7]; OWASP оценивает покрытие ~98%) [2][7].

**Go net/http — нативное решение**: с Go 1.25 в стандартной библиотеке есть `http.NewCrossOriginProtection` — «CrossOriginProtection implements protections against Cross-Site Request Forgery (CSRF) by rejecting non-safe cross-origin browser requests... Cross-origin requests are currently detected with the Sec-Fetch-Site header... or by comparing the hostname of the Origin header with the Host header. Requests without Sec-Fetch-Site or Origin headers are currently assumed to be either same-origin or non-browser requests, and are allowed» [7]. Использование: `http.NewCrossOriginProtection()` → `Handler(mux)`; для не-браузерных клиентов — `AddInsecureBypassPattern` (в проекте есть вебхуки Т-Кассы и `/internal/perf/`), отклонение — 403, кастомизируется через `SetDenyHandler`. Проект на Go 1.26.5 — доступно без обновлений и зависимостей [7].

**JSON API дополнительно**: OWASP: «a simple mitigation is for the server or API to disallow these simple content types» (`application/x-www-form-urlencoded`, `multipart/form-data`, `text/plain`) — кросс-сайтовая форма физически не может отправить `application/json`, а свой `Content-Type` без успешного CORS-префлайта браузер не поставит [2].

### Применительно к проекту

- Current state: единственная CSRF-защита — `SameSite=Lax`. Для same-origin SPA, где **все** мутации — JSON POST/DELETE/PATCH через fetch и **нет** state-changing GET (правило ADR 0018), Lax практически блокирует все классические векторы: кросс-сайтовые form/fetch POST cookie не получают.
- Остаточные риски Lax: (а) 2-минутное «Lax + POST» окно Chrome — топ-левельная кросс-сайтовая форма с POST вскоре после установки cookie (сцена логина: login CSRF — жертву можно залогинить в сессию атакующего; влияние ограничено, но есть); (б) будущий subdomain takeover (site-скоуп); (в) правило «нет state-changing GET» держится только на дисциплине, а не на механизме.
- Вывод: **SameSite=Lax для текущего API достаточен на 95%, но не является полной защитой по критериям OWASP** (нет второго слоя). Дешёвое и идиоматичное закрытие — `net/http.CrossOriginProtection` (Go stdlib, ~10 строк, ноль зависимостей) + пропуск для публичных путей (вебхуки), где он неприменим. CSRF-токен (double-submit signed) для этого проекта избыточен: OWASP предлагает его как один из вариантов defense-in-depth, а требование «минимум один mitigation» уже закрывается origin/Fetch-Metadata-проверкой; для same-origin SPA токен добавляет сложность (хранение/инъекция во фронт) без прироста защиты относительно origin-проверки.
- Сопутствующий фикс малой кровью: в `DecodeJSONBody` проверять `Content-Type: application/json` (сейчас тело парсится независимо от заголовка — `text/plain` CSRF с валидным JSON телом прошёл бы разбор, если бы cookie был доставлен).

---

## 4. Вопрос 3. Чек-лист харденинга cookie-сессий: что уже есть, что кандидат

Составлен по OWASP Session Management Cheat Sheet [1], OWASP CSRF Cheat Sheet [2], OWASP Session Hijacking guidance [1], PortSwigger [4], MDN [5].

| # | Практика | Статус в проекте |
|---|---|---|
| 1 | Энтропия токена ≥ 64 бит, CSPRNG (OWASP рекомендует 128+ бит) | ✅ **Уже есть** — 256 бит `crypto/rand` |
| 2 | Хранение только хеша; токен нигде не логировать | ✅ **Уже есть** — HMAC-SHA256 в БД; токен не логируется (`SanitizeError` на путях ошибок) |
| 3 | `HttpOnly`, `Secure` | ✅ **Уже есть** |
| 4 | `__Host-` префикс (лок на хост + обязательный `Secure`, без `Domain`) | ✅ **Уже есть** (prod; OWASP: «Recommended for session IDs») |
| 5 | `SameSite` как defense-in-depth | ✅ **Уже есть** (`Lax`, ADR 0018) — с documented tradeoff |
| 6 | Нет state-changing GET | ✅ **Уже есть** (правило ADR 0018) — держится дисциплиной, механизмом не защищено |
| 7 | Ротация ID при логине/повышении привилегий (анти-fixation) | ✅ **Частично есть**: при каждом входе выдаётся новая сессия; pre-auth сессии не существуют, fixation невозможна. Роли в рамках сессии не меняются — ротация при privilege change не требуется |
| 8 | Инвалидация при logout / смене критичных данных | ✅ **Уже есть** (logout, logout-all; смена телефона → все другие сессии). Кандидат-усиление: смена **email** сейчас не завершает другие сессии (см. §7) |
| 9 | Server-side revocation, logout уничтожает сессию на сервере | ✅ **Уже есть** |
| 10 | Постоянное (не bearer) сравнение секретов | ✅ **N/A, но закрыто**: lookup идёт по HMAC-хешу как по индексному ключу — секрет в коде не сравнивается вовсе; для логин-кода constant-time уже применён (`subtle.ConstantTimeCompare`) |
| 11 | Throttle аутентификации (транспорт + домен) | ✅ **Уже есть** (rate-limiter ADR 0013; send-throttle; окно попыток 15/30 мин) |
| 12 | Аудит с IP, фоновая чистка истёкших сессий | ✅ **Уже есть** |
| 13 | Idle-таймаут | ✅ **Уже есть** (7 дней sliding — сознательно длинный, продуктовое решение) |
| 14 | Absolute-таймаут | ⚠️ **Есть сейчас, будет убит** решением владельца (pure sliding) — компенсации в §7 |
| 15 | Список устройств + удалённая ревокация сессии | 🆕 **Кандидат в скоуп** — уже решено владельцем; прямо рекомендуется OWASP [1] |
| 16 | Верификация Origin / Fetch Metadata (второй CSRF-слой) | 🆕 **Кандидат** — `net/http.CrossOriginProtection` (§3) |
| 17 | Проверка `Content-Type: application/json` для JSON-эндпоинтов | 🆕 **Кандидат** — одна строка в `DecodeJSONBody` |
| 18 | Step-up реаутентификация на чувствительных операциях | 🟡 **Частично есть** (смена телефона, смена email); кандидат-усиление: подтверждение кодом перед logout-all/ревокацией устройства — по решению владельца |
| 19 | Логирование lifecycle сессий (создание/ревокация; salted hash ID, не raw) | 🟡 **Частично** — аудит login/logout с IP есть; систематический lifecycle-log появится естественно со списком устройств |
| 20 | Биндинг сессии к IP/UA | 🆕 **Опционально, только детект**: OWASP предупреждает — NAT/мобильные сети дают ложные срабатывания, UA подделывается; использовать для алертов/подсказок в списке устройств, не для жёсткого отказа |
| 21 | `Cache-Control: no-store` на аутентифицированных ответах | 🆕 **Кандидат** (низкий приоритет: SPA + fetch, кэширование ответов API минимально) |
| 22 | `Clear-Site-Data` на logout | 🆕 **Опционально** — приятная гигиена, не обязательно |
| 23 | Persistent cookie вместо session cookie | ⚠️ **Осознанное отклонение**: OWASP рекомендует non-persistent, но при sliding/remember-me UX нужен persistent — компенсируется ревокабельностью и idle-окном |
| 24 | Ограничение числа устройств | ❌ **Не нужно** (решение владельца); OWASP лишь предлагает делать осознанный выбор политики параллельных сессий [1] |
| 25 | 2FA, админ-видимость сессий, email о новом входе | ❌ **Вне скоупа** (решение владельца; при этом email-уведомление о новом входе — самый дешёвый детектор угона, кандидат на «потом») |

---

## 5. Вопрос 4. Подводные камни long-lived сессий и как их снижают без капа

1. **Вечная жизнь украденного cookie при поддержании активности** — главный камень (OWASP, §2) [1]. Снижение без капа: idle-окно 7 дней (атакующий обязан генерировать трафик → следы в `LastUsedAt`/аудите/списке устройств), серверная ревокация, список устройств, периодическая **ротация токена** (renewal timeout) — прямо рекомендована OWASP для «sessions open for long periods of time» [1]. Практический вариант для проекта: раз в N дней (например, 14) issuance нового токена для той же сессии (новый cookie, новый хеш, старый хеш инвалидирован) — механика повторяет refresh, но с перегенерацией токена; атомарно в одной транзакции (seam UoW уже есть, ADR 0033).
2. **Отсутствие повторной проверки личности** — за 7+ дней cookie мог сменить владельца (расшаренный ноутбук, кража устройства). OWASP «Reauthentication After Risk Events»: «Requiring users to reauthenticate when they perform sensitive operations or when the application detects suspicious events or potential account compromise helps mitigate session hijacking and unauthorized access — especially when long-lived sessions, such as those set by 'remember me' cookies, are in use» [1]. В проекте step-up уже частично есть: вся аутентификация сама по себе одноразовая (код на email), смена телефона требует свежего кода. Кандидат: требовать код перед сменой email/logout-all, если хотят максимальной строгости.
3. **Частая принудительная реаутентификация — тоже риск**: OWASP ссылается на Tailscale «Why Frequent Reauthentication Can Be a UX Pitfall» — жёсткие таймауты толкают пользователей к ослаблению практик и обходам, что снижает суммарную безопасность [1][11]. Это аргумент в пользу решения владельца (не выкидывать активного пользователя) при условии, что чувствительные операции защищены step-up.
4. **Смена критичных данных не убивает чужие сессии** — стандартное требование: ротация учётных данных должна завершать существующие сессии [1][8]. У проекта это сделано для телефона; для email — нет (кандидат, дёшево: тот же `DeleteByUserIDExcept`).
5. **Невозможность обнаружить угон** — HttpOnly прячет cookie от XSS, но не от локального доступа/вредоносных расширений/снифинга вне HTTPS. Снижения: список устройств (планируется), аудит с IP (есть), опционально email при новом входе (вне скоупа), мониторинг «невозможных путешествий» (опционально, детект-only; WorkOS: логировать IP+UA на сессию и алертить на аномалии) [8].
6. **Утечка сессии в кэш/прокси** — OWASP: ответы с session ID — `Cache-Control: no-store` [1]. Низкий приоритет для данного API (§4, п.21).
7. **Не принимать чужие ID сессий** — strict session management: приложение не должно принимать ID, которые само не создавало [1]. В проекте lookup по HMAC-хешу случайного токена — чужой/подобранный ID просто не найдётся; фиксация сессии невозможна (сессия не существует до верификации кода).

---

## 6. Итог: рекомендованный состав харденинга

### Обязательно (в скоуп ухода от капа, без этого решение считать незакрытым)

| Мера | Пометка |
|---|---|
| Список устройств с ревокацией одной сессии и «завершить все другие» | уже решено владельцем — это и есть обязательная компенсация №1 |
| Сохранение idle-окна 7 дней (сама суть pure sliding) + ревокабельность сервером | уже есть в проекте |
| Step-up реаутентификация на чувствительных операциях (смена телефона — есть; распространить на смену email; при желании на logout-all) | частично есть, кандидат-усиление |
| Инвалидация других сессий при смене email (как уже сделано для телефона) | кандидат, дёшево — переиспользует `DeleteByUserIDExcept` |
| CSRF: второй слой защиты — `net/http.CrossOriginProtection` (Go stdlib ≥1.25, проект на 1.26.5) с bypass для вебхуков/`/internal/perf/`; независимо от ухода от капа | кандидат; сейчас единственный CSRF-слой — SameSite=Lax |

### Опционально (усиливает, не блокирует)

- Периодическая ротация токена сессии (renewal timeout, OWASP-компенсация для длинных сессий) — например, перегенерация токена раз в 14 дней.
- Проверка `Content-Type: application/json` в `DecodeJSONBody`.
- Логирование lifecycle-сессий (создание/ревокация/IP/UA; hash ID, не raw) — приходит само со списком устройств.
- Биндинг IP/UA к сессии — только для отображения в списке устройств и алертов, не для жёсткого отказа (ложные срабатывания NAT/мобильных сетей).
- `Cache-Control: no-store` на аутентифицированных ответах; `Clear-Site-Data` на logout.
- ADR на изменение: фик задокументировать как смену решения ADR 0011 (как ADR 0018 amendments ADR 0011).

### Не нужно (по решениям владельца / оценке риска)

- Абсолютный кап — убирается осознанно; альтернативная «мягкая» версия (редкий кап, например год) — только если владелец передумает.
- Ограничение числа устройств.
- 2FA, админ-видимость сессий, email-уведомления о новом входе — вне скоупа (последнее — самый дешёвый детектор угона на будущее).
- CSRF-токен (double-submit) — избыточен при origin/Fetch-Metadata-проверке в same-origin API.
- Постоянное сравнение токенов в коде — не применимо (lookup по HMAC-ключу в БД); constant-time уже закрыт для логин-кода.

---

## 7. Источники

1. OWASP Session Management Cheat Sheet — https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html (цитаты по тексту: Session ID Length, Session Expiration, Idle/Absolute/Renewal Timeout, Session ID Renewal, Reauthentication After Risk Events, Logout, Web Content Caching, Session Management Defense in Depth, «provide users with visibility/revocation of active sessions»)
2. OWASP Cross-Site Request Forgery Prevention Cheat Sheet — https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html (SameSite как defense-in-depth; Origin/Referer; Sec-Fetch-Site «primary signal» + mandatory fallback; simple content types; Defense in Depth Mitigations)
3. OWASP Authentication Cheat Sheet — https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html
4. PortSwigger Web Security Academy, «Bypassing SameSite cookie restrictions» — https://portswigger.net/web-security/csrf/bypassing-samesite-restrictions (Lax: только safe top-level GET + top-level navigation; обходы; Strict для state-changing cookies)
5. MDN, `Set-Cookie` / SameSite — https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Set-Cookie
6. Google web.dev, «SameSite cookies explained» — https://web.dev/articles/samesite-cookies-explained (Lax-по-умолчанию; «Lax + POST» 2-минутная льгота описана в продолжении «SameSite cookie recipes», https://web.dev/articles/samesite-cookie-recipes)
7. Go net/http godoc, `CrossOriginProtection` (since go1.25.0) — https://pkg.go.dev/net/http#CrossOriginProtection
8. WorkOS, «Session Management Best Practices» — https://workos.com/blog/session-management-best-practices (sliding + absolute вместе; логирование IP/UA; step-up/MFA на чувствительных действиях)
9. LoginRadius, «User Session Management Best Practices» — https://www.loginradius.com/blog/identity/user-session-management-best-practices (idle + absolute + ротация ID)
10. Descope, «Session Timeout Best Practices» — https://www.descope.com/learn/post/session-timeout-best-practices
11. Tailscale, «Why Frequent Reauthentication Can Be a UX Pitfall» — цитируется OWASP Session Management Cheat Sheet (п.1, раздел Reauthentication); прямая загрузка на момент исследования не удалась (сетевой таймаут), утверждение использовано в пересказе OWASP.

Внутренние источники: `docs/adr/0004-cookie-sessions.md`, `docs/adr/0011-sliding-session-expiration.md`, `docs/adr/0018-samesite-lax.md`, `docs/adr/0013-in-memory-rate-limiters.md`, `docs/adr/0036-fintech-domain-language.md`, `apps/backend/internal/identity/CONTEXT.md`, код: `apps/backend/internal/platform/httpsupport/session.go`, `apps/backend/internal/platform/httpsupport/request.go`, `apps/backend/internal/identity/domain/session.go`, `apps/backend/internal/identity/domain/login_code.go`, `apps/backend/internal/identity/application/session_service.go`, `apps/backend/internal/identity/adapters/postgres/session_repository.go`, `apps/backend/internal/identity/adapters/http/auth_handlers.go`, `apps/backend/internal/platform/encryption/aes.go`, `Makefile` (`GO_VERSION := 1.26.5`).
