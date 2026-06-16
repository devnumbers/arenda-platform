CREATE UNIQUE INDEX idx_tenant_contacts_owner_phone
    ON tenant_contacts(owner_id, phone)
    WHERE phone IS NOT NULL;
