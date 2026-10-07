-- Откат: колонки фото сущностей снимаются, схема property_photos
-- восстанавливается без данных (ап-миграция строк не переносила —
-- решение владельца #1220, ADR 0065).

SET lock_timeout = '1s';
SET statement_timeout = '5s';

ALTER TABLE properties
    DROP COLUMN photo_key,
    DROP COLUMN photo_content_type;

ALTER TABLE contacts
    DROP COLUMN photo_key,
    DROP COLUMN photo_content_type;

ALTER TABLE users
    DROP COLUMN photo_key,
    DROP COLUMN photo_content_type;

-- Схема 000038_property_photos как была (индекс и таблица).
CREATE TABLE property_photos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    property_id UUID NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_property_photos_property_id ON property_photos(property_id);
