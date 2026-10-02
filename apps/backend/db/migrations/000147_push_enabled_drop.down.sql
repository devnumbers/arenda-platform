-- Возврат мастера enabled (инверсия 000147, определение 000134): все
-- оставшиеся строки включены. Удалённые 000147 строки (enabled=false)
-- невосстановимы — их устройства до-подпишутся заново.
ALTER TABLE push_subscriptions
    ADD COLUMN enabled boolean NOT NULL DEFAULT true;
