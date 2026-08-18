-- Down migration for the billing core rewrite (issue #245, ADR 0037).
-- The rewrite is destructive: the previous billing DATA cannot be restored.
-- What the down migration must still restore is the pre-rewrite SCHEMA SHAPE:
-- the historical down chain below 104 (000084, 000079, 000067 … 000001) keeps
-- issuing ALTER/UPDATE against these tables, and `migrate down -all` — the
-- contract of the CI "Backend migrations" job and of the up/down/up cycle test
-- — stays executable only if the tables exist again (issues #313, #316).
--
-- The DDL below is the exact shape the four billing tables had reached by
-- migration 000103 (generated against a database migrated up to 103), empty
-- and without seed rows. Re-applying the up migration re-creates the new
-- billing schema with the tariff seed and re-onboards existing owners to the
-- basic plan.

ALTER TABLE user_subscriptions
    DROP CONSTRAINT IF EXISTS user_subscriptions_last_applied_payment_id_fkey;

DROP TABLE IF EXISTS card_binding_sessions;
DROP TABLE IF EXISTS subscription_transitions;
DROP TABLE IF EXISTS subscription_payments;
DROP TABLE IF EXISTS user_subscriptions;
DROP TABLE IF EXISTS payment_methods;
DROP TABLE IF EXISTS tariffs;

DROP FUNCTION IF EXISTS reject_subscription_transition_mutation();

-- Pre-104 tariffs: no is_active and no updated_at — both are 000104 additions.
-- Ids carry no DEFAULT: 000079 moved id generation to the application, and its
-- own down migration restores gen_random_uuid() when it runs.
CREATE TABLE tariffs (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    active_property_limit INT NOT NULL,
    monthly_price_kopecks BIGINT NOT NULL,
    yearly_price_kopecks BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT tariffs_active_property_limit_check CHECK (active_property_limit >= -1),
    CONSTRAINT tariffs_monthly_price_check CHECK (monthly_price_kopecks >= 0),
    CONSTRAINT tariffs_yearly_price_check CHECK (yearly_price_kopecks >= 0)
);

-- Pre-104 payment methods: provider_card_id and exp_date came in with 000031;
-- card binding was the addcard: token-prefix convention (ADR 0037).
CREATE TABLE payment_methods (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL CONSTRAINT payment_methods_provider_check CHECK (provider IN ('fake', 'tkassa')),
    provider_token TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    display_mask TEXT,
    is_active BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    provider_card_id TEXT,
    exp_date TEXT,
    UNIQUE (user_id, token_hash)
);

CREATE INDEX idx_payment_methods_user_active ON payment_methods(user_id, is_active);

CREATE UNIQUE INDEX idx_payment_methods_one_active_per_user
    ON payment_methods(user_id) WHERE is_active;

CREATE TRIGGER trg_payment_methods_updated_at
    BEFORE UPDATE ON payment_methods
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Pre-104 subscriptions: the 000084 current_period column is present, and
-- last_applied_payment_id (000069) had no foreign key yet. The status CHECK
-- constraint name must stay user_subscriptions_status_check: 000067.down drops
-- it by that name without IF EXISTS.
CREATE TABLE user_subscriptions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    tariff_id UUID NOT NULL REFERENCES tariffs(id),
    source TEXT NOT NULL CHECK (source IN ('paid', 'service')),
    status TEXT NOT NULL CONSTRAINT user_subscriptions_status_check CHECK (status IN ('active', 'grace', 'cancelled')),
    valid_until TIMESTAMPTZ,
    auto_renew_enabled BOOLEAN NOT NULL DEFAULT false,
    pending_tariff_id UUID REFERENCES tariffs(id),
    pending_change_at TIMESTAMPTZ,
    pending_period TEXT CHECK (pending_period IN ('month', 'year')),
    active_payment_method_id UUID REFERENCES payment_methods(id) ON DELETE RESTRICT,
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

-- Worker batch partial indexes (ADR 0008) and FK helper indexes, as they stood
-- after 000026/000070.
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

CREATE INDEX idx_user_subscriptions_tariff_id ON user_subscriptions(tariff_id);

CREATE INDEX idx_user_subscriptions_pending_tariff_id ON user_subscriptions(pending_tariff_id);

CREATE INDEX idx_user_subscriptions_active_payment_method_id ON user_subscriptions(active_payment_method_id);

CREATE INDEX idx_user_subscriptions_pending_change
    ON user_subscriptions(pending_change_at)
    WHERE status = 'active' AND pending_tariff_id IS NOT NULL;

-- Pre-104 subscription payments: amounts are non-negative kopecks (the strict
-- positive check is a 000104 tightening), and the legacy partial_refunded
-- status is still in the CHECK — both restored to their 000103 form.
CREATE TABLE subscription_payments (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subscription_id UUID NOT NULL REFERENCES user_subscriptions(id) ON DELETE CASCADE,
    tariff_id UUID NOT NULL REFERENCES tariffs(id),
    payment_method_id UUID REFERENCES payment_methods(id) ON DELETE SET NULL,
    period TEXT NOT NULL CHECK (period IN ('month', 'year')),
    amount_kopecks BIGINT NOT NULL CHECK (amount_kopecks >= 0),
    provider TEXT NOT NULL,
    provider_payment_id TEXT,
    status TEXT NOT NULL CHECK (status IN ('pending', 'succeeded', 'failed', 'refunded', 'partial_refunded', 'refunding')),
    error_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    payment_url TEXT,
    succeeded_at TIMESTAMPTZ,
    refunded_amount_kopecks BIGINT,
    charge_attempts INT NOT NULL DEFAULT 0,
    CONSTRAINT subscription_payments_refunded_amount_nonneg
        CHECK (refunded_amount_kopecks IS NULL OR refunded_amount_kopecks >= 0),
    CONSTRAINT subscription_payments_refunded_amount_lte_amount
        CHECK (refunded_amount_kopecks IS NULL OR refunded_amount_kopecks <= amount_kopecks)
);

CREATE INDEX idx_subscription_payments_user_created ON subscription_payments(user_id, created_at DESC);

CREATE INDEX idx_subscription_payments_user_pending_created
    ON subscription_payments(user_id, created_at DESC)
    WHERE status = 'pending';

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
