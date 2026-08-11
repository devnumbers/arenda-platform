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

// maxReconciliationBatches limits the number of batches processed per tick to
// prevent a single failing lease from causing the worker to spin indefinitely.
const maxReconciliationBatches = 10

// leaseService is the subset of the leases application service used by the
// worker. It keeps the worker decoupled from the concrete service type.
type leaseService interface {
	ListOpenLeasesWithPastEndDate(ctx context.Context, asOf time.Time, limit int) ([]leasesdomain.Lease, error)
	ReconcileRequiresAction(ctx context.Context, leaseID uuid.UUID, asOf time.Time) error
}

// LeaseReconciliationWorker scans open leases with an end date in the past and
// ensures their status moves to requires_action and a requires_action reminder
// exists.
type LeaseReconciliationWorker struct {
	leaseService leaseService
	clock        clock.Clock
	interval     time.Duration
	batchSize    int
	logger       *slog.Logger
	tzResolver   sharedtz.OwnerTimezoneResolver
}

// NewLeaseReconciliationWorker creates a new lease reconciliation worker.
func NewLeaseReconciliationWorker(
	leaseService *leasesapp.LeaseService,
	clock clock.Clock,
	interval time.Duration,
	batchSize int,
	logger *slog.Logger,
	tzResolver sharedtz.OwnerTimezoneResolver,
) *LeaseReconciliationWorker {
	if logger == nil {
		logger = slog.Default()
	}
	return &LeaseReconciliationWorker{
		leaseService: leaseService,
		clock:        clock,
		interval:     interval,
		batchSize:    batchSize,
		logger:       logger,
		tzResolver:   tzResolver,
	}
}

// Run starts the reconciliation loop. It stops when the provided context is cancelled.
func (w *LeaseReconciliationWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	if err := w.tick(ctx); err != nil {
		w.logger.ErrorContext(ctx, "lease reconciliation tick failed", "error", sanitize.Error(err))
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.tick(ctx); err != nil {
				w.logger.ErrorContext(ctx, "lease reconciliation tick failed", "error", sanitize.Error(err))
			}
		}
	}
}

func (w *LeaseReconciliationWorker) tick(ctx context.Context) error {
	asOf := timeutil.Date(w.clock.Now())
	for range maxReconciliationBatches {
		leases, err := w.leaseService.ListOpenLeasesWithPastEndDate(ctx, asOf, w.batchSize)
		if err != nil {
			return fmt.Errorf("list open leases with past end date: %w", err)
		}
		if len(leases) == 0 {
			break
		}

		for _, lease := range leases {
			if err := w.reconcile(ctx, lease); err != nil {
				w.logger.ErrorContext(ctx, "reconcile lease failed", "lease_id", lease.ID, "error", sanitize.Error(err))
			}
		}

		if len(leases) < w.batchSize {
			break
		}
	}
	return nil
}

func (w *LeaseReconciliationWorker) reconcile(ctx context.Context, lease leasesdomain.Lease) error {
	loc, err := w.tzResolver.Resolve(ctx, lease.OwnerID)
	if err != nil {
		return fmt.Errorf("resolve owner timezone: %w", err)
	}
	asOf := timeutil.DateIn(w.clock.Now(), loc)
	return w.leaseService.ReconcileRequiresAction(ctx, lease.ID, asOf)
}
