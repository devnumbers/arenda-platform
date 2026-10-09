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
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/history/historytest"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	realtimetest "github.com/nambers/arenda-planform/apps/backend/internal/realtime/realtimetest"
	"github.com/stretchr/testify/assert"
)

// The fixture names and addresses the rename assertions read back.
const (
	fixtureOldName    = "Old Name"
	fixtureOldAddress = "Old Address"
	renamedName       = "Новое имя"
	renamedAddress    = "Москва, Тверская 1"
)

func newHistoryTestService(t *testing.T, history *historytest.CapturingRecorder) (*PropertyService, uuid.UUID) {
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
		newFakePropertyPhotoStorage(),
		NewTxStoreFactory(repo, fakeSubscriptionLimiter{limit: 10},
			nil, history, fakeUoW{beginner: fakePropertyTxBeginner{}}),
		fakePropertyClock{now: time.Now()},
		testOwnerPolicy{},
		nil,
	)
	return svc, owner
}

func TestHistory_PropertyMutationRows(t *testing.T) {
	t.Parallel()
	history := &historytest.CapturingRecorder{}
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

	if len(history.Entries) != 4 {
		t.Fatalf("journal entries = %d, want 4 (create, rename, pin, archive; re-pin silent)", len(history.Entries))
	}
	want := []struct{ action, text string }{
		{string(historydomain.ActionPropertyCreated), "Добавлен объект: Новая квартира"},
		{string(historydomain.ActionPropertyRenamed), "Название объекта изменено: Переименованная"},
		{string(historydomain.ActionPropertyPinned), "Объект закреплён"},
		{string(historydomain.ActionPropertyArchived), "Объект отправлен в архив"},
	}
	for i, w := range want {
		e := history.Entries[i]
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
	history := &historytest.CapturingRecorder{}
	svc, owner := newHistoryTestService(t, history)
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	newName := renamedName
	if _, err := svc.UpdateProperty(context.Background(), owner, propertyID, UpdatePropertyCommand{
		Name: &newName,
	}); err != nil {
		t.Fatalf("UpdateProperty: %v", err)
	}

	e := history.Entries[0]
	if e.Action != historydomain.ActionPropertyRenamed {
		t.Fatalf("action = %s, want property.renamed", e.Action)
	}
	if e.Context["old_name"] != fixtureOldName || e.Context["new_name"] != renamedName {
		t.Errorf("context = %v, want the old/new names", e.Context)
	}
}

func TestHistory_FailingInsertFailsMutation(t *testing.T) {
	t.Parallel()
	history := &historytest.CapturingRecorder{Err: errors.New("journal down")}
	svc, owner := newHistoryTestService(t, history)
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	name := "Отказ"
	if _, err := svc.UpdateProperty(context.Background(), owner, propertyID, UpdatePropertyCommand{
		Name: &name,
	}); err == nil {
		t.Fatal("UpdateProperty returned nil, want the journal failure to fail the mutation")
	}
}

func TestHistory_MaintenanceFlipRows(t *testing.T) {
	t.Parallel()
	// Протокол D2 (карта #1324, решение #1317 §6): флип active↔maintenance
	// журналируется — started («Изменение») при входе в ремонт, finished
	// («Выполнение») при выходе; статус-ноуп (idempotent PUT того же
	// статуса) строки не пишет — гвард фактического неравенства обязателен,
	// because isUpdatableStatusTransition пропускает from==to.
	history := &historytest.CapturingRecorder{}
	svc, owner := newHistoryTestService(t, history)
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	ctx := context.Background()

	maintenance := string(domain.PropertyStatusMaintenance)
	if _, err := svc.UpdateProperty(ctx, owner, propertyID, UpdatePropertyCommand{Status: &maintenance}); err != nil {
		t.Fatalf("UpdateProperty (to maintenance): %v", err)
	}
	active := string(domain.PropertyStatusActive)
	if _, err := svc.UpdateProperty(ctx, owner, propertyID, UpdatePropertyCommand{Status: &active}); err != nil {
		t.Fatalf("UpdateProperty (back to active): %v", err)
	}
	// Idempotent PUT того же статуса — тишина (ADR 0061 §3: no-op пишет
	// ноль строк).
	if _, err := svc.UpdateProperty(ctx, owner, propertyID, UpdatePropertyCommand{Status: &active}); err != nil {
		t.Fatalf("UpdateProperty (same status): %v", err)
	}

	if len(history.Entries) != 2 {
		t.Fatalf("journal entries = %d, want 2 (started, finished; same-status silent)", len(history.Entries))
	}
	started, finished := history.Entries[0], history.Entries[1]
	if started.Action != historydomain.ActionPropertyMaintenanceStarted || started.BaseAction != historydomain.BaseChanged {
		t.Errorf("started action/base = %s/%s, want maintenance_started/changed", started.Action, started.BaseAction)
	}
	if got := started.Segments.PlainText(); got != "Объект на ремонте" {
		t.Errorf("started text = %q", got)
	}
	if finished.Action != historydomain.ActionPropertyMaintenanceFinished || finished.BaseAction != historydomain.BaseCompleted {
		t.Errorf("finished action/base = %s/%s, want maintenance_finished/completed", finished.Action, finished.BaseAction)
	}
	if got := finished.Segments.PlainText(); got != "Ремонт завершён" {
		t.Errorf("finished text = %q", got)
	}
	for i, e := range history.Entries {
		assertSingleLinkedPropertyRun(t, i, propertyID, e)
	}
}

// assertSingleLinkedPropertyRun pins the maintenance rows' shape: one
// segment, linked at the object's page.
func assertSingleLinkedPropertyRun(t *testing.T, i int, propertyID uuid.UUID, e historydomain.Entry) {
	t.Helper()
	if len(e.Segments) != 1 || e.Segments[0].Link == nil ||
		e.Segments[0].Link.Kind != historydomain.KindProperty || e.Segments[0].Link.ID != propertyID {
		t.Errorf("entry %d segments = %+v, want the single linked object run", i, e.Segments)
	}
}

func TestHistory_MixedStatusPatchWritesTwoRows(t *testing.T) {
	t.Parallel()
	// Смешанный PATCH (название + статус) пишет две строки — прецедент
	// канона CompleteRental (решение #1317 §6); формулировку подтверждает
	// walkthrough.
	history := &historytest.CapturingRecorder{}
	svc, owner := newHistoryTestService(t, history)
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	maintenance := string(domain.PropertyStatusMaintenance)
	newName := "Ремонтная"
	if _, err := svc.UpdateProperty(context.Background(), owner, propertyID, UpdatePropertyCommand{
		Name:   &newName,
		Status: &maintenance,
	}); err != nil {
		t.Fatalf("UpdateProperty: %v", err)
	}
	if len(history.Entries) != 2 {
		t.Fatalf("journal entries = %d, want 2 (rename + maintenance_started)", len(history.Entries))
	}
	if history.Entries[0].Action != historydomain.ActionPropertyRenamed {
		t.Errorf("first row = %s, want property.renamed", history.Entries[0].Action)
	}
	if history.Entries[1].Action != historydomain.ActionPropertyMaintenanceStarted {
		t.Errorf("second row = %s, want maintenance_started", history.Entries[1].Action)
	}
}

func TestHistory_StatusOnlyEditPublishesHistoryFrame(t *testing.T) {
	t.Parallel()
	// Строка журнала — изменение ленты: status-only PATCH со строкой гонит
	// history-якорь realtime-пабликации (ADR 0062), без строки — нет.
	history := &historytest.CapturingRecorder{}
	svc, owner := newHistoryTestService(t, history)
	realtime := bindPropertyRealtime(t, svc)
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	ctx := context.Background()

	maintenance := string(domain.PropertyStatusMaintenance)
	if _, err := svc.UpdateProperty(ctx, owner, propertyID, UpdatePropertyCommand{Status: &maintenance}); err != nil {
		t.Fatalf("UpdateProperty: %v", err)
	}
	assert.Contains(t, realtime.Pairs(), "history:"+propertyID.String())

	// Повторный статус-ноуп не грязнит ленту.
	realtime = &realtimetest.RecordingPublisher{}
	svc.SetRealtimePublisher(realtime)
	if _, err := svc.UpdateProperty(ctx, owner, propertyID, UpdatePropertyCommand{Status: &maintenance}); err != nil {
		t.Fatalf("UpdateProperty (same status): %v", err)
	}
	assert.NotContains(t, realtime.Pairs(), "history:"+propertyID.String())
}
