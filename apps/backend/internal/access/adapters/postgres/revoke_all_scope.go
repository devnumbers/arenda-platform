package postgres

import (
	"context"

	"github.com/google/uuid"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// RevokeAllScope answers the two existence probes of the global «Отозвать
// доступ всем» gate (issue #1310). The endpoint has no target id, so bare
// existence is all it conditions on: the ownership probe (any status —
// archived objects still hold revocable legs) decides proceed vs refusal,
// the membership probe splits the non-owner participant's loud 403 from the
// stranger's privacy-404. Implements the consumer-side accessapp.RevokeAllScope
// port (ADR 0035).
type RevokeAllScope struct {
	db postgres.DBTX
}

// NewRevokeAllScope creates a RevokeAllScope.
func NewRevokeAllScope(db postgres.DBTX) *RevokeAllScope {
	return &RevokeAllScope{db: db}
}

// OwnsAnyProperty reports whether the user owns at least one property, any
// status.
func (r *RevokeAllScope) OwnsAnyProperty(ctx context.Context, userID uuid.UUID) (bool, error) {
	return postgres.New(r.db).OwnerOwnsAnyProperty(ctx, pgconv.UUIDToPgtype(userID))
}

// HoldsAnyMembership reports whether the user holds at least one membership
// row anywhere, any status, any owner.
func (r *RevokeAllScope) HoldsAnyMembership(ctx context.Context, userID uuid.UUID) (bool, error) {
	return postgres.New(r.db).UserHoldsAnyMembership(ctx, pgconv.UUIDToPgtype(userID))
}

// Compile-time check that the adapter satisfies the application port.
var _ accessapp.RevokeAllScope = (*RevokeAllScope)(nil)
