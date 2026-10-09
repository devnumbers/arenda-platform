package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	historytest "github.com/nambers/arenda-planform/apps/backend/internal/history/historytest"
	"github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	realtimetest "github.com/nambers/arenda-planform/apps/backend/internal/realtime/realtimetest"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// SetPropertyPin tests (ticket #577; личный cutover #1307, D11): пин —
// личная строка актёра по праву просмотра — владелец, полный доступ и
// зритель одинаково; повторный пин сохраняет исходное время (идемпотентность
// PUT — порядок среди закреплённых не сдвигается), открепление удаляет
// строку, отсутствие доступа и suspended — privacy-404, архивный объект —
// 409. Личное состояние: строка истории не пишется (D17), realtime-фрейм
// не публикуется (D18), admin-audit не трогается.

var pinClockTime = time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)

func pinTestService(t *testing.T, repo PropertyRepository, policy sharedpolicy.Policy) *PropertyService {
	t.Helper()
	return NewPropertyService(
		repo,
		newFakePropertyPhotoStorage(),
		newPropertyTestFactory(repo, nil),
		fakePropertyClock{now: pinClockTime},
		policy,
		nil,
	)
}

func pinTestProperty(ownerID uuid.UUID) domain.Property {
	return domain.Property{
		ID:      uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		OwnerID: ownerID,
		Name:    testPropertyName,
		Address: testPropertyAddress,
		Type:    domain.PropertyTypeApartment,
		Status:  domain.PropertyStatusActive,
	}
}

// pinTestHistoryService builds the pin service with a capturing history
// recorder (the D17 assertions read the journal's writes).
func pinTestHistoryService(t *testing.T, repo PropertyRepository, history *historytest.CapturingRecorder) *PropertyService {
	t.Helper()
	return NewPropertyService(
		repo,
		newFakePropertyPhotoStorage(),
		NewTxStoreFactory(repo, nil, nil, history, fakeUoW{beginner: fakePropertyTxBeginner{}}),
		fakePropertyClock{now: pinClockTime},
		testOwnerPolicy{},
		nil,
	)
}

func TestPropertyService_SetPropertyPin_PinsWithClockTime(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	property := pinTestProperty(ownerID)
	repo := newFakePropertyRepo(property)
	svc := pinTestService(t, repo, testOwnerPolicy{})

	updated, err := svc.SetPropertyPin(ctx, ownerID, property.ID, true)
	if err != nil {
		t.Fatalf("set pin: %v", err)
	}
	if updated.PinnedAt == nil || !updated.PinnedAt.Equal(pinClockTime) {
		t.Errorf("updated.PinnedAt = %v, want the clock time %v", updated.PinnedAt, pinClockTime)
	}
	if updated.AccessRole != sharedpolicy.RoleOwner {
		t.Errorf("updated.AccessRole = %q, want owner", updated.AccessRole)
	}
	stored, err := repo.ActorPin(ctx, ownerID, property.ID)
	if err != nil {
		t.Fatalf("read actor pin: %v", err)
	}
	if stored == nil || !stored.Equal(pinClockTime) {
		t.Errorf("stored pin = %v, want the clock time %v", stored, pinClockTime)
	}
}

func TestPropertyService_SetPropertyPin_RepinKeepsOriginalTime(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	firstPin := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	property := pinTestProperty(ownerID)
	repo := newFakePropertyRepo(property)
	repo.seedPin(ownerID, property.ID, firstPin)
	svc := pinTestService(t, repo, testOwnerPolicy{})

	updated, err := svc.SetPropertyPin(ctx, ownerID, property.ID, true)
	if err != nil {
		t.Fatalf("re-pin: %v", err)
	}
	if updated.PinnedAt == nil || !updated.PinnedAt.Equal(firstPin) {
		t.Errorf("updated.PinnedAt = %v, want the original %v", updated.PinnedAt, firstPin)
	}
	stored, err := repo.ActorPin(ctx, ownerID, property.ID)
	if err != nil {
		t.Fatalf("read actor pin: %v", err)
	}
	if stored == nil || !stored.Equal(firstPin) {
		t.Errorf("stored pin = %v, want the untouched original %v", stored, firstPin)
	}
}

