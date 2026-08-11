-- Audit actor roles for shared property access (Property Sharing follow-up,
-- issue #166 resolution).
--
-- Until now every property-scoped audit entry was written with actor_role
-- 'owner', masking actions performed by full_access property members as the
-- owner's own. The services now resolve the actor's role through the policy
-- port (internal/shared/policy) and record it, so the journal needs to accept
-- the member roles 'full_access' and 'viewer' (strings identical to the policy
-- Role values). 'viewer' entries appear only through property_member.left
-- (self-exit is allowed for viewers); suspended/none actors never reach a
-- write path, so no new value is needed for them. See ADR 0020.
--
-- The check constraint replaces the inline one from migration 000080.

ALTER TABLE audit_log
    DROP CONSTRAINT audit_log_actor_role_check;

ALTER TABLE audit_log
    ADD CONSTRAINT audit_log_actor_role_check
        CHECK (actor_role IN ('owner', 'admin', 'system', 'anonymous', 'full_access', 'viewer'));
