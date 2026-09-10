# ADR 0053: контекст «Аренда» — схема БД, жизненный цикл и контракты первого среза

## Status

Accepted. Решения гриллинга тикета #528 (2026-09-04, владелец) поверх словаря и 14 решений #527 (`rentals/CONTEXT.md`). Уточнения решений #527, зафиксированные здесь: «Продление» сужено до UI-сценария правки плановой даты (отдельного бэкенд-понятия и аудита `rental.extended` нет); пометка «контакт удалён» у арендатора отменена — после удаления контакта арендатор просто «Контакта нет»; удаление разрешено ещё и для не начавшейся аренды (узкая ревизия решения №10).

## Context

Доменная модель аренды согласована словарём (#527): аренда — период занятости объекта с условиями, управляющая ровно одним Платежом контекста payments (доход, категория `rent`); статусы вычисляемые; незавершённая аренда на объекте ровно одна; итоги — живой расчёт по paid-операциям объекта. Этот ADR — последний слой решений перед реализацией (#529): как модель ложится на PostgreSQL и OpenAPI по правилам репозитория (ADR 0003 sqlc/pgx, ADR 0008 копейки BIGINT, ADR 0019 UUIDv7 app-side, ADR 0028 actor/scope, ADR 0033 UoW). Образцы — ADR 0049 (payments) и ADR 0051 (tasks); TZ собственника — ADR 0048.

## Decision

### 1. Схема (миграция `000120_rentals_context`)

Одна таблица. Полный DDL — каноническая форма решения; миграция повторяет его дословно плюс конвенционный триггер `set_updated_at`:

```sql
-- Аренда = период занятости объекта с условиями. Идентификаторы — UUIDv7 из
-- приложения (ADR 0019); деньги — BIGINT-копейки (ADR 0008). owner_id
-- денормализован из property (ADR 0028: SQL фильтрует по scope-владельцу);
-- тотальное удаление объекта — каскадом. Связь с платежом 1:1 и NOT NULL:
-- платёж аренды нельзя удалить мимо аренды (честный 409 из payments-эндпоинта).
CREATE TABLE rentals (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    property_id UUID NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    payment_id UUID NOT NULL UNIQUE REFERENCES payments(id) ON DELETE RESTRICT,
    contact_id UUID REFERENCES contacts(id) ON DELETE SET NULL,
    start_date DATE NOT NULL,
    planned_end_date DATE,
    completed_date DATE,
    utilities TEXT NOT NULL CHECK (utilities IN ('included', 'meters_only', 'full_receipt')),
    deposit_kopecks BIGINT CHECK (deposit_kopecks >= 0),
    commission_kopecks BIGINT CHECK (commission_kopecks >= 0),
    deposit_return_kopecks BIGINT CHECK (deposit_return_kopecks >= 0),
    deposit_return_comment TEXT,
    comment TEXT CHECK (char_length(comment) <= 2000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT rentals_planned_end_after_start_check
        CHECK (planned_end_date IS NULL OR planned_end_date > start_date),
    CONSTRAINT rentals_completed_after_start_check
        CHECK (completed_date IS NULL OR completed_date >= start_date),
    CONSTRAINT rentals_deposit_return_only_completed_check
        CHECK (completed_date IS NOT NULL OR
               (deposit_return_kopecks IS NULL AND deposit_return_comment IS NULL)),
    CONSTRAINT rentals_deposit_return_comment_needs_amount_check
        CHECK (deposit_return_comment IS NULL OR deposit_return_kopecks IS NOT NULL)
);
```

Дюральный инвариант №12 — «одна незавершённая на объект» (завершённых может быть много) — частичный уникальный индекс:

```sql
CREATE UNIQUE INDEX idx_rentals_one_unfinished_per_property
    ON rentals(property_id) WHERE completed_date IS NULL;
CREATE INDEX idx_rentals_property ON rentals(property_id);
CREATE INDEX idx_rentals_owner ON rentals(owner_id);
```

Обоснования формы:

- **Дата завершения — единственный хранимый статус** (№1): `completed_date DATE NOT NULL`-пометка завершённости; «Ожидает начала / Активная / Ожидает действия» вычисляются на чтении. Статус-колонки и надгробий нет (прецедент tasks).
- **`UNIQUE (payment_id)`**: связь аренды с платежом 1:1 дюрально — один платёж не может управляться двумя арендами.
- **`RESTRICT` на платеже / `SET NULL` на контакте**: решения №4 и №11 — платёж не удалить мимо аренды (409 из payments-эндпоинта), удаление контакта обнуляет ссылку арендатора; владельца контакта проверяет приложение (`contacts.owner_id = scope`, FK существование не проверяет).
- **`planned_end_date NULL`** — бессрочная аренда (№2); очистка в правке превращает срочную в бессрочную и обнуляет `end_date` платежа.
- **Залог, комиссия, возврат залога — записи** (№6): колонки на аренде, операций и платежей не порождают. Возврат возможен только у завершённой (CHECK), комментарий — только при сумме; сумма 0 валидна («не вернул»), связь с размером залога не проверяется.
- **Дня оплаты на аренде нет** (№5): рендер читается из платежа; ответ API дублирует его полем `paymentDay` (см. §4).
- Даты — `DATE`-строки в TZ собственника (ADR 0048), как `since`/`end_date` платежа.

### 2. Статусы и вычисляемые поля чтения

Сервер считает всё сам — клиент пояса не знает (прецедент tasks/payments); `today` — по TZ собственника (ADR 0048):

- `status`: `completed` (есть `completed_date`); иначе `upcoming` (`start_date > today`); иначе `needs_attention` (`planned_end_date` наступила и прошла: `planned_end_date < today`; в день планового окончания ещё активна); иначе `active`. У бессрочной `needs_attention` не бывает (№2).
- `progress`: `paidMonths` — число paid-операций Платежа арендной платы; `totalMonths` — число дней оплаты в периоде `[start_date, planned_end_date]` (только срочная); `monthsRemaining` — полных календарных месяцев от `today` до `planned_end_date` (только срочная). Заголовок детализации «Оплачено N из M месяцев» (макет #531).
- `rentPayment.nextPayment` — единственное будущее planned-вхождение платежа ( payments держат ровно одно): `operationId`, `date`, `amountKopecks`, `daysUntil = date − today`; `null`, когда будущего вхождения нет (после планового окончания, у завершённой).
- `today` — в каждом ответе аренды (границы статусов и секций на клиенте).

### 3. Жизненный цикл и сериализация с payments-тиком

Дисциплина мутаций — конвейер payments (ADR 0049 §3): каждая мутация аренды = role gate (ADR 0028) → `SELECT … FOR UPDATE` строки property → `today` собственника → изменение → аудит в той же транзакции (ADR 0020, in-tx fail-safe) → in-tx прогон тика payments, если платёж менялся.

**Шов между контекстами**: `PaymentService` непригоден напрямую — каждый его вызов открывает собственную транзакцию через собственный `txStoreFactory`. Аренда работает в **одной** UoW-транзакции (ADR 0033): tx-фабрика аренды композитная — строит и rentals-сторы, и payments-сторы (`txStores`) в одном tx; приложение Rentals объявляет порт `RentPaymentGateway` («порты объявляет consumer»), реализация в wiring переиспользует внутреннюю механику payments — конструктор recurrence, план тика и его применение, снос будущего planned — как обычное изменение платежа, без второго лока property и второго конвейера. Правка суммы/дня оплаты/автоплатежа/`end_date` из аренды переставляет единственное будущее planned тем же in-tx тиком.

Мутации:

- **Создание** (№4): аренда + платёж атомарно. Платёжу: `since = start_date` (старт сегодня → ровно `today`; ретро-материализации нет), recurrence `monthly` из дня оплаты — 1…30 → `daysOfMonth=[N]`, 31/«последний день» → `lastDay: true` (одно поведение, №5), `end_date = planned_end_date` (NULL у бессрочной), `amount_kopecks`, `auto_pay`; дефолты: доход, категория `rent`, title «Арендная плата», форма оплаты `transfer`. Вторая незавершённая на объекте — 409 (app-проверка под локом; partial unique страхует гонку).
- **Правка условий** (№14): сумма, день оплаты, автоплатёж, плановое окончание, коммуналка, залог, комиссия, комментарий, контакт — синхронно в платёж (сумма, recurrence-день, `auto_pay`, `end_date`). **Начало не правится.** Плановое окончание правится свободно — вперёд и назад, но не в прошлое (≥ `today`, > начала); отдельного понятия «продление» на бэкенде нет — экран «Продлить аренду» (#533) есть UI-сценарий этого поля с правилом «строго вперёд» (сужение №9, решение #528).
- **Завершение** (№8): явное действие, тело несёт дату (начало ≤ дата ≤ сегодня; «По плану» — подстановка UI). `completed_date` записывается, `end_date` платежа = дате завершения; строго будущие planned (дата > даты завершения) сносятся механикой платежа, наступившие остаются. Возврат залога — опциональные запись-сумма и комментарий. Повторное завершение — 409.
- **Удаление** (№10 + ревизия #528): hard-delete аренды **вместе с платежом** — только для завершённой или **не начавшейся** (`start_date > today`: передумал до старта; все planned у неё будущие и сносятся чисто, просрочек нет). Начавшаяся незавершённая — только через завершение (409). Семантика `keep_overdue=true`: просроченные операции остаются долгом, переживая платёж через `ON DELETE SET NULL` + `origin`. Порядок в транзакции: сначала строка аренды, затем платёж (RESTRICT немедленный). Только Owner.
- **Доступ** (ADR 0028 через policy-порт): Viewer — чтение; Full Access — создание, правка, завершение, оплата; Owner — удаление.
- **Аудит**: `rental.created | rental.updated | rental.completed | rental.deleted`; `extended` нет (продление — правка). Bulk-тик не пишется пооперационно (расширенный гэп ADR 0020).
- **«Оплатить платёж»** на детализации — существующий `POST /properties/{propertyId}/operations/{operationId}/pay` по `nextPayment.operationId`; новый эндпоинт не нужен.

### 4. Контракты первого среза (OpenAPI)

Пути вложены под объект (прецедент payments/contacts); спека — единый файл `api/openapi/openapi.yaml` (ADR 0002, ADR 0049 §4), фронтовый клиент регенерируется.

- `POST /properties/{propertyId}/rentals` — создание аренды (= атомарное создание платежа). Тело: `amountKopecks` (1…10⁹), `paymentDay` — целое 1…31 **или** строка `"last"`, `startDate` (сегодня или позже по TZ собственника, иначе 400), `plannedEndDate?` (строго позже начала), `utilities` (`included | meters_only | full_receipt`), `depositKopecks?`/`commissionKopecks?` (0…10⁹), `contactId?` (из книги владельца скоупа), `comment?` (≤2000), `autoPay`. Ответ — созданная аренда (201). Вторая незавершённая — 409.
- `GET /properties/{propertyId}/rentals` — плоский список `items[]` без пагинации: ожидающая/активная первыми, далее завершённые по `completedDate` — свежие сверху.
- `GET|PATCH|DELETE /properties/{propertyId}/rentals/{rentalId}` — чтение / частичная правка / удаление (см. §3). PATCH: все поля optional; nullable-поля (`plannedEndDate`, `depositKopecks`, `commissionKopecks`, `contactId`, `comment`) — tri-state, явный `null` = очистить; завершённая — 409 на мутации.
- `POST …/rentals/{rentalId}/complete` — завершение (тело: `completedDate`, `depositReturn?: {amountKopecks 0…10⁹, comment?}`); ответ — обновлённая аренда.
- `GET …/rentals/{rentalId}/summary?until=ДАТА` — итоги (№13): `{from, until, incomeKopecks, expenseKopecks, profitKopecks}` — **все paid-операции объекта** (любого платежа и ручные) с датой вхождения в периоде `[start_date, until]`; период — по дате вхождения, не по дате оплаты (опоздавший платёж считается месяцем вхождения). Прибыль = доходы − расходы, может быть отрицательной. Мастер завершения зовёт с выбранной датой **до** фактического завершения (превью); по умолчанию `until = completed_date` (у завершённой) или `today`.
- Ответ аренды: `id`, `propertyId`, `status`, `startDate`, `plannedEndDate?`, `completedDate?`, `utilities`, `depositKopecks?`, `commissionKopecks?`, `depositReturnKopecks?`/`depositReturnComment?`, `tenant`, `comment`, `rentPayment: {paymentId, amountKopecks, paymentDay, autoPay, nextPayment?}`, `progress`, `today`, `createdAt`. Уточнение #531 (2026-09-05): `rentPayment.paymentId` — идентификатор управляемого Платежа для перехода на экран платежа с детализации аренды; платеж не ищется слагом категории — на объекте бывают и другие платежи `rent`.
- `tenant` — встроенный объект `{contactId, firstName, lastName, phone}` или `null`. После удаления контакта — просто `null` без пометки: UI пишет «Контакта нет» (решение #528, отмена пометки из №11).

### 5. Словарь

`rentals/CONTEXT.md` уточняется тем же коммитом: статья «Продление» описывает UI-сценарий правки плановой даты «строго вперёд» (полная правка условий направлению не подчиняется); в статье «Арендатор» пометка о «контакте удалён» не появляется — ссылка просто обнуляется.

## Considered Options

- **Отдельный эндпоинт `POST …/extend`** — отклонён: плановое окончание — обычное поле правки условий (макет «Изменить условия», №14); два пути к одной дате дали бы две расходящиеся валидации. «Продление» — экран поверх PATCH, №9 сужен до UI-правила «строго вперёд».
- **Пометка удалённого контакта (флаг `tenantContactDeleted` или снапшот имени)** — отклонена владельцем: после удаления контакта арендатор просто «Контакта нет»; лишнего поля и денормализации не нужно.
- **Вызов `PaymentService` из use cases аренды** — отклонён: сервис мутирует через собственный конвейер с собственной транзакцией (`txStoreFactory`), вложить его в UoW аренды нельзя; шов — композитная tx-фабрика + порт `RentPaymentGateway` над внутренней механикой payments.
- **Статус-колонка аренды** — отклонён решением №1 (прецедент tasks): хранится только факт завершения.
- **День оплаты колонкой на аренде** — отклонён решением №5: одна механика с payments-recurrence, копия стала бы вторым источником истины.
- **Снапшот итогов при завершении** — отклонён решением №13: живой расчёт по paid-операциям объекта.
- **Отдельная таблица возвратов залога** — отклонена: возврат — разовая запись завершения, две колонки на аренде выражают её целиком (CHECK'и держат консистентность).

## Consequences

- (+) Все дюральные инварианты — в БД: одна незавершённая на объект (partial unique), 1:1 с платежом (`UNIQUE(payment_id)` + RESTRICT), границы дат, «возврат только у завершённой», «комментарий возврата только при сумме», лимит комментария; остальное — идемпотентный тик payments.
- (+) Единая точка сериализации (property row lock) и один in-tx тик на мутацию — та же дисциплина, что в payments; гонки правки аренды с тиком закрыты.
- (+) Клиент TZ-слеп: статусы, прогресс, «дней до платежа», `today` и итоги считает сервер.
- (−) Известная конкуренция за номер миграции: `000120` в тексте этого ADR и в ADR 0052 (tasks-global, ворктри ещё не слит). Финальный номер закрепляется при вливании — следующим свободным (прецедент contacts 000118→000119); имя `000120_rentals_context` в тексте — условное.
- (−) Номер ADR — 0053: 0052 занят tasks-global.
- (~) Приложение обязано поддерживать `owner_id` = `properties.owner_id` при каждой вставке (денормализация ADR 0028) и валидировать `contacts.owner_id = scope` при указании арендатора (FK существование не проверяет владельца).
- (~) Удаление пары аренда+платёж требует порядка: сначала аренда, затем платёж (RESTRICT немедленный).
- Миграция проходит `make migrations-lint` (BIGINT-копейки, UUID без DEFAULT, lock-safety); интеграционные тесты репозиториев, шва с payments и use cases — testcontainers PostgreSQL 18.

## See also

- Карта wayfinder #526; тикет #527 (словарь и 14 решений — вход), #528 (решения этой сессии), #529 (реализация, /tdd).
- ADR 0047–0049 (Payments: словарь, тик и TZ, схема и контракты — образец), ADR 0051 tasks (прецедент нового контекста), ADR 0054 contacts (книга контактов, nullable property), ADR 0046 (снос legacy-аренд), ADR 0028 (actor/scope), ADR 0033 (UoW), ADR 0020 (аудит; расширенный гэп), ADR 0048 (TZ собственника), ADR 0002/0003/0008/0019 (contract-first / pgx+sqlc / копейки / UUIDv7).
- `apps/backend/internal/rentals/CONTEXT.md` — словарь контекста; `CONTEXT-MAP.md` — Properties → Rentals → Payments/Contacts.
