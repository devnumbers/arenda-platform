package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// The grace v2 property-side bridge (ADR 0055): the auto-archive reports the
// ids it archived — the restoration snapshot billing stores — and the restore
// unarchives exactly those ids under the tariff limit, mirroring a manual
// unarchive with its audit and recipient-slot enforcement.

func newBridgeTestService(t *testing.T, repo *fakePropertyRepo, slots *recordingSlotPolicy) *PropertyService {
	t.Helper()
	svc := NewPropertyService(
		repo,
		fakePropertyPhotoRepo{},
		fakePropertyPhotoStorage{},
		newPropertyTestFactory(repo, fakePropertyPhotoRepo{}, nil),
		fakePropertyClock{now: time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)},
		testOwnerPolicy{},
		nil,
	)
	if slots != nil {
		svc.SetRecipientSlotPolicy(slots)
	}
	return svc
}

// TestPropertyService_ArchiveExcessProperties_ReturnsArchivedIDs proves the
// auto-archive reports what it archived, newest first — the restoration
// priority order of the grace snapshot.
func TestPropertyService_ArchiveExcessProperties_ReturnsArchivedIDs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	keepID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	excessAID := uuid.MustParse("33333333-3333-3333-3333-333333333333") // Updated 06-02.
	excessBID := uuid.MustParse("44444444-4444-4444-4444-444444444444") // Updated 06-01.

	repo := newFakePropertyRepo(
		domain.Property{
			ID: keepID, OwnerID: ownerID, Name: "Keep", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive, UpdatedAt: time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC),
		},
		domain.Property{
			ID: excessAID, OwnerID: ownerID, Name: "Excess A", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive, UpdatedAt: time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
		},
		domain.Property{
			ID: excessBID, OwnerID: ownerID, Name: "Excess B", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive, UpdatedAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		},
	)
	svc := newBridgeTestService(t, repo, nil)

	archived, err := svc.ArchiveExcessProperties(ctx, &fakePropertyTx{}, ownerID, 1, nil)
	if err != nil {
		t.Fatalf("ArchiveExcessProperties failed: %v", err)
	}
	if len(archived) != 2 || archived[0] != excessAID || archived[1] != excessBID {
		t.Fatalf("archived ids = %v, want [%s %s] (newest first)", archived, excessAID, excessBID)
	}
}

// TestPropertyService_RestoreGraceArchivedProperties_RestoresUnderLimit proves
// the grace-restore bridge: the snapshotted ids unarchive in the given order
// while the owner's active count stays under the limit, and the count of
// restorations is returned.
func TestPropertyService_RestoreGraceArchivedProperties_RestoresUnderLimit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	activeID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	archivedAID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	archivedBID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	repo := newFakePropertyRepo(
		domain.Property{
			ID: activeID, OwnerID: ownerID, Name: "Survivor", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
		},
		domain.Property{
			ID: archivedAID, OwnerID: ownerID, Name: "Archived A", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusArchived,
		},
		domain.Property{
			ID: archivedBID, OwnerID: ownerID, Name: "Archived B", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusArchived,
		},
	)
	slots := &recordingSlotPolicy{}
	svc := newBridgeTestService(t, repo, slots)

	restored, err := svc.RestoreGraceArchivedProperties(ctx, &fakePropertyTx{}, ownerID,
		[]uuid.UUID{archivedAID, archivedBID}, 2)
	if err != nil {
		t.Fatalf("RestoreGraceArchivedProperties failed: %v", err)
	}
	a, err := repo.GetByIDAndOwner(ctx, archivedAID, ownerID)
	if err != nil {
		t.Fatalf("get restored property: %v", err)
	}
	if a.Status != domain.PropertyStatusActive {
		t.Errorf("restored property status = %q, want active", a.Status)
	}
	b, err := repo.GetByIDAndOwner(ctx, archivedBID, ownerID)
	if err != nil {
		t.Fatalf("get skipped property: %v", err)
	}
	if b.Status != domain.PropertyStatusArchived {
		t.Errorf("over-limit property status = %q, want still archived", b.Status)
	}
	if len(slots.enforcedOnUnarchive) != 1 || slots.enforcedOnUnarchive[0] != archivedAID {
		t.Errorf("unarchive enforcement = %v, want the restored property", slots.enforcedOnUnarchive)
	}
	// The over-limit id is the debt remainder, not lost.
	if len(restored) != 1 || restored[0] != archivedBID {
		t.Errorf("remainder = %v, want [%s]", restored, archivedBID)
	}
}

