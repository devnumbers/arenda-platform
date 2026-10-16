# Ревью интеграции с Т-Кассой в коде billing

Дата: 2026-08-23
Объект: `apps/backend/internal/billing/` — адаптер `adapters/payment/tkassa/**` против официального контракта. Норма ревью: [`docs/research/2026-08-23-tkassa-official-contract.md`](./2026-08-23-tkassa-official-contract.md) (далее «справочник»), плюс ADR 0007/0010/0016/0017/0038/0039 и `CONTEXT.md` контекста billing. Ревью читающее: код не менялся.

## Резюме

Интеграция в целом соответствует официальному контракту Т-Кассы. Подпись Token (исходящие и входящие), форма Init (включая CC/COF-матрицу `OperationInitiatorType`), Charge без `Amount`, GetState как единственный метод сверки, Cancel с `ExternalRequestId`, привязка карты (`AddCustomer`→`AddCard 3DSHOLD`→`GetAddCardState`/вебхук, фильтр `GetCardList` по `A`), синхронные вебхуки с бюджетом 8 с и ответом `200 OK`, идемпотентность повторных доставок и out-of-order через GetState — всё сверено с первоисточником и зафиксировано контрактными тестами.

Подтверждённых расхождений с официальным контрактом — **1** (F1: валидный отказной add-card вебхук получает 400, банк ретраит его до месяца). Ещё **3** пункта — отклонения от контракта «на грани» (наносекунды в `RedirectDueDate`, непринимаемое легаси-значение `NotificationType="ADD_CARD"`, идемпотентность Init по OrderId как предположение). Отдельно — **3** расхождения кода с собственными ADR (F2: 5xx не ретраятся вопреки ADR 0017; F3: код 501 не классифицируется в общем пути; F4: `PARTIAL_REVERSED` маппится не так, как пишет ADR 0017) и ряд документационных неточностей. Критических блокеров нет.

## Таблица находок

