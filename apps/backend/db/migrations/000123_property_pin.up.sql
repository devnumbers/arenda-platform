SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Глобальное закрепление объектов (карта #573, тикет #577). pinned_at — и
-- признак, и время в одном поле: NULL — не закреплён, момент — закреплён;
-- порядок среди закреплённых — по времени закрепления. Отдельная таблица
-- дублировала бы связь 1:1, не имея дюрального инварианта. Архивный объект
-- вне активных списков, поэтому закрепление бессмысленно и сбрасывается тем
-- же UPDATE, что переводит объект в архив — CHECK держит инвариант.
ALTER TABLE properties ADD COLUMN pinned_at TIMESTAMPTZ;
ALTER TABLE properties ADD CONSTRAINT properties_pinned_at_check
    CHECK (pinned_at IS NULL OR status <> 'archived');
