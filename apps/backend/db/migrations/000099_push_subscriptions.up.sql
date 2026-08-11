-- Web Push subscription endpoints per user (RFC 8030/8291/8292). One row per
-- browser/device subscription. The endpoint URL is globally unique (browser
-- push services issue one endpoint per subscription). Re-subscribing on the
-- same device upserts by endpoint. Deleting a user cascades to their
-- subscriptions (FK ON DELETE CASCADE, like free_reminders).
CREATE TABLE push_subscriptions (
    id              uuid          NOT NULL,
    user_id         uuid          NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    endpoint        text          NOT NULL,
    p256dh          text          NOT NULL,
    auth            text          NOT NULL,
    expiration_time timestamptz   NULL,
    created_at      timestamptz   NOT NULL DEFAULT now(),
    updated_at      timestamptz   NOT NULL DEFAULT now(),
    PRIMARY KEY (id)
);

CREATE UNIQUE INDEX uq_push_subscriptions_endpoint ON push_subscriptions (endpoint);
CREATE INDEX idx_push_subscriptions_user ON push_subscriptions (user_id);

CREATE TRIGGER trg_push_subscriptions_updated_at
BEFORE UPDATE ON push_subscriptions
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
