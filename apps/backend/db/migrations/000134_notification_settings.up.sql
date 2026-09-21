SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Настройки уведомлений (карта #734, тикет #743; решение #738, ADR 0058):
-- матрица категория × канал. Email — на аккаунте: одна строка на
-- пользователя с четырьмя настраиваемыми категориями; нет строки = включено
-- (дефолт NOT NULL DEFAULT true). Push — на устройстве: мастер-тумблер
-- enabled и категорийные флаги прямо на push-подписке; выключение — флаг,
-- подписка и настройки сохраняются. Тариф и Системные всегда включены и
-- в настройках не хранятся.

CREATE TABLE notification_email_preferences (
    user_id             uuid    NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    rental              boolean NOT NULL DEFAULT true,
    payments_operations boolean NOT NULL DEFAULT true,
    tasks               boolean NOT NULL DEFAULT true,
    shared_access       boolean NOT NULL DEFAULT true,
    PRIMARY KEY (user_id)
);

ALTER TABLE push_subscriptions
    ADD COLUMN enabled                      boolean NOT NULL DEFAULT true,
    ADD COLUMN category_rental              boolean NOT NULL DEFAULT true,
    ADD COLUMN category_payments_operations boolean NOT NULL DEFAULT true,
    ADD COLUMN category_tasks               boolean NOT NULL DEFAULT true,
    ADD COLUMN category_shared_access       boolean NOT NULL DEFAULT true;
