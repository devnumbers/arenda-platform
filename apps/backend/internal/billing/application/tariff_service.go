package application

import (
	"context"
	"log/slog"

	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// TariffService serves the user-facing tariff views: the active plans users
// can choose from.
type TariffService struct {
	txStoreFactory
	log *slog.Logger
}

// TariffServiceConfig carries the non-transactional dependencies of the
// tariff service. A nil Log defaults to the default logger, matching the
// module's constructor conventions.
type TariffServiceConfig struct {
	Log *slog.Logger
}

// NewTariffService creates a tariff service over the shared factory. Listing
// is a single read, so it goes straight to the factory's non-transactional
// repository — no Unit-of-Work needed (ADR 0033: transactions for writes).
func NewTariffService(factory txStoreFactory, cfg TariffServiceConfig) *TariffService {
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	return &TariffService{txStoreFactory: factory, log: cfg.Log}
}

// ListTariffs returns the active tariffs ordered by price. Hidden tariffs
// (is_active=false) stay referable by foreign keys but are not offered.
func (s *TariffService) ListTariffs(ctx context.Context) ([]domain.Tariff, error) {
	tariffs, err := s.tariffs.List(ctx)
	if err != nil {
		return nil, err
	}
	return tariffs, nil
}

// ListAllTariffs returns every tariff including hidden ones, ordered by price.
// It backs the admin tariff views (issue #247).
func (s *TariffService) ListAllTariffs(ctx context.Context) ([]domain.Tariff, error) {
	tariffs, err := s.tariffs.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	return tariffs, nil
}
