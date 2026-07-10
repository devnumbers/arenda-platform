package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
)

// ScheduledChangeService applies scheduled tariff changes.
type ScheduledChangeService struct {
	deps scheduledChangeServiceDeps
}

// NewScheduledChangeService creates a ScheduledChangeService.
func NewScheduledChangeService(deps scheduledChangeServiceDeps) *ScheduledChangeService {
	return &ScheduledChangeService{deps: deps}
}

// ProcessScheduledChanges applies scheduled tariff changes (usually downgrades to
// the free basic tariff) whose pending_change_at has been reached. Returns the
// number of subscriptions processed.
func (c *ScheduledChangeService) ProcessScheduledChanges(ctx context.Context, now time.Time) (int, error) {
	processed := 0
	for {
		subs, err := c.deps.subscriptions.ListPendingChanges(ctx, now.UTC(), renewalBatchSize)
		if err != nil {
			return processed, fmt.Errorf("list subscriptions with pending change: %w", err)
		}
		if len(subs) == 0 {
			break
		}
		for _, sub := range subs {
			if err := c.applyScheduledChange(ctx, sub, now.UTC()); err != nil {
				c.deps.log.ErrorContext(ctx, "apply scheduled change failed",
					slog.String("subscription_id", sub.ID.String()),
					slog.String("user_id", sub.UserID.String()),
					slog.String("error", sanitize.Error(err)))
				continue
			}
			processed++
		}
		if len(subs) < renewalBatchSize {
			break
		}
	}
	return processed, nil
}

// applyScheduledChange applies a deferred downgrade in a single short transaction
// with no provider call. Per ADR 0008 §3 there is no charge at apply time: it
// locks the subscription, resolves the pending tariff, applies the downgrade
// (which extends valid_until from now, enables auto-renew and clears the pending
// change), archives any properties that exceed the new limit, and commits.
func (c *ScheduledChangeService) applyScheduledChange(ctx context.Context, sub domain.Subscription, now time.Time) error {
	tx, err := c.deps.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txSubscriptions, err := c.deps.subscriptions.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind subscriptions transaction: %w", err)
	}
	txTariffs, err := c.deps.tariffs.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind tariffs transaction: %w", err)
	}

	sub, err = txSubscriptions.GetByIDForUpdate(ctx, sub.ID)
	if err != nil {
		return fmt.Errorf("get subscription for update: %w", err)
	}
	if sub.Status != domain.SubscriptionStatusActive || sub.PendingTariffID == nil || sub.PendingChangeAt == nil || sub.PendingChangeAt.After(now) {
		return nil
	}

	pendingTariff, err := txTariffs.GetByID(ctx, *sub.PendingTariffID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrTariffNotFound
		}
		return fmt.Errorf("get pending tariff: %w", err)
	}

	period := domain.PeriodMonth
	if sub.PendingPeriod != nil {
		period = *sub.PendingPeriod
	}

	oldTariffID := sub.TariffID
	if err := sub.ApplyScheduledDowngrade(pendingTariff, period, now); err != nil {
		return fmt.Errorf("apply scheduled downgrade: %w", err)
	}
	if err := txSubscriptions.Update(ctx, sub); err != nil {
		return fmt.Errorf("update subscription after scheduled downgrade: %w", err)
	}
	if c.deps.propertyArchiver != nil && oldTariffID != pendingTariff.ID {
		if err := c.deps.propertyArchiver.ArchiveExcessProperties(ctx, tx, sub.UserID, pendingTariff.ActivePropertyLimit); err != nil {
			return fmt.Errorf("archive excess properties after scheduled downgrade: %w", err)
		}
	}
	return tx.Commit(ctx)
}
