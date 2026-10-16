SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Down: убрать глобальное закрепление объектов (тикет #577).
ALTER TABLE properties DROP CONSTRAINT IF EXISTS properties_pinned_at_check;
ALTER TABLE properties DROP COLUMN IF EXISTS pinned_at;
