# ADR 0049: контекст «Платежи» — схема БД, тик и контракты первого среза

## Status

Accepted. Amended (2026-09-01, удаление операций — решение владельца из grilling-сессии): хранимый словарь статусов операции расширен третьим значением `cancelled` — статус-надгробие удалённого вхождения (строка остаётся с ключом `(payment_id, date)` и подавляет повторную материализацию тиком; факт оплаты и долг исчезают). Миграция `000117`, словарь — «Отменённая операция» в `payments/CONTEXT.md`.

Amended (2026-09-08, ручной порядок избранных — карта #573, тикет #576): `payments` расширяется колонкой `favorite_order BIGINT NULL` (миграция `000122`) рядом с `is_favorite` (#461) и CHECK-инвариантом `favorite_order IS NULL OR is_favorite`. Решение «позиция на правиле, не отдельной таблицей»: избранный — правило целиком (канон #461), дюрального инварианта, оправдывающего отдельную таблицу, у порядка нет. `NULL` = правило никогда не было в сохранённом порядке — новое избранное сортируется в конец («новое избранное — в конец»); снятие звезды очищает позицию тем же атомарным UPDATE PUT favorite, повторное — снова в конец. Правка порядка — Full Access и выше (матрица звезды #461, `CanEdit`; viewer — 403; решено на code-review #576: иначе read-only участник менял бы то же поле, которое сам записывать не может). Сохранение порядка — `PUT /payments/favorites/order`: плотные позиции 1..N по списку id в одной транзакции (FOR UPDATE-локи по отсортированным id), полная замена — избранное вне списка теряет позицию и уходит в конец (дубли позиций после сохранения невозможны), пустой список сбрасывает порядок; только видимые актёру неархивные избранные правила — невидимое и чужое даёт приватный 404, видимое не-избранное или дубль — 400. Словарь — «Избранное» в `payments/CONTEXT.md`.

## Context

Доменная модель контекста Payments проверена прототипом и перенесена словарём (ADR 0047); драйверы тика и часовой пояс собственника зафиксированы ADR 0048; жизненный цикл платежа и объекта в БД — тикетом #446 (hard-delete платежа, `operations.origin`, тотальное удаление объекта, read-only архив). Этот ADR — последний слой решений перед реализацией: как модель ложится на PostgreSQL и OpenAPI по правилам репозитория (ADR 0003 sqlc/pgx, ADR 0008 копейки BIGINT, ADR 0019 UUIDv7 app-side, ADR 0028 actor/scope, ADR 0033 UoW). Решения приняты grilling-сессией тикета #448 (2026-08-25).

Одновременно фиксируется ревизия модели прототипа: **автоплатёж-догон убран** (решения №4–№5 прототипа пересмотрены владельцем продукта по правилу HANDOFF). Автоматика оплаты — только через корректный ежечасный воркер в день наступления вхождения; ретроспективных компенсаций нет, надёжность воркера закрывается его корректностью и алертингом (та же философия, что у ревизии «догона при чтении» в решении №1 карты #443).

## Decision

### 1. Схема (миграция `000115_payments_context`)

Четыре таблицы. Полный DDL — каноническая форма решения:

```sql
-- Пользовательские категории уровня аккаунта (дефолтный каталог — в коде,
-- tools/payment-categories; ссылка из платежей — слагом). Категория нейтральна
-- к направлению (#447): доход/расход — поле платежа, не справочника.
CREATE TABLE payment_categories (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (owner_id, name)
);
CREATE TRIGGER trg_payment_categories_updated_at
    BEFORE UPDATE ON payment_categories
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Платёж = правило (ADR 0047). Идентификаторы — UUIDv7 из приложения (ADR 0019);
-- деньги — BIGINT-копейки (ADR 0008). owner_id денормализован из property
-- (ADR 0028: SQL фильтрует по scope-владельцу); тотальное удаление объекта —
-- каскадом (тикет #446, ревизия ADR 0025).
CREATE TABLE payments (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    property_id UUID NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('income', 'expense')),
    title TEXT NOT NULL,
    amount_kopecks BIGINT NOT NULL CHECK (amount_kopecks > 0),
    recurrence JSONB NOT NULL
        CHECK (recurrence->>'kind' IN ('daily', 'weekly', 'monthly', 'yearly')),
    since DATE NOT NULL,
    end_date DATE,
    auto_pay BOOLEAN NOT NULL DEFAULT false,
    payment_form TEXT NOT NULL CHECK (payment_form IN ('transfer', 'cash')),
    category_slug TEXT,
    user_category_id UUID REFERENCES payment_categories(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT payments_since_end_date_check
        CHECK (end_date IS NULL OR end_date >= since),
    CONSTRAINT payments_category_exactly_one
        CHECK (num_nonnulls(category_slug, user_category_id) = 1)
);
CREATE TRIGGER trg_payments_updated_at
    BEFORE UPDATE ON payments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Пауза: интервалы [from_date, to_date), to_date IS NULL — активная бессрочная.
-- Отдельная таблица (не jsonb на платеже): не более одной открытой паузы —
-- дюральный инвариант partial unique index; пауза/возобновление — атомарные
-- INSERT/UPDATE без read-modify-write массива.
CREATE TABLE payment_pauses (
    id UUID PRIMARY KEY,
    payment_id UUID NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    from_date DATE NOT NULL,
    to_date DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT payment_pauses_interval_check
        CHECK (to_date IS NULL OR to_date >= from_date)
);
CREATE TRIGGER trg_payment_pauses_updated_at
    BEFORE UPDATE ON payment_pauses
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE UNIQUE INDEX idx_payment_pauses_one_active
    ON payment_pauses(payment_id) WHERE to_date IS NULL;
CREATE INDEX idx_payment_pauses_payment ON payment_pauses(payment_id);

-- Операция = вхождение либо ручной факт. payment_id ON DELETE SET NULL +
-- origin отличает «платёж удалён» (origin='payment' AND payment_id IS NULL)
-- от ручной операции (origin='manual') — тикет #446. Дедупликация
-- материализации — уникальный (payment_id, date); просрочка не хранится.
CREATE TABLE operations (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    property_id UUID NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    payment_id UUID REFERENCES payments(id) ON DELETE SET NULL,
    origin TEXT NOT NULL CHECK (origin IN ('payment', 'manual')),
    date DATE NOT NULL,
    paid_date DATE,
    status TEXT NOT NULL CHECK (status IN ('planned', 'paid')),
    type TEXT NOT NULL CHECK (type IN ('income', 'expense')),
    title TEXT NOT NULL,
    amount_kopecks BIGINT NOT NULL CHECK (amount_kopecks > 0),
    payment_form TEXT CHECK (payment_form IN ('transfer', 'cash')),
    category_label TEXT NOT NULL,
    category_slug TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT operations_paid_consistency
        CHECK ((status = 'paid') = (paid_date IS NOT NULL))
);
CREATE TRIGGER trg_operations_updated_at
    BEFORE UPDATE ON operations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE UNIQUE INDEX idx_operations_payment_date
    ON operations(payment_id, date) WHERE payment_id IS NOT NULL;
CREATE INDEX idx_payments_property ON payments(property_id);
CREATE INDEX idx_payments_owner ON payments(owner_id);
CREATE INDEX idx_payments_user_category ON payments(user_category_id);
CREATE INDEX idx_operations_property_date ON operations(property_id, date);
CREATE INDEX idx_operations_owner_date ON operations(owner_id, date);
```

Обоснования формы:

- **Категория — XOR-ссылка**: `category_slug` (дефолтный каталог в коде, ADR 0047 п.4) либо `user_category_id`; `num_nonnulls = 1` — дюрально. Удаление занятой пользовательской категории — `RESTRICT`: история неприкосновенна благодаря снапшоту `category_label` на операции, живую ссылку владелец перелинковывает вручную. Слаг, убранный из каталога кода, — фолбэк вида «прочее» при чтении.
- **Recurrence — jsonb + CHECK на kind**: SQL регулярность никогда не читает (перечисление вхождений — чистая функция домена, date-строчная арифметика прототипа); полный валидатор — доменный конструктор (прецедент `properties.attributes`).
- **Форма оплаты**: `payment_form` на платеже `NOT NULL` (значения `transfer | cash`); операция наследует снапшотом при материализации/оплате; `NULL` на операции оставляет место ручным операциям (их контракты — позже).
- **Инвариант «ровно одно будущее `planned`»** — app-side, поддерживается тиком; статическим индексом не выражается («будущее» двигается вместе с `today`).
- **`since`** ставится сервером при создании (сегодня по TZ собственника, ADR 0048) и недоступен для правки; генерация задним числом исключена (решение №17 прототипа).

### 2. Автоплатёж без догона (ревизия №4–№5 прототипа)

- Материализация неизменна: все вхождения с `date ≤ today` (≥ `since`, ≤ `end_date`, вне интервалов пауз) материализуются в операции.
- Тик гасит у автоплатежа **только `planned` с `date = today`** (`paid_date = today`) — в день наступления; активная пауза останавливает автогашение (вхождения дня в паузе не генерируются, потому UPDATE естественно no-op).
- `date < today` автоматика **никогда** не трогает — у любого платежа, включая автоплатёж: просрочка/долг, закрытие только вручную.
- Следствие: простой воркера в день наступления оставляет у автоплатежа долг. Компенсаций нет — надёжность обеспечивается корректностью воркера и алертингом «тик не прогонялся» (ADR 0048), не ретроспективными записями.

### 3. Тик в application-слое

- `domain` — чистые функции прототипа: перечисление вхождений с прижатием 31-го, интервалы пауз, горизонт «всё наступившее + ровно одно следующее». Никаких часов и I/O; «сегодня» приходит параметром (CODING_STANDARDS: `clock.Clock`, даты — date-строки).
- `application` — прогон тика per-owner внутри `UoW` (ADR 0033): правила + паузы + существующие ключи операций → INSERT материализации с `ON CONFLICT (payment_id, date) DO NOTHING`, автогашение дня, пересборка единственного будущего `planned`. Порты объявляет consumer: репозитории платежей/операций, резолвер TZ собственника.
- **Сериализация: каждая мутация контекста и каждый прогон тика первым делом берут `SELECT … FOR UPDATE` строки property** внутри своей транзакции (существующий прецедент — ADR 0025 §5, закрытие гонки архивации с тиком). Единая точка упорядочивания закрывает гонки update-vs-tick (два разных «следующих» вхождения), archive-vs-tick, delete-vs-tick. Дедуп-индекс страхует вставки независимо от блокировки.
- **Воркер** — единственный фоновый драйвер: ежечасная регистрация в `platform/scheduler` (`runTickerLoop` + advisory lock лидера), обход зон по ADR 0048 (DISTINCT TZ собственников с платежами на активных/в maintenance объектах → свой `today` на зону), per-owner UoW внутри зоны (изоляция отказов), heartbeat-алерт успешного прогона.
- **Мутации** гоняют тот же прогон тика в своей транзакции после изменения — как `actions.ts` прототипа. Чистые GET не пишут (решение №1 карты #443).

### 4. Контракты первого среза (OpenAPI)

Пути вложены под объект (прецедент photos/contacts); пауза/возобновление — парой POST по образцу `archive`/`unarchive`:

- `POST /properties/{propertyId}/payments` — создание; тело: `type`, `title`, `amountKopecks` (1…10⁹), `recurrence` (oneOf по kind), `paymentForm`, `categorySlug`, опционально `endDate`, `autoPay`. `since` серверное.
- `GET /properties/{propertyId}/payments` — список платежей объекта.
- `GET|PATCH|DELETE /properties/{propertyId}/payments/{paymentId}` — чтение/частичная правка (editable-поля прототипа; `since` и `categorySlug`-ссылку правила меняет PATCH, снимок на операциях не трогает)/удаление; `DELETE` с query `keep_overdue` (default `true`): `planned` с `date ≥ today` сносятся всегда, просроченные — при `false`.
- `POST /properties/{propertyId}/payments/{paymentId}/pause` и `…/resume` — бессрочная пауза с сегодняшнего дня / закрытие интервала сегодняшним.
- `GET /properties/{propertyId}/payments/{paymentId}/operations` — операции платежа; `status: planned | paid | overdue` — **просрочку вычисляет сервер** (по `today` собственника), клиент TZ не знает.
- `POST /properties/{propertyId}/operations/{operationId}/pay` — «Оплатить сейчас»: `planned → paid`, `paidDate = today`, расписание не сдвигается.

Категории: эндпоинтов в первом срезе нет — дефолтный каталог фронтенд берёт из сгенерированного TS-модуля каталога; create-контракт принимает только `categorySlug`. Эндпоинты категорий (GET каталога с пользовательскими + CRUD своих) и оживание `user_category_id` в контрактах — следующим срезом. Доступ — матрица ADR 0028 через policy-порт: Viewer — чтение; Full Access — создание/правка/оплата/пауза; Owner — ещё и удаление платежа.

Спека остаётся **единым файлом** `api/openapi/openapi.yaml` (сегодня 3623 строки; два потребителя — oapi-codegen с embedded-spec и openapi-typescript фронтенда — читают его напрямую). Разбиение отложено с критериями: >6–8k строк, частые конфликты, третий потребитель. Безопасная форма на будущее: источники в `api/openapi/src/` + бандлер (redocly) → закоммиченный собранный `openapi.yaml`, потребители не меняются.

### 5. Ревизии и аудит

- **ADR 0025 → superseded этим ADR**: режимы удаления объекта убраны — удаление тотальное (оба FK из `payments`/`operations` на property — CASCADE), `mode` уходит из API вместе с nullable-`property_id` и `DeletePropertyMode`; detach-семантика «финансовая история без объекта» закрыта вместе с доменом аренд (ADR 0046).
- **ADR 0020 — расширение принятого гэпа**: тик bulk не пишется пооперационно — ни материализация, ни автогашение дня, ни в воркере, ни внутри мутаций (побочки мутации покрываются её собственным audit-действием). Пользовательские мутации — in-tx fail-safe: `payment.created|updated|deleted|paused|resumed`, `operation.paid` (ручная оплата).
- **ADR 0048** — правка скобки о no-op-прогонах (упоминание догона убрано вместе с механизмом).
- `payments/CONTEXT.md`: «Автоплатёж» — гашение в день наступления; «Материализация» — уточнён горизонт (всё наступившее + ровно одно следующее будущее).

## Considered Options

- **Паузы jsonb-массивом на платеже** — отклонено: инвариант одной открытой паузы становится договорённостью, интервалы валидируются только приложением, конкурентные правки массива перетирают друг друга.
- **Recurrence плоскими колонками** (`kind + weekdays[] + day_of_month + month + day`) — отклонено: cross-kind CHECK'и громоздки, sqlc-маппинг шумный, SQL данные всё равно не читает.
- **Категория одной строковой колонкой (слаг или uuid-строка)** — отклонено: нет FK и типобезопасности; **снапшот label на платеже** — отклонено: денормализация там, где нужна живая ссылка (снапшот — только на операциях); **ON DELETE SET NULL у `user_category_id`** — отклонено: молча ломает инвариант «категория задана».
- **Автоплатёж с догоном (статус-кво прототипа №4–№5)** — отвергнут владельцем продукта: автоматика строго в день наступления, ретроспективных записей нет; самолечение простоя воркера признано хуже честного долга.
- **Разбиение openapi.yaml сейчас** — отклонено: два прямых потребителя и требование self-contained embedded-spec делают сплит формой «источники + бандл» с лишним артефактом; порог боли не достигнут.
- **Per-owner last_tick-таблица** — уже отклонена ADR 0048 как отложенная оптимизация; дедуп-индекс делает прогоны между полуночами дешёвым no-op.

## Consequences

- (+) Все дюральные инварианты — в БД: дедуп материализации, одна открытая пауза, согласованность paid/paid_date, XOR-категория, границы since/end_date; остальное — идемпотентный тик.
- (+) Единая точка сериализации (property row lock) вместо разрозненных блокировок; дедуп-индекс страхует даже при её обходе.
- (+) Срез контрактов минимален и самодостаточен для экранов «создание платежа» и «страница платежа» (тикет #449); черновики queries и OpenAPI-фрагмента приложены к резолюции #448 — вход для `/to-spec`.
- (−) Простой воркера в день наступления автоплатежа оставляет долг: закрывается алертингом и корректностью воркера, не компенсацией (решение владельца).
- (−) `RESTRICT` на удаление занятой категории требует перелинковки платежей вручную — честный отказ вместо молчаливой потери ссылки.
- (~) Приложение обязано поддерживать `owner_id` = `properties.owner_id` при каждой вставке (денормализация по ADR 0028); расхождение исключено транзакционным созданием через резолв property.
- Миграция должна проходить `make migrations-lint` (BIGINT-копейки, UUID без DEFAULT, lock-safety); интеграционные тесты репозиториев и тика — testcontainers PostgreSQL 18.

## See also

- Карта wayfinder #443, тикет #448 (решения этой сессии); #446 (жизненный цикл — вход), #445 + ADR 0048 (драйверы и TZ), #447 (каталог категорий).
- ADR 0047 (словарь и границы), ADR 0028 (actor/scope), ADR 0033 (UoW), ADR 0020 (аудит; расширенный гэп), ADR 0025 (superseded), ADR 0003/0008/0019 (pgx+sqlc / копейки / UUIDv7).
- `apps/backend/internal/payments/CONTEXT.md` — словарь (правки «Автоплатёж», «Материализация»).
