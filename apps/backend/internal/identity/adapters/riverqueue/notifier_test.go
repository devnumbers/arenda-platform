package riverqueue

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// fakeTx is a transaction.Tx placeholder: WithTx only stores it, and the
// unbound-client error fires before the driver-level unwrap needs a real pgx
// transaction.
type fakeTx struct {
	transaction.Tx
}

// TestQueue_ScheduleChangedBeforeBindFailsLoudly proves the deferred bind's
// failure mode: a letter scheduled before WireRiverQueue binds the client is
// a wiring mistake, never a silent drop.
func TestQueue_ScheduleChangedBeforeBindFailsLoudly(t *testing.T) {
	t.Parallel()
	q := NewQueue()

	recipient, err := domain.NewEmail("owner@example.com")
	if err != nil {
		t.Fatalf("parse email: %v", err)
	}
	err = q.WithTx(fakeTx{}).ScheduleChanged(context.Background(), application.ContactChangedEvent{
		Kind:      application.ContactChangedEmail,
		UserID:    uuid.Must(uuid.NewV7()),
		Recipient: recipient,
		ChangedAt: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("ScheduleChanged without a bound client: nil error, want one")
	}
	if !strings.Contains(err.Error(), "not bound") {
		t.Fatalf("error = %v, want the not-bound wiring hint", err)
	}
}
