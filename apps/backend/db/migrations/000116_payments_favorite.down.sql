-- Down: drop the favorite flag (ticket #461).
ALTER TABLE payments DROP COLUMN IF EXISTS is_favorite;
