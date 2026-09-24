package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// fakeWriter captures the inserted entries.
type fakeWriter struct {
	entries []domain.Entry
	err     error
}

func (w *fakeWriter) Insert(_ context.Context, entry domain.Entry) error {
	if w.err != nil {
		return w.err
	}
	w.entries = append(w.entries, entry)
	return nil
}

func (w *fakeWriter) WithTx(transaction.Tx) historyapp.EntryWriter { return w }

// fakeActors answers a canned snapshot or a canned error.
type fakeActors struct {
	snap domain.ActorSnapshot
	err  error
}

func (a fakeActors) Snapshot(context.Context, uuid.UUID) (domain.ActorSnapshot, error) {
	return a.snap, a.err
}

func (a fakeActors) WithTx(transaction.Tx) historyapp.ActorSnapshotSource { return a }

// actorName is the canned snapshot the fake actor source answers with.
const actorName = "Иван Иванов"

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func TestRecord_FillsSnapshotAndSearchable(t *testing.T) {
	t.Parallel()

	actor := uuid.Must(uuid.NewV7())
	property := uuid.Must(uuid.NewV7())
	writer := &fakeWriter{}
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	svc := historyapp.NewService(writer, fakeActors{snap: domain.ActorSnapshot{Name: actorName, Email: "ivan@example.com"}}, fixedClock{now})

	entry := domain.PaymentCreated(property, "Электричество")
	entry.PropertyID = property
	entry.ActorID = &actor
	entry.ActorRole = domain.ActorRoleOwner
	if err := svc.Record(context.Background(), entry); err != nil {
		t.Fatalf("Record: %v", err)
	}

	if len(writer.entries) != 1 {
		t.Fatalf("inserted %d entries, want 1", len(writer.entries))
	}
	got := writer.entries[0]
	if got.ID == (uuid.UUID{}) {
		t.Error("Record must assign the entry id")
	}
	if !got.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, now)
	}
	if got.ActorName != actorName || got.ActorEmail != "ivan@example.com" {
		t.Errorf("actor snapshot = %q/%q, want the resolved one", got.ActorName, got.ActorEmail)
	}
	want := "Добавлен платёж: Электричество Иван Иванов ivan@example.com"
	if got.Searchable != want {
		t.Errorf("Searchable = %q, want %q", got.Searchable, want)
	}
	if got.Searchable != got.Segments.PlainText()+" Иван Иванов ivan@example.com" {
		t.Error("Searchable must be the segments' plain text plus the actor snapshot")
	}
}

func TestRecord_KeepsCallerSuppliedSnapshot(t *testing.T) {
	t.Parallel()

	actor := uuid.Must(uuid.NewV7())
	writer := &fakeWriter{}
	svc := historyapp.NewService(writer, fakeActors{snap: domain.ActorSnapshot{Name: "другой"}}, nil)

	entry := domain.MemberLeft("Иван Иванов")
	entry.PropertyID = uuid.Must(uuid.NewV7())
	entry.ActorID = &actor
	entry.ActorName = "Иван Иванов"
	entry.ActorEmail = ""
	if err := svc.Record(context.Background(), entry); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if writer.entries[0].ActorName != actorName {
		t.Errorf("ActorName = %q, want the caller's snapshot to win", writer.entries[0].ActorName)
	}
}

func TestRecord_ActorResolutionFailureFailsTheRecord(t *testing.T) {
	t.Parallel()

	actor := uuid.Must(uuid.NewV7())
	writer := &fakeWriter{}
	svc := historyapp.NewService(writer, fakeActors{err: errors.New("db down")}, nil)

	entry := domain.PropertyPinned(uuid.Must(uuid.NewV7()))
	entry.ActorID = &actor
	if err := svc.Record(context.Background(), entry); err == nil {
		t.Fatal("Record returned nil, want the actor resolution error")
	}
	if len(writer.entries) != 0 {
		t.Error("a failed actor resolution must not insert the entry")
	}
}

func TestRecord_InsertErrorPropagates(t *testing.T) {
	t.Parallel()

	actor := uuid.Must(uuid.NewV7())
	writer := &fakeWriter{err: errors.New("insert failed")}
	svc := historyapp.NewService(writer, fakeActors{}, nil)

	entry := domain.PropertyArchived(uuid.Must(uuid.NewV7()))
	entry.ActorID = &actor
	if err := svc.Record(context.Background(), entry); err == nil {
		t.Fatal("Record returned nil, want the insert error (fail-safe)")
	}
}

func TestNoop_Discards(t *testing.T) {
	t.Parallel()

	var noop historyapp.Noop
	entry := domain.TaskCompletedCleared(3)
	entry.PropertyID = uuid.Must(uuid.NewV7())
	if err := noop.Record(context.Background(), entry); err != nil {
		t.Fatalf("Noop Record: %v", err)
	}
	if noop.WithTx(nil) == nil {
		t.Error("Noop WithTx must return the same recorder")
	}
}

var _ clock.Clock = fixedClock{}
