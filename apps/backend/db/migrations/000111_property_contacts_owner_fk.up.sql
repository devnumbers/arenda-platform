SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- FK for the last NOT NULL domain reference without one (schema audit #322,
-- decision #324): the service always writes owner_id = property.OwnerID
-- (internal/properties/application/property_contact_service.go), which is a
-- users.id — an oversight, not polymorphism. ON DELETE CASCADE matches the
-- sibling owner_id references (tenant_contacts, leases, operations): user
-- deletion already cascades through properties.property_id anyway.
ALTER TABLE property_contacts
    ADD CONSTRAINT property_contacts_owner_id_fkey
        FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE;
