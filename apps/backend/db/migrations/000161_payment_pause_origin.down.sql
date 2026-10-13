-- Откат 000161: колонка происхождения сносится вместе с отметкой. Открытые
-- auto_archive-паузы после отката неотличимы от ручных — данные не теряются,
-- интервалы остаются (журнал каскадит только удалением правила).
SET lock_timeout = '1s';
SET statement_timeout = '5s';

ALTER TABLE payment_change_log
    DROP CONSTRAINT payment_change_log_origin_check;
ALTER TABLE payment_change_log
    DROP COLUMN origin;

ALTER TABLE payment_pauses
    DROP CONSTRAINT payment_pauses_origin_check;
ALTER TABLE payment_pauses
    DROP COLUMN origin;
