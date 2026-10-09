SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Down: свернуть expand-фазу личных закреплений/избранных (#1303). Данные
-- не теряются: замороженные глобальные колонки properties.pinned_at и
-- payments.is_favorite/favorite_order эта миграция не трогала — в окне
-- expand они остаются источником правды до cutover-деплоя, откат просто
-- убирает таблицы, строк которых никто ещё не пишет.
DROP TABLE IF EXISTS user_payment_favorites;
DROP TABLE IF EXISTS user_property_pins;
