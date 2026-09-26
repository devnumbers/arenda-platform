SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Канон derived-read (ADR 0028, тикет #884): «владелец объекта OR активный
-- участник (full_access/viewer) неархивного объекта». Жил в трёх кодировках:
-- SQL-функция actor_can_read_history (000137, имя вело от первого
-- потребителя — журнала «История действий»), инлайн в аудитории
-- realtime-кадров (db/queries/realtime.sql, ADR 0062 §4) и родственные
-- инлайны в actor-scoped глобальных лентах (payments_global.sql,
-- tasks_tasks.sql, properties.sql). Теперь предикат один — эта функция,
-- контекстно-нейтральное имя actor_can_read_property: тело побайтово
-- совпадает с 000137, для читателя чужой/приостановленный доступ отвечает
-- «нет» (privacy-404, существование строк не раскрывается), владелец читает
-- архив (история переживает архив, #163), участнику архив ленту не
-- раскрывает. Носители с иным архивным договором остаются на своих
-- инлайнах: платёжная лента операций держит тумблер include_archived
-- (архивный объект виден активному участнику по явному запросу — функция
-- отвечает «нет»), карточки контактов читают членство без архивного среза.
-- sqlc не умеет шарить текст между запросами, прецеденты — actor_can_manage
-- (000135, #794) и actor_can_read_history (000137, #708). STABLE: читает
-- таблицы, не пишет; PARALLEL SAFE: только чтение.
--
-- Имена параметров намеренно не совпадают с колонками затронутых таблиц:
-- в теле SQL-функции колонка молча побеждает одноимённый параметр
-- (см. комментарий 000135_actor_can_manage).
CREATE FUNCTION actor_can_read_property(target_property_id uuid, acting_user_id uuid)
RETURNS boolean
LANGUAGE sql STABLE PARALLEL SAFE
RETURN (
    EXISTS (
        SELECT 1 FROM properties p
        WHERE p.id = target_property_id
          AND (
               p.owner_id = acting_user_id
               OR (
                   p.status <> 'archived'
                   AND EXISTS (
                        SELECT 1 FROM property_members m
                        WHERE m.property_id = p.id
                          AND m.user_id = acting_user_id
                          AND m.status = 'active'
                      )
                 )
              )
    )
);

DROP FUNCTION IF EXISTS actor_can_read_history(uuid, uuid);
