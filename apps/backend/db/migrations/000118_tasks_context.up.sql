SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Tasks context schema (ADR 0051; контракт схемы — резолюция #496). Две
-- таблицы: правило порождает вхождения-задачи, повтор — TEXT-enum на правиле
-- (параметров повтора в v1 нет — JSONB-прецедент Payments отклонён). Просрочка
-- и «без срока» не хранятся — вычисляются на чтении; статус-надгробий нет:
-- удаление правила жёсткое, невыполненное сносится, журнал выполнений
-- остаётся (rule_id → SET NULL, снимки живут в самой задаче).

-- Правило: настройка на объекте, порождающая задачи. Идентификаторы —
-- UUIDv7 из приложения (ADR 0019); owner_id денормализован из property
-- (ADR 0028); тотальное удаление объекта — каскадом (ADR 0025 в редакции
-- ADR 0049). Время — с точностью до минут; время требует дату, повтор
-- (≠ once) требует якорь-дату: недатированным бывает только разовое правило.
CREATE TABLE task_rules (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    property_id UUID NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    comment TEXT,
    due_date DATE,
    due_time TIME,
    repeat TEXT NOT NULL CHECK (repeat IN ('once', 'daily', 'weekly', 'monthly', 'yearly')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT task_rules_time_requires_date CHECK (due_time IS NULL OR due_date IS NOT NULL),
    CONSTRAINT task_rules_repeat_needs_anchor CHECK (repeat = 'once' OR due_date IS NOT NULL)
);
CREATE TRIGGER trg_task_rules_updated_at
    BEFORE UPDATE ON task_rules
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Задача = вхождение: снимок названия/комментария/срока правила на момент
-- материализации. completed_date вместо статус-колонки: NULL = активная;
-- «Отменить выполнение» = completed_date := NULL и возможно только пока
-- правило живо (rule_id IS NOT NULL). Удалённое правило гасит всё
-- невыполненное физически (use case), выполненные остаются в журнале.
CREATE TABLE tasks (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    property_id UUID NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    rule_id UUID REFERENCES task_rules(id) ON DELETE SET NULL,
    due_date DATE,
    due_time TIME,
    title TEXT NOT NULL,
    comment TEXT,
    completed_date DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT tasks_time_requires_date CHECK (due_time IS NULL OR due_date IS NOT NULL)
);
CREATE TRIGGER trg_tasks_updated_at
    BEFORE UPDATE ON tasks
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Дедупликация материализации: одна задача правила на дату (любого статуса —
-- выполненная тоже держит ключ) и единственная недатированная задача правила.
CREATE UNIQUE INDEX idx_tasks_rule_date ON tasks(rule_id, due_date)
    WHERE rule_id IS NOT NULL AND due_date IS NOT NULL;
CREATE UNIQUE INDEX idx_tasks_rule_undated ON tasks(rule_id)
    WHERE rule_id IS NOT NULL AND due_date IS NULL;
CREATE INDEX idx_task_rules_property ON task_rules(property_id);
CREATE INDEX idx_task_rules_owner ON task_rules(owner_id);
CREATE INDEX idx_tasks_property_date ON tasks(property_id, due_date);
CREATE INDEX idx_tasks_property_completed ON tasks(property_id, completed_date);
CREATE INDEX idx_tasks_owner_date ON tasks(owner_id, due_date);
