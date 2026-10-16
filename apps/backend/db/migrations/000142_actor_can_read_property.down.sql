SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Обратное к 000142: снимается только новое имя; actor_can_read_history up
-- не трогал — expand-contract, ADR 0024 §4; его снимет contract-миграция
-- следующего релиза, когда код со старым именем уйдёт везде.
DROP FUNCTION IF EXISTS actor_can_read_property(uuid, uuid);
