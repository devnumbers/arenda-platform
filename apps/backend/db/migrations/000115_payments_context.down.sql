-- Down: drop the payments context tables in reverse foreign-key order
-- (ADR 0049). Operations reference payments; pauses reference payments;
-- payments reference payment_categories.
DROP TABLE IF EXISTS operations;
DROP TABLE IF EXISTS payment_pauses;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS payment_categories;
