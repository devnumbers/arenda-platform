SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Обратное к 000157: тело предиката до D8 (тикет #1304) — «владелец объекта
-- OR активный full_access-участник», канон тикета #794.
CREATE OR REPLACE FUNCTION actor_can_manage(target_property_id uuid, acting_user_id uuid)
RETURNS boolean
LANGUAGE sql STABLE PARALLEL SAFE
RETURN (
    EXISTS (
        SELECT 1 FROM properties p
        WHERE p.id = target_property_id
          AND p.owner_id = acting_user_id
    )
    OR EXISTS (
        SELECT 1 FROM property_members m
        WHERE m.property_id = target_property_id
          AND m.user_id = acting_user_id
          AND m.status = 'active'
          AND m.role = 'full_access'
    )
);
