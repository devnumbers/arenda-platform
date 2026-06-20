CREATE INDEX idx_operations_lease_not_deleted ON operations(lease_id, operation_date) WHERE deleted_at IS NULL;
CREATE INDEX idx_operations_property_not_deleted ON operations(property_id, operation_date) WHERE deleted_at IS NULL;
CREATE INDEX idx_operations_recurring_not_deleted ON operations(recurring_operation_id, operation_date) WHERE deleted_at IS NULL;
