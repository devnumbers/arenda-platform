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

type fakeBillingRunner struct {
	scheduledErr        error
	renewalErr          error
	pendingUpgradeErr   error
	expiredGraceErr     error
	scheduledCount      int
	renewalCount        int
	pendingUpgradeCount int
	expiredGraceCount   int
	scheduledStarted    chan struct{}
	scheduledDelay      <-chan struct{}
}

func (s *fakeBillingRunner) ProcessScheduledChanges(ctx context.Context, _ time.Time) (int, error) {
	if s.scheduledStarted != nil {
		select {
		case s.scheduledStarted <- struct{}{}:
		default:
		}
	}
	if s.scheduledDelay != nil {
		select {
		case <-s.scheduledDelay:
		case <-ctx.Done():
		}
	}
	return s.scheduledCount, s.scheduledErr
}

func (s *fakeBillingRunner) ProcessRenewals(context.Context, time.Time) (int, error) {
	return s.renewalCount, s.renewalErr
}

func (s *fakeBillingRunner) ProcessExpiredGrace(context.Context, time.Time) (int, error) {
	return s.expiredGraceCount, s.expiredGraceErr
}

func (s *fakeBillingRunner) ProcessPendingUpgradePayments(context.Context, time.Time) (int, error) {
	return s.pendingUpgradeCount, s.pendingUpgradeErr
}

func TestBillingWorker_Tick_SanitizesServiceErrors(t *testing.T) {
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

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	sensitive := "token=secret123 card 1234-5678-9012-3456 phone +79991234567"
	svc := &fakeBillingRunner{
		scheduledErr:    errors.New("scheduled changes failed: " + sensitive),
		renewalErr:      errors.New("renewals failed: " + sensitive),
		expiredGraceErr: errors.New("expired grace failed: " + sensitive),
	}

	w := NewBillingWorker(nil, nil, pool, fakeClockForWorker{now: time.Now()}, time.Hour, logger)
	w.renewals = svc
	w.scheduled = svc

	if err := w.tick(context.Background()); err == nil {
		t.Fatal("expected tick to return errors")
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
	for _, msg := range []string{
		"billing worker scheduled changes processing failed",
		"billing worker renewal processing failed",
		"billing worker expired grace processing failed",
	} {
		if !strings.Contains(logs, msg) {
			t.Errorf("expected log message %q, got:\n%s", msg, logs)
		}
	}
}

func TestBillingWorkerTick_CallsProcessPendingUpgradePayments(t *testing.T) {
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

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	svc := &fakeBillingRunner{pendingUpgradeCount: 3}
	w := NewBillingWorker(nil, nil, pool, fakeClockForWorker{now: time.Now()}, time.Hour, logger)
	w.renewals = svc
	w.scheduled = svc

	if err := w.tick(context.Background()); err != nil {
		t.Fatalf("tick error: %v", err)
	}

	logs := logBuf.String()
	if !strings.Contains(logs, "billing worker finalized pending upgrade payments") {
		t.Errorf("expected pending upgrade processing log, got:\n%s", logs)
	}
	if !strings.Contains(logs, "count=3") {
		t.Errorf("expected count=3 in logs, got:\n%s", logs)
	}
}

func TestBillingWorker_Tick_RequiresPool(t *testing.T) {
	t.Parallel()

	w := NewBillingWorker(nil, nil, nil, fakeClockForWorker{now: time.Now()}, time.Hour, slog.New(slog.DiscardHandler))
	if err := w.tick(context.Background()); err == nil {
		t.Fatal("expected tick to error when pool is nil")
	}
}

func TestBillingWorker_Tick_HoldsAdvisoryLockDuringWork(t *testing.T) {
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

	scheduledStarted := make(chan struct{}, 1)
	scheduledDelay := make(chan struct{})

	svc := &fakeBillingRunner{
		scheduledCount:   1,
		scheduledStarted: scheduledStarted,
		scheduledDelay:   scheduledDelay,
	}

	w := NewBillingWorker(nil, nil, pool, fakeClockForWorker{now: time.Now()}, time.Hour, slog.New(slog.DiscardHandler))
	w.renewals = svc
	w.scheduled = svc

	tickDone := make(chan error, 1)
	go func() {
		tickDone <- w.tick(ctx)
	}()

	select {
	case <-scheduledStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for scheduled changes to start")
	}

	testConn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire test connection: %v", err)
	}
	defer testConn.Release()

	var acquired bool
	if err := testConn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", billingWorkerLockKey).Scan(&acquired); err != nil {
		t.Fatalf("try advisory lock during work: %v", err)
	}
	if acquired {
		t.Fatal("advisory lock should be held while billing work is running")
	}

	close(scheduledDelay)

	select {
	case err := <-tickDone:
		if err != nil {
			t.Fatalf("tick returned unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for tick to complete")
	}

	if err := testConn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", billingWorkerLockKey).Scan(&acquired); err != nil {
		t.Fatalf("try advisory lock after tick: %v", err)
	}
	if !acquired {
		t.Fatal("advisory lock should be released after tick completes")
	}
	_, _ = testConn.Exec(ctx, "SELECT pg_advisory_unlock($1)", billingWorkerLockKey)
}
