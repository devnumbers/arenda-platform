DELETE FROM recurring_operations
WHERE lease_id IS NULL;

ALTER TABLE recurring_operations
    DROP COLUMN periodicity,
    DROP COLUMN comment,
    DROP COLUMN status;

ALTER TABLE recurring_operations
    ALTER COLUMN lease_id SET NOT NULL;
