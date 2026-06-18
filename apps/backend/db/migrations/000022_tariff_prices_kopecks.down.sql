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