| ID | Критичность | Файл:строка | Суть | Рекомендация |
|---|---|---|---|---|
| F1 | Средняя | `tkassa/webhook.go:74-77`, `billing/adapters/http/handlers.go:587-591` | Add-card вебхук с `Status=REJECTED` (валидная подпись, наш терминал) возвращается из `ParseWebhook` как ошибка → хендлер отвечает 400 → банк ретраит бесполезную доставку раз в час 24 ч, затем раз в сутки месяц (справочник §5). Терминальный отказ привязки ретраем не чинится; порт уже имеет `MethodBindingFailed`, и sync-путь через `GetAddCardState` этот случай закрывает, а вебхук-путь — нет. | Парсить отказную привязку в событие (или хотя бы отвечать «обработано»: закрыть сессию как rejected и вернуть 200 OK), оставив не-200 только для дефектов подписи/терминала |
| F2 | Средняя | `tkassa/retry_transport.go:22-23, 177-192` | HTTP 5xx от банка не ретраится — только сетевые таймауты (`net.Error.Timeout()`). ADR 0017 прямо обещает: «HTTP 500/503 … HTTP-клиент повторяет запрос с экспоненциальным backoff и jitter». Контракт допускает HTTP 500 от Init (справочник §2). | Ретраить 5xx хотя бы для идемпотентных методов (GetState, Init) по тем же правилам WroteRequest; обновить ADR 0017, если консервативность намеренная |
| F3 | Средняя-низкая | `tkassa/errors.go:127-151` | Код 501 («терминал не найден») классифицируется только в GetCardList-ветке (`isAccountNotFoundError`); в `classifyProviderError` его нет. ADR 0017 требует маппинга для всех методов (типовой кейс — неверный base URL test-вместо-prod). При 501 на Init/Charge/GetState сентинел не выставится, а метрика посчитает вызов «ok» (ProviderError → StatusOK). | Добавить `501` в `classifyProviderError` (в `ErrProviderAccountNotFound`) |
| F4 | Низкая-средняя | `tkassa/status.go:36` против `docs/adr/0017:146` | ADR 0017 §Polling: «REVERSED/PARTIAL_REVERSED из GetState/webhook … трактуются как failed». Код: `PARTIAL_REVERSED` маппится в `refunded` (статус легаси, в enum 1.28 отсутствует; сворачивание в refunded объяснимо ADR 0037). Комментарий `status.go:10-19` сам себе противоречит (первый абзац обещает failed для обоих). | Синхронизировать ADR 0017 и комментарий с фактическим маппингом; решить осознанно, чем частичная отмена чужого происхождения является для подписочного платежа |
| F5 | Низкая-средняя | `tkassa/status.go:63-64` | `mapCancelStatus` default → `Failed`: неожиданный статус Cancel-ответа (в т.ч. `UNKNOWN` из enum 1.28 — «результат неизвестен») проваливает возврат и откатывает резерв `refunding`, хотя возврат мог произойти. Смягчено `ExternalRequestId` (повтор вернёт текущее состояние) и reconciliation-воркером. | Неопределённые исходы (`UNKNOWN` и прочие вне словаря) держать в `refunding` (под сторожем), как уже сделано для `REVERSING/REFUNDING/ASYNC_REFUNDING` |
| F6 | Низкая | `tkassa/client.go:204-214` | Успех API-вызова проверяется как `!Success && ErrorCode != "" && ErrorCode != "0"`: ответ `Success=false` с пустым кодом пройдёт как успех (для всех методов, кроме Cancel, где есть отдельная проверка `!resp.Success` в `provider.go:445`). | Упростить условие до `!base.Success` ( ErrorCode «0»/пустой при Success=false — дефект ответа банка, а не успех) |
| F7 | Низкая | `tkassa/provider.go:522-535` | Комментарий называет `SuccessAddCardURL`/`FailAddCardURL`/`NotificationURL` «documented names». По справочнику §3 официальная схема AddCard не содержит НИ ОДНОГО URL-поля; per-request передача недокументирована (для NotificationURL док прямо отсылает к менеджеру). Код ведёт себя правильно (токен — только по схемным полям, надбавки после подписи, ровно как требует справочник, п.5 критичных точек, прод-инциденты 9/204), неточен только комментарий. | Переформулировать комментарий: все пять URL-полей — вне схемы, поведение подтверждено прод-опытом, держать на контроле при re-vendor |
| F8 | Низкая | `tkassa/provider.go:738-755`; `docs/adr/0016` (таблица GetCardList) | ADR 0016: «ErrorCode=7 … both are treated as an empty card list». Код возвращает сентинел-ошибку `ErrProviderCustomerNotFound`/`ErrProviderAccountNotFound`, а не пустой список; при этом вызовов `ListPaymentMethods` адаптера из application-слоя нет вовсе (порт определён, но не потребляется — список методов читается из локальной БД). | Либо выровнять ADR с кодом, либо вернуть пустой список; если метод порта не нужен — сократить или покрыть живым вызовом |
| F9 | Низкая | `tkassa/provider.go:254-257` | `RedirectDueDate` сериализуется как `time.Time` → RFC3339Nano: прод-значения из `clock.Now().Add(15m)` почти всегда несут наносекунды («…T12:34:56.123456789Z»). Контрактный формат — `YYYY-MM-DDTHH24:MI:SS+GMT` (целые секунды). Контрактный тест (`contract_test.go:150-154`) сравнивает с `Format(time.RFC3339)` на декорированном дедлайне с нулевыми нс — прод-кейс не ловит. Формально RFC3339/OpenAPI `date-time` совместимо; риск низкий, но это отклонение от документированного формата. | Округлять до секунд перед отправкой (`deadline.Truncate(time.Second)`) и добавить прод-формат в тест |
| F10 | Низкая | `tkassa/retry_transport.go:53-57`; `application/subscription_service.go:480-482` | Идемпотентность Init по OrderId («T-Kassa returns the existing payment when the same OrderId is initialized again») — предположение, в справочнике не зафиксировано (§2 говорит лишь «должен быть уникальным»). На нём держится и ретрай после отправки, и recovery-путь повторного Init. | Зафиксировать поведение на тестовом терминале экспериментально; отразить в ADR 0016 как проверенный факт или убрать из допущений ретрая |
| F11 | Низкая | `tkassa/webhook.go:18-19, 46-55` | `NotificationType` принимается как `NotificationPayment`/`NotificationAddCard`/`AddCard`/пусто. Легаси-значение из доков Тинькофф — `ADD_CARD` (справочник §3: wire-значение не подтверждено) — упадёт в `default` → «unknown notification type» → 400 и месячные ретраи. Смягчено: реальные add-card нотификации поля не несут, дискриминация идёт по `RequestKey`. | Добавить `"ADD_CARD"` в синонимы (дёшево, убирает класс риска) |
| F12 | Косметика | `tkassa/provider.go:349, 477, 504, 577, 637, 685` | Метрики метода местами строковыми литералами (`"Status"`, `"InitAddCard"`, `"GetAddCardState"`, `"RemoveCard"`, `"GetCardList"`, `"AddCustomer"`) вместо констант `method*` из `retry_transport.go:43-50` (`methodGetState` и др.): лейбл «Status» рядом с «GetState» засоряет метрику. | Завести недостающие константы и использовать их во всех RecordRequest |

