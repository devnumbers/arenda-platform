-- Down: убрать ручной порядок избранных платежей (тикет #576).
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_favorite_order_check;
ALTER TABLE payments DROP COLUMN IF EXISTS favorite_order;
