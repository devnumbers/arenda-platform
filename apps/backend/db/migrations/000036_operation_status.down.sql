DROP INDEX IF EXISTS idx_operations_owner_pending_operation_date;

ALTER TABLE operations
    DROP COLUMN IF EXISTS status;