## Детали по осям ревью

### 1. Подпись Token v2 (`sign.go`)

Исходящие (`sign`, sign.go:24-49) и входящие (`verifyToken`, sign.go:105-118) используют один алгоритм:

- Исключаются `Token`, `DATA`, `Data`, `Receipt` по имени и, сверх того, любые `map`/`[]` (shouldSkipValue) — строго «только корневые поля» (справочник §1). Для вебхуков официальное исключение — «Token и вложенные (Data, Receipt)» (§5) — покрыто с запасом.
- `Password` добавляется в набор, ключи сортируются, конкатенируются только значения без разделителей, SHA-256 → hex-lowercase. Числа идут через `json.Number` (байтовое представление провода), bool — `true`/`false`, float — целые сворачиваются к целому виду (stringifyValue/stringifyFloat).
- Пропуск пустых строк и nil — интерпретация нашей стороны (справочник, критичные точки, п.1): конкатенация пустой строки не добавляет байт, поэтому математически эквивалентна включению; null-полей в нотификациях банк не шлёт (§5). Безопасно.
- Сравнение входящего токена — constant-time (`hmac.Equal`), отсутствие `Token` в payload отвергается.
- Тесты: `TestSign`, `TestSignSkipsNullBlankAndNested`, `TestSignFixedVector` (пинованный вектор), `TestSignDoesNotMutateInput`.

`bodyFromStruct` (client.go:142-154) гоняет тело через `json.Marshal` + `UseNumber`-декод — подпись и провайд-байты совпадают.

### 2. Вебхуки (`webhook.go`, `adapters/http/handlers.go`, `payment_service.go`)

- **Ответ**: `WebhookAck()` = `OK` (webhook.go:16); хендлер пишет `200` + `text/plain` только после полной обработки (handlers.go:600-612). Не JSON, не пустой. Соответствует §5.
- **Бюджет**: `webhookProcessBudget = 8 с` (handlers.go:105) в окне 10 с банка; тело ограничено 256 КиБ → 413 (ADR 0039 п.1/6).
- **Две нотификации AUTHORIZED+CONFIRMED** на одностадийный платёж и Charge: дубль-доставка — идемпотентный no-op на всех путях (`finalizeSucceededPayment` — duplicate → nil; `applyPendingNotification` — только backfill ссылки; сессии привязки — `IsOpen()`-гейт). `RebillId` забирается из AUTHORIZED-нотификации родителя (webhook.go:113-116) и из add-card нотификации. ✓
- **Ретраи банка** (час×24, сутки×месяц): любая ошибка обработки → не-200 (400 — дефекты запроса, 404 — неизвестный платёж, 500 — прочее), повторная доставка применяется с нуля атомарно (ADR 0039). ✓ Кроме случая F1 (см. таблицу).
- **Out-of-order succeeded-после-failed**: `reconcileOutOfOrderSuccess` (payment_service.go:319-338) — GetState вне транзакции, только провайдер-подтверждённый успех применяется, иначе no-op. ADR 0010. ✓
- **NotificationAddCard**: дискриминация по `RequestKey` при пустом `NotificationType` и отсутствии `OrderId` (webhook.go:63-69) — ровно сценарий ADR 0017; успешность = `COMPLETED` + `Success=true`; терминальная семантика «COMPLETED — карта привязана» (§3). ✓
- **TerminalKey-сверка** входящих (webhook.go:42-44). ✓

