SET lock_timeout = '1s';
SET statement_timeout = '5s';

ALTER TABLE properties DROP COLUMN photo_blur;
ALTER TABLE contacts DROP COLUMN photo_blur;
ALTER TABLE users DROP COLUMN photo_blur;