// TestPropertyService_RestoreGraceArchivedProperties_SkipsGoneAndActive proves
// the snapshot is a debt, not a command: an id deleted since the snapshot and
// an id the owner already restored manually are skipped, and the unlimited
// limit (negative) restores everything that remains.
func TestPropertyService_RestoreGraceArchivedProperties_SkipsGoneAndActive(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	manuallyActiveID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	archivedID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	deletedID := uuid.MustParse("55555555-5555-5555-5555-555555555555")

	repo := newFakePropertyRepo(
		domain.Property{
			ID: manuallyActiveID, OwnerID: ownerID, Name: "Manually restored", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
		},
		domain.Property{
			ID: archivedID, OwnerID: ownerID, Name: "Grace Archived", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusArchived,
		},
	)
	svc := newBridgeTestService(t, repo, nil)

	restored, err := svc.RestoreGraceArchivedProperties(ctx, &fakePropertyTx{}, ownerID,
		[]uuid.UUID{manuallyActiveID, deletedID, archivedID}, -1)
	if err != nil {
		t.Fatalf("RestoreGraceArchivedProperties failed: %v", err)
	}
	if len(restored) != 0 {
		t.Fatalf("remainder = %v, want nil (unlimited limit settles everything)", restored)
	}
	p, err := repo.GetByIDAndOwner(ctx, archivedID, ownerID)
	if err != nil {
		t.Fatalf("get restored property: %v", err)
	}
	if p.Status != domain.PropertyStatusActive {
		t.Errorf("restored property status = %q, want active", p.Status)
	}
}

// TestPropertyService_RestoreGraceArchivedProperties_ExhaustedLimitReturnsAll
// proves the pure remainder case: a spent limit (0 with an active survivor)
// restores nothing and hands back the whole snapshot as the debt remainder.
func TestPropertyService_RestoreGraceArchivedProperties_ExhaustedLimitReturnsAll(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	activeID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	archivedID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	repo := newFakePropertyRepo(
		domain.Property{
			ID: activeID, OwnerID: ownerID, Name: "Survivor", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
		},
		domain.Property{
			ID: archivedID, OwnerID: ownerID, Name: "Grace Archived", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusArchived,
		},
	)
	svc := newBridgeTestService(t, repo, nil)

	remaining, err := svc.RestoreGraceArchivedProperties(ctx, &fakePropertyTx{}, ownerID,
		[]uuid.UUID{archivedID}, 1)
	if err != nil {
		t.Fatalf("RestoreGraceArchivedProperties failed: %v", err)
	}
	if len(remaining) != 1 || remaining[0] != archivedID {
		t.Fatalf("remainder = %v, want the whole snapshot", remaining)
	}
	p, err := repo.GetByIDAndOwner(ctx, archivedID, ownerID)
	if err != nil {
		t.Fatalf("get archived property: %v", err)
	}
	if p.Status != domain.PropertyStatusArchived {
		t.Errorf("property status = %q, want still archived", p.Status)
	}
}

var _ transaction.Tx = (*fakePropertyTx)(nil)

// TestPropertyService_ArchiveExcessProperties_KeepsChosenProperty proves the
// cancel keep-choice bridge (issue #617): when billing enforces a lowered
// limit with a keepPropertyID, that property survives even when it is not the
// most recently updated one, and the rest of the survivor slots still go to
// the newest.
func TestPropertyService_ArchiveExcessProperties_KeepsChosenProperty(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	keepID := uuid.MustParse("22222222-2222-2222-2222-222222222222")  // Updated 06-01 — the oldest.
	freshID := uuid.MustParse("33333333-3333-3333-3333-333333333333") // Updated 06-03.
	midID := uuid.MustParse("44444444-4444-4444-4444-444444444444")   // Updated 06-02.

	repo := newFakePropertyRepo(
		domain.Property{
			ID: keepID, OwnerID: ownerID, Name: "Chosen", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive, UpdatedAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		},
		domain.Property{
			ID: freshID, OwnerID: ownerID, Name: "Fresh", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive, UpdatedAt: time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC),
		},
		domain.Property{
			ID: midID, OwnerID: ownerID, Name: "Mid", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive, UpdatedAt: time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
		},
	)
	svc := newBridgeTestService(t, repo, nil)

	archived, err := svc.ArchiveExcessProperties(ctx, &fakePropertyTx{}, ownerID, 2, &keepID)
	if err != nil {
		t.Fatalf("ArchiveExcessProperties failed: %v", err)
	}
	// The chosen property takes one survivor slot; the other goes to the
	// freshest — "Mid" is the excess.
	if len(archived) != 1 || archived[0] != midID {
		t.Fatalf("archived ids = %v, want [%s]", archived, midID)
	}
	kept, err := repo.GetByIDAndOwner(ctx, keepID, ownerID)
	if err != nil {
		t.Fatalf("get kept property: %v", err)
	}
	if kept.Status != domain.PropertyStatusActive {
		t.Errorf("chosen property status = %q, want active", kept.Status)
	}
}

