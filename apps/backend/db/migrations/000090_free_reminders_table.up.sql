-- Template table for free (user-created) reminders: arbitrary reminders tied to
-- a property but not to an operation or lease (e.g. renew insurance, check
-- meters). Periodic free reminders repeat indefinitely until deleted. The
-- property FK is ON DELETE CASCADE (like property_photos): free reminders are
-- deleted in both property deletion modes of ADR 0025 without touching the
-- delete use case.
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
