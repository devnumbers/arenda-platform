package postgres

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// TariffRepository persists tariffs.
type TariffRepository struct {
	db     postgres.DBTX
	mu     sync.RWMutex
	byID   map[uuid.UUID]domain.Tariff
	byName map[domain.TariffName]domain.Tariff
	list   []domain.Tariff
}

// NewTariffRepository creates a new tariff repository.
func NewTariffRepository(db postgres.DBTX) *TariffRepository {
	return &TariffRepository{
		db:     db,
		byID:   make(map[uuid.UUID]domain.Tariff),
		byName: make(map[domain.TariffName]domain.Tariff),
	}
}

func (r *TariffRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *TariffRepository) WithTx(tx transaction.Tx) application.TariffRepository {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return &invalidTariffRepository{tx: tx}
	}
	return NewTariffRepository(dbtx)
}

// invalidTariffRepository returns a clear error for every method when an
// unsupported transaction type is passed to WithTx.
type invalidTariffRepository struct {
	tx transaction.Tx
}

func (r *invalidTariffRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Tariff, error) {
	return domain.Tariff{}, fmt.Errorf("billing: unsupported transaction type %T for TariffRepository.GetByID", r.tx)
}

func (r *invalidTariffRepository) GetByName(ctx context.Context, name domain.TariffName) (domain.Tariff, error) {
	return domain.Tariff{}, fmt.Errorf("billing: unsupported transaction type %T for TariffRepository.GetByName", r.tx)
}

func (r *invalidTariffRepository) List(ctx context.Context) ([]domain.Tariff, error) {
	return nil, fmt.Errorf("billing: unsupported transaction type %T for TariffRepository.List", r.tx)
}

func (r *invalidTariffRepository) WithTx(tx transaction.Tx) application.TariffRepository {
	return r
}

// GetByID returns a tariff by ID.
func (r *TariffRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Tariff, error) {
	if tariff, ok := r.cachedByID(id); ok {
		return tariff, nil
	}

	row, err := r.q().GetTariffByID(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Tariff{}, application.ErrNotFound
		}
		return domain.Tariff{}, err
	}
	tariff := mapTariff(row)
	r.store(tariff)
	return tariff, nil
}

// GetByName returns a tariff by its unique name.
func (r *TariffRepository) GetByName(ctx context.Context, name domain.TariffName) (domain.Tariff, error) {
	if tariff, ok := r.cachedByName(name); ok {
		return tariff, nil
	}

	row, err := r.q().GetTariffByName(ctx, string(name))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Tariff{}, application.ErrNotFound
		}
		return domain.Tariff{}, err
	}
	tariff := mapTariff(row)
	r.store(tariff)
	return tariff, nil
}

// List returns all tariffs ordered by price.
func (r *TariffRepository) List(ctx context.Context) ([]domain.Tariff, error) {
	if list, ok := r.cachedList(); ok {
		return list, nil
	}

	rows, err := r.q().ListTariffs(ctx)
	if err != nil {
		return nil, err
	}
	list := mapTariffs(rows)
	r.storeList(list)
	return copyTariffs(list), nil
}

func (r *TariffRepository) cachedByID(id uuid.UUID) (domain.Tariff, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tariff, ok := r.byID[id]
	return tariff, ok
}

func (r *TariffRepository) cachedByName(name domain.TariffName) (domain.Tariff, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tariff, ok := r.byName[name]
	return tariff, ok
}

func (r *TariffRepository) cachedList() ([]domain.Tariff, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.list == nil {
		return nil, false
	}
	return copyTariffs(r.list), true
}

func (r *TariffRepository) store(tariff domain.Tariff) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[tariff.ID] = tariff
	r.byName[tariff.Name] = tariff
}

func (r *TariffRepository) storeList(list []domain.Tariff) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.list = copyTariffs(list)
	for _, tariff := range list {
		r.byID[tariff.ID] = tariff
		r.byName[tariff.Name] = tariff
	}
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

func copyTariffs(src []domain.Tariff) []domain.Tariff {
	dst := make([]domain.Tariff, len(src))
	copy(dst, src)
	return dst
}
