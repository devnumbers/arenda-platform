-- Приватные фото: одна картинка на сущность (карта #1217, решение #1220,
-- тикет #1227, ADR 0065). У объекта, контакта и профиля максимум одно фото:
-- S3-ключ и фактический content-type (определён по магическим байтам на шве
-- загрузки) живут прямо в строке сущности — photo_key NULL, когда фото нет.
-- Выдача и загрузка стримят через бэкенд, ключ наружу не отдаётся.

SET lock_timeout = '1s';
SET statement_timeout = '5s';

ALTER TABLE properties
    ADD COLUMN photo_key TEXT,
    ADD COLUMN photo_content_type TEXT;

ALTER TABLE contacts
    ADD COLUMN photo_key TEXT,
    ADD COLUMN photo_content_type TEXT;

ALTER TABLE users
    ADD COLUMN photo_key TEXT,
    ADD COLUMN photo_content_type TEXT;

-- Таблица property_photos сносится вместе со старым контрактом
-- «до 10 фото» (uploadPropertyPhoto/deletePropertyPhoto, PropertyPhoto,
-- photos[] в DTO): строк в ней нет — на stage/prod всегда стоял
-- PHOTO_STORAGE_PROVIDER=fake, UI загрузки никогда не существовал, данные
-- не переносятся (прецедент ADR 0046/0054).
DROP TABLE property_photos;
