SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- «Отменённая операция» (решение владельца о удалении операций): статус-
-- надгробие. Строка остаётся и держит ключ (payment_id, date), так что тик
-- не материализует отменённое вхождение заново; paid_date при отмене
-- очищается вместе с фактом оплаты (use case), из долга и истории строка
-- выпадает сама (не planned и не paid).
ALTER TABLE operations DROP CONSTRAINT operations_status_check;
ALTER TABLE operations
    ADD CONSTRAINT operations_status_check CHECK (status IN ('planned', 'paid', 'cancelled'));
