-- name: CreatePropertyPhoto :one
INSERT INTO property_photos (property_id, url)
VALUES ($1, $2)
RETURNING *;

-- name: ListPropertyPhotosByPropertyID :many
SELECT * FROM property_photos WHERE property_id = $1 ORDER BY created_at;

-- name: ListPropertyPhotosByPropertyIDs :many
SELECT * FROM property_photos WHERE property_id = ANY($1::uuid[]) ORDER BY created_at;

-- name: CountPropertyPhotosByPropertyID :one
SELECT COUNT(*) FROM property_photos WHERE property_id = $1;
