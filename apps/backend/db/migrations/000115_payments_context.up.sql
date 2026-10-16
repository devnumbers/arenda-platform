SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Payments context schema (ADR 0049, ticket #454). The DDL below is the
-- canonical form fixed by the ADR — schema changes go through a new ADR
-- revision, not through edits here.

-- Пользовательские категории уровня аккаунта (дефолтный каталог — в коде,
-- tools/payment-categories; ссылка из платежей — слагом). Категория нейтральна
-- к направлению (#447): доход/расход — поле платежа, не справочника.
CREATE TABLE payment_categories (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (owner_id, name)
);
CREATE TRIGGER trg_payment_categories_updated_at
    BEFORE UPDATE ON payment_categories
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Платёж = правило (ADR 0047). Идентификаторы — UUIDv7 из приложения (ADR 0019);
-- деньги — BIGINT-копейки (ADR 0008). owner_id денормализован из property
-- (ADR 0028: SQL фильтрует по scope-владельцу); тотальное удаление объекта —
-- каскадом (тикет #446, ревизия ADR 0025).
CREATE TABLE payments (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    property_id UUID NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('income', 'expense')),
    title TEXT NOT NULL,
    amount_kopecks BIGINT NOT NULL CHECK (amount_kopecks > 0),
    recurrence JSONB NOT NULL
        CHECK (recurrence->>'kind' IN ('daily', 'weekly', 'monthly', 'yearly')),
    since DATE NOT NULL,
    end_date DATE,
    auto_pay BOOLEAN NOT NULL DEFAULT false,
    payment_form TEXT NOT NULL CHECK (payment_form IN ('transfer', 'cash')),
    category_slug TEXT,
    user_category_id UUID REFERENCES payment_categories(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT payments_since_end_date_check
        CHECK (end_date IS NULL OR end_date >= since),
    CONSTRAINT payments_category_exactly_one
        CHECK (num_nonnulls(category_slug, user_category_id) = 1)
);
CREATE TRIGGER trg_payments_updated_at
    BEFORE UPDATE ON payments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Пауза: интервалы [from_date, to_date), to_date IS NULL — активная бессрочная.
-- Отдельная таблица (не jsonb на платеже): не более одной открытой паузы —
-- дюральный инвариант partial unique index; пауза/возобновление — атомарные
-- INSERT/UPDATE без read-modify-write массива.
CREATE TABLE payment_pauses (
    id UUID PRIMARY KEY,
    payment_id UUID NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    from_date DATE NOT NULL,
    to_date DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT payment_pauses_interval_check
        CHECK (to_date IS NULL OR to_date >= from_date)
);
CREATE TRIGGER trg_payment_pauses_updated_at
    BEFORE UPDATE ON payment_pauses
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE UNIQUE INDEX idx_payment_pauses_one_active
    ON payment_pauses(payment_id) WHERE to_date IS NULL;
CREATE INDEX idx_payment_pauses_payment ON payment_pauses(payment_id);

-- Операция = вхождение либо ручной факт. payment_id ON DELETE SET NULL +
-- origin отличает «платёж удалён» (origin='payment' AND payment_id IS NULL)
-- от ручной операции (origin='manual') — тикет #446. Дедупликация
-- материализации — уникальный (payment_id, date); просрочка не хранится.
CREATE TABLE operations (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    property_id UUID NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    payment_id UUID REFERENCES payments(id) ON DELETE SET NULL,
    origin TEXT NOT NULL CHECK (origin IN ('payment', 'manual')),
    date DATE NOT NULL,
    paid_date DATE,
    status TEXT NOT NULL CHECK (status IN ('planned', 'paid')),
    type TEXT NOT NULL CHECK (type IN ('income', 'expense')),
    title TEXT NOT NULL,
    amount_kopecks BIGINT NOT NULL CHECK (amount_kopecks > 0),
    payment_form TEXT CHECK (payment_form IN ('transfer', 'cash')),
    category_label TEXT NOT NULL,
    category_slug TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT operations_paid_consistency
        CHECK ((status = 'paid') = (paid_date IS NOT NULL))
);
CREATE TRIGGER trg_operations_updated_at
    BEFORE UPDATE ON operations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE UNIQUE INDEX idx_operations_payment_date
    ON operations(payment_id, date) WHERE payment_id IS NOT NULL;
CREATE INDEX idx_payments_property ON payments(property_id);
CREATE INDEX idx_payments_owner ON payments(owner_id);
CREATE INDEX idx_payments_user_category ON payments(user_category_id);
CREATE INDEX idx_operations_property_date ON operations(property_id, date);
CREATE INDEX idx_operations_owner_date ON operations(owner_id, date);
