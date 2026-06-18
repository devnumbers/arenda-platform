ALTER TABLE user_subscriptions
    ADD COLUMN IF NOT EXISTS valid_until TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS auto_renew_enabled BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS pending_tariff_id UUID REFERENCES tariffs(id),
    ADD COLUMN IF NOT EXISTS pending_change_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS active_payment_method_id UUID,
    ALTER COLUMN status TYPE TEXT USING status::TEXT,
    DROP CONSTRAINT IF EXISTS user_subscriptions_status_check;

ALTER TABLE user_subscriptions
    ADD CONSTRAINT user_subscriptions_status_check
        CHECK (status IN ('active', 'grace', 'blocked', 'cancelled'));
