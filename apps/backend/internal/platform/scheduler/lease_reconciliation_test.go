package scheduler

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	leasesdomain "github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
)

type fakeLeaseService struct {
	leases            []leasesdomain.Lease
	listErr           error
	reconcileErr      error
}

func (s *fakeLeaseService) ListOpenLeasesWithPastEndDate(context.Context, time.Time, int) ([]leasesdomain.Lease, error) {
	return s.leases, s.listErr
}

func (s *fakeLeaseService) ReconcileRequiresAction(context.Context, uuid.UUID, time.Time) error {
	return s.reconcileErr
}

func TestLeaseReconciliationWorker_Tick_SanitizesServiceErrors(t *testing.T) {
	t.Parallel()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	sensitive := "token=secret123 card 1234-5678-9012-3456 phone +79991234567"
	leaseID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	svc := &fakeLeaseService{
		leases:       []leasesdomain.Lease{{ID: leaseID}},
		reconcileErr: errors.New("reconcile failed: " + sensitive),
	}

	w := NewLeaseReconciliationWorker(nil, fakeClockForWorker{now: time.Now()}, time.Hour, 10, logger)
	w.leaseService = svc

	if err := w.tick(context.Background()); err != nil {
		t.Fatalf("tick unexpected error: %v", err)
	}

	logs := logBuf.String()
	forbidden := []string{
		"token=secret123",
		"1234-5678-9012-3456",
		"+79991234567",
	}
	for _, s := range forbidden {
		if bytes.Contains([]byte(logs), []byte(s)) {
			t.Errorf("log contains sensitive substring %q:\n%s", s, logs)
		}
	}
	if !bytes.Contains([]byte(logs), []byte("reconcile lease failed")) {
		t.Errorf("expected reconcile lease failed log, got:\n%s", logs)
	}
}
