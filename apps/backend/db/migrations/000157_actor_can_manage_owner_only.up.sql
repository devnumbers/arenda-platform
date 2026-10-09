SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Решение D8 спеки «Совместный доступ» (карта #1302, тикет #1304):
-- управление участниками — прерогатива собственника. Политика
-- sharedpolicy.CanManageMembers стала owner-only той же правкой, за ней —
-- все 10 мутирующих эндпоинтов участников; этот CREATE OR REPLACE тянет за
-- собой SQL-скоуп: read-скоуп хаба (GET /participants, /summary, /{id} —
-- deep-link на чужого участника становится privacy-404) и removal-листинги
-- «Отозвать и удалить». Активный full_access больше не manage-участник:
-- один человек управляет доступом к своему объекту. STABLE: читает таблицы,
-- не пишет; PARALLEL SAFE: только чтение. Тело до D8 восстанавливает
-- down-миграция; матричный тест
-- TestParticipantRepository_ManageScopePredicateMatrix остаётся гейтом.
CREATE OR REPLACE FUNCTION actor_can_manage(target_property_id uuid, acting_user_id uuid)
RETURNS boolean
LANGUAGE sql STABLE PARALLEL SAFE
RETURN (
    EXISTS (
        SELECT 1 FROM properties p
        WHERE p.id = target_property_id
          AND p.owner_id = acting_user_id
    )
);
