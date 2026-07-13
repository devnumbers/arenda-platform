CREATE TABLE operation_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('income','expense')),
    name TEXT NOT NULL,
    code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (owner_id, type, lower(name))
);

CREATE INDEX idx_operation_categories_owner_id ON operation_categories(owner_id);
CREATE INDEX idx_operation_categories_owner_type ON operation_categories(owner_id, type);

CREATE UNIQUE INDEX idx_operation_categories_owner_code ON operation_categories(owner_id, code) WHERE code IS NOT NULL;

CREATE TRIGGER trg_operation_categories_updated_at
    BEFORE UPDATE ON operation_categories
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
