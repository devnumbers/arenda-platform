SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Обратная сторона 000120: безобъектный срез исчезает вместе со своим
-- контрактом — правила и задачи без объекта удаляются, объектные остаются
-- обязанными своему объекту.
DELETE FROM tasks WHERE property_id IS NULL;
DELETE FROM task_rules WHERE property_id IS NULL;
ALTER TABLE tasks ALTER COLUMN property_id SET NOT NULL;
ALTER TABLE task_rules ALTER COLUMN property_id SET NOT NULL;
