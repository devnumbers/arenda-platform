-- Reset the pro tariff property limit so the next E2E run starts from the
-- canonical value instead of the elevated value used during reminder tests.
UPDATE tariffs
SET active_property_limit = 5
WHERE name = 'pro';

SELECT true AS ok;
