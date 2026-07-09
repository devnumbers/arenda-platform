-- pre-condition guard: blocked is never produced by code
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM user_subscriptions WHERE status = 'blocked') THEN
    RAISE EXCEPTION 'migration 000067 aborted: user_subscriptions still has status=blocked rows; resolve before applying';
  END IF;
END $$;
ALTER TABLE user_subscriptions DROP CONSTRAINT user_subscriptions_status_check;
ALTER TABLE user_subscriptions ADD CONSTRAINT user_subscriptions_status_check CHECK (status IN ('active','grace','cancelled'));
