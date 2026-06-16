CREATE TABLE tenant_contacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    surname TEXT,
    patronymic TEXT,
    phone TEXT,
    email TEXT,
    comment TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_tenant_contacts_updated_at
    BEFORE UPDATE ON tenant_contacts
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE leases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    property_id UUID NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    tenant_contact_id UUID REFERENCES tenant_contacts(id) ON DELETE SET NULL,
    status TEXT NOT NULL CHECK (status IN ('awaiting_start','active','requires_action','completed','archived')),
    start_date DATE NOT NULL,
    end_date DATE,
    rent_amount_kopecks BIGINT NOT NULL CHECK (rent_amount_kopecks >= 0),
    deposit_amount_kopecks BIGINT NOT NULL CHECK (deposit_amount_kopecks >= 0) DEFAULT 0,
    payment_day INT NOT NULL CHECK (payment_day BETWEEN 1 AND 31),
    comment TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_leases_owner_status ON leases(owner_id, status);
CREATE INDEX idx_leases_property_status ON leases(property_id, status);
CREATE INDEX idx_leases_owner_updated_at ON leases(owner_id, updated_at DESC);

CREATE TRIGGER trg_leases_updated_at
    BEFORE UPDATE ON leases
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE recurring_operations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    property_id UUID NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    lease_id UUID NOT NULL REFERENCES leases(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('income','expense')),
    category TEXT NOT NULL,
    amount_kopecks BIGINT NOT NULL CHECK (amount_kopecks >= 0),
    start_date DATE NOT NULL,
    payment_day INT NOT NULL,
    end_date DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_recurring_operations_updated_at
    BEFORE UPDATE ON recurring_operations
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE operations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    property_id UUID NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    lease_id UUID REFERENCES leases(id) ON DELETE CASCADE,
    recurring_operation_id UUID REFERENCES recurring_operations(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('income','expense')),
    category TEXT NOT NULL,
    amount_kopecks BIGINT NOT NULL CHECK (amount_kopecks >= 0),
    operation_date DATE NOT NULL,
    comment TEXT,
    is_exception BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_operations_lease_date ON operations(lease_id, operation_date);
CREATE INDEX idx_operations_property_date ON operations(property_id, operation_date);

CREATE TRIGGER trg_operations_updated_at
    BEFORE UPDATE ON operations
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
