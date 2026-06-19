ALTER TABLE user_subscriptions
    DROP COLUMN IF EXISTS auto_renew_enabled,
    DROP COLUMN IF EXISTS pending_tariff_id,
    DROP COLUMN IF EXISTS pending_change_at,
    DROP COLUMN IF EXISTS valid_until,
    DROP COLUMN IF EXISTS active_payment_method_id;

ALTER TABLE user_subscriptions
    DROP CONSTRAINT IF EXISTS user_subscriptions_status_check;

-- The rolled-back schema does not support the 'grace' status, so convert any
-- grace rows to 'blocked' before restoring the original check.
UPDATE user_subscriptions
SET status = 'blocked'
WHERE status = 'grace';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'user_subscriptions_status_check'
          AND conrelid = 'user_subscriptions'::regclass
    ) THEN
        ALTER TABLE user_subscriptions
            ADD CONSTRAINT user_subscriptions_status_check
                CHECK (status IN ('active', 'blocked', 'cancelled'));
    END IF;
END $$;
