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

// Distinct advisory-lock keys for the parallel DB-backed tick tests: they
// share one test database, and leader election would make concurrent ticks
// skip each other's work (a pre-existing flake surfaced by issue #337 runs).
const (
	billingSanitizeTestLockKey int64 = 0x7E59
	billingUpgradeTestLockKey  int64 = 0x7E60
	billingHoldTestLockKey     int64 = 0x7E61
)

type fakeBillingRunner struct {
	scheduledErr        error
	renewalErr          error
	graceRetryErr       error
	pendingUpgradeErr   error
	graceReminderErr    error
	expiredGraceErr     error
	scheduledCount      int
	renewalCount        int
	graceRetryCount     int
	pendingUpgradeCount int
	graceReminderCount  int
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

func (s *fakeBillingRunner) ProcessGraceRetries(context.Context, time.Time) (int, error) {
	return s.graceRetryCount, s.graceRetryErr
}

func (s *fakeBillingRunner) ProcessExpiredGrace(context.Context, time.Time) (int, error) {
	return s.expiredGraceCount, s.expiredGraceErr
}

func (s *fakeBillingRunner) ProcessGraceExpiryReminders(context.Context, time.Time) (int, error) {
	return s.graceReminderCount, s.graceReminderErr
}

func (s *fakeBillingRunner) ProcessPendingUpgradePayments(context.Context, time.Time) (int, error) {
	return s.pendingUpgradeCount, s.pendingUpgradeErr
}

func (s *fakeBillingRunner) ProcessExpiredPendingPayments(context.Context, time.Time) (int, error) {
	return 0, nil
}

func (s *fakeBillingRunner) ProcessExpiredBindingSessions(context.Context, time.Time) (int, error) {
	return 0, nil
}

func (s *fakeBillingRunner) ExportStuckPaymentMetrics(context.Context, time.Time) error {
	return nil
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

	sensitive := testSensitivePayload
	svc := &fakeBillingRunner{
		scheduledErr:    errors.New("scheduled changes failed: " + sensitive),
		renewalErr:      errors.New("renewals failed: " + sensitive),
		expiredGraceErr: errors.New("expired grace failed: " + sensitive),
	}

	w := NewBillingWorker(nil, nil, nil, pool, fakeClockForWorker{now: time.Now()}, time.Hour, logger)
	w.renewals = svc
	w.scheduled = svc
	w.lockKey = billingSanitizeTestLockKey

	if err := w.tick(context.Background()); err == nil {
		t.Fatal("expected tick to return errors")
	}

	logs := logBuf.String()
	forbidden := []string{
		testSecretToken,
		testCardNumber,
		testUserPhone,
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
	w := NewBillingWorker(nil, nil, nil, pool, fakeClockForWorker{now: time.Now()}, time.Hour, logger)
	w.renewals = svc
	w.scheduled = svc
	w.lockKey = billingUpgradeTestLockKey

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

	w := NewBillingWorker(nil, nil, nil, nil, fakeClockForWorker{now: time.Now()}, time.Hour, slog.New(slog.DiscardHandler))
	if err := w.tick(context.Background()); err == nil {
		t.Fatal("expected tick to error when pool is nil")
	}
}

// TestBillingWorker_TickOnce_RunsOnePass proves the admin-triggered tick of
// the time-travel rig (issue #665): TickOnce runs the same leader-elected
// pass synchronously — without the pool it fails loudly, and with the
// database it runs every phase once and answers nil.
func TestBillingWorker_TickOnce_RunsOnePass(t *testing.T) {
	t.Parallel()

	// The no-pool wiring fails the tick instead of pretending it ran.
	rigless := NewBillingWorker(nil, nil, nil, nil, fakeClockForWorker{now: time.Now()}, time.Hour, slog.New(slog.DiscardHandler))
	if err := rigless.TickOnce(context.Background()); err == nil {
		t.Fatal("expected TickOnce to error when pool is nil")
	}

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

	svc := &fakeBillingRunner{pendingUpgradeCount: 1}
	w := NewBillingWorker(nil, nil, nil, pool, fakeClockForWorker{now: time.Now()}, time.Hour, slog.New(slog.DiscardHandler))
	w.renewals = svc
	w.scheduled = svc
	w.lockKey = billingUpgradeTestLockKey

	if err := w.TickOnce(ctx); err != nil {
		t.Fatalf("TickOnce() error = %v", err)
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

	w := NewBillingWorker(nil, nil, nil, pool, fakeClockForWorker{now: time.Now()}, time.Hour, slog.New(slog.DiscardHandler))
	w.renewals = svc
	w.scheduled = svc
	w.lockKey = billingHoldTestLockKey

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
	if err := testConn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", w.lockKey).Scan(&acquired); err != nil {
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

	if err := testConn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", w.lockKey).Scan(&acquired); err != nil {
		t.Fatalf("try advisory lock after tick: %v", err)
	}
	if !acquired {
		t.Fatal("advisory lock should be released after tick completes")
	}
	// The unlock returns the advisory lock held only by this test; a failure
	// would leak it to sibling integration tests sharing the database.
	if _, err := testConn.Exec(ctx, "SELECT pg_advisory_unlock($1)", w.lockKey); err != nil {
		t.Errorf("unlock advisory lock after tick: %v", err)
	}
}
