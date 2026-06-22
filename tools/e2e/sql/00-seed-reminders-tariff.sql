-- Raise the pro tariff property limit so the deep reminder tests can create
-- multiple properties without hitting the default cap.
UPDATE tariffs
SET active_property_limit = 100
WHERE name = 'pro';

SELECT true AS ok;