// TestPropertyService_ArchiveExcessProperties_StaleKeepFallsBackToNewest
// proves the fallback: a keepPropertyID that is no longer one of the owner's
// active properties (archived, deleted, foreign) changes nothing — the
// default most-recently-updated survivor rule holds.
func TestPropertyService_ArchiveExcessProperties_StaleKeepFallsBackToNewest(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	staleKeepID := uuid.MustParse("22222222-2222-2222-2222-222222222222") // Archived before the expiry.
	freshID := uuid.MustParse("33333333-3333-3333-3333-333333333333")     // Updated 06-03.
	olderID := uuid.MustParse("44444444-4444-4444-4444-444444444444")     // Updated 06-02.

	repo := newFakePropertyRepo(
		domain.Property{
			ID: staleKeepID, OwnerID: ownerID, Name: "Stale", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusArchived, UpdatedAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		},
		domain.Property{
			ID: freshID, OwnerID: ownerID, Name: "Fresh", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive, UpdatedAt: time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC),
		},
		domain.Property{
			ID: olderID, OwnerID: ownerID, Name: "Older", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive, UpdatedAt: time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC),
		},
	)
	svc := newBridgeTestService(t, repo, nil)

	archived, err := svc.ArchiveExcessProperties(ctx, &fakePropertyTx{}, ownerID, 1, &staleKeepID)
	if err != nil {
		t.Fatalf("ArchiveExcessProperties failed: %v", err)
	}
	if len(archived) != 1 || archived[0] != olderID {
		t.Fatalf("archived ids = %v, want [%s]", archived, olderID)
	}
}

// TestPropertyService_ActivePropertyExists proves the keep-choice validation
// read of the bridge (issue #617): active and maintenance properties count —
// they occupy tariff slots — while archived, foreign and missing ids do not.
func TestPropertyService_ActivePropertyExists(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	activeID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	maintenanceID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	archivedID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	repo := newFakePropertyRepo(
		domain.Property{
			ID: activeID, OwnerID: ownerID, Name: "Active", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusActive,
		},
		domain.Property{
			ID: maintenanceID, OwnerID: ownerID, Name: "Maintenance", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusMaintenance,
		},
		domain.Property{
			ID: archivedID, OwnerID: ownerID, Name: "Archived", Address: testPropertyAddress,
			Type: domain.PropertyTypeApartment, Status: domain.PropertyStatusArchived,
		},
	)
	svc := newBridgeTestService(t, repo, nil)

	for _, tc := range []struct {
		name       string
		propertyID uuid.UUID
		scope      uuid.UUID
		want       bool
	}{
		{"active", activeID, ownerID, true},
		{"maintenance occupies a slot", maintenanceID, ownerID, true},
		{"archived", archivedID, ownerID, false},
		// A foreign id is the repository's scope miss — ErrNotFound like a
		// missing one; the SQL-side owner filter is the repo's contract.
		{"missing", uuid.MustParse("66666666-6666-6666-6666-666666666666"), ownerID, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := svc.ActivePropertyExists(ctx, &fakePropertyTx{}, tc.scope, tc.propertyID)
			if err != nil {
				t.Fatalf("ActivePropertyExists failed: %v", err)
			}
			if got != tc.want {
				t.Errorf("ActivePropertyExists = %v, want %v", got, tc.want)
			}
		})
	}
}
