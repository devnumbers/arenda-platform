package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// TariffRepository persists tariffs.
type TariffRepository struct {
	db postgres.DBTX
}

// NewTariffRepository creates a new tariff repository.
func NewTariffRepository(db postgres.DBTX) *TariffRepository {
	return &TariffRepository{db: db}
}

func (r *TariffRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *TariffRepository) WithTx(tx transaction.Tx) application.TariffRepository {
	return NewTariffRepository(tx.(postgres.DBTX))
}

// GetByName returns a tariff by its unique name.
func (r *TariffRepository) GetByName(ctx context.Context, name domain.TariffName) (domain.Tariff, error) {
	row, err := r.q().GetTariffByName(ctx, string(name))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Tariff{}, application.ErrNotFound
		}
		return domain.Tariff{}, err
	}
	return mapTariff(row), nil
}

// List returns all tariffs ordered by price.
func (r *TariffRepository) List(ctx context.Context) ([]domain.Tariff, error) {
	rows, err := r.q().ListTariffs(ctx)
	if err != nil {
		return nil, err
	}
	return mapTariffs(rows), nil
}

func mapTariff(row postgres.Tariff) domain.Tariff {
	return domain.Tariff{
		ID:                  uuid.UUID(row.ID.Bytes),
		Name:                domain.TariffName(row.Name),
		ActivePropertyLimit: int(row.ActivePropertyLimit),
		MonthlyPriceKopecks: row.MonthlyPriceKopecks,
		YearlyPriceKopecks:  row.YearlyPriceKopecks,
	}
}

func mapTariffs(rows []postgres.Tariff) []domain.Tariff {
	result := make([]domain.Tariff, len(rows))
	for i, row := range rows {
		result[i] = mapTariff(row)
	}
	return result
}
