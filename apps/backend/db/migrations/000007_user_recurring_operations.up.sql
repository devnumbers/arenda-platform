ALTER TABLE recurring_operations
    ALTER COLUMN lease_id DROP NOT NULL;

ALTER TABLE recurring_operations
    ADD COLUMN periodicity TEXT NOT NULL DEFAULT 'monthly' CHECK (periodicity = 'monthly'),
    ADD COLUMN comment TEXT,
    ADD COLUMN status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused'));

UPDATE recurring_operations
SET periodicity = 'monthly',
    status = 'active'
WHERE periodicity IS NULL
   OR status IS NULL;
