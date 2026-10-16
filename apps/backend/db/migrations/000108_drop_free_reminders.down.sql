-- Revert 000108 structurally, following the original migrations 000090
-- (table) and 000091 (column + constraint): recreate free_reminders, the
-- reminders.free_reminder_id column and the 'free' branch of
-- exactly_one_target. The enum values were never removed (#277), so no type
-- work is needed. Data destroyed by the up migration is NOT restored
-- (#268): templates, concrete rows, channel settings and delivery history
-- are gone for good; only the pre-deploy dump can bring them back manually.
CREATE TABLE free_reminders (
    id          uuid         NOT NULL,
    owner_id    uuid         NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    property_id uuid         NOT NULL REFERENCES properties (id) ON DELETE CASCADE,
    title       text         NOT NULL CHECK (length(title) BETWEEN 1 AND 50),
    trigger_at  timestamptz  NOT NULL,
    periodicity text         NOT NULL DEFAULT 'once'
                  CHECK (periodicity IN ('once', 'daily', 'weekly', 'monthly', 'yearly')),
    created_at  timestamptz  NOT NULL DEFAULT now(),
    updated_at  timestamptz  NOT NULL DEFAULT now(),
    PRIMARY KEY (id)
);

CREATE INDEX idx_free_reminders_owner    ON free_reminders (owner_id);
CREATE INDEX idx_free_reminders_property ON free_reminders (property_id);

CREATE TRIGGER trg_free_reminders_updated_at
BEFORE UPDATE ON free_reminders
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

ALTER TABLE reminders
    ADD COLUMN free_reminder_id uuid REFERENCES free_reminders (id) ON DELETE CASCADE;

ALTER TABLE reminders DROP CONSTRAINT exactly_one_target;
ALTER TABLE reminders ADD CONSTRAINT exactly_one_target CHECK (
    (target_type = 'operation' AND operation_id IS NOT NULL AND lease_id IS NULL) OR
    (target_type = 'recurring_operation' AND recurring_operation_id IS NOT NULL AND operation_id IS NULL AND lease_id IS NULL) OR
    (target_type = 'lease' AND lease_id IS NOT NULL AND operation_id IS NULL) OR
    (target_type = 'free' AND free_reminder_id IS NOT NULL AND operation_id IS NULL AND recurring_operation_id IS NULL AND lease_id IS NULL)
);

CREATE INDEX idx_reminders_free_reminder ON reminders (free_reminder_id);
