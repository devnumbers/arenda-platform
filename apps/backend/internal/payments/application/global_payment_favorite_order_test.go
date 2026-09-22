package application

// The favorites manual order's unit tests (ticket #576): the save use case's
// application-layer contract — dense positions in the list order, the
// full-replacement clear of the unplaced favorites, the deadlock-safe lock
// order, the duplicate/foreign/non-favorite/viewer verdicts and the audit
// entry — against func-backed fakes; the visibility predicate, the per-row
// role and the positions themselves are the store's (integration tests).

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// capturingAudit records the entries the save writes so the test asserts the
// audit trail without a database.
type capturingAudit struct {
	entries []auditdomain.Entry
}

func (c *capturingAudit) Record(_ context.Context, entry auditdomain.Entry) error {
	c.entries = append(c.entries, entry)
	return nil
}

func (c *capturingAudit) WithTx(transaction.Tx) auditapp.Recorder { return c }

// fakeFavoriteOrderStore plays GlobalPaymentOrderStore: the lock answers
// with canned rows and captures the actor and the ids it was asked to lock;
// the position writes land in a map the assertions read back and the
// clear-outside pass records its keep list.
type fakeFavoriteOrderStore struct {
	rows      []GlobalPaymentFavoriteLock
	err       error
	gotActor  uuid.UUID
	gotIDs    []uuid.UUID
	positions map[uuid.UUID]int64
	gotKeep   []uuid.UUID
	cleared   bool
}

func newFakeFavoriteOrderStore(rows []GlobalPaymentFavoriteLock) *fakeFavoriteOrderStore {
	return &fakeFavoriteOrderStore{
		rows:      rows,
		positions: map[uuid.UUID]int64{},
	}
}

func (f *fakeFavoriteOrderStore) LockVisibleFavorites(
	_ context.Context, actor uuid.UUID, ids []uuid.UUID,
) ([]GlobalPaymentFavoriteLock, error) {
	f.gotActor = actor
	f.gotIDs = ids
	if f.err != nil {
		return nil, f.err
	}
	return f.rows, nil
}

func (f *fakeFavoriteOrderStore) SaveFavoritePosition(_ context.Context, id uuid.UUID, position int64) error {
	f.positions[id] = position
	return nil
}

func (f *fakeFavoriteOrderStore) ClearFavoriteOrdersOutside(
	_ context.Context, _ uuid.UUID, keepIDs []uuid.UUID,
) error {
	f.cleared = true
	f.gotKeep = keepIDs
	return nil
}

func (f *fakeFavoriteOrderStore) WithTx(transaction.Tx) (GlobalPaymentOrderStore, error) {
	return f, nil
}

// newOrderService builds the global service over the given reader and order
// store with the noop seats filled, a fake UoW and a capturing audit; the
// harness returns both fakes the assertions read.
func newOrderService(
	reader GlobalPaymentReader, orders *fakeFavoriteOrderStore, audit *capturingAudit,
) (*GlobalPaymentService, *fakeUoW) {
	uow := &fakeUoW{}
	factory := NewTxStoreFactory(
		&fakeTickStore{}, noopPaymentStore{}, noopOperationStore{}, noopPropertyStore{},
		orders, audit, nil, uow,
	)
	calendar := fakeGlobalCalendar{todays: map[uuid.UUID]time.Time{
		uuid.Must(uuid.NewV7()): utcDate(2026, time.September, 8),
	}}
	return NewGlobalPaymentService(reader, calendar, factory), uow
}

// favoriteLock builds one locked owner-scope row.
func favoriteLock(id uuid.UUID) GlobalPaymentFavoriteLock {
	return GlobalPaymentFavoriteLock{ID: id, IsFavorite: true, Role: sharedpolicy.RoleOwner}
}

// The happy path (ticket #576): the submitted list order becomes dense
// 1..N positions; the lock runs on the sorted ids (the deadlock-safe order)
// for the actor; the clear pass keeps exactly the submitted ids; one audit
// entry in the actor's own scope describes the mass write.
func TestSaveFavoriteOrderAssignsDensePositions(t *testing.T) {
	t.Parallel()

	actor := uuid.Must(uuid.NewV7())
	idA := uuid.Must(uuid.NewV7())
	idB := uuid.Must(uuid.NewV7())
	idC := uuid.Must(uuid.NewV7())
	orders := newFakeFavoriteOrderStore([]GlobalPaymentFavoriteLock{
		favoriteLock(idA), favoriteLock(idB), favoriteLock(idC),
	})
	var audit capturingAudit
	service, uow := newOrderService(&fakeGlobalReader{}, orders, &audit)

	err := service.SaveFavoriteOrder(t.Context(), actor, []uuid.UUID{idC, idA, idB})
	if err != nil {
		t.Fatalf("SaveFavoriteOrder: %v", err)
	}
	if uow.runs != 1 {
		t.Fatalf("tx runs = %d, want 1 (atomic save)", uow.runs)
	}
	for id, position := range map[uuid.UUID]int64{idC: 1, idA: 2, idB: 3} {
		if got := orders.positions[id]; got != position {
			t.Errorf("position of %s = %d, want %d", id, got, position)
		}
	}
	if len(orders.positions) != 3 {
		t.Errorf("positions written = %d, want 3", len(orders.positions))
	}
	assertSortedLock(t, orders.gotIDs)
	if orders.gotActor != actor {
		t.Errorf("lock actor = %s, want %s", orders.gotActor, actor)
	}
	if !orders.cleared || len(orders.gotKeep) != 3 {
		t.Fatalf("clear-outside keep = (%v, %v), want the three placed ids", orders.cleared, orders.gotKeep)
	}
	if orders.gotKeep[0].String() >= orders.gotKeep[1].String() || orders.gotKeep[1].String() >= orders.gotKeep[2].String() {
		t.Errorf("keep ids = %v, want sorted", orders.gotKeep)
	}
	assertOrderAudit(t, audit.entries, actor, sharedpolicy.AuditActorRole(sharedpolicy.RoleOwner))
}

