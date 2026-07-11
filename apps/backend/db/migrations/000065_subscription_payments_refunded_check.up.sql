ALTER TABLE subscription_payments
ADD CONSTRAINT subscription_payments_refunded_amount_nonneg
CHECK (refunded_amount_kopecks IS NULL OR refunded_amount_kopecks >= 0);
