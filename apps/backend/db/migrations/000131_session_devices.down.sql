-- The columns are nullable/defaulted, so dropping them restores the old shape.
-- Session rows deleted by the up-migration cannot be restored.
ALTER TABLE sessions
    DROP COLUMN city,
    DROP COLUMN os,
    DROP COLUMN browser_major,
    DROP COLUMN browser,
    DROP COLUMN device_type,
    DROP COLUMN user_agent,
    DROP COLUMN last_ip,
    DROP COLUMN previous_token_hash,
    DROP COLUMN rotated_at;