// assertSortedLock checks the lock ran on the submitted ids in their sorted
// order — the deadlock-safety.
func assertSortedLock(t *testing.T, ids []uuid.UUID) {
	t.Helper()
	if len(ids) != 3 {
		t.Fatalf("lock ids = %v, want 3", ids)
	}
	if ids[0].String() >= ids[1].String() || ids[1].String() >= ids[2].String() {
		t.Errorf("lock ids = %v, want sorted", ids)
	}
}

// assertOrderAudit checks the save's single audit entry: the payment's
// updated action with no single entity, in the given acting role.
func assertOrderAudit(t *testing.T, entries []auditdomain.Entry, actor uuid.UUID, role auditdomain.ActorRole) {
	t.Helper()
	if len(entries) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.Action != auditdomain.ActionPaymentUpdated || entry.EntityType != auditdomain.EntityPayment {
		t.Errorf("audit = (%s, %s), want the payment's updated action", entry.Action, entry.EntityType)
	}
	if entry.EntityID != nil {
		t.Errorf("audit entity id = %s, want nil (no single entity)", *entry.EntityID)
	}
	if entry.ActorID == nil || *entry.ActorID != actor {
		t.Errorf("audit actor = %v, want %s", entry.ActorID, actor)
	}
	if entry.ActorRole != role {
		t.Errorf("audit role = %s, want %s", entry.ActorRole, role)
	}
}

// A duplicate id is invalid input — the client's list is broken; nothing
// runs, nothing writes.
func TestSaveFavoriteOrderRejectsDuplicates(t *testing.T) {
	t.Parallel()

	id := uuid.Must(uuid.NewV7())
	orders := newFakeFavoriteOrderStore([]GlobalPaymentFavoriteLock{favoriteLock(id)})
	var audit capturingAudit
	service, uow := newOrderService(&fakeGlobalReader{}, orders, &audit)

	err := service.SaveFavoriteOrder(t.Context(), uuid.Must(uuid.NewV7()), []uuid.UUID{id, id})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if uow.runs != 0 || len(orders.positions) != 0 || len(audit.entries) != 0 {
		t.Errorf("side effects = (%d tx, %d positions, %d audit), want none",
			uow.runs, len(orders.positions), len(audit.entries))
	}
}

// A submitted id the lock cannot return — unknown, foreign or invisible —
// is the privacy-preserving 404; the transaction rolls back empty (the
// visibility answer is the lock itself) and nothing is written or leaked
// about which of the three it was.
func TestSaveFavoriteOrderRejectsInvisible(t *testing.T) {
	t.Parallel()

	visible := uuid.Must(uuid.NewV7())
	foreign := uuid.Must(uuid.NewV7())
	orders := newFakeFavoriteOrderStore([]GlobalPaymentFavoriteLock{favoriteLock(visible)})
	var audit capturingAudit
	service, uow := newOrderService(&fakeGlobalReader{}, orders, &audit)

	err := service.SaveFavoriteOrder(t.Context(), uuid.Must(uuid.NewV7()), []uuid.UUID{visible, foreign})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if len(orders.positions) != 0 || len(audit.entries) != 0 || orders.cleared {
		t.Errorf("side effects = (%d positions, %d audit, cleared %v), want none",
			len(orders.positions), len(audit.entries), orders.cleared)
	}
	if uow.runs != 1 {
		t.Errorf("tx runs = %d, want 1 (rolled back empty)", uow.runs)
	}
}

// A visible rule that is not a favorite cannot take a favorite position —
// the client's list is stale; invalid input, the transaction rolls back
// empty.
func TestSaveFavoriteOrderRejectsNonFavorite(t *testing.T) {
	t.Parallel()

	favorite := uuid.Must(uuid.NewV7())
	unstarred := uuid.Must(uuid.NewV7())
	orders := newFakeFavoriteOrderStore([]GlobalPaymentFavoriteLock{
		favoriteLock(favorite),
		{ID: unstarred, IsFavorite: false, Role: sharedpolicy.RoleOwner},
	})
	var audit capturingAudit
	service, uow := newOrderService(&fakeGlobalReader{}, orders, &audit)

	err := service.SaveFavoriteOrder(t.Context(), uuid.Must(uuid.NewV7()), []uuid.UUID{favorite, unstarred})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if len(orders.positions) != 0 || len(audit.entries) != 0 || orders.cleared {
		t.Errorf("side effects = (%d positions, %d audit, cleared %v), want none",
			len(orders.positions), len(audit.entries), orders.cleared)
	}
	if uow.runs != 1 {
		t.Errorf("tx runs = %d, want 1 (rolled back empty)", uow.runs)
	}
}