func TestPropertyService_SetPropertyPin_UnpinDeletesRow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	property := pinTestProperty(ownerID)
	repo := newFakePropertyRepo(property)
	svc := pinTestService(t, repo, testOwnerPolicy{})

	if _, err := svc.SetPropertyPin(ctx, ownerID, property.ID, true); err != nil {
		t.Fatalf("pin: %v", err)
	}
	updated, err := svc.SetPropertyPin(ctx, ownerID, property.ID, false)
	if err != nil {
		t.Fatalf("unpin: %v", err)
	}
	if updated.PinnedAt != nil {
		t.Errorf("updated.PinnedAt = %v, want nil", updated.PinnedAt)
	}
	stored, err := repo.ActorPin(ctx, ownerID, property.ID)
	if err != nil {
		t.Fatalf("read actor pin: %v", err)
	}
	if stored != nil {
		t.Errorf("stored pin = %v, want the row gone", stored)
	}
}

func TestPropertyService_SetPropertyPin_UnpinWithoutRowIsNoop(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	property := pinTestProperty(ownerID)
	repo := newFakePropertyRepo(property)
	svc := pinTestService(t, repo, testOwnerPolicy{})

	updated, err := svc.SetPropertyPin(ctx, ownerID, property.ID, false)
	if err != nil {
		t.Fatalf("unpin without a pin row: %v", err)
	}
	if updated.PinnedAt != nil {
		t.Errorf("updated.PinnedAt = %v, want nil", updated.PinnedAt)
	}
}

func TestPropertyService_SetPropertyPin_ViewerCanPin(t *testing.T) {
	t.Parallel()

	// D11 #1307: пин — личное состояние по праву просмотра; зритель
	// закрепляет объект в своём порядке (было ErrForbidden на CanEdit).
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	property := pinTestProperty(ownerID)
	repo := newFakePropertyRepo(property)
	svc := pinTestService(t, repo, staticRolePolicy{role: sharedpolicy.RoleViewer})

	updated, err := svc.SetPropertyPin(ctx, ownerID, property.ID, true)
	if err != nil {
		t.Fatalf("viewer pin: %v", err)
	}
	if updated.PinnedAt == nil || !updated.PinnedAt.Equal(pinClockTime) {
		t.Errorf("updated.PinnedAt = %v, want the clock time", updated.PinnedAt)
	}
}

func TestPropertyService_SetPropertyPin_FullAccessCanPin(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	property := pinTestProperty(ownerID)
	repo := newFakePropertyRepo(property)
	svc := pinTestService(t, repo, staticRolePolicy{role: sharedpolicy.RoleFullAccess})

	if _, err := svc.SetPropertyPin(ctx, ownerID, property.ID, true); err != nil {
		t.Fatalf("full access pin: %v", err)
	}
}

func TestPropertyService_SetPropertyPin_NoRoleNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	property := pinTestProperty(ownerID)
	repo := newFakePropertyRepo(property)
	svc := pinTestService(t, repo, staticRolePolicy{role: sharedpolicy.RoleNone})

	if _, err := svc.SetPropertyPin(ctx, ownerID, property.ID, true); !errors.Is(err, ErrNotFound) {
		t.Errorf("stranger pin err = %v, want ErrNotFound", err)
	}
}

func TestPropertyService_SetPropertyPin_SuspendedNotFound(t *testing.T) {
	t.Parallel()

	// Suspended-членство — privacy-404 и на пине: существование объекта
	// не раскрывается (спека #1307; сигнал ErrAccessSuspended живёт только
	// на чтении детали).
	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	property := pinTestProperty(ownerID)
	repo := newFakePropertyRepo(property)
	svc := pinTestService(t, repo, staticRolePolicy{role: sharedpolicy.RoleSuspended})

	if _, err := svc.SetPropertyPin(ctx, ownerID, property.ID, true); !errors.Is(err, ErrNotFound) {
		t.Errorf("suspended pin err = %v, want ErrNotFound", err)
	}
}

func TestPropertyService_SetPropertyPin_ArchivedConflict(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	property := pinTestProperty(ownerID)
	property.Status = domain.PropertyStatusArchived
	repo := newFakePropertyRepo(property)
	svc := pinTestService(t, repo, testOwnerPolicy{})

	if _, err := svc.SetPropertyPin(ctx, ownerID, property.ID, true); !errors.Is(err, ErrArchivedProperty) {
		t.Errorf("archived pin err = %v, want ErrArchivedProperty", err)
	}
}

