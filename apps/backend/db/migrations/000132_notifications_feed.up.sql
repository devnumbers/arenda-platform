SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Хранимая лента уведомлений (карта #734, тикет #739; модель и каталог —
-- решение #737). Одно событие = строка на каждого получателя (fan-out); в
-- строке — снимок текста (заголовок/тело/лейбл контекста), payload-ссылки
-- и личные флаги прочитанности/удаления. Момент события — UTC instant;
-- группировка «Сегодня/Вчера» — дело экранного тикета (#744).

-- Таксономия #737: категория = группа per-channel настроек и иконка
-- (4 настраиваемые + Тариф и Системные, всегда включены). Слаги категорий
-- финализируются контрактом #743 по этому же набору.
CREATE TYPE notification_category AS ENUM (
    'rental',
    'payments_operations',
    'tasks',
    'shared_access',
    'tariff',
    'system'
);

-- Каталог типов событий v1 (решение #737, 15 типов). Мёртвые значения
-- notification_event_type (напоминания, #438/#277) остаются в типе навсегда;
-- legacy-значение subscription_grace мёртво с унификацией (#740/#741): grace
-- живёт в ленте и пайплайне, прямой канал больше не читает тип.
ALTER TYPE notification_event_type ADD VALUE 'rental_completed';
ALTER TYPE notification_event_type ADD VALUE 'payment_due';
ALTER TYPE notification_event_type ADD VALUE 'payment_overdue';
ALTER TYPE notification_event_type ADD VALUE 'task_overdue';
ALTER TYPE notification_event_type ADD VALUE 'property_invitation';
ALTER TYPE notification_event_type ADD VALUE 'invitation_accepted';
ALTER TYPE notification_event_type ADD VALUE 'access_revoked';
ALTER TYPE notification_event_type ADD VALUE 'access_paused';
ALTER TYPE notification_event_type ADD VALUE 'access_resumed';
ALTER TYPE notification_event_type ADD VALUE 'member_left';
ALTER TYPE notification_event_type ADD VALUE 'subscription_payment_failed';
ALTER TYPE notification_event_type ADD VALUE 'subscription_payment_reminder';
ALTER TYPE notification_event_type ADD VALUE 'subscription_payment_succeeded';
ALTER TYPE notification_event_type ADD VALUE 'subscription_plan_changed';
ALTER TYPE notification_event_type ADD VALUE 'system_maintenance';

CREATE TABLE notifications (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category notification_category NOT NULL,
    event_type notification_event_type NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    context_label TEXT,
    payload JSONB NOT NULL,
    dedup_key TEXT NOT NULL,
    read_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Дедуп (#737): повторная публикация с тем же ключом строку не создаёт;
-- ключ включает получателя, поэтому пара (user_id, dedup_key) уникальна.
CREATE UNIQUE INDEX idx_notifications_user_dedup_key
    ON notifications (user_id, dedup_key);

-- Лента: keyset-пагинация (канон #597) по (created_at DESC, id DESC)
-- среза пользователя.
CREATE INDEX idx_notifications_user_created
    ON notifications (user_id, created_at DESC, id DESC);

-- Счётчик непрочитанных и фильтр «Непрочитанные»: непрочитанные
-- неудалённые строки пользователя (#737).
CREATE INDEX idx_notifications_user_unread
    ON notifications (user_id, created_at DESC, id DESC)
    WHERE read_at IS NULL AND deleted_at IS NULL;

-- Старые per-event-type × per-channel настройки (ADR 0030) уходят без
-- переноса: сброс opt-out'ов grace — решение #738, категория «Тариф» честно
-- всегда включена для всех. Новый контракт настроек (email per-account,
-- push per-device) — #743; преемник ADR — 0056.
DROP TABLE user_notification_channel_preferences;
