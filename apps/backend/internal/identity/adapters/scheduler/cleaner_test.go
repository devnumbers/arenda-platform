package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// fakeDeleter is a function-field stub that satisfies all three deleter ports
// (ExpiredSessionDeleter, ExpiredLoginCodeDeleter, StaleAttemptDeleter) via
// structural typing. DeleteExpiredBefore and DeleteStaleBefore both increment
// the same counter and return the same configured count and error.
type fakeDeleter struct {
	err     error
	deleted int64
	calls   atomic.Int32
}

func (d *fakeDeleter) DeleteExpiredBefore(_ context.Context, _ time.Time) (int64, error) {
	d.calls.Add(1)
	return d.deleted, d.err
}

func (d *fakeDeleter) DeleteStaleBefore(_ context.Context, _ time.Time) (int64, error) {
	d.calls.Add(1)
	return d.deleted, d.err
}

// capturingDeleter captures the "before" timestamp passed to each delete call
// so a test can verify the retention window is derived correctly from the clock.
type capturingDeleter struct {
	before time.Time
}

func (d *capturingDeleter) DeleteExpiredBefore(_ context.Context, before time.Time) (int64, error) {
	d.before = before
	return 0, nil
}

func (d *capturingDeleter) DeleteStaleBefore(_ context.Context, before time.Time) (int64, error) {
	d.before = before
	return 0, nil
}

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

// recordHandler captures slog records so tests can assert on log attributes.
type recordHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *recordHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r.Clone())
	return nil
}

func (h *recordHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *recordHandler) WithGroup(string) slog.Handler { return h }

// int64Attr returns the value of the named Int64 attribute on the last record
// with the given message.
func (h *recordHandler) int64Attr(message, key string) (int64, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, r := range slices.Backward(h.records) {
		if r.Message != message {
			continue
		}
		var value int64
		found := false
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == key {
				value = a.Value.Int64()
				found = true
				return false
			}
			return true
		})
		return value, found
	}
	return 0, false
}

func TestNewCleaner_NilLoggerDefaultsToSlogDefault(t *testing.T) {
	t.Parallel()
	c := NewCleaner(&fakeDeleter{}, &fakeDeleter{}, &fakeDeleter{}, &fakeClock{},
		time.Minute, time.Hour, nil)
	if c == nil {
		t.Fatal("NewCleaner returned nil")
	}
	if c.logger == nil {
		t.Fatal("logger is nil, want slog.Default()")
	}
}

func TestCleaner_Clean_CallsAllDeletersWithRetentionWindow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	retention := 24 * time.Hour
	wantBefore := now.Add(-retention)

	// Each deleter captures the "before" value it receives.
	codes := &capturingDeleter{}
	sessions := &capturingDeleter{}
	attempts := &capturingDeleter{}
	c := NewCleaner(sessions, codes, attempts, &fakeClock{now: now},
		time.Minute, retention, slog.New(slog.DiscardHandler))

	c.clean(t.Context())

	// Every deleter receives the same "before" timestamp: now - retention.
	for name, d := range map[string]*capturingDeleter{"codes": codes, "sessions": sessions, "attempts": attempts} {
		if !d.before.Equal(wantBefore) {
			t.Errorf("%s before = %v, want %v (now - retention)", name, d.before, wantBefore)
		}
	}
}

func TestCleaner_Clean_LogsDeletedCountsPerTable(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	codes := &fakeDeleter{deleted: 5}
	sessions := &fakeDeleter{deleted: 7}
	attempts := &fakeDeleter{deleted: 9}
	h := &recordHandler{}
	c := NewCleaner(sessions, codes, attempts, &fakeClock{now: now},
		time.Minute, time.Hour, slog.New(h))

	c.clean(t.Context())

	for key, want := range map[string]int64{
		"login_codes_deleted":    5,
		"sessions_deleted":       7,
		"login_attempts_deleted": 9,
	} {
		got, ok := h.int64Attr("cleanup completed", key)
		if !ok {
			t.Errorf("log record %q has no %q attribute", "cleanup completed", key)
			continue
		}
		if got != want {
			t.Errorf("%s = %d, want %d", key, got, want)
		}
	}
}

