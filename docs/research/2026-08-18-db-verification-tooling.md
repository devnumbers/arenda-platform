# Research: проверки БД сверх действующего контура — SQL, схема, миграции, целостность данных

- Тикет: [#322](https://github.com/devnumbers/arenda-platform/issues/322) (карта #319)
- Дата: 2026-08-18
- Метод: статический анализ репо + живые пробы (docker `postgres:18-alpine`, все 107 миграций применены; `sqlc vet v1.31.1` прогнан на живой базе) + web-research по первоисточникам (docs.sqlc.dev, squawkhq.com / sbdchd/squawk docs, postgresql.org/docs/18, pgtap.org, pypi, GitHub).
- Границы: отвергнутое картой #289 не переоткрывается без новой причины — Atlas (платно), sqlfluff/pgspot/pganalyze-lint (не тот скоуп) перепроверялись только на предмет «изменилась ли цена». Не изменилась.

## 0. Действующий контур (зафиксированные факты)

- **squawk v2.62.0** (`.squawk.toml`): lock-safety; `assume_in_transaction = true`, `pg_version = "18"`; правило-уровневые исключения `require-lock-timeout`/`require-statement-timeout`/`constraint-missing-not-valid` с пометкой «ввести конвенцию для новых миграций — отдельное решение» (= зона 3 этого research).
- **Доменные правила** `tools/migration-lint/domain-rules.mjs`: деньги-BIGINT (запрет numeric/decimal/float-типов тотально), запрет DEFAULT на id (ADR 0019); statement-aware парсинг; 27 контракт-тестов.
- **up/down/up-цикл**: `TestMigrationsUpDownUpCycle` (`apps/backend/internal/platform/database/migrations_cycle_integration_test.go`, собственный testcontainers-контейнер) + CI-джоба `backend-migrations`.
- **Freshness-гейты**: `make backend-sqlc-check` (sqlc v1.31.1 generate + diff по хэшам), `backend-openapi-check`, `frontend-api-check`.
- **EXPLAIN-прецедент**: `TestLoginCodeQueryPlan_IndexUsageForGetLatest` (`apps/backend/internal/identity/application/login_code_query_plan_integration_test.go`) — уже точечный план-тест с сидированием 10k строк + `ANALYZE`.
- **Масштаб**: 107 up-миграций (+107 down), 19 файлов `db/queries`, 289 именованных запросов, sqlc → `internal/platform/generated/postgres`; вся цепочка миграций на пустой базе `postgres:18-alpine` применяется суммарно < 1 с (замер пробы; живой объём stage неизвестен, но таблицы продукта ранней стадии — 30 таблиц).

## 1. SQL-запросы

Первоисточник: [docs.sqlc.dev — vet](https://docs.sqlc.dev/en/latest/howto/vet.html).

Факты по sqlc vet (сверено с живой пробой на pinned v1.31.1 + PostgreSQL 18):

- **Built-in правило одно: `sqlc/db-prepare`** — каждая query прогоняется через prepare против живой БД (`database.uri` в конфиге). Правил `query-arg/limit` и подобных в актуальной документации **нет** — упоминание в тикете, видимо, из старых материалов; всё остальное — кастомные правила.
- **Кастомные правила — CEL-выражения** (`rule: query.sql.contains("DELETE")`, `query.params.size() > 1`, `query.cmd == "exec"`); объявляются в `rules:` конфига, подключаются списком `rules:` у пакета; оптаут аннотацией `/* @sqlc-vet-disable */` (можно с именами правил).
- **EXPLAIN-правила с v1.20.0**: с подключённой БД доступны `postgresql.explain.plan.total_cost`, `postgresql.explain.plan.node_type == 'Seq Scan'` и т.п. — т.е. «EXPLAIN-гейт» у sqlc есть из коробки; наша v1.31.1 поддерживает.
- **Схему sqlc сам не накатывает** — нужна уже мигрированная база (database.uri на testcontainers/local).
- **Живая проба**: `sqlc vet` с `sqlc/db-prepare` + два кастомных правила против базы со всеми 107 миграциями — **db-prepare прошёл по всем 289 запросам** (запросы и схема консистентны); правило `node_type == 'Seq Scan'` дало 6 находок (GetUserByID/ByPhone/ByEmail и др.) — все **ложные на пустой базе**: планировщик на пустых таблицах всегда выбирает seq scan. EXPLAIN-гейт осмыслен только против базы с сидированными данными реалистичного объёма (как в существующем login_codes-тесте).
- **Факт про SELECT ***: CEL-правило `query.sql.contains("SELECT *")` **не находит ничего** — sqlc раскрывает `*` в список колонок до этапа vet; правило `contains("*")` ловит только `COUNT(*)`-запросы. Т.е. писать no-star правило для CEL бессмысленно. Запрет `SELECT *` в sqlc-контуре и так не нужен: sqlc типизирует результат по схеме, дрейф ловится generate + freshness-гейтом, а не звёздочкой.

| Кандидат | Цена | Выгода | Рекомендация |
| --- | --- | --- | --- |
| `sqlc/db-prepare` в CI (make-цель с testcontainers/локальной базой + `rules:` в sqlc.yaml) | Низкая: один make-target по образцу `backend-sqlc-check` (поднять контейнер → применить миграции → `sqlc vet`); в репо-конфиг добавится `database.uri`/`rules` (wiring — решает тикет #324) | Реальная компиляция каждого запроса против PostgreSQL 18 с применёнными миграциями — ловит дрейф «запрос vs схема» и типовые ошибки до рантайма; свежесть миграций проверяется «бесплатно» (миграция, ломающая запросы, роняет vet) | **Принять** |
| CEL-паттерн-правила (no-DELETE, params-limit и т.п.) | Минимальная (строки в yaml) | Узкие точечные запреты; универсальной потребности сейчас нет | Держать в запасе; вводить по конкретному правилу, не пакетом |
| EXPLAIN-гейт в sqlc vet (`Seq Scan`, total_cost) | Средняя: осмыслен только против сидированной базы; сид-скрипты — та же цена, что и Go-EXPLAIN-тесты | Покрытие всех запросов разом, но с ложными срабатываниями без качественного сидирования | **Не принимать** как общий гейт; расширять точечными integration-тестами по образцу `TestLoginCodeQueryPlan_IndexUsageForGetLatest` (уже в стеке, сидирование и ANALYZE уже решены там) |
| Запрет SELECT * | — | — | **Не нужно**: sqlc раскрывает `*` и типизирует по схеме; freshness-гейт ловит дрейф (факт пробы выше) |

## 2. Схема: полнота инвариантов

Живая проба: база со всеми 107 миграциями, аудит через `information_schema`/`pg_constraint` (скрипты одноразовые, в `.tmp/sqlc-vet/reference-schema.sql` — эталонный `pg_dump --schema-only`, 2236 строк).

Факты по схеме:

- **30 таблиц** (вкл. `schema_migrations`); **56 FK** на 27 таблицах; без FK только `users`, `tariffs`, `schema_migrations` (корневые/справочные — корректно).
- **Деньги защищены полностью**: все 9 money-колонок (`*_kopecks`) имеют CHECK неотрицательности (`>= 0`, у `subscription_payments.amount_kopecks` — `> 0`), плюс инвариант refund: `refunded_amount_kopecks IS NULL OR = amount_kopecks`.
- **Enum-статусы** — CHECK-списками (`leases`, `operations`, `property_members`, `subscription_payments`, `user_subscriptions`); прецедент доведения инвариантов до БД — миграция `000103_identity_data_invariants` (CHECK порядка времени для `sessions`, `login_attempts`) с явной формулировкой в комментарии: «инварианты, которые домен не гарантирует для путей вне приложения».
- **Найденные пробелы-кандидаты** (защищено только кодом, могло бы быть констрейнтом):
  1. `leases`: нет `CHECK (end_date IS NULL OR end_date > start_date)` — порядок дат только в коде (end_date nullable = открытая аренда; семантику подтвердить по домену leases).
  2. `login_codes`: нет `CHECK (expires_at > created_at)` — 000103 закрыл sessions, но не login_codes.
  3. `property_contacts.owner_id` — `UUID NOT NULL` без FK на `users`: единственная NOT NULL доменная ссылка без FK в схеме (полиморфный `audit_log.entity_id` и провайдерские `provider_*_id` легитимно без FK). Либо осознанное решение (уточнить), либо недосмотр.
- **Подходы аудита**: готового open-source «аудитора полноты инвариантов» нет (это предметное знание, не механика). Рабочие подходы: (а) information_schema/`pg_constraint`-запросы против testcontainers-базы — разовые или как Go-тест; (б) инвариант-тесты в существующем integration-стиле (INSERT с нарушением → ожидание ошибки констрейнта); (в) pgTAP ([pgtap.org](https://pgtap.org/documentation.html): `col_not_null`, `has_fk`, `fk_ok`, `has_pk`) — зрелый TAP-фреймворк именно для таких тестов, но это новый extension в образе БД + отдельный раннер (`pg_prove`) — новая зависимость ради того, что Go-тест против testcontainers делает теми же SQL-запросами.

| Кандидат | Цена | Выгода | Рекомендация |
| --- | --- | --- | --- |
| Разовый/триггерный information_schema-аудит (запросы этого research уже готовы: ссылки-без-FK, money-CHECK-покрытие, range-CHECK-покрытие) | Низкая: разовые SQL-запросы против testcontainers-базы | Систематический список «инвариант только в коде» вместо интуиции | **Принять** как практику при добавлении новых сущностей (запросы сохранить в тикете/скрипте) |
| Точечные новые CHECK (3 кандидата выше) | Низкая: одна миграция по образцу 000103 | Защита инварианта для всех путей записи, не только приложения | **Принять после доменной сверки** (решение — #324/#325) |
| Go integration-тесты инвариантов (INSERT-нарушение → ошибка) | Средняя | Проверяет, что констрейнт реально существует и не «забыт» откатом | Опционально для критичных инвариантов (биллинг), не как общий гейт |
| pgTAP | Средняя: extension в образе + `pg_prove` + TAP-конвенции | Те же проверки, что Go-тестом, в стороннем фреймворке | **Не принимать** — дублирует возможности существующего стека |

## 3. Миграции: конвенции сверх squawk

Первоисточники: [squawk require-timeout-settings](https://github.com/sbdchd/squawk/blob/master/docs/docs/require-timeout-settings.md) (правило раз_split на `require-lock-timeout` + `require-statement-timeout`, [PR #1233](https://github.com/sbdchd/squawk/pull/1233)), [squawk safe_migrations](https://squawkhq.com/docs/safe_migrations), [PostgreSQL 18 ALTER TABLE](https://www.postgresql.org/docs/18/sql-altertable.html).

Факты:

- **Таймаут-правила squawk** требуют в начале каждого файла: `SET lock_timeout = '1s'; SET statement_timeout = '5s';`. Обоснование (safe_migrations): без lock_timeout миграция, вставшая в очередь за долгой транзакцией, держит запросы приложения за собой (очередь за ACCESS EXCLUSIVE) — сервис может лечь; с коротким lock_timeout миграция просто падает и ретраится. `SET` внутри транзакции golang-migrate действует на неё же — самодостаточно, не зависит от настроек коннекта/роли (squawk-доки допускают «таймауты уже настроены на коннекте» как escape hatch — но у нас миграции гоняют и CLI (`make migrate-up`), и testdb; файл-уровень надёжнее).
- **Внедрение без ретрофита истории**: у squawk нет baseline-файла и glob-диапазонов в `excluded_paths` — включение правил потребовало бы перечислить все 107 исторических файлов. Дешевле — правило с cutoff в своём `domain-rules.mjs`: «файлы с номером >= N обязаны начинаться с SET lock_timeout/statement_timeout» (~30 строк, парсер уже есть, контракт-тесты уже есть). Squawk-правила после этого включить и удалить точечное исключение не выйдет без списка путей — оставить их выключенными, конвенцию держит доменное правило.
- **NOT VALID + VALIDATE** (PostgreSQL 18): `ADD CONSTRAINT ... NOT VALID` не сканирует таблицу (FK — SHARE ROW EXCLUSIVE вместо ACCESS EXCLUSIVE у CHECK), `VALIDATE CONSTRAINT` сканирует под SHARE UPDATE EXCLUSIVE, не блокируя DML. **Выгоды в одной транзакции нет** — смысл именно в двух коммитах (сначала констрейнт, потом валидация). У нас golang-migrate = одна транзакция на файл → нужно два файла (и два down), при том что вся цепочка на пустой базе исполняется < 1 с, таблицы малы, бэкфиллы исторически делались в той же транзакции (`000035`, `000077`). Squawk-правило `constraint-missing-not-valid` уже выключено решением #295 по этой причине.
- **Замер пробы**: 107 миграций на `postgres:18-alpine` — суммарно менее секунды CPU-времени на файлах; риск «долгой миграции» на наших объёмах сегодня не от размера таблиц, а от очереди блокировок за длинной транзакцией приложения — что закрывает именно lock_timeout.

| Кандидат | Цена | Выгода | Рекомендация |
| --- | --- | --- | --- |
| `SET lock_timeout`/`statement_timeout` в каждом новом файле + правило с cutoff в domain-rules.mjs | Низкая: ~30 строк правила + тесты; 2 строки в каждой новой миграции; историю не трогаем | Устраняет главный операционный риск миграций (зависание за длинной транзакцией с каскадной очередью); самодостаточно на любом пути применения | **Принять** (cutoff-номер и значения таймаутов — параметр решения #324; доковый ориентир squawk 1s/5s) |
| NOT VALID + VALIDATE как конвенция для FK/CHECK | Средняя: два файла на констрейнт, усложнение down и up/down/up-цикла | Нужна только при больших таблицах с живым трафиком | **Не принимать** сейчас; вернуть к вопросу, когда появятся таблицы, где VALIDATE-скан заметен (trigger-based gate review) |
| CONCURRENTLY-индексы | — | — | Уже решено (#295): невозможно в транзакции golang-migrate; без новой причины не трогать |

## 4. Живая БД stage: периодическая проверка целостности

Факты (web-research):

- **Готового open-source аналога DBCC-проверок для vanilla PostgreSQL нет** — общепринятая практика: собственные SQL-скрипты (orphan rows, нарушения инвариантов) по расписанию ([обсуждение dba.se](https://dba.stackexchange.com/questions/55762/database-consistency-checker-in-postgresql)); пакет `pg_integrity_check` — проприетарный Postgres Professional.
- **Физическая целостность**: встроенный extension [amcheck](https://www.postgresql.org/docs/18/amcheck.html) — `bt_index_check` берёт лишь AccessShareLock (как SELECT), безопасен на живой базе, не требует суперпользователя; `verify_heapam` для таблиц. Ловит то, что не ловят checksums (логическая порча). Для нас — на будущее как «страховка» при подозрениях, не ежедневная потребность на малых объёмах.
- **Дрейф схемы миграции-vs-факт**: эталон — testcontainers-база со всеми миграциями (эталонный `pg_dump --schema-only` этого research: `.tmp/sqlc-vet/reference-schema.sql`, 2236 строк). Сравнение: `pg_dump --schema-only` обеих баз + diff — нулевые новые зависимости, самодостаточно в CI. Специализированный диффер `migra` **депрекирован в 2024** (последний релиз ~2022); живые форки есть (`migra-maintained`, PyPI, релиз 2025-06), но Python-зависимость ради задачи, решаемой pg_dump+diff, не нужна.
- **Дрейф данных-инвариантов**: SQL-запросы вида `SELECT ... WHERE NOT EXISTS (FK-партнёр)` для всех 56 FK + специфичные доменные запросы (например, незакрытые lease с end_date < start_date, refund-инвариант). БД-уровень FK уже гарантирует отсутствие orphan для 27 таблиц — проверять стоит «мягкие» места: полиморфные ссылки (`audit_log.entity_id`), ссылочные колонки без FK (`property_contacts.owner_id`), инварианты без констрейнта.
- **Механика**: scheduled GitHub Actions job (`schedule:`) на существующем self-hosted runner (деплой-контур уже работает по SSH с него — docs/adr/0024); job: поднять testcontainers-эталон → `pg_dump --schema-only` stage и эталона → diff → psql-запросы инвариантов; расхождение ≠ ноль → алерт (существующий контур Uptrace-email не для CI-артефактов — проще issue/уведомление runner-джобы).

| Кандидат | Цена | Выгода | Рекомендация |
| --- | --- | --- | --- |
| Scheduled CI job: дрейф схемы (pg_dump diff эталон↔stage) | Низкая: ~1 workflow-файл, psql-клиент из docker-образа postgres, SSH-доступ уже отработан в деплой-контуре | Ловит ручные изменения схемы на stage (сегодня невидимы); дешёвый «canary» целостности контура | **Принять** |
| Scheduled CI job: SQL-запросы инвариантов/orphan на stage | Низкая: SQL-файл с запросами; ненулевой счётчик = red job | Ловит дрейф данных против доменных инвариантов (вкл. «мягкие» ссылки без FK) | **Принять** (набор запросов — решение #324; первые кандидаты из зоны 2) |
| migra / форки | Python-инструмент в CI | Красивый диф вместо сырого pg_dump-diff | **Не принимать** (оригинал мёртв, форк — новая зависимость ради косметики) |
| amcheck (`bt_index_check`) в scheduled job | Низкая: `CREATE EXTENSION` + один SQL | Физическая порча индексов; на малых объёмах вероятность низкая | Опционально добавить в тот же job одной строкой; не самостоятельный кандидат |
| Готовые «DBCC-аналоги» | — | — | Таковых в open source нет (факт выше); единственный packaged — проприетарный |

## Итог: что нести в решение (#324)

**Принять (цена низкая, выгода реальная):**

1. `sqlc/db-prepare` в CI (живая база с миграциями; пробой подтверждено: 289/289 готово, ловит дрейф запрос-схема).
2. `SET lock_timeout`/`statement_timeout` в новых миграциях + cutoff-правило в `domain-rules.mjs` (историю не трогаем, squawk-правила остаются выключенными).
3. Scheduled CI-джоба на self-hosted runner: pg_dump-diff дрейфа схемы stage + SQL-запросы инвариантов/orphan; опционально amcheck `bt_index_check` туда же.

**Принять после доменной сверки (миграция по образцу 000103):** CHECK `leases (end_date IS NULL OR end_date > start_date)`, CHECK `login_codes (expires_at > created_at)`, решение по FK `property_contacts.owner_id → users`.

**Не принимать (без новой причины не возвращаться):** общий EXPLAIN-гейт в sqlc vet (шум без сидирования; точечные Go-план-тесты уже покрывают паттерн), NOT VALID+VALIDATE как конвенция (объёмы; одна транзакция на файл), pgTAP (дубль стека), migra/форки (мёртв/лишняя зависимость), запрет SELECT * (sqlc раскрывает и типизирует — факты пробы). Переоткрытие Atlas/sqlfluff/pgspot/pganalyze — новой причины не появилось.

## Первоисточники

- sqlc vet (built-in `sqlc/db-prepare`, CEL-правила, EXPLAIN-правила v1.20+, `@sqlc-vet-disable`): https://docs.sqlc.dev/en/latest/howto/vet.html
- Squawk: список правил — https://squawkhq.com/docs/rules; require-timeout-settings (сплит на require-lock-timeout/require-statement-timeout) — https://github.com/sbdchd/squawk/blob/master/docs/docs/require-timeout-settings.md, PR https://github.com/sbdchd/squawk/pull/1233; обоснование lock_timeout — https://squawkhq.com/docs/safe_migrations
- PostgreSQL 18: ALTER TABLE (NOT VALID/VALIDATE, уровни блокировок) — https://www.postgresql.org/docs/18/sql-altertable.html; amcheck — https://www.postgresql.org/docs/18/amcheck.html
- pgTAP (функции теста схемы) — https://pgtap.org/documentation.html
- Отсутствие open-source DBCC-аналога, практика SQL-скриптов — https://dba.stackexchange.com/questions/55762/database-consistency-checker-in-postgresql
- migra: депрекация оригинала (2024, последний релиз ~2022), форк migra-maintained (2025-06) — PyPI/GitHub (см. поиск в тикете).

## Артефакты пробы

- Эталонная схема после 107 миграций: `.tmp/sqlc-vet/reference-schema.sql` (pg_dump --schema-only, postgres:18-alpine).
- Проба sqlc vet: `.tmp/sqlc-vet/sqlc.yaml` (db-prepare + no-seq-scan/no-star правила), `.tmp/sqlc-vet/probe.yaml` (contains("*")). Пробный контейнер `arenda-research-pg` поднят и погашен этим research-сеансом; чужой `arenda-local-postgres-1` не трогался.
