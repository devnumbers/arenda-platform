package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
)

// The admin tariff management operations of issue #256: creating a plan,
// editing its prices, property limit and activity. Everything lives on
// TariffService — the owner of the tariff vocabulary — beside the listing
// views it already serves. Each operation lands the write and its audit
// record (actor admin) in one transaction; after a committed write the shared
// repository's cached reads are invalidated, so the admin screen and every
// price-charging flow observe the new values immediately instead of after the
// cache TTL.

// tariffAuditContext captures the resulting field values of an admin tariff
// write — the incident-review trail of a pricing change.
func tariffAuditContext(tariff domain.Tariff) map[string]any {
	return map[string]any{
		auditKeyTariffName:      string(tariff.Name),
		"active_property_limit": tariff.ActivePropertyLimit,
		"monthly_price_kopecks": tariff.MonthlyPriceKopecks,
		"yearly_price_kopecks":  tariff.YearlyPriceKopecks,
		"is_active":             tariff.IsActive,
	}
}

// invalidateAfterCommit drops the shared repository's cached reads after a
// committed admin write. It is best-effort by design: a failure only means
// readers may serve the previous values until the cache TTL expires — the
// write itself is durable, so it must not fail the already-committed result
// (a failed answer would send the admin into a retry that ends in 409). The
// failure is warned about instead of silently discarded.
func (s *TariffService) invalidateAfterCommit(ctx context.Context) {
	if err := s.tariffs.Invalidate(ctx); err != nil {
		s.log.WarnContext(ctx, "tariff cache invalidation failed after admin write; stale values may be served until the cache TTL",
			slog.String("error", sanitize.Error(err)))
	}
}

// CreateTariff creates a new plan (issue #256). The name must come from the
// closed TariffName vocabulary the frozen user contract pins, and a name that
// already exists answers ErrAlreadyExists — the database's unique constraint
// is the durable backstop.
func (s *TariffService) CreateTariff(ctx context.Context, adminID uuid.UUID, req CreateTariffRequest) (domain.Tariff, error) {
	tariff, err := domain.NewTariff(req.Name, req.ActivePropertyLimit, req.MonthlyPriceKopecks, req.YearlyPriceKopecks, true)
	if err != nil {
		return domain.Tariff{}, err
	}

	err = s.runInTx(ctx, func(stores *txStores) error {
		created, err := stores.tariffs.Create(ctx, tariff)
		if err != nil {
			if errors.Is(err, ErrAlreadyExists) {
				return ErrTariffAlreadyExists
			}
			return err
		}
		tariff = created
		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &adminID,
			ActorRole:  auditdomain.ActorRoleAdmin,
			Action:     auditdomain.ActionTariffCreated,
			EntityType: auditdomain.EntityTariff,
			EntityID:   &created.ID,
			Context:    tariffAuditContext(created),
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Tariff{}, err
	}
	s.invalidateAfterCommit(ctx)
	return tariff, nil
}

// UpdateTariff edits a plan's prices, property limit and activity
// (issue #256); hiding is IsActive=false. The name is immutable. Paid periods
// already granted are not repriced: payments snapshot the amount at charge
// time, so the next renewal simply reads the updated price.
func (s *TariffService) UpdateTariff(ctx context.Context, adminID, id uuid.UUID, req UpdateTariffRequest) (domain.Tariff, error) {
	var updated domain.Tariff
	err := s.runInTx(ctx, func(stores *txStores) error {
		current, err := stores.tariffs.GetByID(ctx, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrTariffNotFound
			}
			return fmt.Errorf("get tariff: %w", err)
		}
		tariff := domain.Tariff{
			ID:                  current.ID,
			Name:                current.Name,
			ActivePropertyLimit: req.ActivePropertyLimit,
			MonthlyPriceKopecks: req.MonthlyPriceKopecks,
			YearlyPriceKopecks:  req.YearlyPriceKopecks,
			IsActive:            req.IsActive,
		}
		if err := tariff.Validate(); err != nil {
			return err
		}
		saved, err := stores.tariffs.Update(ctx, tariff)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrTariffNotFound
			}
			return fmt.Errorf("update tariff: %w", err)
		}
		updated = saved
		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &adminID,
			ActorRole:  auditdomain.ActorRoleAdmin,
			Action:     auditdomain.ActionTariffUpdated,
			EntityType: auditdomain.EntityTariff,
			EntityID:   &saved.ID,
			Context:    tariffAuditContext(saved),
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Tariff{}, err
	}
	s.invalidateAfterCommit(ctx)
	return updated, nil
}
