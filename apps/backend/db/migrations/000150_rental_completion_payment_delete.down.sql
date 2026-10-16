SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Зеркальный возврат схемы к 000120: ссылка на платёж снова обязательна,
-- колонки Архива условий сносятся. NOT NULL на payment_id и обратный бекфилл
-- невозможны: удалённые Завершением платежи и их planned-операции
-- невосстановимы — down-миграции восстанавливают схему, не данные
-- (прецедент 000119.down, 000148.down). Завершённые аренды остаются без
-- ссылки на платёж; новые завершения под старой схемой не удалить платёж
-- (rentals RESTRICT FK) — откат возвращает мир до ревизии #1161.
ALTER TABLE rentals
    DROP CONSTRAINT rentals_archive_lifecycle_check,
    DROP CONSTRAINT rentals_payment_link_lifecycle_check;

ALTER TABLE rentals
    DROP COLUMN rent_auto_pay,
    DROP COLUMN rent_payment_day,
    DROP COLUMN rent_amount_kopecks;
