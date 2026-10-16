SET lock_timeout = '1s';
SET statement_timeout = '5s';

ALTER TABLE property_contacts DROP CONSTRAINT IF EXISTS property_contacts_owner_id_fkey;
