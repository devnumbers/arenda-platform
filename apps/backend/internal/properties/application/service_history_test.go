package application

// The action journal of the property mutations (карта #704, тикет #707,
// ADR 0061): every manual action writes its row inside the transaction with
// the row-text catalog's action id, the idempotent re-pin writes nothing,
// and a failing journal insert fails the mutation (fail-safe).

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type fakeHistory struct {
	entries []historydomain.Entry
	err     error
}

func (h *fakeHistory) Record(_ context.Context, e historydomain.Entry) error {
	if h.err != nil {
		return h.err
	}
	h.entries = append(h.entries, e)
	return nil
}

func (h *fakeHistory) WithTx(transaction.Tx) historyapp.Recorder { return h }

// The fixture names and addresses the rename assertions read back.
const (
	fixtureOldName    = "Old Name"
	fixtureOldAddress = "Old Address"
	renamedName       = "Новое имя"
	renamedAddress    = "Москва, Тверская 1"
)

func newHistoryTestService(t *testing.T, history *fakeHistory) (*PropertyService, uuid.UUID) {
	t.Helper()
	owner := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	property := domain.Property{
		ID:      propertyID,
		OwnerID: owner,
		Name:    fixtureOldName,
		Address: fixtureOldAddress,
		Type:    domain.PropertyTypeApartment,
		Status:  domain.PropertyStatusActive,
	}
	repo := newLockingFakePropertyRepo(property)
	svc := NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		NewTxStoreFactory(repo, fakePropertyPhotoRepo{}, fakeSubscriptionLimiter{limit: 10},
			nil, history, fakeUoW{beginner: fakePropertyTxBeginner{}}),
		fakePropertyClock{now: time.Now()},
		testOwnerPolicy{},
		nil,
	)
	return svc, owner
}

func TestHistory_PropertyMutationRows(t *testing.T) {
	t.Parallel()
	history := &fakeHistory{}
	svc, owner := newHistoryTestService(t, history)
	ctx := context.Background()

	created, err := svc.CreateProperty(ctx, owner, CreatePropertyCommand{
		Name: "Новая квартира", Address: renamedAddress, Type: "apartment",
	})
	if err != nil {
		t.Fatalf("CreateProperty: %v", err)
	}
	newName := "Переименованная"
	if _, err := svc.UpdateProperty(ctx, owner, created.ID, UpdatePropertyCommand{Name: &newName}); err != nil {
		t.Fatalf("UpdateProperty: %v", err)
	}
	if _, err := svc.SetPropertyPin(ctx, owner, created.ID, true); err != nil {
		t.Fatalf("SetPropertyPin: %v", err)
	}
	// The idempotent re-pin keeps the pin time and writes no row.
	if _, err := svc.SetPropertyPin(ctx, owner, created.ID, true); err != nil {
		t.Fatalf("SetPropertyPin (re-pin): %v", err)
	}
	if _, err := svc.ArchiveProperty(ctx, owner, created.ID); err != nil {
		t.Fatalf("ArchiveProperty: %v", err)
	}

	if len(history.entries) != 4 {
		t.Fatalf("journal entries = %d, want 4 (create, rename, pin, archive; re-pin silent)", len(history.entries))
	}
	want := []struct{ action, text string }{
		{string(historydomain.ActionPropertyCreated), "Добавлен объект: Новая квартира"},
		{string(historydomain.ActionPropertyRenamed), "Название объекта изменено: Новая квартира → Переименованная"},
		{string(historydomain.ActionPropertyPinned), "Объект закреплён"},
		{string(historydomain.ActionPropertyArchived), "Объект отправлен в архив"},
	}
	for i, w := range want {
		e := history.entries[i]
		if string(e.Action) != w.action {
			t.Errorf("entry %d action = %s, want %s", i, e.Action, w.action)
		}
		if e.Segments.PlainText() != w.text {
			t.Errorf("entry %d text = %q, want %q", i, e.Segments.PlainText(), w.text)
		}
		if e.PropertyID == (uuid.UUID{}) || e.ActorID == nil {
			t.Errorf("entry %d must carry the scope and the actor", i)
		}
		if e.ActorRole != historydomain.ActorRoleOwner {
			t.Errorf("entry %d actor role = %s, want owner", i, e.ActorRole)
		}
	}
}

func TestHistory_RenameCarriesOldAndNew(t *testing.T) {
	t.Parallel()
	history := &fakeHistory{}
	svc, owner := newHistoryTestService(t, history)
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	newName := renamedName
	if _, err := svc.UpdateProperty(context.Background(), owner, propertyID, UpdatePropertyCommand{
		Name: &newName,
	}); err != nil {
		t.Fatalf("UpdateProperty: %v", err)
	}

	e := history.entries[0]
	if e.Action != historydomain.ActionPropertyRenamed {
		t.Fatalf("action = %s, want property.renamed", e.Action)
	}
	if e.Context["old_name"] != fixtureOldName || e.Context["new_name"] != renamedName {
		t.Errorf("context = %v, want the old/new names", e.Context)
	}
}

func TestHistory_FailingInsertFailsMutation(t *testing.T) {
	t.Parallel()
	history := &fakeHistory{err: errors.New("journal down")}
	svc, owner := newHistoryTestService(t, history)
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	name := "Отказ"
	if _, err := svc.UpdateProperty(context.Background(), owner, propertyID, UpdatePropertyCommand{
		Name: &name,
	}); err == nil {
		t.Fatal("UpdateProperty returned nil, want the journal failure to fail the mutation")
	}
}
