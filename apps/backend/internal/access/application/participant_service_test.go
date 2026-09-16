package application

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
)

// testPendingEmail is the pending invitee email fixture of the service tests.
const testPendingEmail = "pending@x.ru"

// fakeParticipantRead is the func-free ParticipantReadModel double: fixed
// fixtures, ADR 0035.
type fakeParticipantRead struct {
	props       []ParticipantScopeProperty
	members     []ParticipantMembershipRow
	invitations []ParticipantInvitationRow
}

func (f *fakeParticipantRead) ListScopeProperties(ctx context.Context, actorID uuid.UUID) ([]ParticipantScopeProperty, error) {
	return f.props, nil
}

func (f *fakeParticipantRead) ListMembershipsByProperties(
	ctx context.Context, propertyIDs []uuid.UUID,
) ([]ParticipantMembershipRow, error) {
	return f.members, nil
}

func (f *fakeParticipantRead) ListInvitationsByProperties(
	ctx context.Context, propertyIDs []uuid.UUID,
) ([]ParticipantInvitationRow, error) {
	return f.invitations, nil
}

type fakeParticipantUsers struct {
	users map[uuid.UUID]MemberUser
}

func (f *fakeParticipantUsers) GetByID(ctx context.Context, id uuid.UUID) (MemberUser, error) {
	u, ok := f.users[id]
	if !ok {
		return MemberUser{}, domain.ErrUserNotFound
	}
	return u, nil
}

func (f *fakeParticipantUsers) GetByEmail(ctx context.Context, email string) (MemberUser, error) {
	return MemberUser{}, domain.ErrUserNotFound
}

type fakeParticipantEmails struct {
	emails map[uuid.UUID]string
}

func (f *fakeParticipantEmails) GetEmail(ctx context.Context, userID uuid.UUID) (string, error) {
	return f.emails[userID], nil
}

type fakeAccessibleCounter struct {
	count int
}

func (f *fakeAccessibleCounter) CountActiveByUser(ctx context.Context, userID uuid.UUID) (int, error) {
	return f.count, nil
}

// participantFixture wires a service over two scoped properties of owner O:
// U1 holds active access to both, U2 is suspended on P1, E is a pending
// invitation to P2.
type participantFixture struct {
	svc    *ParticipantService
	read   *fakeParticipantRead
	scope  []ParticipantScopeProperty
	owner  uuid.UUID
	u1     uuid.UUID
	u2     uuid.UUID
	p1, p2 uuid.UUID
}

func newParticipantFixture(t *testing.T) *participantFixture {
	t.Helper()
	f := &participantFixture{
		owner: uuid.Must(uuid.NewV7()),
		u1:    uuid.Must(uuid.NewV7()),
		u2:    uuid.Must(uuid.NewV7()),
	}
	f.p1, f.p2 = uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	f.scope = []ParticipantScopeProperty{
		{ID: f.p1, OwnerID: f.owner, Title: "Вторая"},
		{ID: f.p2, OwnerID: f.owner, Title: "Первая"},
	}
	fullOnFirst := ParticipantMembershipRow{
		PropertyID: f.p1, UserID: f.u1, Role: domain.RoleFullAccess, Status: domain.MemberStatusActive,
	}
	viewOnSecond := ParticipantMembershipRow{
		PropertyID: f.p2, UserID: f.u1, Role: domain.RoleViewer, Status: domain.MemberStatusActive,
	}
	suspendedOnFirst := ParticipantMembershipRow{
		PropertyID: f.p1, UserID: f.u2, Role: domain.RoleFullAccess, Status: domain.MemberStatusSuspended,
	}
	f.read = &fakeParticipantRead{
		props: f.scope,
		members: []ParticipantMembershipRow{
			fullOnFirst,
			viewOnSecond,
			suspendedOnFirst,
		},
		invitations: []ParticipantInvitationRow{
			{PropertyID: f.p2, Email: testPendingEmail, Role: domain.RoleViewer},
		},
	}
	name := "Иван"
	surname := "Иванов"
	phone := "+79991234567" // Masks to «+7*******67» in the display name.
	f.svc = NewParticipantService(
		f.read,
		&fakeParticipantUsers{users: map[uuid.UUID]MemberUser{
			f.u1: {ID: f.u1, Name: &name, Surname: &surname},
			f.u2: {ID: f.u2, Phone: phone},
		}},
		&fakeParticipantEmails{emails: map[uuid.UUID]string{f.u1: "u1@x.ru"}},
		&fakeAccessibleCounter{count: 1},
		slog.New(slog.DiscardHandler),
	)
	return f
}

// indexParticipants keys the list by user id and by email for the assertions.
func indexParticipants(list []Participant) (byUserID map[uuid.UUID]Participant, byEmailAddress map[string]Participant) {
	byUser := map[uuid.UUID]Participant{}
	byEmailRow := map[string]Participant{}
	for _, p := range list {
		if p.UserID != (uuid.UUID{}) {
			byUser[p.UserID] = p
		}
		if p.Email != "" {
			byEmailRow[p.Email] = p
		}
	}
	return byUser, byEmailRow
}

