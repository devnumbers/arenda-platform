package application

import (
	"context"

	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// TariffService serves the user-facing tariff views: the active plans users
// can choose from.
type TariffService struct {
	txStoreFactory
}

// NewTariffService creates a tariff service over the shared factory. Listing
// is a single read, so it goes straight to the factory's non-transactional
// repository — no Unit-of-Work needed (ADR 0033: transactions for writes).
func NewTariffService(factory txStoreFactory) *TariffService {
	return &TariffService{txStoreFactory: factory}
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
