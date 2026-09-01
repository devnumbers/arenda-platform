-- Обратная сторона 000117: старый словарь статусов. Выполнение падает, если
-- в таблице остались отменённые операции, — это сознательная защита down.
ALTER TABLE operations DROP CONSTRAINT operations_status_check;
ALTER TABLE operations
    ADD CONSTRAINT operations_status_check CHECK (status IN ('planned', 'paid'));
