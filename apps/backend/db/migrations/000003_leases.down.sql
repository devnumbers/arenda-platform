DROP TRIGGER IF EXISTS trg_operations_updated_at ON operations;
DROP TABLE IF EXISTS operations;

DROP TRIGGER IF EXISTS trg_recurring_operations_updated_at ON recurring_operations;
DROP TABLE IF EXISTS recurring_operations;

DROP TRIGGER IF EXISTS trg_leases_updated_at ON leases;
DROP TABLE IF EXISTS leases;

DROP TRIGGER IF EXISTS trg_tenant_contacts_updated_at ON tenant_contacts;
DROP TABLE IF EXISTS tenant_contacts;
