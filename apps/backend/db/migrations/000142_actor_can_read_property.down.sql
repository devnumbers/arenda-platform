SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Обратное к 000142: возвращается имя actor_can_read_history (000137) с
-- тем же телом, канон снова привязан к журналу истории.
CREATE FUNCTION actor_can_read_history(target_property_id uuid, acting_user_id uuid)
RETURNS boolean
LANGUAGE sql STABLE PARALLEL SAFE
RETURN (
    EXISTS (
        SELECT 1 FROM properties p
        WHERE p.id = target_property_id
          AND (
               p.owner_id = acting_user_id
               OR (
                   p.status <> 'archived'
                   AND EXISTS (
                        SELECT 1 FROM property_members m
                        WHERE m.property_id = p.id
                          AND m.user_id = acting_user_id
                          AND m.status = 'active'
                      )
                 )
              )
    )
);

DROP FUNCTION IF EXISTS actor_can_read_property(uuid, uuid);
