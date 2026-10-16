package application

// The archive round-trip's system pauses, the properties side (тикет
// #1342): the lifecycle changes drive the payments bridge inside their
// transaction — the archive pauses the object's live rules, the manual
// unarchive and the grace restore close only the auto_archive pauses. The
// bridge itself is the payments context's (its matrix lives there); here
// the recording fake pins the wiring: which leg, whose transaction, whose
// actor.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// recordingArchivePauses pins the bridge calls the lifecycle changes make.
type recordingArchivePauses struct {
	pauseErr  error
	resumeErr error

	pauseCalls  []archivePausesCall
	resumeCalls []archivePausesCall
}

type archivePausesCall struct {
	scope      uuid.UUID
	propertyID uuid.UUID
	actor      uuid.UUID
	role       sharedpolicy.Role
	tx         transaction.Tx
}

func (r *recordingArchivePauses) PauseAllForArchive(
	_ context.Context, tx transaction.Tx, scope, propertyID, actor uuid.UUID, role sharedpolicy.Role,
) error {
	r.pauseCalls = append(r.pauseCalls, archivePausesCall{scope, propertyID, actor, role, tx})
	return r.pauseErr
}

func (r *recordingArchivePauses) ResumeArchivePauses(
	_ context.Context, tx transaction.Tx, scope, propertyID, actor uuid.UUID, role sharedpolicy.Role,
) error {
	r.resumeCalls = append(r.resumeCalls, archivePausesCall{scope, propertyID, actor, role, tx})
	return r.resumeErr
}

func newArchivePausesTestService(t *testing.T, repo *lockingFakePropertyRepo, bridge *recordingArchivePauses) *PropertyService {
	t.Helper()
	svc := NewPropertyService(
		repo,
		newFakePropertyPhotoStorage(),
		newPropertyTestFactory(repo, fakeSubscriptionLimiter{limit: 100}),
		fakePropertyClock{now: time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)},
		testOwnerPolicy{},
		nil,
	)
	if bridge != nil {
		svc.SetArchivePausesBridge(bridge)
	}
	return svc
}

func TestPropertyService_ArchiveProperty_PausesRules(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	repo := newLockingFakePropertyRepo(domain.Property{
		ID: propertyID, OwnerID: ownerID, Name: testPropertyName, Address: testPropertyAddress,
		Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
	})
	bridge := &recordingArchivePauses{}
	svc := newArchivePausesTestService(t, repo, bridge)

	if _, err := svc.ArchiveProperty(ctx, ownerID, propertyID); err != nil {
		t.Fatalf("ArchiveProperty failed: %v", err)
	}
	if len(bridge.pauseCalls) != 1 {
		t.Fatalf("pause calls = %d, want 1", len(bridge.pauseCalls))
	}
	call := bridge.pauseCalls[0]
	if call.scope != ownerID || call.propertyID != propertyID || call.actor != ownerID {
		t.Errorf("pause call = %+v, want the owner's scope, the archived property, the owner as actor", call)
	}
	if call.role != sharedpolicy.RoleOwner {
		t.Errorf("role = %q, want owner", call.role)
	}
	if call.tx == nil {
		t.Error("pause call received no transaction — the pauses must ride the archive's commit")
	}
	if len(bridge.resumeCalls) != 0 {
		t.Errorf("resume calls = %d, want 0 on archive", len(bridge.resumeCalls))
	}
}

func TestPropertyService_ArchiveProperty_BridgeFailureRollsBack(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	repo := newLockingFakePropertyRepo(domain.Property{
		ID: propertyID, OwnerID: ownerID, Name: testPropertyName, Address: testPropertyAddress,
		Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
	})
	bridge := &recordingArchivePauses{pauseErr: errors.New("payments down")}
	svc := newArchivePausesTestService(t, repo, bridge)

	// Ошибка моста валит всю архивацию: паузы необязательными не бывают.
	if _, err := svc.ArchiveProperty(ctx, ownerID, propertyID); err == nil {
		t.Fatal("ArchiveProperty succeeded despite the bridge failure — the pauses must not be skippable")
	}
}

func TestPropertyService_UnarchiveProperty_ResumesAutoPauses(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	propertyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	repo := newLockingFakePropertyRepo(domain.Property{
		ID: propertyID, OwnerID: ownerID, Name: testPropertyName, Address: testPropertyAddress,
		Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusArchived,
	})
	bridge := &recordingArchivePauses{}
	svc := newArchivePausesTestService(t, repo, bridge)

	if _, err := svc.UnarchiveProperty(ctx, ownerID, propertyID); err != nil {
		t.Fatalf("UnarchiveProperty failed: %v", err)
	}
	if len(bridge.resumeCalls) != 1 {
		t.Fatalf("resume calls = %d, want 1", len(bridge.resumeCalls))
	}
	call := bridge.resumeCalls[0]
	if call.scope != ownerID || call.propertyID != propertyID || call.actor != ownerID {
		t.Errorf("resume call = %+v, want the owner's scope, the restored property, the owner as actor", call)
	}
	if call.tx == nil {
		t.Error("resume call received no transaction")
	}
	if len(bridge.pauseCalls) != 0 {
		t.Errorf("pause calls = %d, want 0 on unarchive", len(bridge.pauseCalls))
	}
}

func TestPropertyService_GraceRestore_ResumesAutoPauses(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	restoredID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	repo := newFakePropertyRepo(domain.Property{
		ID: restoredID, OwnerID: ownerID, Name: testPropertyName, Address: testPropertyAddress,
		Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusArchived,
	})
	bridge := &recordingArchivePauses{}
	svc := newBridgeTestService(t, repo, nil)
	svc.SetArchivePausesBridge(bridge)

	remaining, err := svc.RestoreGraceArchivedProperties(ctx, &fakePropertyTx{}, ownerID, []uuid.UUID{restoredID}, 5)
	if err != nil {
		t.Fatalf("RestoreGraceArchivedProperties failed: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("remaining = %v, want empty", remaining)
	}
	if len(bridge.resumeCalls) != 1 {
		t.Fatalf("resume calls = %d, want 1 per restored property", len(bridge.resumeCalls))
	}
	call := bridge.resumeCalls[0]
	if call.propertyID != restoredID || call.scope != ownerID || call.actor != ownerID {
		t.Errorf("resume call = %+v, want the restored property in the owner's scope", call)
	}
}
