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

// ProcessScheduledChanges applies due scheduled tariff changes whose target
// period is free (price 0 — in practice downgrades to the free basic tariff).
// Scheduled downgrades to a paid tariff are skipped: they stay due and are
// charged at apply time by ProcessRenewals, which runs right after this job in
// the billing worker tick. Returns the number of subscriptions processed;
// skipped paid downgrades are not counted because they remain in the selection
// until the renewal charge applies and clears them.
func (c *ScheduledChangeService) ProcessScheduledChanges(ctx context.Context, now time.Time) (int, error) {
	now = now.UTC()
	processed := 0
	for {
		subs, err := c.deps.subscriptions.ListPendingChanges(ctx, now, renewalBatchSize)
		if err != nil {
			return processed, fmt.Errorf("list subscriptions with pending change: %w", err)
		}
		if len(subs) == 0 {
			break
		}
		batchProcessed := 0
		batchSkipped := 0
		for _, sub := range subs {
			applied, err := c.applyScheduledChange(ctx, sub, now)
			if err != nil {
				c.deps.log.ErrorContext(ctx, "apply scheduled change failed",
					slog.String("subscription_id", sub.ID.String()),
					slog.String("user_id", sub.UserID.String()),
					slog.String("error", sanitize.Error(err)))
				continue
			}
			if !applied {
				// Paid downgrade left for the renewal charge; a skip is not
				// an error and the row is not counted as processed.
				batchSkipped++
				continue
			}
			batchProcessed++
		}
		processed += batchProcessed
		if len(subs) < renewalBatchSize {
			break
		}
		if batchProcessed == 0 {
			// A full batch with zero applies means every item stays in the
			// selection: failed items retry on the next tick, while skipped
			// paid downgrades wait for their renewal charge. Either way,
			// re-reading the same rows would spin, so stop here. Warn only
			// when nothing was skipped, i.e. the batch made no progress due
			// to failures alone.
			if batchSkipped == 0 {
				c.deps.log.WarnContext(ctx, "batch made no progress; deferring to next tick",
					slog.String("op", "apply scheduled changes"))
			}
			break
		}
	}
	return processed, nil
}

// applyScheduledChange applies a deferred downgrade to a free target period in
// a single short transaction with no provider call: it locks the subscription,
// resolves the pending tariff, applies the downgrade (which extends valid_until
// from now, enables auto-renew and clears the pending change), archives any
// properties that exceed the new limit, and commits.
//
// A downgrade to a PAID target period is not applied here: it must be charged
// at apply time, and charging belongs to the renewal path, where
// resolveRenewalTariffAndAmount resolves the pending tariff/period/price and a
// successful charge applies the change (a failed charge moves the subscription
// to grace). The subscription is left untouched — it stays in the
// pending-change selection until the renewal clears it — and the skip is
// reported as applied=false so the caller does not count it as processed.
func (c *ScheduledChangeService) applyScheduledChange(ctx context.Context, sub domain.Subscription, now time.Time) (bool, error) {
	tx, err := c.deps.beginner.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txSubscriptions, err := c.deps.subscriptions.WithTx(tx)
	if err != nil {
		return false, fmt.Errorf("bind subscriptions transaction: %w", err)
	}
	txTariffs, err := c.deps.tariffs.WithTx(tx)
	if err != nil {
		return false, fmt.Errorf("bind tariffs transaction: %w", err)
	}

	sub, err = txSubscriptions.GetByIDForUpdate(ctx, sub.ID)
	if err != nil {
		return false, fmt.Errorf("get subscription for update: %w", err)
	}
	// The scheduled-change preconditions (active status, matching pending
	// tariff/period, change due) are enforced by ApplyScheduledDowngrade. The
	// lookup below only resolves the pending tariff to pass into it; a nil
	// PendingTariffID skips the lookup and makes the domain method reject.
	var pendingTariff domain.Tariff
	if sub.PendingTariffID != nil {
		pendingTariff, err = txTariffs.GetByID(ctx, *sub.PendingTariffID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return false, ErrTariffNotFound
			}
			return false, fmt.Errorf("get pending tariff: %w", err)
		}
	}

	period := domain.PeriodMonth
	if sub.PendingPeriod != nil {
		period = *sub.PendingPeriod
	}

	// A paid target period must be charged at apply time; charging is the
	// renewal path's job. Leave the pending change in place and skip.
	price := pendingTariff.MonthlyPriceKopecks
	if period == domain.PeriodYear {
		price = pendingTariff.YearlyPriceKopecks
	}
	if price > 0 {
		c.deps.log.InfoContext(ctx, "paid scheduled downgrade left for renewal charge",
			slog.String("subscription_id", sub.ID.String()),
			slog.String("user_id", sub.UserID.String()))
		return false, nil
	}

	oldTariffID := sub.TariffID
	if err := sub.ApplyScheduledDowngrade(pendingTariff, period, now); err != nil {
		return false, fmt.Errorf("apply scheduled downgrade: %w", err)
	}
	if err := txSubscriptions.Update(ctx, sub); err != nil {
		return false, fmt.Errorf("update subscription after scheduled downgrade: %w", err)
	}
	if c.deps.propertyArchiver != nil && oldTariffID != pendingTariff.ID {
		if err := c.deps.propertyArchiver.ArchiveExcessProperties(ctx, tx, sub.UserID, pendingTariff.ActivePropertyLimit); err != nil {
			return false, fmt.Errorf("archive excess properties after scheduled downgrade: %w", err)
		}
	}
	if c.deps.recipientSlotEnforcer != nil && oldTariffID != pendingTariff.ID {
		if err := c.deps.recipientSlotEnforcer.EnforceRecipientLimit(ctx, tx, sub.UserID, "scheduled_downgrade"); err != nil {
			return false, fmt.Errorf("enforce recipient slot limit after scheduled downgrade: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit scheduled downgrade transaction: %w", err)
	}
	return true, nil
}
