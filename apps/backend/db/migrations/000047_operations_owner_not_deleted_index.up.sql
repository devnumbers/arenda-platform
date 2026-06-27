CREATE INDEX IF NOT EXISTS idx_operations_owner_not_deleted
  ON operations(owner_id, operation_date DESC)
  WHERE deleted_at IS NULL;
