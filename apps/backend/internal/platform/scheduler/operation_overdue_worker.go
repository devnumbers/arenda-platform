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
	interval         time.Duration
	batchSize        int
	logger           *slog.Logger
	tzResolver       sharedtz.OwnerTimezoneResolver
}

// NewOperationOverdueWorker creates a new operation overdue worker.
func NewOperationOverdueWorker(
	operationService *leasesapp.OperationService,
	clock clock.Clock,
	interval time.Duration,
	batchSize int,
	logger *slog.Logger,
	tzResolver sharedtz.OwnerTimezoneResolver,
) *OperationOverdueWorker {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &OperationOverdueWorker{
		operationService: operationService,
		clock:            clock,
		interval:         interval,
		batchSize:        batchSize,
		logger:           logger,
		tzResolver:       tzResolver,
	}
}

// Run starts the overdue detection loop. It stops when the provided context is cancelled.
func (w *OperationOverdueWorker) Run(ctx context.Context) {
	w.logger.InfoContext(ctx, "operation overdue worker started", "interval", w.interval.String())

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	if err := w.tick(ctx); err != nil {
		w.logger.ErrorContext(ctx, "operation overdue tick failed", "error", sanitize.Error(err))
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.tick(ctx); err != nil {
				w.logger.ErrorContext(ctx, "operation overdue tick failed", "error", sanitize.Error(err))
			}
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
