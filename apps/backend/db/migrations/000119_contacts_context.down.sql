-- Down restores the exact schema of property_contacts as of migration 117
-- (000087 with the owner FK of 000111), without data (precedent ADR 0046):
-- the pre-contacts app code inserts without an id and relies on the
-- historical DEFAULT, so the restored table keeps it — down migrations are
-- exempt from the id-column-default lint by construction.
-- The contacts context (ADR 0051) is dropped; its book data is not restored.

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

ALTER TABLE property_contacts
    ADD CONSTRAINT property_contacts_owner_id_fkey
        FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE;

DROP TABLE contacts;
