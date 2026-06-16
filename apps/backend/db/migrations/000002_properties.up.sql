CREATE TABLE properties (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('apartment','room','apartments','house','commercial','office','warehouse','garage','parking','land')),
    address TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL CHECK (status IN ('active','maintenance','archived')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_properties_owner_status ON properties(owner_id, status);
CREATE INDEX idx_properties_owner_updated_at ON properties(owner_id, updated_at DESC);

CREATE TRIGGER trg_properties_updated_at
    BEFORE UPDATE ON properties
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