### 3. Init (`provider.go:199-285`, `subscription_service.go:513-530`, `workers.go:767-780`)

- Обязательные: `TerminalKey`, `Amount` (int64-копейки, тип из спеки — `spec.gen.go:2787`), `OrderId` = UUIDv7 (36 ≤ 50), `Token`. `CustomerKey` передаётся всегда — обязательное условие `Recurrent=Y` выполняется тривиально.
- `PayType="O"`; `Recurrent=Y` только при `SaveMethod` (CIT-родитель); MIT-Init поле не несёт — contract-тест фиксирует отсутствие в map-теле (contract_test.go:137-143).
- `DATA.OperationInitiatorType`: `1` (CIT CC) для родителя, `R` (MIT COF Recurring) для продления; пустой инициатор отвергается тотальным маппером (provider.go:185-197) — защита от 1126. ✓
- `Description` ≤ 140 с обрезкой по рунам (description.go:57-62) — русский текст не режется посередине руны. Отправляется всегда.
- `NotificationURL`/`SuccessURL`/`FailURL` per-request (пункт про перекрытие терминальных настроек, §2). ✓
- `RedirectDueDate`: только CIT (15 мин из `Config.PaymentFormTTL`, config.go:47), MIT не передаёт — ADR 0017. Найдено отклонение формата — F9.
- `DATA`-лимиты не нарушаются: одна пара, ключ задан спекой (`OperationInitiatorType` — 23 знака против общего лимита ключа 20; это внутреннее противоречие доков банка к их же prescribed-полю, не наша проблема).

### 4. Привязка карты (provider.go:474-631, 682-787)

- `AddCustomer` → дубль (код 7) трактуется как успех и флоу продолжается (provider.go:504-515). ✓ (справочник §3).
- `AddCard` c `CheckType=3DSHOLD` (RebillId выдаётся всегда — обоснование ADR 0017). ✓ Токен считается до добавления URL-надбавок; тело дополняется `RedirectUrl`/`FailRedirectUrl`/`SuccessAddCardURL`/`FailAddCardURL`/`NotificationURL` после подписи (provider.go:546-552) — поведение подтверждено прод-инцидентами 9/204 и п.5 критичных точек справочника; comment-неточность — F7. Contract-тест пинует и наличие extras, и их исключение из токена (contract_test.go:477-500).
- `GetAddCardState`: все 8 статусов словаря; терминалы COMPLETED/REJECTED; промежуточные → pending (status.go:72-84). Запасной канал токена — self-healing sync (`payment_method_service.go:256-309`): поллинг открытых сессий, `ErrProviderBindingNotFound` (код 502) закрывает сессию. ✓
- `GetCardList`: «включая удаленные» — фильтр `Status=="A"` (provider.go:775), `I`/`D` отбрасываются. ✓ Расхождение обработки 7/501 с ADR 0016 — F8.
- `RemoveCard` по `CardId`; 107/231 → `ErrProviderMethodNotFound` (errors.go:84-86). ✓

### 5. Рекурренты (workers.go:694-780, provider.go:287-341)

