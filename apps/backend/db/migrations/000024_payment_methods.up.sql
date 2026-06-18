CREATE TABLE payment_methods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    provider_token TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    display_mask TEXT,
    is_active BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, token_hash)
);

CREATE INDEX idx_payment_methods_user_active ON payment_methods(user_id, is_active);

CREATE UNIQUE INDEX idx_payment_methods_one_active_per_user
ON payment_methods(user_id) WHERE is_active;

CREATE TABLE subscription_payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subscription_id UUID NOT NULL REFERENCES user_subscriptions(id) ON DELETE CASCADE,
    tariff_id UUID NOT NULL REFERENCES tariffs(id),
    payment_method_id UUID REFERENCES payment_methods(id) ON DELETE SET NULL,
    period TEXT NOT NULL CHECK (period IN ('month', 'year')),
    amount_kopecks BIGINT NOT NULL CHECK (amount_kopecks >= 0),
    provider TEXT NOT NULL,
    provider_payment_id TEXT,
    status TEXT NOT NULL CHECK (status IN ('pending', 'succeeded', 'failed')),
    error_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_subscription_payments_user_created ON subscription_payments(user_id, created_at DESC);

CREATE INDEX idx_subscription_payments_provider_payment_id ON subscription_payments(provider_payment_id);

UPDATE user_subscriptions SET active_payment_method_id = NULL
WHERE active_payment_method_id IS NOT NULL
  AND active_payment_method_id NOT IN (SELECT id FROM payment_methods);

ALTER TABLE user_subscriptions
    ADD CONSTRAINT user_subscriptions_active_payment_method_id_fkey
        FOREIGN KEY (active_payment_method_id) REFERENCES payment_methods(id)
        ON DELETE RESTRICT;

CREATE INDEX idx_user_subscriptions_active_payment_method_id ON user_subscriptions(active_payment_method_id);
