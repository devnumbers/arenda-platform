-- Reassign any subscriptions that reference the tariffs being removed to basic
-- before deleting them, so the foreign-key constraint is not violated.
UPDATE user_subscriptions
SET tariff_id = (SELECT id FROM tariffs WHERE name = 'basic')
WHERE tariff_id IN (SELECT id FROM tariffs WHERE name IN ('pro', 'business'));

UPDATE user_subscriptions
SET pending_tariff_id = NULL
WHERE pending_tariff_id IN (SELECT id FROM tariffs WHERE name IN ('pro', 'business'));

DELETE FROM tariffs WHERE name IN ('pro', 'business');

ALTER TABLE tariffs DROP CONSTRAINT IF EXISTS tariffs_active_property_limit_check;
ALTER TABLE tariffs ADD CONSTRAINT tariffs_active_property_limit_check
    CHECK (active_property_limit >= 0);

ALTER TABLE tariffs
    ALTER COLUMN monthly_price_kopecks TYPE NUMERIC(14,2) USING (monthly_price_kopecks / 100.0);
ALTER TABLE tariffs
    ALTER COLUMN yearly_price_kopecks TYPE NUMERIC(14,2) USING (yearly_price_kopecks / 100.0);

ALTER TABLE tariffs RENAME COLUMN monthly_price_kopecks TO monthly_price;
ALTER TABLE tariffs RENAME COLUMN yearly_price_kopecks TO yearly_price;
