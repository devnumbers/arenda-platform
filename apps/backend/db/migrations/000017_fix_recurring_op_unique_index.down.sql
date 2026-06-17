DROP INDEX IF EXISTS idx_operations_recurring_date_unique;
CREATE UNIQUE INDEX idx_operations_recurring_date_unique
ON operations(recurring_operation_id, operation_date)
WHERE recurring_operation_id IS NOT NULL;
