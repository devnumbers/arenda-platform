SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Down: убрать trgm-индексы поиска и обёртку поиска контактов (тикет #598).
-- Расширение pg_trgm остаётся в базе: прецедент pgcrypto (000001) —
-- расширения не дропаются, откат снимает только объекты этой миграции.
DROP INDEX IF EXISTS idx_properties_address_trgm;
DROP INDEX IF EXISTS idx_properties_name_trgm;
DROP INDEX IF EXISTS idx_contacts_search_trgm;
DROP FUNCTION IF EXISTS contacts_search_text(text, text, text, text, text, text, text);
DROP INDEX IF EXISTS idx_operations_category_label_trgm;
DROP INDEX IF EXISTS idx_operations_title_trgm;
DROP INDEX IF EXISTS idx_payments_title_trgm;
