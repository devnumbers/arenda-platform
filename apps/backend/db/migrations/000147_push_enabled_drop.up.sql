SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Снос вырожденного мастера enabled (карта #1024, спека #1028 §5, решение
-- владельца 01.10): вариант Б задним числом — строка есть = устройство
-- включено, строки нет = выключено; выключение удаляет подписку целиком
-- (идемпотентный DELETE /push/subscriptions). Строки enabled=false от старой
-- модели становятся отсутствием строки — их браузеры до-лечит фронтовый heal
-- (слайс #1038); колонка сносится, категорийные флаги остаются.
DELETE FROM push_subscriptions WHERE NOT enabled;
ALTER TABLE push_subscriptions DROP COLUMN enabled;
