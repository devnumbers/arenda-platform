package scheduler

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	leasesdomain "github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
)

// fakeOverdueService counts scans; every scan returns an empty candidate list.
type fakeOverdueService struct {
	scans atomic.Int32
}

func (s *fakeOverdueService) ListAllOverdueCandidates(context.Context, time.Time, int) ([]leasesdomain.Operation, error) {
	s.scans.Add(1)
	return nil, nil
}

func (s *fakeOverdueService) ProcessOverdueOperation(context.Context, uuid.UUID, uuid.UUID, time.Time) (bool, error) {
	return false, nil
}

func TestNextDailyRun_AnchorIsNextMidnightUTC(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{"before midnight", dateUTC(2026, 8, 13, 23, 59, 59), dateUTC(2026, 8, 14, 0, 0, 0)},
		{"exactly at anchor rolls to next day", dateUTC(2026, 8, 14, 0, 0, 0), dateUTC(2026, 8, 15, 0, 0, 0)},
		{"midday rolls to next midnight", dateUTC(2026, 8, 14, 12, 34, 56), dateUTC(2026, 8, 15, 0, 0, 0)},
		{"non-UTC input normalized", time.Date(2026, 8, 13, 21, 0, 0, 0, time.FixedZone("MSK", 3*3600)), dateUTC(2026, 8, 14, 0, 0, 0)},
		{"month rollover", dateUTC(2026, 12, 31, 10, 0, 0), dateUTC(2027, 1, 1, 0, 0, 0)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := nextDailyRun(tc.now); !got.Equal(tc.want) {
				t.Fatalf("nextDailyRun(%v) = %v, want %v", tc.now, got, tc.want)
			}
		})
	}
}

func dateUTC(year, month, day, hour, minute, sec int) time.Time {
	return time.Date(year, time.Month(month), day, hour, minute, sec, 0, time.UTC)
}

// TestOperationOverdueWorker_Run_NoScanAtStartup pins the deliberate absence
// of a startup scan: the daily anchor owns the schedule, so a deploy must not
// trigger work.
func TestOperationOverdueWorker_Run_NoScanAtStartup(t *testing.T) {
	t.Parallel()

	svc := &fakeOverdueService{}
	w := newOverdueWorkerForTest(svc)
	// Next run far in the future: any scan within the test can only be a
	// startup scan, which must not happen.
	w.nextRun = func(time.Time) time.Time { return time.Now().Add(time.Hour) }

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()

	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after context cancel")
	}

	if scans := svc.scans.Load(); scans != 0 {
		t.Fatalf("scans = %d, want 0 (no scan at startup)", scans)
	}
}

// TestOperationOverdueWorker_Run_ScansAtAnchor verifies the scan fires when
// the anchor time arrives and the timer is re-armed for the next day.
func TestOperationOverdueWorker_Run_ScansAtAnchor(t *testing.T) {
	t.Parallel()

	svc := &fakeOverdueService{}
	w := newOverdueWorkerForTest(svc)
	anchorCount := atomic.Int32{}
	w.nextRun = func(now time.Time) time.Time {
		anchorCount.Add(1)
		return now.Add(10 * time.Millisecond)
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()

	// The anchor fires within a second even on a loaded runner.
	deadline := time.After(time.Second)
	for svc.scans.Load() == 0 {
		select {
		case <-deadline:
			cancel()
			t.Fatal("no scan fired at the scheduled anchor within 1s")
		case <-time.After(2 * time.Millisecond):
		}
	}
	cancel()
	<-done

	// After a scan the loop re-arms the timer: nextRun was called at least
	// twice (initial arm + re-arm after the fired scan).
	if arms := anchorCount.Load(); arms < 2 {
		t.Fatalf("nextRun calls = %d, want >= 2 (initial arm + re-arm)", arms)
	}
}

func newOverdueWorkerForTest(svc *fakeOverdueService) *OperationOverdueWorker {
	w := NewOperationOverdueWorker(nil, fakeClockForWorker{now: time.Now()}, 100,
		slog.New(slog.DiscardHandler), fakeTzResolver{})
	w.operationService = svc
	return w
}
