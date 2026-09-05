-- Down drops the rentals context (ADR 0053) without data restoration.
-- The managed payment survives: the FK on rentals.payment_id is RESTRICT for
-- the application's delete order, so dropping the rentals table first is what
-- releases it; payments rows themselves are owned by the payments context
-- (ADR 0049) and stay.

DROP TABLE rentals;