// TestPropertyService_SetPropertyPin_WritesNoHistory pins the D17 silence:
// the personal pin state journals nothing — pin, idempotent re-pin and
// unpin all leave the action journal untouched.
func TestPropertyService_SetPropertyPin_WritesNoHistory(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	property := pinTestProperty(ownerID)
	repo := newFakePropertyRepo(property)
	history := &historytest.CapturingRecorder{}
	svc := pinTestHistoryService(t, repo, history)

	if _, err := svc.SetPropertyPin(ctx, ownerID, property.ID, true); err != nil {
		t.Fatalf("pin: %v", err)
	}
	if _, err := svc.SetPropertyPin(ctx, ownerID, property.ID, true); err != nil {
		t.Fatalf("re-pin: %v", err)
	}
	if _, err := svc.SetPropertyPin(ctx, ownerID, property.ID, false); err != nil {
		t.Fatalf("unpin: %v", err)
	}
	if len(history.Entries) != 0 {
		t.Errorf("journal entries = %d, want 0 (D17: личный пин не пишется в историю)", len(history.Entries))
	}
}

// TestPropertyService_SetPropertyPin_PublishesNoFrame pins the D18 silence:
// the personal pin state dispatches no realtime frame — even the actor's
// own tabs re-read on their next load instead of an invalidation.
func TestPropertyService_SetPropertyPin_PublishesNoFrame(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	property := pinTestProperty(ownerID)
	repo := newFakePropertyRepo(property)
	svc := pinTestService(t, repo, testOwnerPolicy{})
	realtime := &realtimetest.RecordingPublisher{}
	svc.SetRealtimePublisher(realtime)

	if _, err := svc.SetPropertyPin(ctx, ownerID, property.ID, true); err != nil {
		t.Fatalf("pin: %v", err)
	}
	if _, err := svc.SetPropertyPin(ctx, ownerID, property.ID, false); err != nil {
		t.Fatalf("unpin: %v", err)
	}
	if len(realtime.Publications) != 0 {
		t.Errorf("realtime publications = %d, want 0 (D18: личное состояние фреймом не публикуется)", len(realtime.Publications))
	}
}

// pinTwoActorsPolicy distinguishes the owner from the member: the member
// holds the viewer role on the property, the owner the owner role — the
// per-actor pin isolation's policy double.
type pinTwoActorsPolicy struct {
	owner  uuid.UUID
	member uuid.UUID
}

func (p pinTwoActorsPolicy) Role(ctx context.Context, actor, _ uuid.UUID) (sharedpolicy.Role, error) {
	return p.RoleForProperty(ctx, actor, uuid.Nil)
}

func (p pinTwoActorsPolicy) RoleForProperty(_ context.Context, actor, _ uuid.UUID) (sharedpolicy.Role, error) {
	if actor == p.owner {
		return sharedpolicy.RoleOwner, nil
	}
	if actor == p.member {
		return sharedpolicy.RoleViewer, nil
	}
	return sharedpolicy.RoleNone, nil
}

// TestPropertyService_SetPropertyPin_IsPerActor pins the D11/D14 isolation
// regression: the member's pin reorders the member's list and leaves the
// owner's list untouched — a personal state, not a property flag.
func TestPropertyService_SetPropertyPin_IsPerActor(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	memberID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	first := domain.Property{
		ID:      uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		OwnerID: ownerID,
		Name:    "First",
		Address: testPropertyAddress,
		Type:    domain.PropertyTypeApartment,
		Status:  domain.PropertyStatusActive,
	}
	second := domain.Property{
		ID:      uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		OwnerID: ownerID,
		Name:    "Second",
		Address: testPropertyAddress,
		Type:    domain.PropertyTypeApartment,
		Status:  domain.PropertyStatusActive,
	}

	policy := pinTwoActorsPolicy{owner: ownerID, member: memberID}
	repo := &orderedListRepo{
		fakePropertyRepo: *newFakePropertyRepo(first, second),
		list:             []domain.Property{first, second},
	}
	svc := pinTestService(t, repo, policy)

	if _, err := svc.SetPropertyPin(ctx, memberID, second.ID, true); err != nil {
		t.Fatalf("member pin: %v", err)
	}

	memberList, err := svc.ListProperties(ctx, memberID)
	if err != nil {
		t.Fatalf("member list: %v", err)
	}
	if len(memberList.Items) != 2 || memberList.Items[0].ID != second.ID || memberList.Items[1].ID != first.ID {
		t.Fatalf("member list = [%s, %s], want the member's pin first",
			memberList.Items[0].ID, memberList.Items[1].ID)
	}

	ownerList, err := svc.ListProperties(ctx, ownerID)
	if err != nil {
		t.Fatalf("owner list: %v", err)
	}
	if len(ownerList.Items) != 2 || ownerList.Items[0].ID != first.ID || ownerList.Items[1].ID != second.ID {
		t.Fatalf("owner list = [%s, %s], want the untouched order (D11: чужой пин мой экран не меняет)",
			ownerList.Items[0].ID, ownerList.Items[1].ID)
	}
}

