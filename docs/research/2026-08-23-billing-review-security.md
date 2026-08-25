# Ревью безопасности модуля billing

- Дата: 2026-08-23.
- Объект: `apps/backend/internal/billing/` целиком — HTTP-адаптер (`adapters/http/`), application-сервисы, domain, postgres-репозитории (`adapters/postgres/`), адаптер Т-Кассы (`adapters/payment/tkassa/`), события (`adapters/events/dispatcher.go`), плюс места входа вебхука и admin-эндпоинтов: композиция роутов в `apps/backend/internal/platform/httpserver/server.go`, wiring провайдера в `apps/backend/cmd/api/wire/billing.go`, гейт провайдера в `apps/backend/internal/platform/config/config.go`.
- Вход для: research-тикет «Ревью безопасности модуля billing» карты wayfinder. Оборонный аудит собственного кода проекта; ревью читающее, без изменений кода.
- Метод: построчное чтение кода против справочника контракта Т-Кассы (`docs/research/2026-08-23-tkassa-official-contract.md`), чек-листа практик (`docs/research/2026-08-23-subscription-billing-best-practices.md`, темы 6–9) и ADR 0010/0039/0004/0020/0034/0037/0038; каждая находка привязана к `файл:строка`, для каждой оценены компенсирующие контроли. Поведение chi при дублирующей регистрации роута сверено с исходниками `go-chi/chi/v5 v5.3.0` (`tree.go`, `InsertRoute` → `setEndpoint` «Insert or update the node's leaf handler»).

## Резюме

**Вердикт: критичных уязвимостей нет; модуль построен по лучшим практикам платёжной обработки.** Подпись вебхука Token v2 проверяется до любой обработки (constant-time), суммы применяются только из персистентного состояния БД, идемпотентность обеспечена статусным автоматом плюс уникальными индексами схемы, refund-сага защищена резервным статусом `refunding` и сторожем-воркером, charge-токены карт шифруются at rest, SQL полностью параметризован, аудит пишется в транзакции бизнес-операции (ADR 0020).

Найдено 2 находки среднего уровня (обе — прочность границы, не прямой эксплойт), 6 низких и 1 информационная. Самая значимая: гарантия admin-изоляции биллинг-эндпоинтов держится на недокументированном поведении chi «последняя регистрация побеждает» при пере-регистрации сгенерированных роутов, а хелпер `adminActor` проверяет только наличие актора в контексте, не роль — компенсирующий контроль есть и работает, но не покрыт тестом на уровне композиции. Вторая: у инициации привязки карты нет пер-пользовательского лимита, единственный рубеж — глобальный IP-рейт-лимитер.

## Таблица находок

| ID | Критичность | Вектор (одной фразой) | Файл:строка | Рекомендация |
|---|---|---|---|---|
| SEC-01 | средняя | Владелец-сессия доходит до admin-эндпоинта billing (рефанд/смена тарифа), если пере-регистрация chi-роута с `AdminOnlyMiddleware` потеряется при рефакторинге — `adminActor` роль не проверяет | `apps/backend/internal/platform/httpserver/server.go:206-226`; `apps/backend/internal/billing/adapters/http/handlers.go:708-716` | Проверять `actor.Role == RoleAdmin` в `adminActor` (second line of defence) и/или добавить композиционный тест: owner-сессия → 403 на `POST /admin/subscription/payments/{id}/refund` |
| SEC-02 | средняя | Аутентифицированный пользователь спамит `POST /subscription/payment-methods`, каждая итерация бьёт в `AddCustomer`/`AddCard` Т-Кассы и создаёт строку сессии — нет пер-пользовательского лимита и дедупликации открытых binding-сессий | `apps/backend/internal/billing/application/payment_method_service.go:132-147`; `apps/backend/internal/platform/httpserver/server.go:123`; `apps/backend/internal/platform/config/config.go:338-339` | Пер-пользовательский rate limit на инициацию привязки и/или отказ при N открытых сессиях пользователя |
| SEC-03 | низкая | Подписанный вебхук с суммой, не совпадающей с платежом в БД, проходит без сигнала — расхождение (неверный терминал, чужой OrderId с тем же UUID) не детектируется | `apps/backend/internal/billing/adapters/payment/tkassa/webhook.go:150-164`; `apps/backend/internal/billing/application/payment_service.go:347-371` | Сверять `n.AmountKopecks != 0 && != payment.AmountKopecks` → reject/alert (чек-лист практик №47) |
| SEC-04 | низкая | Админ задаёт тариф с ценой порядка int64-max (максимум в контракте/домене/схеме отсутствует) — «сломанная» цена уходит в Init | `apps/backend/api/openapi/openapi.yaml` (AdminCreateTariffRequest, `minimum: 0` без `maximum`); `apps/backend/internal/billing/domain/tariff.go:75-83`; миграция 000104 (CHECK `>= 0`) | Верхняя граница цены в контракте и `Tariff.Validate` |
| SEC-05 | низкая | Любой, кто достучится до dev/stage-хоста с `PAYMENT_PROVIDER=fake`, подтверждает чужой платёж по UUID без сессии и проверки принадлежности (IDOR) | `apps/backend/internal/billing/adapters/http/fake_confirm.go:82-112` | Ограничить роуты localhost / требовать сессию владельца; сегодня двойной компенсирующий контроль (см. детали) |
| SEC-06 | низкая | Пик доставок вебхука Т-Кассы упирается в общий IP-лимитер 20 rps/40 burst → ложные 429, финализация платежей откладывается на часовые ретраи провайдера | `apps/backend/internal/platform/httpserver/server.go:123`; `apps/backend/internal/platform/config/config.go:338-339` | Отдельный лимит/полоса для `/webhooks` (по IP провайдера) либо exemption с собственным лимитом |
| SEC-07 | низкая | Chargeback-статус `REVERSED` после локального `succeeded` — no-op: платёж остаётся успешным, подписка активной (деньги возвращены банку, сервис не отобран) | `apps/backend/internal/billing/adapters/payment/tkassa/status.go:38-40`; `apps/backend/internal/billing/application/payment_service.go:403-407` | Обрабатывать REVERSED-after-succeeded как рефанд/эскалацию (практики №43) |
| SEC-08 | низкая | Истёкшие `card_binding_sessions` не удаляются никогда — неограниченный рост таблицы (в сочетании с SEC-02 — вектор заполнения) | `apps/backend/db/queries/billing.sql:353-373` (только insert/get/update); репозиторий без delete | Периодическая чистка истёкших сессий в воркере |
| SEC-09 | информационная | `int(v.Payment.AmountKopecks)` в DTO усекает сумму на 32-бит платформе (деплой 64-бит — сейчас не проявляется) | `apps/backend/internal/billing/adapters/http/handlers.go:407, 882` | Использовать int64 в DTO-маппинге |

Критичных (эксплуатируемых посторонним без компенсации) находок нет.

## Детали

### SEC-01. Прочность admin-изоляции billing-эндпоинтов (средняя)

Сгенерированный OpenAPI-роутер регистрирует все admin-маршруты **без** middleware (`httpserver/server.go:206-209`, `openapi.HandlerWithOptions` с `BaseRouter`), после чего каждый из них пере-регистрируется через `r.With(httpsupport.AdminOnlyMiddleware)` (`server.go:215-246`). Хендлеры billing при этом различают актора через `adminActor` (`handlers.go:708-716`), который возвращает `(adminID, true)` при **любом** акторе в контексте — роль не сверяется: комментарий опирается на то, что «AdminOnlyMiddleware гарантирует роль на смонтированных роутах».

Компенсирующие контроли (работают сегодня):
- chi v5 при повторной регистрации того же method+pattern обновляет хендлер узла — проверено по исходникам `chi@v5.3.0/tree.go` (`InsertRoute` → `setEndpoint`: «Insert or update the node's leaf handler»); пере-регистрация с middleware действительно побеждает.
- `AdminOnlyMiddleware` (`httpsupport/admin_middleware.go:10-23`) корректно требует `actor.RoleAdmin`, иначе 403.

Остаточный риск: ни один тест не проверяет связку «не-админ → 403» на уровне роутинга — `handlers_test.go:798` строит admin-запрос через `httpsupport.WithActor(..., RoleAdmin)` напрямую в контекст, минуя middleware и мукс. Обновление chi, смена генератора или перенос регистрации сломает гейт молча. Рекомендация: роль-чек внутри `adminActor` + один композиционный тест.

### SEC-02. Инициация привязки карты без пер-пользовательского лимита (средняя)

`AddPaymentMethod` → `startBinding` (`payment_method_service.go:132-147`) на каждый вызов делает `BindPaymentMethod` у провайдера (у Т-Кассы это `AddCustomer` + `AddCard`, `tkassa/provider.go:494-559`) и создаёт строку сессии. Единственный рубеж — глобальный IP-лимитер 20 rps / burst 40 (`config.go:338-339`, применяется ко всем роутам, `server.go:123`): держатель валидной сессии может гонять ~72k инициаций в час, создавая нагрузку на терминал Т-Кассы и строки в `card_binding_sessions`.

Компенсирующие контроли: сессия без подтверждения не создаёт метод оплаты (TTL-гейт, см. «что сделано правильно»); ChargePayment по несуществующему токену невозможен; ущерб финансовый — нулевой, операционный — шум у провайдера и рост таблицы (SEC-08). Для сравнения: тарифные платежи ограничены сильнее — one-pending unique-индекс `(user_id, tariff_id, period) WHERE status='pending'` (миграция 000104) схлопывает спам `ChangeTariff` в возврат существующего pending-платежа.

### SEC-03. Сумма вебхука не сверяется с платёжом (низкая)

`getAmount` (`tkassa/webhook.go:150-164`) парсит `Amount` в int64 (отрицательные проходят парсинг), но `finalizePayment`/`applyRefundNotification` (`payment_service.go:347-371, 543-564`) значение `n.AmountKopecks` не читают: применяемые суммы всегда берутся из строки `subscription_payments`. Это сознательная и правильная политика «не доверять клиенту» (ADR 0010) — но перекрёстная проверка суммы стоила бы одну строку и закрыла бы сценарий рассинхрона терминалов/окружений (сумма подписана провайдером, так что подделка исключена — только детекция аномалии).

### SEC-04. Нет верхней границы цены тарифа (низкая)

Контракт (`AdminCreateTariffRequest/AdminUpdateTariffRequest`: `minimum: 0`, без `maximum`), домен (`Tariff.Validate`, `tariff.go:75-83`) и схема (CHECK `monthly_price_kopecks >= 0`) допускают цену до int64-max. Эксплуатация требует админ-роли (внутренняя угроза/ошибка), внешних векторов нет; Init с такой суммой Т-Касса отвергнет. Рекомендация — санити-максимум в контракте.

### SEC-05. Fake-confirm эндпоинты: IDOR без авторизации (низкая, двойная компенсация)

`POST /internal/fake-subscription-payment/{id}/confirm` и `POST /internal/fake-card-binding/{requestKey}/confirm` (`fake_confirm.go:82-112`) монтируются без auth-middleware (`server.go:257-260`) и подтверждают **чужой** платёж/сессию по угаданному UUID без проверки принадлежности (UUIDv7 — не переборные, но не секрет). Компенсирующие контроли сильные и двухслойные:
1. `wire/billing.go:102-105` — `FakeConfirms` строится только при активном fake-провайдере; при tkassa хендлеры nil, роуты не монтируются вовсе.
2. `config.go:617-619` — `PAYMENT_PROVIDER=fake` отвергается при `APP_ENV` != local/dev на старте процесса.

Остаточный риск — только публично доступный dev-стенд с fake-провайдером. Рекомендация: session + ownership-чек в хендлерах (дёшево, устраняет класс).

### SEC-06. Вебхук делит общий IP-лимитер (низкая)

Роут `/webhooks/payment/{provider}` проходит через `rateLimitMiddleware(deps.IPRateLimiter)` (`server.go:123`) с дефолтом 20 rps / burst 40 на IP. Доставки Т-Кассы идут с ограниченного пула IP провайдера; при массовом продлении (начало месяца) или волне ретраев возможны ложные 429. Компенсация: провайдер ретраит (час × 24 ч, сутки × месяц), плюс собственный `ReconcilePendingPayments`-воркер (`workers.go:1087-1090`) добирает потерянное — потери денег/событий нет, есть задержка финализации. Отдельный лимит для `/webhooks` снял бы и вопрос DoS-полосы для валидных доставок.

### SEC-07. Chargeback после успеха не применяется (низкая)

`mapStatus` (`tkassa/status.go:38-40`) мапит `REVERSED` в `failed`; `finalizeFailedPayment` (`payment_service.go:403-407`) — no-op для не-pending платежа. Итог: диспут/отмена после `CONFIRMED` оставляет локальный платёж `succeeded`, подписку активной. `REFUNDED`-вебхук при этом обрабатывается корректно (`applyRefundNotification`), так что пробел только в REVERSED-ветке. Финансовой ошибки нет (деньги вернулись плательщику без нашего участия), но entitlement расходится с провайдером (чек-лист №43).

### SEC-08. Истёкшие binding-сессии не чистятся (низкая)

`card_binding_sessions` не имеет delete-путей ни в коде, ни в SQL (`billing.sql:353-373`); истёкшие сессии лишь помечаются при касании вебхуком/sync. Рост таблицы неограничен; в связке с SEC-02 — целевой вектор заполнения. Чистка в воркере закрывает обе хвостовые части.

### SEC-09. `int(...)` для сумм в DTO (информационная)

`handlers.go:407, 882`: `AmountKopecks: int(v.Payment.AmountKopecks)` — на 64-бит платформах int == int64 (деплой darwin/arm64, linux/amd64), усечения нет; но маппинг в контрактный int64 напрямую был бы устойчивее к портированию.

## Что сделано правильно (контроли, которые уже стоят)

**Подлинность вебхука**
- Подпись Token v2 проверяется ДО любой обработки: `ParseWebhook` → `verifyToken` (`tkassa/webhook.go:30-44`, `sign.go:105-118`), сравнение constant-time (`hmac.Equal`), алгоритм строго по официальной странице (исключение `Token`/`DATA`/`Data`/`Receipt` и любых вложенных объектов, `Password` в наборе, сортировка ключей, конкатенация значений, SHA-256 hex-lower — `sign.go:24-49`).
- Сверка `TerminalKey` уведомления с терминалом инстанса (`webhook.go:42-44`); провайдер-дискриминация имени в пути (`payment_service.go:159-162` → 400 при несовпадении).
- Replay-уведомления безвредны: у Token v2 нет timestamp (контракт провайдера), но повторное применение — идемпотентный no-op по статусному автомату (см. ниже), а «succeeded после failed» применяется только после подтверждения `GetState` у провайдера вне транзакции (`payment_service.go:319-338`, ADR 0039 п.5).
- Тело ограничено 256 КиБ с 413 без обработки (`handlers.go:557-573`), бюджет обработки 8 с (`handlers.go:575`, ADR 0039), 200 с телом `OK` — только после полного применения, ошибки дефектов запроса → 400, неизвестный платёж → 404, остальное → 500 для ретраев провайдера (`handlers.go:578-597`).
- Вебхук не гейтится readonly-режимом и не требует сессии (`readonly_middleware.go:33-47` — `/webhooks` в exempt; `session.go:165-196` — анонимные запросы проходят) — аутентичность обеспечивает подпись (практики №40).

**Идемпотентность и двойные применения**
- Статусный автомат платежа (`domain/subscription_payment.go`) отвергает повторную финализацию (`MarkSucceeded`/`MarkFailed` только из pending); повторная доставка — no-op c 200 (`payment_service.go:444-446, 403-407`).
- Двойное применение подписочных эффектов отсечено `last_applied_payment_id` (`payment_service.go:503-505`).
- БД — последний рубеж: partial unique `(user_id, tariff_id, period) WHERE status='pending'` (двойной клик/гонка инициации), unique `(provider, provider_payment_id)`, одна активная карта на пользователя (`idx_payment_methods_one_active_per_user`) — миграция 000104.
- Refund-сага: резерв `refunding` под `SELECT FOR UPDATE` до вызова провайдера (`admin_payment_service.go:142-174`), провайдер вне транзакции, `ExternalRequestId = paymentID` — идемпотентность Cancel на стороне Т-Кассы (`tkassa/provider.go:418-424`), дубликат-ответ разряжается сверкой статуса (`resolveDuplicateRefund`), неопределённый исход остаётся под сторожем `ReconcileStaleRefunds` (`workers.go:1170-1241`). Аномалия «вернул меньше полной суммы» не финализируется, а оставляется на разбор (`admin_payment_service.go:98-109`).
- Renewal-воркер перед списанием перечитывает статус у провайдера, исключая двойной charge (`workers.go:723-741`); неопределённый charge не повторяется вслепую, а капится `ChargeAttemptLimit` (`workers.go:887-960`).
- Retry-транспорт различает идемпотентные методы (GetState/Init-by-OrderId) и неидемпототные (Charge/Cancel/AddCard — ретрай только если запрос не ушёл в сеть, по `httptrace.WroteRequest`) — `retry_transport.go:52-61, 172-192`.

**Транзакционность и деньги**
- Все денежные/статусные изменения — в одной транзакции через UoW-фабтуру (`stores.go:186-280`, ADR 0033): платёж + подписка + история переходов + аудит атомарны (ADR 0039 п.3).
- Потерянных обновлений нет: `GetByIDForUpdate`/`GetByUserIDForUpdate` на каждом мутационном пути (`stores.go:68-133`, `billing.sql:71,77,183,322,331,365`); воркеры перепроверяют выборку под блокировкой (`lockInSelection`, `stores.go:86-101`).
- Деньги — int64-копейки во всех слоях; цены задаются per-period (умножения «цена × период» нет вообще, переполнение невозможно); DB CHECK `amount_kopecks > 0` и `refunded_amount_kopecks = amount_kopecks` (только полный возврат, ADR 0037); суммы применяются из БД, не из вебхука; refunded-сумма при Cancel берётся из `OriginalAmount−NewAmount` ответа провайдера с fallback на запрошенную (`tkassa/provider.go:457-464`).

**Секреты и PII**
- TerminalKey/Password не логируются и не попадают в метрики/события; спаны и логи tkassa несут только id, provider, суммы (`provider.go:206-270` и др.); метрики низкокардинальные (`payment/metrics.go`).
- Charge-токен (RebillId), CardId, ExpDate шифруются at rest (AES-GCM), уникальность — по HMAC-хэшу, не по плейнтексту (`payment_method_repository.go:148-174`, `platform/encryption/aes.go`); PAN не хранится, только маска провайдера.
- Ошибки наружу — санитизированные RFC 7807 problems с фиксированными текстами; внутренняя причина только в лог (`httpsupport/problem.go:54-59`), тест прямо проверяет отсутствие утечки (`handlers_test.go:789-794`).
- Аудит-контекст — белый список полей (id/суммы/enum), без телефонов/токенов/PAN (ADR 0020).

**SQL**
- Все запросы sqlc-параметризованы; сортировка админского листинга — через фиксированные CASE-ветки по белому списку полей (`billing.sql:250-259` + `admin_payment_service.go:461-505` — whitelist limit/sort/order/status). Конкатенации строк в SQL нет.
- Миграция 000104 перестроила схему с CHECK-ограничениями статусов/периодов/провайдеров и парциальными индексами воркеров.

**Авторизация и границы**
- Все пользовательские эндпоинты берут `ownerID` из сессии и работают только со своими данными; доступ к payment-method по чужому UUID отвергается как not found (`methodForUpdate`, `stores.go:121-133`); листинги скоуплены по `user_id` на уровне SQL.
- Admin-эндпоинты (включая рефанд и ручную синхронизацию) — за `AdminOnlyMiddleware` с атрибуцией действующего админа в transition log и аудите (ADR 0020/0034).
- Readonly-гейт (`readonly_gate.go:34-43`, `readonly_middleware.go`) блокирует мутации данных при истёкшей подписке, но сознательно пропускает recovery-пути (`/subscription/*`, `/tariffs`, `/auth`, `/webhooks`, `/admin`) — «гейт только блокирует, никогда не дарует»; обход через `/internal` ведёт лишь к fake-confirm (SEC-05, закрыто env-гейтом).
- Валидация входа: закрытые словари тарифов/периодов (`domain.ParseTariffName`/`ParseSubscriptionPeriod`), лимит grace-extension 1–90 дней (`admin_subscription_service.go:172-173`),TermType с проверкой даты не в прошлом (`serviceTermValidUntil`), фильтры админ-листинга с whitelist. Mass assignment отсутствует — DTO узкие, маппинг явный.

**Аудит и наблюдаемость**
- Каждая мутация (создание платежа, финализация, рефанд, sync, смена тарифа, назначение служебной подписки, активация/удаление метода) пишет запись в `audit_log` в той же транзакции через `txStores.audit` (ADR 0020 Approach A, fail-safe).
- История переходов подписки — append-only, from-сторона захватывается автоматически (`transition.go:52-92`), «нет смены состояния без записи в журнал» — by construction.

## Известные принятые ограничения (из ADR 0020, актуальны на дату ревью)

- Создание renewal-charge воркером не пишется в аудит (финализация — пишется).
- `ConfirmFakePayment` (dev-flow) не аудируется.
- Upsert-пути payment-method (`ON CONFLICT`) не различают insert/update и не аудируются поштучно.
- Дубль `payment_method.added` при пере-доставке add-card вебхука, заявленный в ADR 0020, в текущем коде устранён: повторная доставка выходит из `applyMethodBoundNotification` до аудита (`payment_service.go:210-213` — `!session.IsOpen() → nil`).

## Рекомендованный порядок закрытия

1. SEC-01 (роль-чек в `adminActor` + композиционный тест) — дёшево, снимает самый весомый остаточный риск.
2. SEC-02 + SEC-08 (лимит на инициацию привязки и чистка сессий) — один PR.
3. SEC-03 (сверка суммы вебхука с БД) — одна проверка + алерт.
4. SEC-06 (отдельная полоса лимита для `/webhooks`), SEC-07 (REVERSED-after-succeeded), SEC-04 (max цены), SEC-05 (auth на fake-confirm), SEC-09 (int64 в DTO) — по мере касания файлов.
