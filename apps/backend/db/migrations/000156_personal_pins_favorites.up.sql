SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Личные закрепления объектов и избранные платежи (спека #1302, модель
-- #1299, expand-тикет #1303). Пин и звезда становятся связью конкретного
-- пользователя с объектом/платежом вместо глобальных колонок
-- properties.pinned_at (000123) и payments.is_favorite/favorite_order
-- (000116/000122): «строка есть = состояние» — пин пишет INSERT .. ON
-- CONFLICT DO NOTHING (повторный пин время не меняет), открепление —
-- DELETE; звезда — INSERT, снятие — DELETE, позиция порядка живёт на строке
-- избранного (NULL = в конец порядка), поэтому CHECK «позиция только у
-- избранного» структурно не нужен. Архивация личные строки не трогает
-- (D16): разархивирование возвращает объект с личным пином. Обратный индекс
-- по entity — каскады удаления и обратные проходы (прецедент 000094), PK с
-- ведущим user_id покрывает выборки «мои закреплённые»/«мои избранные».
--
-- Expand-фаза по прецеденту 000100→000102: старые колонки замораживаются и
-- продолжают работать до cutover-деплоя (без dual-write); все чтения и
-- записи переключает cutover, физический снос колонок — contract-миграция.
CREATE TABLE user_property_pins (
    user_id     uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    property_id uuid        NOT NULL REFERENCES properties (id) ON DELETE CASCADE,
    pinned_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, property_id)
);
CREATE INDEX idx_user_property_pins_property ON user_property_pins (property_id);

CREATE TABLE user_payment_favorites (
    user_id        uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    payment_id     uuid        NOT NULL REFERENCES payments (id) ON DELETE CASCADE,
    favorite_order bigint      NULL,            -- NULL = в конец порядка
    created_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, payment_id)
);
CREATE INDEX idx_user_payment_favorites_payment ON user_payment_favorites (payment_id);

-- Backfill (D11/D12): существующие глобальные состояния переезжают
-- собственнику объекта; время пина и позиция порядка перевозятся как есть —
-- порядок списка собственника не меняется на cutover, NULL-позиция остаётся
-- NULL («новое избранное — в конец»). Участники начинают с чистого листа
-- автоматически: их строк в глобальных колонках нет.
INSERT INTO user_property_pins (user_id, property_id, pinned_at)
SELECT owner_id, id, pinned_at
FROM properties
WHERE pinned_at IS NOT NULL;

INSERT INTO user_payment_favorites (user_id, payment_id, favorite_order)
SELECT owner_id, id, favorite_order
FROM payments
WHERE is_favorite;
