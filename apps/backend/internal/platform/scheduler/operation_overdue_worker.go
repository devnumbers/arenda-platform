package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	leasesapp "github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	leasesdomain "github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/timeutil"
	sharedtz "github.com/nambers/arenda-planform/apps/backend/internal/shared/tzresolver"
)

// maxOverdueBatches caps the number of batches processed per tick to bound the
// amount of work done in a single tick.
const maxOverdueBatches = 10

// operationService is the subset of the leases application service used by the
// overdue worker. It keeps the worker decoupled from the concrete service type.
type operationService interface {
	ListAllOverdueCandidates(ctx context.Context, asOf time.Time, limit int) ([]leasesdomain.Operation, error)
	ProcessOverdueOperation(ctx context.Context, ownerID, operationID uuid.UUID, asOf time.Time) (bool, error)
}

// OperationOverdueWorker scans pending operations whose operation_date is in the
// past and transitions them to overdue, scheduling an overdue reminder once per
// operation.
type OperationOverdueWorker struct {
	operationService operationService
	clock            clock.Clock
	batchSize        int
	logger           *slog.Logger
	tzResolver       sharedtz.OwnerTimezoneResolver
	// nextRun maps "now" to the next scheduled scan instant. Production
	// default is nextDailyRun; tests override it to shrink the delay.
	nextRun func(time.Time) time.Time
}

// NewOperationOverdueWorker creates a new operation overdue worker. The scan
// schedule is owned by nextDailyRun — daily at 00:00 UTC — not by the caller.
func NewOperationOverdueWorker(
	operationService *leasesapp.OperationService,
	clk clock.Clock,
	batchSize int,
	logger *slog.Logger,
	tzResolver sharedtz.OwnerTimezoneResolver,
) *OperationOverdueWorker {
	if batchSize <= 0 {
		batchSize = 100
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &OperationOverdueWorker{
		operationService: operationService,
		clock:            clk,
		batchSize:        batchSize,
		logger:           logger,
		tzResolver:       tzResolver,
		nextRun:          nextDailyRun,
	}
}

// nextDailyRun returns the next 00:00 UTC strictly after now. The anchor is
// UTC midnight because overdue candidacy is date-based on the server's UTC
// clock (ListAllOverdueCandidates compares operation_date against the UTC
// date): new candidates become visible exactly at UTC midnight, so an earlier
// anchor would find nothing and a later one only delays the scan. Owner-local
// reminders still dispatch at their scheduled 10:00 local time.
func nextDailyRun(now time.Time) time.Time {
	n := now.UTC()
	next := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	if !next.After(n) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

// Run starts the overdue detection loop: one scan per day, anchored to 00:00
// UTC. It stops when the provided context is cancelled. There is no scan at
// startup by design — the daily anchor owns the schedule, so deploys do not
// trigger work; a run missed while the process is down waits for the next
// anchor.
func (w *OperationOverdueWorker) Run(ctx context.Context) {
	w.logger.InfoContext(ctx, "operation overdue worker started", "schedule", "daily at 00:00 UTC")

	now := w.clock.Now()
	timer := time.NewTimer(w.nextRun(now).Sub(now))
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if err := w.tick(ctx); err != nil {
				w.logger.ErrorContext(ctx, "operation overdue tick failed", "error", sanitize.Error(err))
			}
			now = w.clock.Now()
			timer.Reset(w.nextRun(now).Sub(now))
		}
	}
}

func (w *OperationOverdueWorker) tick(ctx context.Context) error {
	asOf := timeutil.Date(w.clock.Now())
	for range maxOverdueBatches {
		ops, err := w.operationService.ListAllOverdueCandidates(ctx, asOf, w.batchSize)
		if err != nil {
			return fmt.Errorf("list all overdue candidates: %w", err)
		}
		if len(ops) == 0 {
			break
		}

		for _, op := range ops {
			if err := w.process(ctx, op); err != nil {
				w.logger.ErrorContext(ctx, "process overdue operation failed",
					"operation_id", op.ID,
					"owner_id", op.OwnerID,
					"error", sanitize.Error(err),
				)
			}
		}

		if len(ops) < w.batchSize {
			break
		}
	}
	return nil
}

func (w *OperationOverdueWorker) process(ctx context.Context, op leasesdomain.Operation) error {
	loc, err := w.tzResolver.Resolve(ctx, op.OwnerID)
	if err != nil {
		return fmt.Errorf("resolve owner timezone: %w", err)
	}
	asOf := timeutil.DateIn(w.clock.Now(), loc)
	changed, err := w.operationService.ProcessOverdueOperation(ctx, op.OwnerID, op.ID, asOf)
	if err != nil {
		return fmt.Errorf("process overdue operation: %w", err)
	}
	if !changed {
		return nil
	}

	w.logger.InfoContext(ctx, "operation overdue reminder scheduled",
		"operation_id", op.ID,
		"owner_id", op.OwnerID,
	)
	return nil
}

// Compile-time interface check.
var _ operationService = (*leasesapp.OperationService)(nil)
