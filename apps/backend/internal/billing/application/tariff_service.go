package application

import (
	"context"
	"fmt"

	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// TariffService handles read-only tariff queries.
type TariffService struct {
	deps flowDeps
}

// NewTariffService creates a TariffService.
func NewTariffService(deps flowDeps) *TariffService {
	return &TariffService{deps: deps}
}

// ListTariffs returns all tariffs ordered by price.
func (s *TariffService) ListTariffs(ctx context.Context) ([]domain.Tariff, error) {
	list, err := s.deps.tariffs.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tariffs: %w", err)
	}

	return list, nil
}
