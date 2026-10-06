SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Откат 000150: колонка-штамп снимается вместе с ограничением; добавленное
-- значение notification_event_type удалить нельзя (PostgreSQL не снимает
-- значения enum; прецедент 000139.down, #277) — значение остаётся мёртвым.
ALTER TABLE operations DROP COLUMN paid_source;

SELECT 1;
