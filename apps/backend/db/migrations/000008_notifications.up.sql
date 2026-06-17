CREATE TYPE notification_target_type AS ENUM ('operation', 'recurring_operation', 'lease');
CREATE TYPE notification_event_type AS ENUM ('operation_due', 'lease_expiring', 'lease_requires_action');
CREATE TYPE notification_status AS ENUM ('pending', 'sent', 'failed', 'cancelled');

CREATE TABLE reminders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_type notification_target_type NOT NULL,
    operation_id UUID REFERENCES operations(id) ON DELETE CASCADE,
    recurring_operation_id UUID REFERENCES recurring_operations(id) ON DELETE CASCADE,
    lease_id UUID REFERENCES leases(id) ON DELETE CASCADE,
    property_id UUID REFERENCES properties(id) ON DELETE CASCADE,
    event_type notification_event_type NOT NULL,
    status notification_status NOT NULL DEFAULT 'pending',
    scheduled_at TIMESTAMPTZ NOT NULL,
    sent_at TIMESTAMPTZ,
    failed_attempts INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ,
    message_title TEXT NOT NULL,
    message_body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT exactly_one_target CHECK (
        (target_type = 'operation' AND operation_id IS NOT NULL AND lease_id IS NULL) OR
        (target_type = 'recurring_operation' AND recurring_operation_id IS NOT NULL AND operation_id IS NULL AND lease_id IS NULL) OR
        (target_type = 'lease' AND lease_id IS NOT NULL AND operation_id IS NULL AND recurring_operation_id IS NULL)
    )
);

CREATE UNIQUE INDEX idx_reminders_active_unique
ON reminders (owner_id, target_type, operation_id, recurring_operation_id, lease_id, event_type)
WHERE status IN ('pending', 'sending');

CREATE INDEX idx_reminders_due ON reminders (scheduled_at, next_attempt_at) WHERE status = 'pending';
CREATE INDEX idx_reminders_owner ON reminders (owner_id, status, scheduled_at);
CREATE INDEX idx_reminders_operation ON reminders (operation_id);
CREATE INDEX idx_reminders_recurring ON reminders (recurring_operation_id);
CREATE INDEX idx_reminders_lease ON reminders (lease_id);

CREATE TABLE sent_sms_reminders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reminder_id UUID REFERENCES reminders(id) ON DELETE SET NULL,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    phone TEXT NOT NULL,
    message TEXT NOT NULL,
    provider_response TEXT,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
