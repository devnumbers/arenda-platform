SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- DB-level price ceiling for tariffs (issue #425, spec #419): the domain gate
-- (Tariff.Validate, MaxTariffPriceKopecks = 10 000 000) rejects an above-
-- ceiling price first; these constraints are the durable backstop. A per-
-- period price above 100 000 ₽ is an admin slip, never a plan. Seeded plans
-- (max 890 000) fit.
ALTER TABLE tariffs ADD CONSTRAINT chk_tariffs_monthly_price_ceiling
    CHECK (monthly_price_kopecks <= 10000000);
ALTER TABLE tariffs ADD CONSTRAINT chk_tariffs_yearly_price_ceiling
    CHECK (yearly_price_kopecks <= 10000000);
