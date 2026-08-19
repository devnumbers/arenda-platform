package scheduler

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
)

// fakePaymentReconciliationService is a scheduler.PaymentReconciler stub
// returning canned results.
type fakePaymentReconciliationService struct {
	count       int
	err         error
	refundCount int
	refundErr   error
}

func (s *fakePaymentReconciliationService) ReconcilePendingPayments(context.Context, time.Time) (int, error) {
	return s.count, s.err
}

func (s *fakePaymentReconciliationService) ReconcileStaleRefunds(context.Context, time.Time) (int, error) {
	return s.refundCount, s.refundErr
}

func TestPaymentReconciliationWorker_Tick_LogsCount(t *testing.T) {
	t.Parallel()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	svc := &fakePaymentReconciliationService{count: 7}
	w := NewPaymentReconciliationWorker(nil, nil, fakeClockForWorker{now: time.Now()}, 5*time.Minute, logger)
	w.payments = svc

	if err := w.tick(context.Background()); err != nil {
		t.Fatalf("tick error: %v", err)
	}

	logs := logBuf.String()
	if !strings.Contains(logs, "payment reconciliation processed pending payments") {
		t.Errorf("expected processed log, got:\n%s", logs)
	}
	if !strings.Contains(logs, "count=7") {
		t.Errorf("expected count=7 in logs, got:\n%s", logs)
	}
}

func TestPaymentReconciliationWorker_Tick_SanitizesServiceErrors(t *testing.T) {
	t.Parallel()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	sensitive := "token=secret123 card 1234-5678-9012-3456 phone +79991234567"
	svc := &fakePaymentReconciliationService{err: errors.New("reconcile failed: " + sensitive)}
	w := NewPaymentReconciliationWorker(nil, nil, fakeClockForWorker{now: time.Now()}, 5*time.Minute, logger)
	w.payments = svc

	if err := w.tick(context.Background()); err == nil {
		t.Fatal("expected tick to return error")
	}

	logs := logBuf.String()
	forbidden := []string{
		"token=secret123",
		"1234-5678-9012-3456",
		"+79991234567",
	}
	for _, s := range forbidden {
		if strings.Contains(logs, s) {
			t.Errorf("log contains sensitive substring %q:\n%s", s, logs)
		}
	}
	if !strings.Contains(logs, "payment reconciliation failed") {
		t.Errorf("expected reconciliation failed log, got:\n%s", logs)
	}
}

func TestPaymentReconciliationWorker_Tick_HoldsAdvisoryLockDuringWork(t *testing.T) {
	t.Parallel()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := database.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	defer pool.Close()

	started := make(chan struct{}, 1)
	delay := make(chan struct{})

	svc := &fakePaymentReconciliationService{
		count: 1,
	}

	w := NewPaymentReconciliationWorker(nil, pool, fakeClockForWorker{now: time.Now()}, 5*time.Minute, slog.New(slog.DiscardHandler))
	w.payments = &delayedPaymentReconciliationService{
		fakePaymentReconciliationService: svc,
		started:                          started,
		delay:                            delay,
	}

	tickDone := make(chan error, 1)
	go func() {
		tickDone <- w.tick(ctx)
	}()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for reconcile to start")
	}

	testConn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire test connection: %v", err)
	}
	defer testConn.Release()

	var acquired bool
	if err := testConn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", paymentReconciliationWorkerLockKey).Scan(&acquired); err != nil {
		t.Fatalf("try advisory lock during work: %v", err)
	}
	if acquired {
		t.Fatal("advisory lock should be held while payment reconciliation work is running")
	}

	close(delay)

	select {
	case err := <-tickDone:
		if err != nil {
			t.Fatalf("tick returned unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for tick to complete")
	}

	if err := testConn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", paymentReconciliationWorkerLockKey).Scan(&acquired); err != nil {
		t.Fatalf("try advisory lock after tick: %v", err)
	}
	if !acquired {
		t.Fatal("advisory lock should be released after tick completes")
	}
	// The unlock returns the advisory lock held only by this test; a failure
	// would leak it to sibling integration tests sharing the database.
	if _, err := testConn.Exec(ctx, "SELECT pg_advisory_unlock($1)", paymentReconciliationWorkerLockKey); err != nil {
		t.Errorf("unlock advisory lock after tick: %v", err)
	}
}

type delayedPaymentReconciliationService struct {
	*fakePaymentReconciliationService
	started chan struct{}
	delay   <-chan struct{}
}

func TestPaymentReconciliationWorker_Run_StopsOnContextCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	svc := &fakePaymentReconciliationService{count: 0}
	w := NewPaymentReconciliationWorker(nil, nil, fakeClockForWorker{now: time.Now()}, time.Hour, slog.New(slog.DiscardHandler))
	w.payments = svc

	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}
}

func (s *delayedPaymentReconciliationService) ReconcilePendingPayments(ctx context.Context, now time.Time) (int, error) {
	if s.started != nil {
		select {
		case s.started <- struct{}{}:
		default:
		}
	}
	if s.delay != nil {
		select {
		case <-s.delay:
		case <-ctx.Done():
		}
	}
	return s.fakePaymentReconciliationService.ReconcilePendingPayments(ctx, now)
}