// The write gate is the favorite star's (#461, Full Access+): a viewer may
// read the favorites feed but not reorder it — forbidden.
func TestSaveFavoriteOrderRejectsViewer(t *testing.T) {
	t.Parallel()

	visible := uuid.Must(uuid.NewV7())
	orders := newFakeFavoriteOrderStore([]GlobalPaymentFavoriteLock{
		{ID: visible, IsFavorite: true, Role: sharedpolicy.RoleViewer},
	})
	var audit capturingAudit
	service, _ := newOrderService(&fakeGlobalReader{}, orders, &audit)

	err := service.SaveFavoriteOrder(t.Context(), uuid.Must(uuid.NewV7()), []uuid.UUID{visible})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
	if len(orders.positions) != 0 || len(audit.entries) != 0 || orders.cleared {
		t.Errorf("side effects = (%d positions, %d audit, cleared %v), want none",
			len(orders.positions), len(audit.entries), orders.cleared)
	}
}

// A save over the actor's own rules and a shared Full Access rule acts in
// the membership's role in the audit trail — not the owner's.
func TestSaveFavoriteOrderMixedScopesAuditRole(t *testing.T) {
	t.Parallel()

	own := uuid.Must(uuid.NewV7())
	shared := uuid.Must(uuid.NewV7())
	orders := newFakeFavoriteOrderStore([]GlobalPaymentFavoriteLock{
		favoriteLock(own),
		{ID: shared, IsFavorite: true, Role: sharedpolicy.RoleFullAccess},
	})
	var audit capturingAudit
	service, _ := newOrderService(&fakeGlobalReader{}, orders, &audit)

	err := service.SaveFavoriteOrder(t.Context(), uuid.Must(uuid.NewV7()), []uuid.UUID{own, shared})
	if err != nil {
		t.Fatalf("SaveFavoriteOrder: %v", err)
	}
	assertOrderAudit(t, audit.entries, orders.gotActor, sharedpolicy.AuditActorRole(sharedpolicy.RoleFullAccess))
}

// An empty list is the full-replacement degenerate: nothing is placed,
// the clear pass resets every visible favorite's order, one audit entry
// records the reset.
func TestSaveFavoriteOrderEmptyListResets(t *testing.T) {
	t.Parallel()

	orders := newFakeFavoriteOrderStore(nil)
	var audit capturingAudit
	service, uow := newOrderService(&fakeGlobalReader{}, orders, &audit)

	if err := service.SaveFavoriteOrder(t.Context(), uuid.Must(uuid.NewV7()), nil); err != nil {
		t.Fatalf("SaveFavoriteOrder: %v", err)
	}
	if uow.runs != 1 || !orders.cleared || len(orders.gotKeep) != 0 {
		t.Errorf("save = (%d tx, cleared %v, keep %v), want one clearing tx with an empty keep list",
			uow.runs, orders.cleared, orders.gotKeep)
	}
	if len(orders.positions) != 0 {
		t.Errorf("positions written = %d, want 0", len(orders.positions))
	}
}

// The feed rows carry the manual order out to the items (ticket #576): a
// saved position travels verbatim, a never-saved rule travels nil.
func TestListGlobalPaymentsCarriesFavoriteOrder(t *testing.T) {
	t.Parallel()

	owner := uuid.Must(uuid.NewV7())
	today := utcDate(2026, time.September, 8)
	position := int64(2)
	reader := &fakeGlobalReader{
		owners: []uuid.UUID{owner},
		rules: []GlobalPaymentRuleRow{
			{ID: uuid.Must(uuid.NewV7()), PropertyID: uuid.Must(uuid.NewV7()), Today: today, IsFavorite: true, FavoriteOrder: &position},
			{ID: uuid.Must(uuid.NewV7()), PropertyID: uuid.Must(uuid.NewV7()), Today: today, IsFavorite: true},
		},
	}
	var audit capturingAudit
	service, _ := newOrderService(reader, newFakeFavoriteOrderStore(nil), &audit)

	feed, err := service.ListGlobalPayments(t.Context(), uuid.Must(uuid.NewV7()))
	if err != nil {
		t.Fatalf("ListGlobalPayments: %v", err)
	}
	if got := feed.Items[0].FavoriteOrder; got == nil || *got != position {
		t.Errorf("first favoriteOrder = %v, want %d", got, position)
	}
	if got := feed.Items[1].FavoriteOrder; got != nil {
		t.Errorf("second favoriteOrder = %v, want nil (never saved)", got)
	}
}
