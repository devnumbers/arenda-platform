ALTER TABLE subscription_payments
ADD CONSTRAINT subscription_payments_refunded_amount_lte_amount
CHECK (refunded_amount_kopecks IS NULL OR refunded_amount_kopecks <= amount_kopecks);
