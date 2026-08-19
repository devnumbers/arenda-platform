package scheduler

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database"
)

// Advisory-lock keys reserved for these tests; distinct so the parallel
// DB-backed tests do not elect each other's leader. Must not collide with the
// production worker keys.
const (
	workerLoopRunTestLockKey  int64 = 0x7E57
	workerLoopSkipTestLockKey int64 = 0x7E58
)

func TestRunTickerLoop_RunsImmediateTickAndStopsOnCancel(t *testing.T) {
	t.Parallel()

	// Buffered so the immediate tick never blocks the loop under test.
	ticks := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		runTickerLoop(ctx, "test", time.Hour, slog.New(slog.DiscardHandler), func(context.Context) error {
			select {
			case ticks <- struct{}{}:
			default:
			}
			return nil
		})
	}()

	select {
	case <-ticks:
	case <-time.After(5 * time.Second):
		t.Fatal("immediate tick did not run")
	}

	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("loop did not stop after context cancellation")
	}
}

func TestRunTickerLoop_LoggedTickErrorDoesNotStopLoop(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	// Buffered enough for the success ticks observed before cancellation;
	// sends are non-blocking so a late tick cannot stall the loop.
	called := make(chan struct{}, 4)
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		runTickerLoop(ctx, "test", time.Millisecond, logger, func(context.Context) error {
			if calls.Add(1) == 1 {
				return errors.New("boom")
			}
			select {
			case called <- struct{}{}:
			default:
			}
			return nil
		})
	}()

	select {
	case <-called:
	case <-time.After(5 * time.Second):
		t.Fatal("loop stopped after the first tick failed")
	}
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("loop did not stop after context cancellation")
	}

	logs := logBuf.String()
	for _, want := range []string{
		"worker started",
		"worker=test",
		"worker tick failed",
		"boom",
	} {
		if !strings.Contains(logs, want) {
			t.Errorf("expected log to contain %q, got:\n%s", want, logs)
		}
	}
}

func TestWithAdvisoryTickLock_RunsWorkAndReleasesLock(t *testing.T) {
	t.Parallel()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx := t.Context()
	pool, err := database.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	defer pool.Close()

	sentinel := errors.New("reconcile boom")
	var ran atomic.Bool
	err = withAdvisoryTickLock(ctx, pool, workerLoopRunTestLockKey, "test", slog.New(slog.DiscardHandler), func(context.Context) error {
		ran.Store(true)
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected work error to pass through, got %v", err)
	}
	if !ran.Load() {
		t.Fatal("expected work to run under the lock")
	}

	probe, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire probe connection: %v", err)
	}
	defer probe.Release()
	var acquired bool
	if err := probe.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", workerLoopRunTestLockKey).Scan(&acquired); err != nil {
		t.Fatalf("try advisory lock after work: %v", err)
	}
	if !acquired {
		t.Fatal("advisory lock should be released after work returns")
	}
	// The unlock returns the advisory lock held only by this test; a failure
	// would leak it to sibling integration tests sharing the database.
	if _, err := probe.Exec(ctx, "SELECT pg_advisory_unlock($1)", workerLoopRunTestLockKey); err != nil {
		t.Errorf("unlock advisory lock after work: %v", err)
	}
}

func TestWithAdvisoryTickLock_SkipsWhenAnotherInstanceHoldsLock(t *testing.T) {
	t.Parallel()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx := t.Context()
	pool, err := database.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	defer pool.Close()

	holder, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire holder connection: %v", err)
	}
	defer holder.Release()
	if _, err := holder.Exec(ctx, "SELECT pg_advisory_lock($1)", workerLoopSkipTestLockKey); err != nil {
		t.Fatalf("hold advisory lock: %v", err)
	}
	defer func() {
		// A leaked session-level advisory lock would poison sibling tests on
		// the shared integration database.
		if _, err := holder.Exec(ctx, "SELECT pg_advisory_unlock($1)", workerLoopSkipTestLockKey); err != nil {
			t.Errorf("unlock advisory lock: %v", err)
		}
	}()

	var logBuf bytes.Buffer
	var ran atomic.Bool
	err = withAdvisoryTickLock(ctx, pool, workerLoopSkipTestLockKey, "test", slog.New(slog.NewTextHandler(&logBuf, nil)), func(context.Context) error {
		ran.Store(true)
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error when the lock is held elsewhere, got %v", err)
	}
	if ran.Load() {
		t.Fatal("work must not run when another instance holds the lock")
	}
	if !strings.Contains(logBuf.String(), "worker tick skipped, another instance holds the lock") {
		t.Errorf("expected skip log, got:\n%s", logBuf.String())
	}
}
