SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Зеркальный снос: ключи — инфраструктурный кэш, данные продукта не теряются.
DROP TABLE IF EXISTS idempotency_keys;
