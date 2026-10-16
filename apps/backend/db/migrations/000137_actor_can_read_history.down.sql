SET lock_timeout = '1s';
SET statement_timeout = '5s';

DROP FUNCTION IF EXISTS actor_can_read_history(uuid, uuid);