- Последовательность строго по сценарию §4: MIT `Init` (сумма задаётся здесь) → pre-charge `GetState` (защита от двойного списания) → `Charge`(`PaymentId`,`RebillId`) без `Amount` и без переадресации. Отсутствие `Amount` в теле пинуется тестом на уровне raw map (contract_test.go:309-313). ✓
- CIT/MIT-признаки — см. §3. Ошибки рекуррента: 104 → `ErrProviderRecurringFailed`, 262 → `ErrProviderSavedMethodExpired`, 103/116/1051 → `ErrProviderInsufficientFunds`, 1125/1126 → `ErrProviderInvalidOperation` (errors.go:127-151). ✓
- Неопределённый исход Charge (сетевой сбой) — `recoverUncertainCharge`: GetState-сверка; недоступен статус → платёж остаётся pending на следующий тик, попытки капятся `ChargeAttemptLimit` (workers.go:887-942). ✓

### 6. Статусы (`status.go`)

- Все 24 значения enum 1.28 присутствуют константами; сверх того — легаси `ASYNC_REFUNDING`/`PARTIAL_REVERSED`/`3DS_FAILED` и ADR 0017-шные `PAY_CHECKING`/`CONFIRM_CHECKING` (обработка оставлена сознательно — справочник, критичные точки п.7). ✓
- Терминальность: `CONFIRMED` → succeeded; `REJECTED`/`AUTH_FAIL`/`CANCELED`/`DEADLINE_EXPIRED` (+`REVERSED`, `3DS_FAILED`) → failed; `REFUNDED`/`PARTIAL_REFUNDED` → refunded; процессные и `UNKNOWN` → pending; **default → pending, не провал**. ✓
- Расхождения: F4 (`PARTIAL_REVERSED`), F5 (default в `mapCancelStatus` → Failed).

### 7. Проверка статуса: GetState

`PaymentStatus` (provider.go:344-394) — `GetState` по `PaymentId`; легаси `CheckStatus`/`GetPayment` не используются нигде. `RebillId` из ответа — запасной канал charge-токена (потерянная доставка). Вызовы: pre-charge и post-error в renewal-воркере, out-of-order-сверка вебхука, reconciliation-воркер, admin sync. ✓ (справочник §7).

### 8. Cancel/возвраты (provider.go:396-471, admin_payment_service.go:71-128)

- Возврат всегда полный: `Amount` = сумма платежа; `ExternalRequestId` = внутренний UUID — идемпотентный ключ, сетевые дубли разряжает провайдер (справочник §8: «Если такой запрос уже есть, вернётся текущее состояние»). ✓
- `refundedAmount = max(OriginalAmount−NewAmount, 0)` при наличии полей, иначе запрошенная сумма (provider.go:457-464). ✓
- Трёхфазная сага (резерв `refunding` → вызов вне транзакции → финализация) с обработкой `ErrProviderDuplicateOperation` через GetState-сверку и сторожем reconciliation-воркера; аномально малая сумма возврата оставляет резерв на ручное разбирательство. ✓
- `mapCancelStatus`: `REVERSED`→refunded (отмена NEW-платежа как «полный возврат» домена — осознанно), `REFUNDING/REVERSING/ASYNC_REFUNDING`→refunding (в полёте). Недостаток default-ветки — F5.

### 9. retry_transport.go

- Ретраятся **только** сетевые таймауты (`net.Error.Timeout()`); `context.Canceled` — никогда. HTTP 4xx/5xx не ретраятся вовсе — см. F2.
- Идемпотентность по методам (retryOnTimeoutMethods): `GetState` (чистое чтение) и `Init` (идемпотентен по OrderId — предположение F10) — ретраятся и после отправки; `Charge`/`Cancel`/`AddCustomer`/`AddCard`/`RemoveCard` — только пока запрос не ушёл на провод (httptrace `WroteRequest`). Тесты пинуют оба режима (TestRetryTransportChargeNotRetriedAfterRequestSent / InitRetriedAfterRequestSent / WriteMethodsNoDuplicateAfterReadTimeout). ✓
- Backoff экспоненциальный с clamp от переполнения, full-jitter на crypto/rand (gosec G404), sleep уважает ctx. Код 119 и «повторите позже»-коды не ретраятся — для 119 это правильно (жёсткий отказ эмитента, §9).

