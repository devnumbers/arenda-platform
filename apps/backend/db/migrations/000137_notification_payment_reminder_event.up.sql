-- Событие ленты «Напоминание о платеже» (карта #822, #824): категория
-- payments_operations, нога издателя платежей. Значение enum добавляется
-- рядом с payment_due/payment_overdue (000132).
ALTER TYPE notification_event_type ADD VALUE 'payment_reminder';
