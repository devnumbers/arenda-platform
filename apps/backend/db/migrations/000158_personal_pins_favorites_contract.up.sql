SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Contract phase (ADR 0030, прецедент 000102): финал expand/contract-цепочки
-- личных закреплений и избранных платежей (спека #1302, тикет #1311). Состояние
-- живёт в user_property_pins и user_payment_favorites (000156), все чтения и
-- записи переключили cutover'ы #1307/#1308, глобальные колонки заморожены —
-- данные уже перевезены бэкфиллом 000156. Сносим их вместе с CHECK'ами:
-- properties.pinned_at и properties_pinned_at_check (000123),
-- payments.is_favorite/favorite_order и payments_favorite_order_check
-- (000116/000122). CHECK снимается перед колонками явно; строки личных таблиц
-- не трогаются — единственное состояние пина и звезды остаётся в них.
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_favorite_order_check;
ALTER TABLE payments DROP COLUMN IF EXISTS favorite_order;
ALTER TABLE payments DROP COLUMN IF EXISTS is_favorite;

ALTER TABLE properties DROP CONSTRAINT IF EXISTS properties_pinned_at_check;
ALTER TABLE properties DROP COLUMN IF EXISTS pinned_at;
