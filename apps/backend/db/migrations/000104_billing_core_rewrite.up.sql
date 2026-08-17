-- Destructive rewrite of the billing context schema (issue #245, ADR 0037).
--
-- The four core billing tables are dropped and recreated from scratch: the old
-- schema accumulated 25+ incremental migrations, dead constraints (the legacy
-- partial_refunded status) and placeholder semantics (card binding via a
-- payment-method token prefix) that the rewritten module must not inherit.
-- Billing data loss on stage/prod is an explicit owner decision recorded in
-- ADR 0037. The migration applies both to an empty database and on top of old
-- billing data (which is discarded).
--
-- New elements versus the old schema:
--   * tariffs.is_active — hiding a tariff no longer breaks referential
--     integrity (FK columns keep pointing at the hidden row);
--   * subscription_transitions — the immutable subscription transition log
--     (status/tariff changes with reason, initiator and payment reference);
--   * card_binding_sessions — card-binding sessions with a request key and
--     TTL, replacing the addcard: token-placeholder convention;
--   * the legacy partial_refunded payment status is gone.

DROP TABLE IF EXISTS subscription_payments;
DROP TABLE IF EXISTS user_subscriptions;
DROP TABLE IF EXISTS payment_methods;
DROP TABLE IF EXISTS tariffs;

-- Tariffs: plans with prices in integer kopecks (ADR 0036) and an active flag
-- so a plan can be hidden from users without violating FK integrity.
CREATE TABLE tariffs (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    active_property_limit INT NOT NULL CHECK (active_property_limit >= -1),
    monthly_price_kopecks BIGINT NOT NULL CHECK (monthly_price_kopecks >= 0),
    yearly_price_kopecks BIGINT NOT NULL CHECK (yearly_price_kopecks >= 0),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_tariffs_updated_at
    BEFORE UPDATE ON tariffs
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Payment methods: provider-marked card tokens. Token uniqueness is enforced
-- per user by hash; exactly one active method per user is enforced by a
-- partial unique index. The card-binding flow (issue #251) fills this table.
CREATE TABLE payment_methods (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL CHECK (provider IN ('fake', 'tkassa')),
    provider_token TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    provider_card_id TEXT,
    display_mask TEXT,
    exp_date TEXT,
    is_active BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, token_hash)
);

CREATE INDEX idx_payment_methods_user_active ON payment_methods(user_id, is_active);

CREATE UNIQUE INDEX idx_payment_methods_one_active_per_user
    ON payment_methods(user_id) WHERE is_active;

CREATE TRIGGER trg_payment_methods_updated_at
    BEFORE UPDATE ON payment_methods
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Subscriptions: the ADR 0008 lifecycle state (active/grace/cancelled, paid or
-- service source, deferred tariff change at the end of the paid period).
CREATE TABLE user_subscriptions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    tariff_id UUID NOT NULL REFERENCES tariffs(id),
    source TEXT NOT NULL CHECK (source IN ('paid', 'service')),
    status TEXT NOT NULL CHECK (status IN ('active', 'grace', 'cancelled')),
    valid_until TIMESTAMPTZ,
    auto_renew_enabled BOOLEAN NOT NULL DEFAULT false,
    pending_tariff_id UUID REFERENCES tariffs(id),
    pending_change_at TIMESTAMPTZ,
    pending_period TEXT CHECK (pending_period IN ('month', 'year')),
    active_payment_method_id UUID REFERENCES payment_methods(id) ON DELETE RESTRICT,
    -- the FK is added after subscription_payments exists (circular reference)
    last_applied_payment_id UUID,
    current_period TEXT CHECK (current_period IN ('month', 'year')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT user_subscriptions_pending_change_at_check
        CHECK (pending_change_at IS NULL OR valid_until IS NULL OR pending_change_at >= valid_until)
);

CREATE TRIGGER trg_user_subscriptions_updated_at
    BEFORE UPDATE ON user_subscriptions
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Worker batch partial indexes (ADR 0008 lifecycle phases, issue #252).
CREATE INDEX idx_user_subscriptions_up_for_renewal
    ON user_subscriptions(valid_until, id)
    WHERE status = 'active' AND auto_renew_enabled = true;

CREATE INDEX idx_user_subscriptions_expired_grace
    ON user_subscriptions(valid_until, id)
    WHERE status = 'grace';

CREATE INDEX idx_user_subscriptions_expired_non_renewing
    ON user_subscriptions(valid_until, id)
    WHERE status = 'active' AND auto_renew_enabled = false;

CREATE INDEX idx_user_subscriptions_expired_cancelled
    ON user_subscriptions(valid_until, id)
    WHERE status = 'cancelled';

-- Foreign-key helper indexes.
CREATE INDEX idx_user_subscriptions_tariff_id ON user_subscriptions(tariff_id);
CREATE INDEX idx_user_subscriptions_pending_tariff_id ON user_subscriptions(pending_tariff_id);
CREATE INDEX idx_user_subscriptions_active_payment_method_id ON user_subscriptions(active_payment_method_id);

CREATE INDEX idx_user_subscriptions_pending_change
    ON user_subscriptions(pending_change_at)
    WHERE status = 'active' AND pending_tariff_id IS NOT NULL;

-- Subscription payments: the processing-world money events (ADR 0036). Amounts
-- are positive integer kopecks; refunds are full-amount only (the refunded
-- amount, when present, must equal the full amount; the legacy
-- partial_refunded status is dropped). Provider references are unique per
-- provider and a partial unique index keeps one pending upgrade payment per
-- user/tariff/period (idempotency against double initiation).
CREATE TABLE subscription_payments (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subscription_id UUID NOT NULL REFERENCES user_subscriptions(id) ON DELETE CASCADE,
    tariff_id UUID NOT NULL REFERENCES tariffs(id),
    payment_method_id UUID REFERENCES payment_methods(id) ON DELETE SET NULL,
    period TEXT NOT NULL CHECK (period IN ('month', 'year')),
    amount_kopecks BIGINT NOT NULL CHECK (amount_kopecks > 0),
    provider TEXT NOT NULL CHECK (provider IN ('fake', 'tkassa')),
    provider_payment_id TEXT,
    payment_url TEXT,
    status TEXT NOT NULL CHECK (status IN ('pending', 'succeeded', 'failed', 'refunded', 'refunding')),
    refunded_amount_kopecks BIGINT,
    charge_attempts INT NOT NULL DEFAULT 0,
    error_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    succeeded_at TIMESTAMPTZ,
    CONSTRAINT subscription_payments_refunded_amount_full
        CHECK (refunded_amount_kopecks IS NULL OR refunded_amount_kopecks = amount_kopecks)
);

CREATE INDEX idx_subscription_payments_user_created ON subscription_payments(user_id, created_at DESC);

CREATE UNIQUE INDEX idx_subscription_payments_provider_payment
    ON subscription_payments(provider, provider_payment_id);

CREATE UNIQUE INDEX idx_subscription_payments_one_pending_upgrade
    ON subscription_payments(user_id, tariff_id, period) WHERE status = 'pending';

CREATE INDEX idx_subscription_payments_tariff_id ON subscription_payments(tariff_id);

CREATE INDEX idx_subscription_payments_payment_method_id ON subscription_payments(payment_method_id);

CREATE INDEX idx_subscription_payments_subscription_succeeded
    ON subscription_payments(subscription_id, created_at DESC)
    WHERE status = 'succeeded';

CREATE INDEX idx_subscription_payments_pending_created
    ON subscription_payments(created_at)
    WHERE status = 'pending' AND provider_payment_id IS NOT NULL;

CREATE TRIGGER trg_subscription_payments_updated_at
    BEFORE UPDATE ON subscription_payments
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- The applied-payment reference is subject to referential integrity; ON DELETE
-- SET NULL keeps user/subscription cascades from failing on the circular
-- subscription <-> payment reference.
ALTER TABLE user_subscriptions
    ADD CONSTRAINT user_subscriptions_last_applied_payment_id_fkey
        FOREIGN KEY (last_applied_payment_id) REFERENCES subscription_payments(id)
        ON DELETE SET NULL;

-- Subscription transitions: the immutable transition log (issue #245). Every
-- status or tariff change of a subscription is appended here with its reason,
-- initiator (user/admin/system) and the payment that caused it, so incidents
-- can be reconstructed without guessing. The first transition (subscription
-- creation) has no prior status, hence nullable from_status/from_tariff_id.
CREATE TABLE subscription_transitions (
    id UUID PRIMARY KEY,
    subscription_id UUID NOT NULL REFERENCES user_subscriptions(id) ON DELETE CASCADE,
    from_status TEXT CHECK (from_status IN ('active', 'grace', 'cancelled')),
    to_status TEXT NOT NULL CHECK (to_status IN ('active', 'grace', 'cancelled')),
    from_tariff_id UUID REFERENCES tariffs(id),
    to_tariff_id UUID NOT NULL REFERENCES tariffs(id),
    reason TEXT NOT NULL,
    initiator_type TEXT NOT NULL CHECK (initiator_type IN ('user', 'admin', 'system')),
    initiator_id UUID REFERENCES users(id),
    payment_id UUID REFERENCES subscription_payments(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_subscription_transitions_subscription
    ON subscription_transitions(subscription_id, created_at DESC);

CREATE INDEX idx_subscription_transitions_created
    ON subscription_transitions(created_at DESC);

-- The log is append-only: UPDATE is rejected at the database level, so history
-- cannot be rewritten. DELETE is deliberately not blocked: cascading deletes
-- (user erasure) fire row-level triggers and must be able to remove the log
-- together with its subscription. Applications never issue DELETE directly.
CREATE OR REPLACE FUNCTION reject_subscription_transition_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'subscription_transitions is append-only';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_subscription_transitions_immutable
    BEFORE UPDATE ON subscription_transitions
    FOR EACH ROW
    EXECUTE FUNCTION reject_subscription_transition_mutation();

-- Card binding sessions: one row per initiated card binding at a provider
-- (issue #251). The request key is unique per provider; expired sessions are
-- pruned by TTL and never produce payment methods.
CREATE TABLE card_binding_sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL CHECK (provider IN ('fake', 'tkassa')),
    request_key TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('new', 'completed', 'rejected')),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, request_key)
);

CREATE INDEX idx_card_binding_sessions_user ON card_binding_sessions(user_id, created_at DESC);

CREATE INDEX idx_card_binding_sessions_open
    ON card_binding_sessions(expires_at) WHERE status = 'new';

CREATE TRIGGER trg_card_binding_sessions_updated_at
    BEFORE UPDATE ON card_binding_sessions
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Tariff seed. Ids are generated by PostgreSQL 18 uuidv7() because seeds run
-- outside the application; all runtime ids are app-generated UUIDv7 (ADR 0019).
INSERT INTO tariffs (id, name, active_property_limit, monthly_price_kopecks, yearly_price_kopecks, is_active)
VALUES
    (uuidv7(), 'basic', 1, 0, 0, true),
    (uuidv7(), 'pro', 5, 49000, 440000, true),
    (uuidv7(), 'business', -1, 99000, 890000, true)
ON CONFLICT (name) DO NOTHING;

-- Re-onboard every existing owner to the basic plan. Paid subscriptions,
-- payment methods and payment history are lost (owner decision, ADR 0037), but
-- the platform must keep working for existing owners: without a subscription
-- row the property limiter allows nothing. The transition log records the
-- reset with its own reason so the mass change is visible in history.
INSERT INTO user_subscriptions (id, user_id, tariff_id, source, status, auto_renew_enabled)
SELECT uuidv7(), u.id, t.id, 'paid', 'active', false
FROM users u
JOIN tariffs t ON t.name = 'basic'
WHERE u.role = 'owner'
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO subscription_transitions (id, subscription_id, to_status, to_tariff_id, reason, initiator_type)
SELECT uuidv7(), s.id, 'active', s.tariff_id, 'schema_reset', 'system'
FROM user_subscriptions s;