### 10. Success/ErrorCode-семантика и метрики

- `Success=true` нигде не трактуется как успех платежа: исход определяет `Status` (Init/Charge/GetState возвращают `mapStatus(...)` отдельно от Success). `ErrorCode` при успехе статуса игнорируется, при failed — прокидывается в результат (`ChargePayment`/`PaymentStatus`), пустой/`"0"` — не ошибка (paymentErrorCode). Соответствует §2/§7 («Success=true означает успех вызова, не платежа»; для СБП — «ориентируйтесь на статус»). ✓
- Каталог кодов (справочник, критичные точки п.по ошибкам): покрыты 7, 9, 10, 12, 100, 103/116/1051, 104, 107/231, 204/205, 255, 262, 502, 1125/1126 — с тестами (TestClassifyProviderError). Пробелы: 501 в общем пути (F3); 119 и блок 501–513 привязки (510 «карта уже привязана», 511 3DS и т.д.) без сентинелов — низкий приоритет.
- `metrics.go`: лейблы только `provider`/`operation`/`status` (RED); PAN, суммы, токены, идентификаторы не утекают. `metricStatus` (errors.go:51-70): бизнес-отказы провайдера (ProviderError и сентинелы) = «ok», битая интеграция (204/205, 9/12/1125/1126) = «error» — сопоставимо между провайдерами. ✓ Несогласованность лейблов — F12.
- Логирование: PAN/токены в логи не пишутся (в провайдер-логах — только id/суммы); ошибки санитизируются через `sanitize.Error`.

## Сверка «контракт → код»

| Пункт контракта (справочник) | Код | Статус |
|---|---|---|
| §1 Token: корневые поля, −Token/DATA/Data/Receipt, +Password, сортировка, конкатенация значений, SHA-256 hex | `sign.go:24-49` | ✓ |
| §1/§5 Проверка входящей подписи тем же алгоритмом | `sign.go:105-118` (constant-time) | ✓ |
| §2 Init: обязательные поля, Amount int64, OrderId≤50 | `provider.go:233-243`; `spec.gen.go:2783-2787` | ✓ (UUIDv7=36) |
| §2 Description ≤140 | `description.go:57-62` | ✓ (обрезка по рунам) |
| §2 Recurrent=Y ⇒ CustomerKey обязателен | CustomerKey шлётся всегда | ✓ |
| §2 DATA-лимиты; OperationInitiatorType 1/R; пустой → 1126 | `provider.go:185-230` | ✓ |
| §2 RedirectDueDate | `provider.go:254-257` | ~ формат с наносекундами (F9) |
| §2 per-request NotificationURL/SuccessURL/FailURL | `provider.go:239-241` | ✓ |
| §4 Charge: Init задаёт сумму; Amount в Charge не предусмотрен | `provider.go:308-312`; contract-тест | ✓ |
| §4 Charge-нотификации: те же две AUTHORIZED+CONFIRMED | см. §2 выше | ✓ |
| §3 AddCustomer дубль → 7 → не ошибка | `provider.go:504-515` | ✓ |
| §3 AddCard CheckType 3DSHOLD; без URL в схеме; токен без надбавок | `provider.go:520-552` | ✓ (комментарий — F7) |
| §3 GetAddCardState: 8 статусов, терминал COMPLETED | `status.go:72-84` | ✓ |
| §3 GetCardList включая удалённые → фильтр A | `provider.go:773-786` | ✓ |
| §3 RemoveCard по CardId | `provider.go:634-677` | ✓ |
| §5 Ответ 200 + тело OK | `webhook.go:24-26`, `handlers.go:600-606` | ✓ |
| §5 Окно 10 с | бюджет 8 с (`handlers.go:105`) | ✓ |
| §5 Ретраи: не-200 при сбое | `handlers.go:578-597` | ✓ (кроме F1) |
| §5 NotificationAddCard: RequestKey-дискриминация | `webhook.go:63-69` | ~ (F11 — легаси `ADD_CARD`) |
| §6 Enum 1.28; терминальность CONFIRMED/REJECTED; легаси-статусы оставить; default pending | `status.go:20-44` | ~ (F4) |
| §7 GetState по PaymentId; RebillId в ответе | `provider.go:344-394` | ✓ |
| §7 «Ориентируйтесь на статус», ErrorCode≠Success | `client.go:204-214`, `provider.go:337-339` | ✓ (F6 — пустой код при Success=false) |
| §8 Cancel: Amount-частичность/полный; ExternalRequestId; OriginalAmount−NewAmount | `provider.go:418-464` | ✓ |
| §9 Ретраи: только безопасные; 119 не ретраить | `retry_transport.go:177-192` | ~ (F2 — 5xx) |
| Ошибки 7/9/10/100/103/104/107/116/204/205/255/262/502/1125/1126 | `errors.go:127-151` | ~ (F3 — 501) |

