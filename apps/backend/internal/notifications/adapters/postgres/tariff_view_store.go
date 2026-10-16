package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// TariffViewStore answers the tariff events' display questions (#752) over
// the owning tariffs table. Read-only — the events publish through the
// pipeline, nothing is written here.
type TariffViewStore struct {
	queries *postgres.Queries
}

// NewTariffViewStore creates the tariff events' view store over the pool.
func NewTariffViewStore(db postgres.DBTX) *TariffViewStore {
	return &TariffViewStore{queries: postgres.New(db)}
}

// The adapter satisfies the consumer-declared port (CODING_STANDARDS).
var _ application.TariffEventViewSource = (*TariffViewStore)(nil)

// TariffView returns the plan's display snapshot: the slug the copy's display
// name resolves from (application.TariffDisplayName).
func (s *TariffViewStore) TariffView(ctx context.Context, tariffID uuid.UUID) (application.TariffView, error) {
	slug, err := s.queries.GetTariffEventView(ctx, pgconv.UUIDToPgtype(tariffID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.TariffView{}, fmt.Errorf("tariff %s not found: %w", tariffID, application.ErrNotFound)
		}
		return application.TariffView{}, fmt.Errorf("get tariff view %s: %w", tariffID, err)
	}
	return application.TariffView{Name: slug}, nil
}
