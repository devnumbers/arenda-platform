SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Favorite flag on the payment rule (ticket #461, ADR 0049 §4 second
-- contracts slice). The rule is the favorited entity — operations are its
-- ephemeral occurrences and manual facts carry no star. The flag is written
-- by an atomic UPDATE through PUT favorite; there is no read-modify-write.
ALTER TABLE payments ADD COLUMN is_favorite BOOLEAN NOT NULL DEFAULT false;
