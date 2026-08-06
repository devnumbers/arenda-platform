-- Pending property member invitations by email (T5, issue #161). Emails are
-- stored lowercase; lookups compare with lower() on the parameter side too.

-- name: CreatePropertyMemberInvitation :one
INSERT INTO property_member_invitations (id, property_id, email, role, invited_by, last_sent_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetPropertyMemberInvitation :one
SELECT * FROM property_member_invitations
WHERE id = $1 AND property_id = $2;

-- name: GetPropertyMemberInvitationByEmail :one
SELECT * FROM property_member_invitations
WHERE property_id = sqlc.arg('property_id')::uuid AND lower(email) = lower(sqlc.arg('email')::text);

-- name: ListPropertyMemberInvitations :many
SELECT * FROM property_member_invitations
WHERE property_id = $1
ORDER BY created_at ASC;

-- name: ListPendingInvitationsByEmail :many
-- All pending invitations for an email, oldest first: activation at
-- registration is FIFO across properties (T5, issue #161).
SELECT * FROM property_member_invitations
WHERE lower(email) = lower(sqlc.arg('email')::text)
ORDER BY created_at ASC;

-- name: UpdatePropertyMemberInvitationRole :one
UPDATE property_member_invitations SET role = $3
WHERE id = $1 AND property_id = $2
RETURNING *;

-- name: UpdatePropertyMemberInvitationLastSentAt :exec
UPDATE property_member_invitations SET last_sent_at = $3
WHERE id = $1 AND property_id = $2;

-- name: DeletePropertyMemberInvitation :exec
DELETE FROM property_member_invitations
WHERE id = $1 AND property_id = $2;
