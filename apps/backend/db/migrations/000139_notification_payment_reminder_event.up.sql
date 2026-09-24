-- Событие ленты «Напоминание о платеже» (карта #822, #824): категория
-- payments_operations, нога издателя платежей.

-- Порядок каталога: в группе Платежи и операции, после payment_overdue,
-- перед task_overdue (AFTER по уже существующему значению — требование
-- require-enum-value-ordering).
ALTER TYPE notification_event_type ADD VALUE 'payment_reminder' AFTER 'payment_overdue';
