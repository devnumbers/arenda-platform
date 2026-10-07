SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Откат 000152: журнал живёт и умирает с платежом — таблица снимается
-- целиком (история правок до внедрения пуста, бэкфилла нет и невозможен,
-- ADR 0065); маркер ручного названия снимается с колонкой.
DROP TABLE payment_change_log;

ALTER TABLE payments DROP COLUMN title_is_manual;
