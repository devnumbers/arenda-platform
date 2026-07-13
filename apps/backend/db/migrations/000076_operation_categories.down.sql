DROP INDEX IF EXISTS idx_operation_categories_owner_code;
DROP TRIGGER IF EXISTS trg_operation_categories_updated_at ON operation_categories;
DROP TABLE IF EXISTS operation_categories;
