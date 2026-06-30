DROP INDEX IF EXISTS idx_recurring_operations_owner_not_deleted;

ALTER TABLE recurring_operations
    DROP COLUMN deleted_at;