// assertNamedUserAggregate checks the fully-named user's row: the all-
// properties badge, the display data and the per-object legs with titles.
func assertNamedUserAggregate(t *testing.T, u1 Participant) {
	t.Helper()
	if u1.AggregateStatus != domain.ParticipantStatusAllProperties || u1.AccessibleCount != 2 {
		t.Errorf("u1 aggregate = %s/%d, want all_properties/2", u1.AggregateStatus, u1.AccessibleCount)
	}
	if u1.DisplayName != "Иван Иванов" || u1.Email != "u1@x.ru" {
		t.Errorf("u1 display = %q email = %q", u1.DisplayName, u1.Email)
	}
	if len(u1.Properties) != 2 {
		t.Fatalf("u1 properties = %d, want 2", len(u1.Properties))
	}
	// Property legs carry the scope's titles and per-object roles.
	byTitle := map[string]ParticipantProperty{}
	for _, pp := range u1.Properties {
		byTitle[pp.Title] = pp
	}
	if pp := byTitle["Первая"]; pp.Role != domain.RoleViewer || pp.Status != domain.ParticipantEntryActive {
		t.Errorf("u1 on «Первая» = %s/%s", pp.Role, pp.Status)
	}
}

// TestListParticipants builds the aggregate list: per-person grouping, per-
// object entries with titles, the aggregate status rule and the display data.
func TestListParticipants(t *testing.T) {
	t.Parallel()
	f := newParticipantFixture(t)

	got, err := f.svc.ListParticipants(context.Background(), f.owner)
	if err != nil {
		t.Fatalf("ListParticipants: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 participants (2 users + 1 pending email), got %d: %+v", len(got), got)
	}

	byUser, byEmailRow := indexParticipants(got)
	assertNamedUserAggregate(t, byUser[f.u1])

	u2 := byUser[f.u2]
	if u2.AggregateStatus != domain.ParticipantStatusLimitExceeded || u2.AccessibleCount != 0 {
		t.Errorf("u2 aggregate = %s/%d, want limit_exceeded/0", u2.AggregateStatus, u2.AccessibleCount)
	}

	pending := byEmailRow[testPendingEmail]
	if pending.UserID != (uuid.UUID{}) || pending.Email != testPendingEmail {
		t.Errorf("pending participant = %+v, want zero user id and the invitee email", pending)
	}
	if pending.AggregateStatus != domain.ParticipantStatusPartial || pending.AccessibleCount != 0 {
		t.Errorf("pending aggregate = %s/%d, want partial/0", pending.AggregateStatus, pending.AccessibleCount)
	}
}

// TestListParticipants_ManagingActorIsNotTheirOwnParticipant checks the
// contract «the actor themself never appears»: a full_access manager reading
// the list is scoped to the object they manage but is not listed among the
// participants — neither in the list nor addressable by their own uuid
// (issue #693).
func TestListParticipants_ManagingActorIsNotTheirOwnParticipant(t *testing.T) {
	t.Parallel()
	f := newParticipantFixture(t)
	ctx := context.Background()

	// Fixture user u1 is a full_access member on p1; their scope is that object.
	got, err := f.svc.ListParticipants(ctx, f.u1)
	if err != nil {
		t.Fatalf("ListParticipants: %v", err)
	}
	byUser, _ := indexParticipants(got)
	if _, ok := byUser[f.u1]; ok {
		t.Errorf("the reading actor appeared in their own participant list: %+v", got)
	}
	// The suspended member of the managed object is still there.
	if _, ok := byUser[f.u2]; !ok {
		t.Errorf("u2 missing from the u1-scoped list: %+v", got)
	}

	if _, err = f.svc.GetParticipant(ctx, f.u1, f.u1.String()); err == nil {
		t.Errorf("GetParticipant(self) = nil error, want ErrParticipantNotFound")
	}
}

// TestListParticipants_SortedByDisplayName checks the deterministic list
// order: display name ascending, pending email rows after the named people.
func TestListParticipants_SortedByDisplayName(t *testing.T) {
	t.Parallel()
	f := newParticipantFixture(t)

	got, err := f.svc.ListParticipants(context.Background(), f.owner)
	if err != nil {
		t.Fatalf("ListParticipants: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 participants, got %d", len(got))
	}
	// Byte-wise display-name order (the client re-sorts for its «Имя» chip):
	// the masked-phone row first, then the named user, pending emails last.
	if got[0].DisplayName != "+7********67" {
		t.Errorf("first row = %q, want the masked-phone user", got[0].DisplayName)
	}
	if got[1].UserID != f.u1 || got[1].DisplayName != "Иван Иванов" {
		t.Errorf("second row = %q/%q, want the named user", got[1].DisplayName, got[1].Email)
	}
	if got[2].Email != testPendingEmail || got[2].DisplayName != "" {
		t.Errorf("last row = %q/%q, want the pending email", got[2].DisplayName, got[2].Email)
	}
}

