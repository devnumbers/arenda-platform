ALTER TABLE operations
    ADD COLUMN source_operation_date DATE;

UPDATE operations
SET source_operation_date = operation_date
WHERE source_operation_date IS NULL
  AND (
      recurring_operation_id IS NOT NULL
      OR (lease_id IS NOT NULL AND category = 'rent' AND is_exception = false)
  );
