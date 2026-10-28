SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Blur-up кадр приватных фото (решение #1594, тикет #1603 — аменд ADR 0065):
-- второй производный артефакт фото рядом с thumb. Кадр — jpeg длинной
-- стороной 16px, готовый base64 (~0.5–1 КБ) живёт колонкой рядом с
-- photo_key: производные payload'ы читают его из уже выбранной строки, ноль
-- походов в storage на каждый круг. Хранение производного объекта в storage
-- отвергнуто (лишний поход на каждую сборку DTO); отдельный ?size=blur
-- запрос отвергнут (лишний раунд-трип на каждый круг). NULL — штатное
-- состояние: webp не декодируется, кадр ≤16px не нужен, до ленивой
-- материализации колонка пуста («next request retries» по образцу thumb).
ALTER TABLE properties ADD COLUMN photo_blur TEXT;
ALTER TABLE contacts ADD COLUMN photo_blur TEXT;
ALTER TABLE users ADD COLUMN photo_blur TEXT;
