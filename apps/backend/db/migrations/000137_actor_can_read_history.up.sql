SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Предикат read-скоупа журнала «История действий» (карта #704, тикет #708,
-- ADR 0061 §7): «владелец объекта OR активный участник (full_access/viewer)
-- неархивного объекта». Читает журнал владелец при любом статусе объекта —
-- история переживает архив и ведётся годами; участнику архив ленту не
-- раскрывает (канон невидимости архива, #163). Приостановленное участие
-- и чужие объекты отвечают «нет» — для читателя это privacy-404 (существование
-- строк не раскрывается). До тикета #708 предикат нигде не шарился: лента,
-- опции фильтров (участники тремя ногами UNION) и объекты фильтров зовут его
-- из пяти мест; sqlc не умеет шарить текст между запросами, прецедент —
-- actor_can_manage (000135, #794). STABLE: читает таблицы, не пишет;
-- PARALLEL SAFE: только чтение.
--
-- Имена параметров намеренно не совпадают с колонками затронутых таблиц:
-- в теле SQL-функции колонка молча побеждает одноимённый параметр
-- (см. комментарий 000135_actor_can_manage).
CREATE FUNCTION actor_can_read_history(target_property_id uuid, acting_user_id uuid)
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
