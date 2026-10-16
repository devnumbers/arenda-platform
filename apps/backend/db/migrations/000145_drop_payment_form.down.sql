-- Возврат «Формы оплаты» (инверсия 000145, тикет #1006): определения 000115.
-- У существующих строк формы нет — правило получает 'transfer' (дефолт гасится
-- сразу после бэкфилла), снапшот операции остаётся nullable (ручные операции).
ALTER TABLE payments
    ADD COLUMN payment_form TEXT NOT NULL DEFAULT 'transfer'
    CHECK (payment_form IN ('transfer', 'cash'));
ALTER TABLE payments
    ALTER COLUMN payment_form DROP DEFAULT;
ALTER TABLE operations
    ADD COLUMN payment_form TEXT CHECK (payment_form IN ('transfer', 'cash'));
