-- name: CreatePropertyPhoto :one
INSERT INTO property_photos (id, property_id, url)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListPropertyPhotosByPropertyID :many
SELECT * FROM property_photos WHERE property_id = $1 ORDER BY created_at;

-- name: ListPropertyPhotosByPropertyIDs :many
SELECT * FROM property_photos WHERE property_id = ANY($1::uuid[]) ORDER BY created_at;

-- name: CountPropertyPhotosByPropertyID :one
SELECT COUNT(*) FROM property_photos WHERE property_id = $1;

-- name: GetPropertyPhotoByID :one
SELECT * FROM property_photos WHERE id = $1;

-- name: GetPropertyPhotoByIDAndPropertyID :one
SELECT * FROM property_photos WHERE id = $1 AND property_id = $2;

-- name: DeletePropertyPhoto :exec
DELETE FROM property_photos WHERE id = $1;
