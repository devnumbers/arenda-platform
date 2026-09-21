-- Откат 000131: таблица ленты, категория и дропнутые настройки.
-- Добавленные значения notification_event_type удалить нельзя (PostgreSQL
-- не снимает значения enum; прецедент #277) — они остаются мёртвыми.

DROP TABLE notifications;
DROP TYPE notification_category;

-- Схема user_notification_channel_preferences восстановлена по миграции
-- 000100; строки и хранившиеся opt-out'ы невосстановимы.
CREATE TABLE user_notification_channel_preferences (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type notification_event_type NOT NULL,
    channel notification_channel NOT NULL,
    allowed BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, event_type, channel)
);

CREATE TRIGGER trg_user_notification_channel_preferences_updated_at
BEFORE UPDATE ON user_notification_channel_preferences
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
