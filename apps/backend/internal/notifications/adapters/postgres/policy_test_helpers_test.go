package postgres

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// pgUUID converts a uuid.UUID into the pgtype.UUID the generated queries
// take (shared by the notifications postgres integration tests).
func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}
