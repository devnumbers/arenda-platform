SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- GIN-индексы pg_trgm под все ILIKE-поля поиска платформы (карта #596,
-- тикет #598): платежи, операции, контакты, объекты. До этого поиск —
-- ILIKE '%…%' по seq scan; trigram-индекс даёт Bitmap Index Scan на
-- подстрочный поиск, включая ILIKE с неконстантным паттерном-параметром.
-- CONCURRENTLY недоступен (миграция golang-migrate исполняется одной
-- транзакцией, .squawk.toml assume_in_transaction) — таблицы на текущих
-- объёмах малы, обычный CREATE INDEX в транзакции принят конвенцией #324.

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX idx_payments_title_trgm
    ON payments USING gin (title gin_trgm_ops);

CREATE INDEX idx_operations_title_trgm
    ON operations USING gin (title gin_trgm_ops);

CREATE INDEX idx_operations_category_label_trgm
    ON operations USING gin (category_label gin_trgm_ops);

-- Поиск контактов матчит склейку полей одним выражением
-- (db/queries/contacts.sql). Голый concat_ws — STABLE, а индексное
-- выражение обязано быть IMMUTABLE, поэтому склейка объявлена обёрткой
-- с фиксированными text-аргументами и константным разделителем: для них
-- concat_ws детерминирован, метка IMMUTABLE честна. Запрос ищет через ту
-- же функцию — иначе planner не сопоставит индекс с предикатом.
CREATE FUNCTION contacts_search_text(
    first_name text, last_name text, patronymic text,
    role text, phone text, email text, messenger_username text
) RETURNS text
LANGUAGE sql IMMUTABLE PARALLEL SAFE
RETURN concat_ws(' ', first_name, last_name, patronymic, role, phone, email, messenger_username);

CREATE INDEX idx_contacts_search_trgm
    ON contacts USING gin (
        (contacts_search_text(first_name, last_name, patronymic, role, phone, email, messenger_username))
        gin_trgm_ops
    );

CREATE INDEX idx_properties_name_trgm
    ON properties USING gin (name gin_trgm_ops);

CREATE INDEX idx_properties_address_trgm
    ON properties USING gin (address gin_trgm_ops);
