-- Reverse migration 000098: restrict audit_log.actor_role back to the 000080
-- set.
--
-- Re-adding the narrower CHECK fails if the journal already contains
-- 'full_access' or 'viewer' entries; callers rolling back after shared-access
-- activity should reconcile those rows first.

ALTER TABLE audit_log
    DROP CONSTRAINT audit_log_actor_role_check;

ALTER TABLE audit_log
    ADD CONSTRAINT audit_log_actor_role_check
        CHECK (actor_role IN ('owner', 'admin', 'system', 'anonymous'));
