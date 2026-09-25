-- name: ListRealtimePropertyReaders :many
-- The frame audience of one object (карта #714, тикет #716; ADR 0062 §4):
-- the owner plus the active members — the derived read access (ADR 0028)
-- resolved at the moment of publication. A suspended membership is no access
-- (the status filter, issue #158 T4 canon); a deleted property resolves to
-- nobody, so its frames simply die.
SELECT p.owner_id FROM properties p WHERE p.id = $1
UNION
SELECT m.user_id FROM property_members m WHERE m.property_id = $1 AND m.status = 'active';
