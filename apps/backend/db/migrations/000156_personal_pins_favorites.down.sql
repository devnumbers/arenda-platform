SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Down: свернуть expand-фазу личных закреплений/избранных (#1303). В окне
-- expand замороженные глобальные колонки properties.pinned_at и
-- payments.is_favorite/favorite_order остаются источником правды, личные
-- строки ещё никем не пишутся — откат просто убирает таблицы. После
-- contract-сноса 000158 откат через эту миграцию теряет личные строки:
-- глобальные колонки уже снесены, а backfill из личных таблиц в 000156.down
-- нет — полный возврат на 155 осмыслен только вместе с этим окном.
DROP TABLE IF EXISTS user_payment_favorites;
DROP TABLE IF EXISTS user_property_pins;
