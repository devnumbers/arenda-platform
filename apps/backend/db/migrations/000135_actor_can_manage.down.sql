SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Down: убрать канон предиката manage-скоупа (тикет #794). Прецедент
-- contacts_search_text (000124): функция сносится, хотя живые запросы
-- db/queries её зовут, — откат схемы сопровождается откатом кода.
DROP FUNCTION IF EXISTS actor_can_manage(uuid, uuid);
