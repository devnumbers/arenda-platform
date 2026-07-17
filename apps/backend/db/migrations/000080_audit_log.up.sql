-- Audit log: append-only journal of user, admin, and system actions.
-- Ids are application-generated UUIDv7 (ADR 0019), so id has no DEFAULT;
-- created_at is supplied by the application. actor_id uses ON DELETE SET NULL
-- so that journal entries survive deletion of the acting user.
CREATE TABLE audit_log (
    id          uuid PRIMARY KEY,
    created_at  timestamptz NOT NULL,
    actor_id    uuid REFERENCES users (id) ON DELETE SET NULL,
    actor_role  text NOT NULL CHECK (actor_role IN ('owner', 'admin', 'system', 'anonymous')),
    action      text NOT NULL,
    entity_type text,
    entity_id   uuid,
    context     jsonb NOT NULL DEFAULT '{}'::jsonb,
    request_id  text,
    ip          inet
);

CREATE INDEX idx_audit_log_created_at ON audit_log (created_at DESC);
CREATE INDEX idx_audit_log_actor_created_at ON audit_log (actor_id, created_at DESC);
CREATE INDEX idx_audit_log_entity ON audit_log (entity_type, entity_id, created_at DESC);
CREATE INDEX idx_audit_log_action ON audit_log (action, created_at DESC);
