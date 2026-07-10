package postgres

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// TariffRepository persists tariffs.
type TariffRepository struct {
	db       postgres.DBTX
	mu       sync.RWMutex
	byID     map[uuid.UUID]domain.Tariff
	byName   map[domain.TariffName]domain.Tariff
	list     []domain.Tariff
	cachedAt time.Time
	ttl      time.Duration
	clock    clock.Clock
}

// NewTariffRepository creates a new tariff repository.
func NewTariffRepository(db postgres.DBTX, ttl time.Duration, clk clock.Clock) *TariffRepository {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	if clk == nil {
		clk = clock.Real{}
	}
	return &TariffRepository{
		db:     db,
		byID:   make(map[uuid.UUID]domain.Tariff),
		byName: make(map[domain.TariffName]domain.Tariff),
		ttl:    ttl,
		clock:  clk,
	}
}

func (r *TariffRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
// The returned instance has its own empty in-memory cache and TTL, independent
// of the parent instance, so transactional reads always see the latest database
// state within the transaction.
func (r *TariffRepository) WithTx(tx transaction.Tx) (application.TariffRepository, error) {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return nil, fmt.Errorf("billing.TariffRepository.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewTariffRepository(dbtx, r.ttl, r.clock), nil
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
	if r.staleLocked() {
		return domain.Tariff{}, false
	}
	tariff, ok := r.byID[id]
	return tariff, ok
}

func (r *TariffRepository) cachedByName(name domain.TariffName) (domain.Tariff, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.staleLocked() {
		return domain.Tariff{}, false
	}
	tariff, ok := r.byName[name]
	return tariff, ok
}

func (r *TariffRepository) cachedList() ([]domain.Tariff, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.staleLocked() || r.list == nil {
		return nil, false
	}
	return copyTariffs(r.list), true
}

func (r *TariffRepository) staleLocked() bool {
	return r.cachedAt.IsZero() || r.clock.Now().Sub(r.cachedAt) >= r.ttl
}

// store caches a single tariff in this repository instance and refreshes the
// instance-level cachedAt timestamp.
func (r *TariffRepository) store(tariff domain.Tariff) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[tariff.ID] = tariff
	r.byName[tariff.Name] = tariff
	r.cachedAt = r.clock.Now()
}

// storeList caches a full tariff list in this repository instance, refreshes the
// instance-level cachedAt timestamp, and also populates the by-ID/by-name maps.
func (r *TariffRepository) storeList(list []domain.Tariff) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.list = copyTariffs(list)
	for _, tariff := range list {
		r.byID[tariff.ID] = tariff
		r.byName[tariff.Name] = tariff
	}
	r.cachedAt = r.clock.Now()
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
