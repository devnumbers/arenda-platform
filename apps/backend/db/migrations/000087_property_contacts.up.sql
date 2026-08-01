CREATE TABLE property_contacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    property_id UUID NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    owner_id UUID NOT NULL,
    name TEXT NOT NULL,
    phone TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_property_contacts_property_id ON property_contacts(property_id);
CREATE INDEX idx_property_contacts_owner_id ON property_contacts(owner_id);

CREATE TRIGGER trg_property_contacts_updated_at
    BEFORE UPDATE ON property_contacts
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
