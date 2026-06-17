-- HAZARD: this rollback may fail if a non-deleted operation shares
-- (recurring_operation_id, operation_date) with a previously soft-deleted one,
-- because the restored unique index does not exclude deleted_at IS NOT NULL rows.
DROP INDEX IF EXISTS idx_operations_recurring_date_unique;
CREATE UNIQUE INDEX idx_operations_recurring_date_unique
ON operations(recurring_operation_id, operation_date)
WHERE recurring_operation_id IS NOT NULL;