// TestListParticipants_DuplicateMembershipRows checks the active-wins collapse:
// an (active, suspended) pair of rows on one (property, user) yields a single
// active entry (the partial unique index tolerates both rows in the DB).
func TestListParticipants_DuplicateMembershipRows(t *testing.T) {
	t.Parallel()
	f := newParticipantFixture(t)
	f.read.members = append(f.read.members,
		ParticipantMembershipRow{PropertyID: f.p1, UserID: f.u2, Role: domain.RoleFullAccess, Status: domain.MemberStatusActive},
	)

	got, err := f.svc.ListParticipants(context.Background(), f.owner)
	if err != nil {
		t.Fatalf("ListParticipants: %v", err)
	}
	var u2 *Participant
	for i := range got {
		if got[i].UserID == f.u2 {
			u2 = &got[i]
		}
	}
	if u2 == nil {
		t.Fatalf("u2 missing in %+v", got)
	}
	if len(u2.Properties) != 1 || u2.Properties[0].Status != domain.ParticipantEntryActive {
		t.Fatalf("u2 properties = %+v, want a single active entry", u2.Properties)
	}
}

// TestListParticipants_PendingEmailMergesIntoRegisteredUser checks that a
// leftover pending invitation whose email already belongs to a registered
// participant collapses into that user's aggregate.
func TestListParticipants_PendingEmailMergesIntoRegisteredUser(t *testing.T) {
	t.Parallel()
	f := newParticipantFixture(t)
	f.read.invitations = append(f.read.invitations,
		ParticipantInvitationRow{PropertyID: f.p1, Email: "U1@X.RU", Role: domain.RoleViewer},
	)

	got, err := f.svc.ListParticipants(context.Background(), f.owner)
	if err != nil {
		t.Fatalf("ListParticipants: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected the pending row to merge, got %d participants: %+v", len(got), got)
	}
	var u1 *Participant
	for i := range got {
		if got[i].UserID == f.u1 {
			u1 = &got[i]
		}
	}
	if u1 == nil || len(u1.Properties) != 2 {
		t.Fatalf("u1 = %+v, want a single aggregate with both legs", u1)
	}
}

// TestListParticipants_EmptyScope checks that an actor with no scoped
// properties gets an empty list, not an error.
func TestListParticipants_EmptyScope(t *testing.T) {
	t.Parallel()
	f := newParticipantFixture(t)
	f.read.props = nil

	got, err := f.svc.ListParticipants(context.Background(), f.owner)
	if err != nil {
		t.Fatalf("ListParticipants: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected an empty list, got %+v", got)
	}
}

// TestGetParticipant covers the three identifier forms of the participant
// page: a registered user's uuid, a pending invitee email (case-insensitive),
// and the privacy-preserving not-found for everything else (issue #698's
// deep-link 404 policy).
func TestGetParticipant(t *testing.T) {
	t.Parallel()
	f := newParticipantFixture(t)
	ctx := context.Background()

	u1, err := f.svc.GetParticipant(ctx, f.owner, f.u1.String())
	if err != nil {
		t.Fatalf("GetParticipant by uuid: %v", err)
	}
	if u1.UserID != f.u1 || u1.AggregateStatus != domain.ParticipantStatusAllProperties {
		t.Errorf("u1 = %+v", u1)
	}

	pending, err := f.svc.GetParticipant(ctx, f.owner, "Pending@X.RU")
	if err != nil {
		t.Fatalf("GetParticipant by email: %v", err)
	}
	if pending.Email != testPendingEmail {
		t.Errorf("pending = %+v", pending)
	}

	if _, err = f.svc.GetParticipant(ctx, f.owner, uuid.Must(uuid.NewV7()).String()); !errors.Is(err, domain.ErrParticipantNotFound) {
		t.Errorf("unknown uuid: err = %v, want ErrParticipantNotFound", err)
	}
	if _, err = f.svc.GetParticipant(ctx, f.owner, "not-an-id"); !errors.Is(err, domain.ErrParticipantNotFound) {
		t.Errorf("garbage id: err = %v, want ErrParticipantNotFound", err)
	}
	// A registered user's email resolves to the same aggregate as their uuid.
	byEmail, err := f.svc.GetParticipant(ctx, f.owner, "u1@x.ru")
	if err != nil {
		t.Fatalf("GetParticipant by registered email: %v", err)
	}
	if byEmail.UserID != f.u1 {
		t.Errorf("by-email = %+v, want u1", byEmail)
	}
}

// TestParticipantsSummary checks the hub counters: the participants count from
// the aggregates and the shared-properties count from the recipient-slot
// counter of the reading actor.
func TestParticipantsSummary(t *testing.T) {
	t.Parallel()
	f := newParticipantFixture(t)

	got, err := f.svc.Summary(context.Background(), f.owner)
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if got.ParticipantsCount != 3 {
		t.Errorf("ParticipantsCount = %d, want 3", got.ParticipantsCount)
	}
	if got.AccessiblePropertiesCount != 1 {
		t.Errorf("AccessiblePropertiesCount = %d, want 1", got.AccessiblePropertiesCount)
	}
}
