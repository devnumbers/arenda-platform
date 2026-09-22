-- name: InsertActionJournal :one
INSERT INTO action_journal (
    id, property_id, actor_id, actor_role, actor_name, actor_email,
    kind, action, base_action, segments, searchable, context, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING id;
