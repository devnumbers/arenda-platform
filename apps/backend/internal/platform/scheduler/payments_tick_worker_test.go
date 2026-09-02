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

// fakePaymentsTicker is a scheduler.PaymentsTicker stub returning a canned
// error and recording the sweep instants it was called with.
type fakePaymentsTicker struct {
	err   error
	calls []time.Time
}

func (f *fakePaymentsTicker) RunZoneTicks(_ context.Context, now time.Time) error {
	f.calls = append(f.calls, now)
	return f.err
}

type delayedPaymentsTicker struct {
	fakePaymentsTicker
	started chan struct{}
	delay   <-chan struct{}
}

func (f *delayedPaymentsTicker) RunZoneTicks(ctx context.Context, now time.Time) error {
	select {
	case f.started <- struct{}{}:
	default:
	}
	select {
	case <-f.delay:
	case <-ctx.Done():
	}
	return f.fakePaymentsTicker.RunZoneTicks(ctx, now)
}

func TestPaymentsTickWorker_Tick_LogsCompletion(t *testing.T) {
	t.Parallel()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	now := time.Date(2026, 8, 25, 20, 0, 0, 0, time.UTC)
	svc := &fakePaymentsTicker{}
	w := NewPaymentsTickWorker(svc, nil, fakeClockForWorker{now: now}, time.Hour, logger)

	if err := w.tick(context.Background()); err != nil {
		t.Fatalf("tick error: %v", err)
	}

	logs := logBuf.String()
	if !strings.Contains(logs, `worker="payments tick"`) || !strings.Contains(logs, "zone tick sweep completed") {
		t.Errorf("expected completion log for the payments worker, got:\n%s", logs)
	}
	// The sweep runs on the clock's instant, not on time.Now — the zone's
	// "today" must be reproducible.
	if len(svc.calls) != 1 || !svc.calls[0].Equal(now) {
		t.Errorf("sweep called with %v, want exactly one call at %v", svc.calls, now)
	}
}

func TestPaymentsTickWorker_Tick_SanitizesServiceErrors(t *testing.T) {
	t.Parallel()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	sensitive := testSensitivePayload
	svc := &fakePaymentsTicker{err: errors.New("zone sweep failed: " + sensitive)}
	w := NewPaymentsTickWorker(svc, nil, fakeClockForWorker{now: time.Now()}, time.Hour, logger)

	if err := w.tick(context.Background()); err == nil {
		t.Fatal("expected tick to return error")
	}

	logs := logBuf.String()
	for _, s := range []string{testSecretToken, testCardNumber, testUserPhone} {
		if strings.Contains(logs, s) {
			t.Errorf("log contains sensitive substring %q:\n%s", s, logs)
		}
	}
	if !strings.Contains(logs, "zone tick sweep failed") || !strings.Contains(logs, `worker="payments tick"`) {
		t.Errorf("expected sweep failed log for the payments worker, got:\n%s", logs)
	}
}

func TestPaymentsTickWorker_Tick_HoldsAdvisoryLockDuringWork(t *testing.T) {
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

	w := NewPaymentsTickWorker(&delayedPaymentsTicker{
		fakePaymentsTicker: fakePaymentsTicker{},
		started:            started,
		delay:              delay,
	}, pool, fakeClockForWorker{now: time.Now()}, time.Hour, slog.New(slog.DiscardHandler))

	tickDone := make(chan error, 1)
	go func() {
		tickDone <- w.tick(ctx)
	}()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the sweep to start")
	}

	testConn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire test connection: %v", err)
	}
	defer testConn.Release()

	var acquired bool
	if err := testConn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", paymentsTickWorkerLockKey).Scan(&acquired); err != nil {
		t.Fatalf("try advisory lock during work: %v", err)
	}
	if acquired {
		t.Fatal("advisory lock should be held while the payments tick sweep is running")
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

	if err := testConn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", paymentsTickWorkerLockKey).Scan(&acquired); err != nil {
		t.Fatalf("try advisory lock after tick: %v", err)
	}
	if !acquired {
		t.Fatal("advisory lock should be released after tick completes")
	}
	// The unlock returns the advisory lock held only by this test; a failure
	// would leak it to sibling integration tests sharing the database.
	if _, err := testConn.Exec(ctx, "SELECT pg_advisory_unlock($1)", paymentsTickWorkerLockKey); err != nil {
		t.Errorf("unlock advisory lock after tick: %v", err)
	}
}

func TestPaymentsTickWorker_Run_StopsOnContextCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	w := NewPaymentsTickWorker(&fakePaymentsTicker{}, nil, fakeClockForWorker{now: time.Now()}, time.Hour, slog.New(slog.DiscardHandler))

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
