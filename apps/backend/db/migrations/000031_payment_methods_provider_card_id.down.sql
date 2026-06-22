ALTER TABLE payment_methods
    DROP COLUMN IF EXISTS provider_card_id,
    DROP COLUMN IF EXISTS exp_date;
