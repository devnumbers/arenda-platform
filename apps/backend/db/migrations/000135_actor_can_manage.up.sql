SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Предикат manage-скоупа участников (карта #692, тикет #794): «владелец
-- объекта OR активный full_access-участник» — инвариант приватности раздела
-- «Совместный доступ». До #794 жил тремя текстовыми копиями в телах запросов
-- db/queries/participants.sql: sqlc не умеет шарить текст между запросами,
-- копии держались синхронными комментарием-каноном и матричным тестом
-- TestParticipantRepository_ManageScopePredicateMatrix. Теперь канон один —
-- эта функция, запросы зовут её, матричный тест остаётся гейтом: дрейф
-- семантики краснит тест, а не открывает тихую дыру. STABLE: читает
-- таблицы, не пишет; PARALLEL SAFE: только чтение.
--
-- Имена параметров намеренно не совпадают с колонками затронутых таблиц:
-- в теле SQL-функции колонка молча побеждает одноимённый параметр
-- (docs/postgresql: «the column name will take precedence»). Параметр
-- property_id в тени m.property_id сравнивал колонку саму с собой —
-- предикат отвечал «может управлять» и на чужих объектах; поймал
-- TestParticipantRepository_RemovalScopeMemberships (фикстура из трёх
-- объектов), где матрица с одним объектом слепа.
CREATE FUNCTION actor_can_manage(target_property_id uuid, acting_user_id uuid)
RETURNS boolean
LANGUAGE sql STABLE PARALLEL SAFE
RETURN (
    EXISTS (
        SELECT 1 FROM properties p
        WHERE p.id = target_property_id
          AND p.owner_id = acting_user_id
    )
    OR EXISTS (
        SELECT 1 FROM property_members m
        WHERE m.property_id = target_property_id
          AND m.user_id = acting_user_id
          AND m.status = 'active'
          AND m.role = 'full_access'
    )
);