## Что сделано правильно

1. **Подпись — эталонная реализация**: единый алгоритм на исходящих и входящих, исключение вложенных структур сверх поимённого списка, constant-time сравнение, пинованный тест-вектор; сериализация тела и подпись байтово-совместимы (UseNumber).
2. **Контрактные тесты против вендоренной спеки** (`contract_test.go`): strict-decode каждого запроса в `spec.*`-типы плюс raw-map проверки намеренных отсутствий (`Amount` в Charge, `Recurrent` в MIT-Init) — дрейф ловится компиляцией и тестами, а не продом (ADR 0016 работает как задуман).
3. **CC/COF-матрица инициаторов**: тотальный маппинг с отвержением пустого значения — класс ошибок 1126 закрыт статически; прод-инцидент с неверным `OperationInitiatorType` (ADR 0017 Context) невозможен по построению.
4. **Идемпотентность на каждом слое**: дубль-вебхуки — no-op по статусу платежа/`LastAppliedPaymentID`/состоянию сессии; refund — `ExternalRequestId` + резерв-сага; Charge — pre-charge GetState и WroteRequest-гейт в ретрай-транспорте (двойное списание исключено на транспортном уровне).
5. **Out-of-order и неопределённость**: succeeded-после-failed и сбойный Charge разрешаются только через GetState как источник истины, вне транзакций — ADR 0010/0039 реализованы буквально.
6. **Безопасность вебхук-поверхности**: проверка подписи до парсинга, сверка TerminalKey, лимит тела 256 КиБ, бюджет 8 с, sanitize ошибок в логах, ack строго `OK` после полной обработки.
7. **Чистота порта**: application не знает ни одного провайдер-словаря; все wire-детали (URL-надбавки AddCard, легаси-статусы, словарь ошибок, формат описания) инкапсулированы в адаптере; один активный провайдер (ADR 0038) с компайл-тайм-проверками интерфейсов.
8. **Метрики без чувствительных данных** и с продуманной RED-семантикой «бизнес-отказ ≠ ошибка интеграции», кроме явно выведенных в error 204/205/9/12/1125/1126.
9. **Спека как источник**: vendored 1.28, единственный патч — `Common.additionalProperties` с fail-loud `patch.sh`; re-vendor-процедура задокументирована; сверка с upstream на день ревью сходится (справочник §10).

## Непроверяемое по коду (внешние зависимости)

- Поведение URL-надбавок AddCard (оба набора имён) — подтверждено только прод-опытом; держать в чек-листе каждого re-vendor.
- Идемпотентность Init по OrderId (F10) и wire-значение `NotificationType` add-card-нотификации (F11) — эксперимент на тестовом терминале.
- TTL `RequestKey`-сессии и `RebillId` — контрактом не заданы; наша защита — локальный TTL сессии привязки и перепривязка при 262/104.
