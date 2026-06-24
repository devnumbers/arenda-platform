ALTER TABLE operations
    ADD COLUMN status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'overdue', 'paid', 'received'));

CREATE INDEX idx_operations_owner_pending_operation_date ON operations(owner_id, operation_date)
    WHERE deleted_at IS NULL AND status = 'pending';
