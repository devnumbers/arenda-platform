SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Событие ленты «Автоплатёж исполнен» (#1169, карта #1162; решение
-- владельца по гриллингу #1167: событие — только об автосписании, ручная
-- оплата молчит) и штамп его источника.

-- Штамп источника оплаты: тик автоплатежа гасит вхождение строго в его день
-- (ADR 0049 §2) — paid_source = 'auto_pay'; «Оплатить сейчас» вручную —
-- 'manual'. NULL у planned и у исторических paid-строк: событие смотрит
-- только на живой день, ретроспективы штампов не требуется.
ALTER TABLE operations ADD COLUMN paid_source text
  CHECK (paid_source IN ('manual', 'auto_pay'));

-- Порядок каталога: в группе Платежи и операции, после payment_reminder
-- (AFTER по уже существующему значению — требование
-- require-enum-value-ordering).
ALTER TYPE notification_event_type ADD VALUE 'payment_auto_paid' AFTER 'payment_reminder';
