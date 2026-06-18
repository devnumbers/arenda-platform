ALTER TABLE tariffs
    DROP CONSTRAINT IF EXISTS tariffs_active_property_limit_check;

ALTER TABLE tariffs
    ADD CONSTRAINT tariffs_active_property_limit_check CHECK (active_property_limit >= -1);

ALTER TABLE tariffs RENAME COLUMN monthly_price TO monthly_price_kopecks;
ALTER TABLE tariffs RENAME COLUMN yearly_price TO yearly_price_kopecks;

ALTER TABLE tariffs
    ALTER COLUMN monthly_price_kopecks TYPE BIGINT USING (monthly_price_kopecks * 100)::bigint;
ALTER TABLE tariffs
    ALTER COLUMN yearly_price_kopecks TYPE BIGINT USING (yearly_price_kopecks * 100)::bigint;

INSERT INTO tariffs (id, name, active_property_limit, monthly_price_kopecks, yearly_price_kopecks)
VALUES
    (gen_random_uuid(), 'pro', 5, 49000, 440000),
    (gen_random_uuid(), 'business', -1, 99000, 890000)
ON CONFLICT (name) DO NOTHING;
