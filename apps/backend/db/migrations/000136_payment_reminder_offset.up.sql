-- Напоминание о платеже (карта #822, #824): за сколько дней предупреждать о
-- вхождении правила — 1, 3 или 7. NULL — напоминаний нет (выбор опционален).
-- Автоплатёж поле не исключает: напоминание живёт независимо от auto_pay
-- (решение владельца, #823). Индекс — по образцу старого 000050.
ALTER TABLE payments ADD COLUMN reminder_offset_days INT CHECK (reminder_offset_days IN (1, 3, 7));
CREATE INDEX idx_payments_reminder_offset ON payments(reminder_offset_days) WHERE reminder_offset_days IS NOT NULL;
