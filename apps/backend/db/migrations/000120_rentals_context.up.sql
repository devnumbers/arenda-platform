SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Rentals context schema (ADR 0053, ticket #529). The DDL below is the
-- canonical form fixed by the ADR — schema changes go through a new ADR
-- revision, not through edits here.

-- Аренда = период занятости объекта с условиями. Идентификаторы — UUIDv7 из
-- приложения (ADR 0019); деньги — BIGINT-копейки (ADR 0008). owner_id
-- денормализован из property (ADR 0028: SQL фильтрует по scope-владельцу);
-- тотальное удаление объекта — каскадом. Связь с платежом 1:1 и NOT NULL:
-- платёж аренды нельзя удалить мимо аренды (честный 409 из payments-эндпоинта).
CREATE TABLE rentals (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    property_id UUID NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    payment_id UUID NOT NULL UNIQUE REFERENCES payments(id) ON DELETE RESTRICT,
    contact_id UUID REFERENCES contacts(id) ON DELETE SET NULL,
    start_date DATE NOT NULL,
    planned_end_date DATE,
    completed_date DATE,
    utilities TEXT NOT NULL CHECK (utilities IN ('included', 'meters_only', 'full_receipt')),
    deposit_kopecks BIGINT CHECK (deposit_kopecks >= 0),
    commission_kopecks BIGINT CHECK (commission_kopecks >= 0),
    deposit_return_kopecks BIGINT CHECK (deposit_return_kopecks >= 0),
    deposit_return_comment TEXT,
    comment TEXT CHECK (char_length(comment) <= 2000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT rentals_planned_end_after_start_check
        CHECK (planned_end_date IS NULL OR planned_end_date > start_date),
    CONSTRAINT rentals_completed_after_start_check
        CHECK (completed_date IS NULL OR completed_date >= start_date),
    CONSTRAINT rentals_deposit_return_only_completed_check
        CHECK (completed_date IS NOT NULL OR
               (deposit_return_kopecks IS NULL AND deposit_return_comment IS NULL)),
    CONSTRAINT rentals_deposit_return_comment_needs_amount_check
        CHECK (deposit_return_comment IS NULL OR deposit_return_kopecks IS NOT NULL)
);
CREATE TRIGGER trg_rentals_updated_at
    BEFORE UPDATE ON rentals
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Дюральный инвариант №12: незавершённая аренда на объекте ровно одна
-- (завершённых может быть много).
CREATE UNIQUE INDEX idx_rentals_one_unfinished_per_property
    ON rentals(property_id) WHERE completed_date IS NULL;
CREATE INDEX idx_rentals_property ON rentals(property_id);
CREATE INDEX idx_rentals_owner ON rentals(owner_id);
