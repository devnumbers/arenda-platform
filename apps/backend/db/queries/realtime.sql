-- name: ListRealtimePropertyReaders :many
-- The frame audience of one object (карта #714, тикет #716; ADR 0062 §4):
-- the derived read access (ADR 0028) resolved at the moment of publication.
-- The predicate lives in one place — the SQL function actor_can_read_property
-- (000142, the successor of actor_can_read_history 000137): the owner reads
-- at any property status, an active member only while the property is not
-- archived (the archive's invisibility canon #163), a suspended membership
-- is no access (#158 T4), a deleted property resolves to nobody — its frames
-- simply die. The query enumerates the candidates (the owner plus the
-- object's members — a small set) and lets the function give the verdict, so
-- the canon is not re-encoded here.
SELECT c.user_id
FROM (
       SELECT p.owner_id AS user_id FROM properties p WHERE p.id = $1
       UNION
       SELECT m.user_id FROM property_members m WHERE m.property_id = $1
     ) c
WHERE actor_can_read_property($1, c.user_id);
