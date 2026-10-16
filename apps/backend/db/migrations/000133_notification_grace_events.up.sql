SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- 000133: grace-события в ленте (карта #734, #741).
-- Перевод grace-уведомлений billing (#253) с прямого канала на пайплайн
-- лента + очередь: событиям нужны значения notification_event_type.
-- Легаси-значение subscription_grace (прямой канал) остаётся в типе
-- навсегда как мёртвое (прецедент #277); домен о нём больше не знает.

-- Порядок каталога: grace-пара после subscription_plan_changed, перед
-- system_maintenance (BEFORE по уже существующему значению — требование
-- require-enum-value-ordering).
ALTER TYPE notification_event_type ADD VALUE 'subscription_grace_entered' BEFORE 'system_maintenance';
ALTER TYPE notification_event_type ADD VALUE 'subscription_grace_expiring' BEFORE 'system_maintenance';