// orderedListRepo returns a fixed order from ListActiveByOwner (the map-based
// fake's order is random) so the list-sorting test is deterministic.
type orderedListRepo struct {
	fakePropertyRepo
	list []domain.Property
}

func (r *orderedListRepo) ListActiveByOwner(_ context.Context, _ uuid.UUID) ([]domain.Property, error) {
	return append([]domain.Property(nil), r.list...), nil
}

var _ PropertyRepository = (*orderedListRepo)(nil)

// TestPropertyService_ListProperties_PinnedFirstStable pins rise above the
// unpinned in the merged list — among themselves by the actor's pin time,
// the unpinned keeping the repository's order (ticket #577; the pin travels
// from the actor's personal rows since the #1307 cutover — a shared
// property the actor pinned rises the same way).
func TestPropertyService_ListProperties_PinnedFirstStable(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ownerID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	sharedOwnerID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	pinA := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	pinB := time.Date(2026, 9, 2, 8, 0, 0, 0, time.UTC)
	unpinned := domain.Property{
		ID:      uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		OwnerID: ownerID,
		Name:    "Unpinned",
		Address: testPropertyAddress,
		Type:    domain.PropertyTypeApartment,
		Status:  domain.PropertyStatusActive,
	}
	pinnedA := domain.Property{
		ID:      uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		OwnerID: ownerID,
		Name:    "Pinned First",
		Address: testPropertyAddress,
		Type:    domain.PropertyTypeApartment,
		Status:  domain.PropertyStatusActive,
	}
	sharedPinnedB := domain.Property{
		ID:      uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		OwnerID: sharedOwnerID,
		Name:    "Shared Pinned",
		Address: testPropertyAddress,
		Type:    domain.PropertyTypeApartment,
		Status:  domain.PropertyStatusActive,
	}

	repo := &orderedListRepo{
		fakePropertyRepo: *newFakePropertyRepo(unpinned, pinnedA, sharedPinnedB),
		list:             []domain.Property{unpinned, pinnedA},
	}
	repo.seedPin(ownerID, pinnedA.ID, pinA)
	repo.seedPin(ownerID, sharedPinnedB.ID, pinB)
	svc := pinTestService(t, repo, testOwnerPolicy{})
	svc.SetSharedMemberships(fakeSharedMemberships{memberships: []SharedMembership{
		{PropertyID: sharedPinnedB.ID, Role: sharedpolicy.RoleFullAccess},
	}})

	list, err := svc.ListProperties(ctx, ownerID)
	if err != nil {
		t.Fatalf("list properties: %v", err)
	}
	want := []uuid.UUID{pinnedA.ID, sharedPinnedB.ID, unpinned.ID}
	if len(list.Items) != len(want) {
		t.Fatalf("list len = %d, want %d", len(list.Items), len(want))
	}
	for i, id := range want {
		if list.Items[i].ID != id {
			t.Errorf("list[%d] = %s, want %s", i, list.Items[i].ID, id)
		}
	}
	// The pinned rows carry the actor's pinnedAt from the overlay; the
	// unpinned row stays nil.
	if list.Items[0].PinnedAt == nil || !list.Items[0].PinnedAt.Equal(pinA) {
		t.Errorf("list[0].PinnedAt = %v, want %v", list.Items[0].PinnedAt, pinA)
	}
	if list.Items[1].PinnedAt == nil || !list.Items[1].PinnedAt.Equal(pinB) {
		t.Errorf("list[1].PinnedAt = %v, want %v", list.Items[1].PinnedAt, pinB)
	}
	if list.Items[2].PinnedAt != nil {
		t.Errorf("list[2].PinnedAt = %v, want nil", list.Items[2].PinnedAt)
	}
}
