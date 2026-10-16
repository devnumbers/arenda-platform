SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Снос «Формы оплаты» (перевод/наличные, тикет #1006, карта #1005, решение
-- владельца 01.10): поле write-only — нигде не читается пользователем, ценности
-- не несёт (ревизия решения №3 ADR 0047). Колонка правила и её снапшот в
-- операциях уходят целиком; данные не переносятся.
ALTER TABLE payments DROP COLUMN payment_form;
ALTER TABLE operations DROP COLUMN payment_form;
