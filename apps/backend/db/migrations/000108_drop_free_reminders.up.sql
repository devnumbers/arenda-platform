-- Contract phase of the FreeReminder removal (spec #380; decisions #268 —
-- no data migration, #277 — this exact drop list). The code contract
-- (#381/#382) is already deployed: nothing reads or writes free reminders
-- anymore. The enum values 'free'/'free_reminder' stay in the PostgreSQL
-- types forever (#277); the worker's eternal target_type <> 'free' filter
-- remains the guard of the dead value.

-- 1. Per-channel settings of the removed event type.
DELETE FROM user_notification_channel_preferences WHERE event_type = 'free_reminder';

-- 2. Concrete rows of free templates. Email/push delivery rows go with them
--    via their ON DELETE CASCADE foreign keys.
DELETE FROM reminders WHERE target_type = 'free';

-- 3. Rebuild exactly_one_target without the 'free' branch.
ALTER TABLE reminders DROP CONSTRAINT exactly_one_target;
ALTER TABLE reminders ADD CONSTRAINT exactly_one_target CHECK (
    (target_type = 'operation' AND operation_id IS NOT NULL AND lease_id IS NULL) OR
    (target_type = 'recurring_operation' AND recurring_operation_id IS NOT NULL AND operation_id IS NULL AND lease_id IS NULL) OR
    (target_type = 'lease' AND lease_id IS NOT NULL AND operation_id IS NULL)
);

-- 4. The template link column; idx_reminders_free_reminder goes with it.
ALTER TABLE reminders DROP COLUMN free_reminder_id;

-- 5. The template table itself.
DROP TABLE free_reminders;
