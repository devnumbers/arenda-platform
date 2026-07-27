ALTER TABLE operations
    DROP CONSTRAINT IF EXISTS operations_status_check;

ALTER TABLE operations
    ADD CONSTRAINT operations_status_check
        CHECK (status IN ('pending', 'overdue', 'paid', 'received', 'unconfirmed'));
