UPDATE recurring_operations
SET reminder_offset_days = NULL
WHERE reminder_offset_days IS NOT NULL
  AND reminder_offset_days NOT IN (1, 3, 7);

ALTER TABLE recurring_operations
  DROP CONSTRAINT IF EXISTS recurring_operations_reminder_offset_days_check;

ALTER TABLE recurring_operations
  ADD CONSTRAINT recurring_operations_reminder_offset_days_check
  CHECK (reminder_offset_days IN (1, 3, 7));

UPDATE operations AS o
SET reminder_offset_days = ro.reminder_offset_days,
    updated_at = now()
FROM recurring_operations AS ro
WHERE o.recurring_operation_id = ro.id
  AND o.owner_id = ro.owner_id
  AND ro.reminder_offset_days IN (1, 3, 7)
  AND o.is_exception = false
  AND o.operation_date >= CURRENT_DATE
  AND o.deleted_at IS NULL
  AND o.reminder_offset_days IS DISTINCT FROM ro.reminder_offset_days;
