# Research: приём клиентской бизнес-даты «задним числом» в API записи фактов

Тикет #1391 (карта #1390), 2026-10-13. Исследователь: AFK research-сессия.
Вопрос: как правильно (industry best practices) принимать от клиента бизнес-дату задним числом в API записи фактов — чтобы бекенд и контракт (#1392) были сделаны архитектурно верно.
Опора: [donnemartin/system-design-primer](https://github.com/donnemartin/system-design-primer) + первоисточники (ISO 8601, PostgreSQL, Stripe, OWASP, RFC 9110, Google Dataflow paper).

## TL;DR

Дизайн, согласованный с владельцем (optional `date` YYYY-MM-DD; absent = «сегодня владельца»; будущая → reject 400; `date = paid_date = выбранная`; нижнего пола нет; created_at/updated_at + audit log + история; Idempotency-Key), **совпадает с лучшими практиками по всем пяти осям исследования**. Ни одного пункта, требующего пересмотра, не нашлось. Для #1392 остаётся аккуратно оформить контракт и валидацию; главный набор граблей — «рассинхрон двойных дат» и «дедуп по бизнес-дате» — уже закрыт конструкцией схемы.

## 1. Лучшие практики (с источниками)

### 1.1. Формат на проводе: ISO 8601 full-date `YYYY-MM-DD`, без времени и зоны

- ISO 8601 определяет календарную дату как `YYYY-MM-DD`; «meaning of any element depends on the context of its use» — стандарт не навязывает зону элементу без времени (Wikipedia/ISO 8601, «Calendar dates», «Omitting time and zone»).
- В OpenAPI `format: date` по спецификации — это ровно RFC 3339 `full-date` (§5.6), то есть `YYYY-MM-DD` (OpenAPI 3.0.3, таблица Data Types).
- Date-only значение **правильно** не несёт зоны: это календарный факт («день, когда что-то произошло»), а не момент времени. Зона появляется у источника семантики — у нас это таймзона владельца (ADR 0048), а не у полезной нагрузки. Промежуточные интерпретации «полночь по UTC/по локали клиента» — известная ловушка: библиотеки, парсящие date-only как полночь локали, сдвигают дату на день при конвертации смещений (Wikipedia/ISO 8601, «Pitfalls with date-only values»).
- Вывод: клиент присылает только `YYYY-MM-DD`; никакого `datetime`/`timestamp` от клиента, никаких полуночей на проводе.

### 1.2. Хранение: бизнес-дата — `DATE`, момент записи — `TIMESTAMPTZ`

- PostgreSQL `date` — «date (no time of day)», разрешение 1 день, без зоны — ровно семантика календарного факта (PostgreSQL docs, Table 8.9 Data Types).
- Момент записи — это уже точка на временной оси: PostgreSQL wiki «Don't Do This» прямо предписывает `timestamptz` (хранит инстанс в UTC, арифметика корректна через пояса и DST), а не `timestamp without time zone` — «a picture of a calendar and a clock rather than a point in time».
- Смешивать типы нельзя не потому, что «timestamp хуже», а потому что это разные сущности: отчёты, сортировки и границы периодов по хозяйственной дате должны быть стабильны независимо от того, когда и из какого пояса запись внесли. `DATE` даёт это по построению; timestamp с полуночью какой-то зоны — нет.

### 1.3. Две даты записи: бизнес-дата (event/value date) vs дата записи (processing/booking date)

- В обработке потоков это каноническое различие event time vs processing time: запись должна нести время события, а не время обработки — потребители требуют «event-time ordering and windowing by features of the data themselves» (Google, *The Dataflow Model*, VLDB 2015, abstract); искажения от processing time (задержки, ретраи, простои) ломают любую группировку по времени.
- В банкинге/бухучёте то же самое: **value date** (дата финансовой эффективности записи) vs **booking date** (дата, когда запись внесена в систему); различие существует ровно потому, что факты вносят задним числом, а проценты/отчёты считаются по value date (Wikipedia, «Value date»).
- Следствие для API: обе даты должны храниться раздельно и никогда не подменять друг друга. Backdating — нормальная операция записи фактов; «вылечить» её перезаписью created_at — ошибка аудита.

### 1.4. Валидация на границе: сервер — авторитет «сегодня»; reject, а не clamp

- Кто авторитетен по «сегодня»: сервер. Date-only полезная нагрузка без зоны и означает «сегодня в календаре источника семантики» — источник семантики определяет сервер по владельцу данных (у нас: ADR 0048, `users.timezone`, IANA; мутации считают «сегодня» по поясу владельца объекта, не действующего пользователя — п.4 ADR). Клиентские часы ненадёжны по построению.
- Reject vs молчаливый clamp: **reject (400)**. 400 — «server would not process the request due to something the server considered to be a client error… repeating the request without modification will fail with the same error» (MDN/RFC 9110 §15.5.1); принцип REST — «Self-descriptive error message… Use status codes, don't reinvent the wheel» (system-design-primer, раздел REST). Молчаливый clamp нарушает самодостаточность контракта: клиент думает, что записал одно, а записано другое — рассинхрон фактов, который никто не увидит в UI и логах. Clamp уместен там, где есть семантически верная цель подстановки (у нас нет: единственное верное значение «сегодня» клиент и так получает опущением поля).
- Семантика будущего для факта «рождён оплаченным»: оплата, которой не было, невозможна. Будущее в этой предметной области — домен *плановых* операций (в нашей системе они существуют: `status=planned`, тик материализации), а не ручного факта. Reject будущей даты — не произвол, а граница двух сущностей.
- Grace-период (допуск «завтра» при полуночи): это компенсация рассинхрона клиентских часов при timestamp-протоколах. При date-only + серверном авторитете он не нужен: клиент не претендует на знание «сейчас», он выбирает календарный день. Крайний случай смены пояса владельцем (ADR 0048 п.5: переезд на запад локально «откатывает» сегодня) reject по серверному today обрабатывает самокорректно — граница одна и серверная.

### 1.5. Идемпотентность: Idempotency-Key достаточен, бизнес-дата — не ключ дедупа

- POST по HTTP не идемпотентен («Can be called many times without different outcomes» — верно для GET/PUT/DELETE, не для POST: system-design-primer, таблица HTTP verbs) → повторам нужен явный механизм.
- Канонический паттерн — Stripe Idempotent Requests: клиент генерирует ключ (рекомендация — UUID), сервер сохраняет status+body первого ответа, повтор с тем же ключом возвращает сохранённый ответ; сравнение параметров оригинала защищает от «тот же ключ — другое тело»; TTL; результаты валидационных ошибок не сохраняются (docs.stripe.com/api/idempotent_requests).
- С клиентской датой это работает без изменений: дата входит в тело → входит в request-hash → ретрай того же логического запроса (та же дата) возвращает исходный 201; изменение даты с тем же ключом — клиентский баг, 409. Отдельного «дедупа по дате» не нужно и вредно: несколько реальных фактов в один календарный день — норма учёта. Дедуп-ключ материализации `(payment_id, date)` уникален для правил тика — manual-факт в него не попадает по построению (payment_id NULL).
- Вывод: существующий `platform/idempotency` (Reserve-first, replay, request-hash, 409 in-progress) покрывает вопрос полностью.

### 1.6. Аудит: «кто когда записал» и бизнес-дата — раздельные факты

- OWASP Logging Cheat Sheet: журнал должен фиксировать «when, where, who and what», рекомендована защита от подделки («tamper detection», «store or copy log data to read-only media»).
- В базе: `created_at/updated_at TIMESTAMPTZ DEFAULT now()` (момент записи, серверные часы, не трогается приложением) + отдельный audit log + пользовательская история (ADR 0061). Бизнес-дата в бизнес-колонках, момент записи в метаколонках — их уже нельзя перепутать на уровне типов.
- Backdating не должен трогать created_at: «когда внесли» ≠ «когда произошло». Наш дизайн это соблюдает.

### 1.7. Типичные грабли (сводка по источникам)

1. **Рассинхрон двойных дат** — хранить бизнес-дату в одной колонке, а «оплачено» в другой, и разъезжаться. Закрыто записью `date = paid_date = выбранная` при рождении факта оплаченным.
2. **Дедуп по бизнес-дате** — случайно отбрасывать легальные факты одного дня. Закрыто отсутствием уникальности на `(owner, date)`; дедуп материализации — только `(payment_id, date)`.
3. **Сортировки/отчёты** — лента, периоды и сводки должны читать бизнес-дату, чтобы бекдейт встал в правильный день. В нашем контексте уже так: `date_from/date_to`, `sort=paid_date`, keyset-курсор и summary читают те же колонки (`OperationSortKey`, ticket #992) — отдельной работы нет, нужны только тесты.
4. **Полночь/зона на проводе** — клиент не присылает время и зону; «сегодня» считает сервер (п.1.1, 1.4).
5. **Молчаливые правки ввода** — clamp/нормализация в сторону клиента; только явный 400 (п.1.4).

## 2. Сопоставление с нашим дизайном

| Ось | Лучшая практика | Наш дизайн | Вердикт |
|---|---|---|---|
| Формат | ISO 8601 full-date `YYYY-MM-DD`, OpenAPI `format: date` | optional `date` YYYY-MM-DD; контекст уже работает в `format: date` (`date_from/date_to`, `OperationResponse.date/paidDate`), транспорт — `openapi_types.Date` | Канонично; новое поле в том же формате |
| Хранение | Бизнес-дата `DATE`, момент `TIMESTAMPTZ` | `operations.date DATE NOT NULL`, `paid_date DATE`, `created_at/updated_at TIMESTAMPTZ DEFAULT now()` (миграция 000115) | Канонично |
| Авторитет «сегодня» | Сервер по календарю владельца данных | ADR 0048: «сегодня» в TZ владельца объекта (`users.timezone`, IANA, дефолт Europe/Moscow), в мутациях уже приходит `today` | Канонично |
| Будущая дата | Reject 400, самодостаточный текст ошибки | reject будущей даты владельца → 400 | Взять; оформить текст ошибки |
| Нижний пол | Прошлое без ограничения — норма для учёта (value date ретроспективна по определению) | нет нижнего пола | Канонично; не вводить искусственных ограничений («не раньше создания объекта» и т.п.) — YAGNI |
| Идемпотентность | Idempotency-Key + request-hash + replay (Stripe) | `platform/idempotency`: Reserve-first, replay 201/409, request-hash покрывает тело с датой | Достаточно; изменений нет |
| Аудит | created_at (record date) ≠ бизнес-дата; who/when/what; защита от подделки | created_at/updated_at + audit log + история (ADR 0061) | Канонично; created_at не трогать |

**Что изменить**: по существу — ничего. Косметика контракта:

- описание `CreateOperationRequest`/`POST …/operations` сегодня говорит «The date is server-set — today in the property owner's timezone» — при появлении `date` переформулировать: дата выбирается клиентом, отсутствует → «сегодня владельца», будущая → 400 (иначе контракт врёт документацией).
- Ошибка будущей даты — `ErrInvalidInput` (→ 400, существующее отображение), текст должен называть поле и правило («дата операции не может быть в будущем»), чтобы повтор без изменения запроса был осмысленно невозможен (RFC 9110/MDN).

**Что у нас уже канонично**: всё перечисленное в таблице, включая то, что грабли №1–№3 закрыты схемой и существующими чтениями (`SortByPaidDate`, period filters, keyset).

## 3. Чек-лист для тикета бекенда #1392

1. **OpenAPI**: в `CreateOperationRequest` добавить optional `date`: `type: string, format: date` (RFC 3339 full-date); описание — absent → today в TZ владельца (ADR 0048), future → 400, нижнего пола нет; обновить description самого POST (см. «Что изменить»). `OperationResponse` менять не нужно (`date`/`paidDate` уже `format: date`).
2. **Транспорт**: передать дату в `CreateOperationCommand` как `*openapi_types.Date` (паттерн `date_from/date_to` в `operation_handlers.go`); невалидный формат на проводе → 400 декодером.
3. **Домен** (`CreateOperation`): `chosen = cmd.Date ?? today`; валидация `chosen > today` (today — из owner calendar, уже в `runMutation`) → `ErrInvalidInput` с текстом про будущее. Рождение факта: `Date = PaidDate = chosen`, `Status = paid`, origin manual, payment_id NULL, id/created_at — без изменений.
4. **Idempotency**: изменений нет; дата автоматически входит в request-hash. Ретрай с тем же ключом и датой → исходный 201; смена даты под тем же ключом → 409 (существующая семантика `ErrKeyBodyMismatch`).
5. **Аудит/история**: `created_at` — момент записи (не выбранная дата); запись history `OperationCreated` уже несёт бизнес-дату — проверить, что это ожидаемо для UI истории.
6. **Тесты (TDD, швы: `CreateOperation`, `validateManualOperation`, транспорт)**:
   - absent → обе колонки = today владельца (существующее поведение не ломается);
   - вчерашняя дата → пишется в обе колонки, 201, status paid;
   - сегодняшняя == today → 201;
   - завтрашняя → 400 с текстом про будущее; дата на год вперёд → 400;
   - дата участником с Full Access в другом поясе — «сегодня» по владельцу объекта (ADR 0048 п.4);
   - идемпотентный ретрай с датой: тот же ключ+тело → 201 replay; другой ключ + та же дата → второй факт того же дня (дедупа нет);
   - бекдейт-факт попадает в правильный период `date_from/date_to` и стоит в правильном месте при `sort=paid_date` (грабля №3).
7. **Фронт-контракт** (вне #1392): пикер не предлагает будущие даты; текст 400 показывается как есть.

## Источники

- [donnemartin/system-design-primer](https://github.com/donnemartin/system-design-primer) — таблица HTTP-вербов (идемпотентность POST), раздел REST («Use status codes, don't reinvent the wheel»). Важно: сам primer глубже идемпотентности/таймзон не идёт — по этим осям опираемся на первоисточники ниже.
- [ISO 8601 — Wikipedia](https://en.wikipedia.org/wiki/ISO_8601) — календарная дата YYYY-MM-DD, отсутствие времени/зоны, ловушки date-only.
- [OpenAPI 3.0.3 — Data Types](https://spec.openapis.org/oas/v3.0.3#data-types) — `format: date` = RFC 3339 `full-date`.
- [PostgreSQL: Documentation — 8.5 DateTime Types](https://www.postgresql.org/docs/current/datatype-datetime.html) — семантика `date` (4 байта, день, без зоны) и `timestamptz` (инстанс в UTC).
- [PostgreSQL Wiki — Don't Do This](https://wiki.postgresql.org/wiki/Don%27t_Do_This) — не хранить моменты в `timestamp without time zone`.
- [Stripe — Idempotent Requests](https://docs.stripe.com/api/idempotent_requests) — Idempotency-Key: сохранение status+body, сравнение параметров, TTL, UUID-ключи.
- [Google — The Dataflow Model (VLDB 2015)](https://research.google/pubs/pub43864/) — event-time ordering как требование корректности потребителей.
- [Value date — Wikipedia](https://en.wikipedia.org/wiki/Value_date) — value date vs booking date: финансовая эффективность записи против момента внесения.
- [OWASP Logging Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Logging_Cheat_Sheet.html) — «when, where, who, what», tamper detection.
- [MDN — 400 Bad Request](https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/400) (по RFC 9110 §15.5.1) — семантика reject'а невалидного ввода.

## Локальные файлы, с которыми сверялись

- `apps/backend/internal/payments/application/operation_service.go` — `CreateOperation`, `CreateOperationCommand` (даты нет — server-set), `validateManualOperation`, `OperationSortKey` (#992).
- `apps/backend/api/openapi/openapi.yaml` — `POST /properties/{propertyId}/operations` (#1515), `CreateOperationRequest` (#7662), `OperationsDateFrom/To` (#5380, `format: date`), `OperationResponse`.
- `docs/adr/0048-payments-tick-drivers-owner-timezone.md` — источник «сегодня», TZ владельца объекта, смена пояса.
- `apps/backend/db/migrations/000115_payments_context.up.sql` — `operations.date DATE NOT NULL`, `paid_date DATE`, `created_at/updated_at TIMESTAMPTZ`, unique `(payment_id, date)`.
- `apps/backend/internal/platform/idempotency/service.go` — Reserve-first, replay, request-hash, 409.
- `docs/adr/0061-action-history.md` — пользовательская история.
