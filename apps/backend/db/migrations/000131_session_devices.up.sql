-- Devices on sessions (map #724, ticket #728): every session carries the
-- client description captured once at creation (raw User-Agent, parsed device
-- type, browser + major, OS), the most recent client IP and its GeoIP city,
-- and the token-rotation bookkeeping (rotated_at + previous hash for the
-- in-flight grace window).
ALTER TABLE sessions
    ADD COLUMN rotated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN previous_token_hash TEXT,
    ADD COLUMN last_ip INET,
    ADD COLUMN user_agent TEXT NOT NULL DEFAULT '',
    ADD COLUMN device_type TEXT NOT NULL DEFAULT 'unknown',
    ADD COLUMN browser TEXT NOT NULL DEFAULT '',
    ADD COLUMN browser_major INT,
    ADD COLUMN os TEXT NOT NULL DEFAULT '',
    ADD COLUMN city TEXT;

-- Ticket decision: the device feature invalidates every existing session once
-- (all users re-login); old rows carry no device or rotation data anyway.
DELETE FROM sessions;
