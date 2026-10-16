ALTER TABLE tariffs DROP CONSTRAINT IF EXISTS chk_tariffs_monthly_price_ceiling;
ALTER TABLE tariffs DROP CONSTRAINT IF EXISTS chk_tariffs_yearly_price_ceiling;
