SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Revert 000158: восстановить структуру в точности как 000116/000122/000123 и
-- обратно backfill-ить глобальное состояние из личных таблиц — точная инверсия
-- бэкфилла 000156 (SELECT owner_id): глобальное состояние — это состояние
-- собственника, строки участников глобально непредставимы и на личности не
-- переносятся. Личные таблицы не трогаются: 000158.down откатывает только
-- снос колонок, повторный up снимает их снова на заполненной схеме.
ALTER TABLE payments ADD COLUMN is_favorite BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE payments ADD COLUMN favorite_order BIGINT;
ALTER TABLE payments ADD CONSTRAINT payments_favorite_order_check
    CHECK (favorite_order IS NULL OR is_favorite);

ALTER TABLE properties ADD COLUMN pinned_at TIMESTAMPTZ;
ALTER TABLE properties ADD CONSTRAINT properties_pinned_at_check
    CHECK (pinned_at IS NULL OR status <> 'archived');

UPDATE payments SET is_favorite = true, favorite_order = upf.favorite_order
FROM user_payment_favorites upf
WHERE upf.payment_id = payments.id
  AND upf.user_id = payments.owner_id;

-- Архивный объект остаётся незакреплённым: восстановленный CHECK запрещает
-- пин на архивной строке, а личные строки архивация не трогает (D16) — их
-- время в глобальную колонку вернуть некуда.
UPDATE properties SET pinned_at = upp.pinned_at
FROM user_property_pins upp
WHERE upp.property_id = properties.id
  AND upp.user_id = properties.owner_id
  AND properties.status <> 'archived';
