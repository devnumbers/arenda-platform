SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Down: убрать журнал действий (карта #704, тикет #707). Индексы
-- снимаются самим DROP TABLE. Расширение pg_trgm остаётся в базе:
-- прецедент 000124 — расширения не дропаются, откат снимает только
-- объекты этой миграции.
DROP TABLE IF EXISTS action_journal;
