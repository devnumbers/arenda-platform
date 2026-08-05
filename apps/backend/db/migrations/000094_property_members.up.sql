-- Property memberships for shared object access (T3, issue #156).
-- Roles are restricted to 'full_access' and 'viewer'; the object owner is
-- synthesized in the application layer (ListMembers) and never stored here, so
-- ownership is a single source of truth in properties.owner_id and cannot be
-- revoked. See ADR 0028 (object data access model) and ADR 0025 (property
-- deletion modes: memberships cascade with the property).
--
-- IDs are application-generated UUIDv7 (ADR 0019): no DB DEFAULT.
CREATE TABLE property_members (
    id          UUID         PRIMARY KEY,
    property_id UUID         NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    user_id     UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role        TEXT         NOT NULL CHECK (role IN ('full_access', 'viewer')),
    granted_by  UUID         NOT NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    -- One active membership per (property, user).
    UNIQUE (property_id, user_id)
);

CREATE INDEX idx_property_members_property_id ON property_members(property_id);
CREATE INDEX idx_property_members_user_id     ON property_members(user_id);

CREATE TRIGGER trg_property_members_set_updated_at
    BEFORE UPDATE ON property_members
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
