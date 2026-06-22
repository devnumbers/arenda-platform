ALTER TABLE payment_methods
    ADD COLUMN provider_card_id TEXT,
    ADD COLUMN exp_date TEXT;
