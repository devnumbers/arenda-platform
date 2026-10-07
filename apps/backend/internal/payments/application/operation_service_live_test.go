package application

// The unit tests of the operation page's live-presentation fallbacks
// (ticket #1190): the rule-store outcomes the integration fixtures cannot
// reach. A rule that vanishes mid-read keeps the operation's frozen
// snapshots; any other rule-store failure fails the read.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
)

// stubOperationStore serves one prebuilt operation from Get; the embedded
// interface keeps every unread method out of the way.
type stubOperationStore struct {
	OperationStore
	op domain.Operation
}

func (s stubOperationStore) Get(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Operation, error) {
	return s.op, nil
}

// failingRuleStore fails every rule load with the given error.
type failingRuleStore struct {
	PaymentStore
	err error
}

func (s failingRuleStore) Get(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Payment, error) {
	return domain.Payment{}, s.err
}

// stubLiveReadProperties resolves the scope owner; with a nil policy the
// read path needs nothing else from it.
type stubLiveReadProperties struct {
	PropertyStore
	owner uuid.UUID
}

func (s stubLiveReadProperties) Get(context.Context, uuid.UUID) (PropertyRef, error) {
	return PropertyRef{OwnerID: s.owner}, nil
}

// stubLiveReadCalendar pins the owner's today for the view status.
type stubLiveReadCalendar struct{ OwnerCalendar }

func (stubLiveReadCalendar) Today(context.Context, uuid.UUID) (time.Time, error) {
	return time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC), nil
}

func newLiveReadService(operations OperationStore, payments PaymentStore, owner uuid.UUID) *OperationService {
	return &OperationService{
		txStoreFactory: txStoreFactory{
			operations: operations,
			payments:   payments,
			properties: stubLiveReadProperties{owner: owner},
		},
		calendar: stubLiveReadCalendar{},
	}
}

// frozenOperation is a rule-born operation with its materialization
// snapshots; the tests assert on them surviving the rule-store outcomes.
func frozenOperation(t *testing.T) domain.Operation {
	t.Helper()
	paymentID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	slug := "old-slug"
	return domain.Operation{
		ID:            paymentID, // Any id — the stub never filters.
		PaymentID:     &paymentID,
		Origin:        domain.OriginPayment,
		Date:          time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC),
		Status:        domain.StatusPaid,
		Title:         "Старое название",
		CategoryLabel: "Старая категория",
		CategorySlug:  &slug,
	}
}

func TestGetOperation_VanishedRuleKeepsFrozenSnapshots(t *testing.T) {
	t.Parallel()
	owner, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	op := frozenOperation(t)
	svc := newLiveReadService(stubOperationStore{op: op}, failingRuleStore{err: ErrNotFound}, owner)

	item, err := svc.GetOperation(context.Background(), owner, op.PropertyID, op.ID)
	if err != nil {
		t.Fatalf("get operation: %v", err)
	}
	if item.Operation.Title != "Старое название" || item.Operation.CategoryLabel != "Старая категория" ||
		item.Operation.CategorySlug == nil || *item.Operation.CategorySlug != "old-slug" {
		t.Fatalf("operation = %q/%q/%v, want the frozen snapshots standing",
			item.Operation.Title, item.Operation.CategoryLabel, item.Operation.CategorySlug)
	}
	if item.ViewStatus != domain.ViewStatusPaid {
		t.Fatalf("view status = %s, want paid", item.ViewStatus)
	}
}

func TestGetOperation_RuleStoreFailureFailsTheRead(t *testing.T) {
	t.Parallel()
	owner, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	op := frozenOperation(t)
	boom := errors.New("boom")
	svc := newLiveReadService(stubOperationStore{op: op}, failingRuleStore{err: boom}, owner)

	if _, err := svc.GetOperation(context.Background(), owner, op.PropertyID, op.ID); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the rule-store failure propagated", err)
	}
}
