-- Откат 000160: колонка хранимой завершённости сносится вместе с признаком —
-- завершённые правила теряют отметку и возвращаются в живые (данных операций
-- миграция не касалась). Словарь действий журнала: значение 'completed'
-- констрейнтом ДОПУСКАЕТСЯ и дальше — откатываемый код его больше не пишет,
-- но строки завершений живых правил в payment_change_log остаются (журнал
-- каскадит только удалением правила), и пересоздание CHECK без значения
-- уронило бы откат на таких строках.
SET lock_timeout = '1s';
SET statement_timeout = '5s';

ALTER TABLE payments
    DROP COLUMN completed_at;

ALTER TABLE payment_change_log
    DROP CONSTRAINT payment_change_log_action_check;
ALTER TABLE payment_change_log
    ADD CONSTRAINT payment_change_log_action_check
    CHECK (action IN ('updated', 'paused', 'resumed', 'completed'));
