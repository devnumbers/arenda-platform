-- Note: clean up any recurring_operations rows with payment_day outside 1-31 first.
ALTER TABLE recurring_operations ADD CONSTRAINT chk_recurring_operations_payment_day
CHECK (payment_day BETWEEN 1 AND 31);