func TestCleaner_Clean_DelegateErrorDoesNotAbortRemainingDeleters(t *testing.T) {
	t.Parallel()
	dbErr := errors.New("db down")
	// Sessions fails; codes and attempts must still be called despite the error.
	sessions := &fakeDeleter{err: dbErr}
	codes := &fakeDeleter{}
	attempts := &fakeDeleter{}
	c := NewCleaner(sessions, codes, attempts, &fakeClock{now: time.Now()},
		time.Minute, time.Hour, slog.New(slog.DiscardHandler))

	// clean itself must not return the error (it is logged inside cleanExpired).
	c.clean(t.Context())

	if sessions.calls.Load() != 1 {
		t.Fatalf("sessions calls = %d, want 1 despite error", sessions.calls.Load())
	}
	if codes.calls.Load() != 1 {
		t.Fatalf("codes calls = %d, want 1 (must run even after sessions error)", codes.calls.Load())
	}
	if attempts.calls.Load() != 1 {
		t.Fatalf("attempts calls = %d, want 1 (must run even after sessions error)", attempts.calls.Load())
	}
}

func TestCleaner_CleanExpired_SwallowsContextCanceled(t *testing.T) {
	t.Parallel()
	sessions := &cancelDeleter{}
	c := NewCleaner(sessions, &fakeDeleter{}, &fakeDeleter{}, &fakeClock{now: time.Now()},
		time.Minute, time.Hour, slog.New(slog.DiscardHandler))

	c.clean(t.Context())

	if sessions.calls.Load() != 1 {
		t.Fatalf("sessions calls = %d, want 1", sessions.calls.Load())
	}
}

// cancelDeleter returns context.Canceled to exercise the swallow path.
type cancelDeleter struct{ calls atomic.Int32 }

func (d *cancelDeleter) DeleteExpiredBefore(_ context.Context, _ time.Time) (int64, error) {
	d.calls.Add(1)
	return 0, context.Canceled
}

func TestCleaner_Run_CleansImmediatelyBeforeFirstTick(t *testing.T) {
	t.Parallel()
	sessions := &fakeDeleter{}
	codes := &fakeDeleter{}
	attempts := &fakeDeleter{}
	// The interval is an hour, so no ticker fire can explain a clean cycle:
	// only the startup cycle can have run.
	c := NewCleaner(sessions, codes, attempts, &fakeClock{now: time.Now()},
		time.Hour, time.Hour, slog.New(slog.DiscardHandler))

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		c.Run(ctx)
		close(done)
	}()
	cancel()
	<-done

	for name, d := range map[string]*fakeDeleter{"codes": codes, "sessions": sessions, "attempts": attempts} {
		if calls := d.calls.Load(); calls != 1 {
			t.Fatalf("%s calls = %d, want 1 (startup cycle before any tick)", name, calls)
		}
	}
}

func TestCleaner_Run_StopsOnContextCancel(t *testing.T) {
	t.Parallel()
	sessions := &fakeDeleter{}
	codes := &fakeDeleter{}
	attempts := &fakeDeleter{}
	interval := 10 * time.Millisecond
	c := NewCleaner(sessions, codes, attempts, &fakeClock{now: time.Now()},
		interval, time.Hour, slog.New(slog.DiscardHandler))

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		c.Run(ctx)
		close(done)
	}()

	// Let at least one tick fire beyond the startup cycle.
	time.Sleep(interval * 3)
	cancel()
	select {
	case <-done:
		// Run returned on cancel — success.
	case <-time.After(time.Second):
		t.Fatal("Run did not return after context cancel")
	}

	// The startup cycle plus at least one tick should have run.
	if codes.calls.Load() < 2 || sessions.calls.Load() < 2 || attempts.calls.Load() < 2 {
		t.Fatal("no tick cycle ran beyond the startup cycle before cancel")
	}
}

// Compile-time guard that fakeDeleter satisfies both deleter ports.
var (
	_ ExpiredSessionDeleter   = (*fakeDeleter)(nil)
	_ ExpiredLoginCodeDeleter = (*fakeDeleter)(nil)
	_ StaleAttemptDeleter     = (*fakeDeleter)(nil)
	_ clock.Clock             = (*fakeClock)(nil)
)
