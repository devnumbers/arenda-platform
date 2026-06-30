ALTER TABLE recurring_operations
    ADD COLUMN deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_recurring_operations_owner_not_deleted
  ON recurring_operations(owner_id, created_at DESC)
  WHERE deleted_at IS NULL;
