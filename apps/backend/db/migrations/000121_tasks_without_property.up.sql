SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Задачи без объекта (ADR 0052, карта #518, тикет #520): ссылка на объект
-- в task_rules и tasks становится опциональной. Правило или задача, созданные
-- явно без объекта, живут в книге владельца: owner_id денормализован самим
-- владельцем, «сегодня» и просрочка — по users.timezone владельца (ADR 0048).
-- FK остаётся ON DELETE CASCADE: удаление объекта по-прежнему уносит его
-- правила и задачи целиком — безобъектными они не становятся. NULL-ключи в
-- b-tree индексах не участвуют в выборках объектного среза; owner-скоуп
-- обслуживают существующие idx_task_rules_owner и idx_tasks_owner_date.
ALTER TABLE task_rules ALTER COLUMN property_id DROP NOT NULL;
ALTER TABLE tasks ALTER COLUMN property_id DROP NOT NULL;
