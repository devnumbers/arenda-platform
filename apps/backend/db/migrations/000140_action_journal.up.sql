SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Журнал действий «История» (карта #704, тикет #707, ADR 0061): продуктовая
-- лента ручных действий по объектам для владельца и участников. Отдельный
-- контекст и таблица — audit_log (ADR 0020) не трогается: там админская
-- дисциплина PII без человекочитаемых снимков, здесь снимки названий имён
-- и почт участников — канон (решение #706).
--
-- Запись безусловная для всех объектов, только ручные действия пользователя;
-- строка живёт вечно вместе с объектом: FK CASCADE — при удалении объекта
-- записи каскадятся (записи property.deleted не существует — она была бы
-- мертворождённой), удаление пользователя обезличивает actor_id (SET NULL),
-- но снимки имени/почты остаются в строке.
--
-- Поиск по большим объёмам — гибрид исследования #705: материализованная при
-- записи колонка searchable (конкатенация сегментов + имя + почта актёра) с
-- двумя GIN — trgm (подстроки, названия, email) и to_tsvector('russian')
-- STORED (морфология); обе ноги — один всегда-OR предикат при чтении:
-- prefix-FTS OR ILIKE-trgm (ресерч #839); маршрутизация trgm/fts/both
-- снесена #842. Расширение pg_trgm создаётся миграцией 000124.
CREATE TABLE action_journal (
    id          UUID PRIMARY KEY,
    property_id UUID NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    actor_id    UUID REFERENCES users(id) ON DELETE SET NULL,
    actor_role  TEXT NOT NULL,
    actor_name  TEXT NOT NULL DEFAULT '',
    actor_email TEXT NOT NULL DEFAULT '',
    kind        TEXT NOT NULL,
    action      TEXT NOT NULL,
    base_action TEXT NOT NULL,
    segments    JSONB NOT NULL,
    searchable  TEXT NOT NULL,
    search_tsv  TSVECTOR GENERATED ALWAYS AS (to_tsvector('russian', searchable)) STORED,
    context     JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL
);

-- Курсорные развёртки чтения (ADR 0061 §7): по объекту (лента объекта и
-- общая), по актёру («Действия участника»), по виду действия (группы
-- фильтра). Ключ keyset — (created_at DESC, id DESC).
CREATE INDEX idx_action_journal_property_created
    ON action_journal (property_id, created_at DESC, id DESC);

CREATE INDEX idx_action_journal_actor_created
    ON action_journal (actor_id, created_at DESC, id DESC);

CREATE INDEX idx_action_journal_kind_created
    ON action_journal (kind, created_at DESC, id DESC);

CREATE INDEX idx_action_journal_searchable_trgm
    ON action_journal USING gin (searchable gin_trgm_ops);

CREATE INDEX idx_action_journal_search_tsv
    ON action_journal USING gin (search_tsv);
