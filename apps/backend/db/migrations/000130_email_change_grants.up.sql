SET lock_timeout = '1s';
SET statement_timeout = '5s';

CREATE TABLE email_change_grants (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One live grant per user: issuing a new grant replaces the previous one
-- (delete + insert in the issuing transaction), and the unique index makes
-- that invariant durable.
CREATE UNIQUE INDEX idx_email_change_grants_user_id ON email_change_grants(user_id);
