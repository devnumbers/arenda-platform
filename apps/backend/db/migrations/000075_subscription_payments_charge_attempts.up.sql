ALTER TABLE subscription_payments ADD COLUMN IF NOT EXISTS charge_attempts INT NOT NULL DEFAULT 0;
