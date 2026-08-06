-- Pending property member invitations by email (T5, issue #161).
-- An invitation is stored while the invited email is not registered; it never
-- expires (no TTL, no sweeper job) and activates automatically when a user
-- registers with the same email. Cancellation deletes the row, so re-inviting
-- a previously cancelled email is free. Roles mirror property_members; the
-- invitee's email is stored in lowercase for case-insensitive matching and
-- never appears in audit context (ADR 0020).
--
-- IDs are application-generated UUIDv7 (ADR 0019): no DB DEFAULT.
CREATE TABLE property_member_invitations (
    id           UUID         PRIMARY KEY,
    property_id  UUID         NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    email        TEXT         NOT NULL,
    role         TEXT         NOT NULL CHECK (role IN ('full_access', 'viewer')),
    invited_by   UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    last_sent_at TIMESTAMPTZ  NOT NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- One pending invitation per (property, email), case-insensitive.
CREATE UNIQUE INDEX idx_property_member_invitations_property_email
    ON property_member_invitations (property_id, lower(email));

-- Activation lookup at registration: all pending invitations for an email.
CREATE INDEX idx_property_member_invitations_lower_email
    ON property_member_invitations (lower(email));

CREATE TRIGGER trg_property_member_invitations_set_updated_at
    BEFORE UPDATE ON property_member_invitations
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
