package application

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// This file covers the mutation side of the owner's participant aggregate
// (issue #694): the multi-object invite, «Пригласить в объект» and «Отозвать
// и удалить» — through the real ParticipantMutationService + AccessService +
// MembershipPolicy + SlotCoordinator on in-memory ports (the same seam as
// invitation_service_test.go).

// capturingRecorder is an audit Recorder double that keeps every entry for
// assertions.
type capturingRecorder struct {
	entries []auditdomain.Entry
}

func (r *capturingRecorder) Record(_ context.Context, entry auditdomain.Entry) error {
	r.entries = append(r.entries, entry)
	return nil
}

func (r *capturingRecorder) WithTx(transaction.Tx) auditapp.Recorder { return r }

// actions returns the recorded entries with the given audit action.
func (r *capturingRecorder) actions(action auditdomain.Action) []auditdomain.Entry {
	var out []auditdomain.Entry
	for _, e := range r.entries {
		if e.Action == action {
			out = append(out, e)
		}
	}
	return out
}

// mutationFixture bundles ParticipantMutationService with its in-memory
// dependencies.
type mutationFixture struct {
	repo        *memRepo
	invitations *memInvitationsRepo
	owners      staticResolver
	statuses    fakeStatuses
	lookup      *fakeLookup
	emails      fakeEmailResolver
	titles      fakePropertyTitles
	mailer      *fakeAccessMailer
	clk         *fixedClock
	limiter     *fakeRecipientLimiter
	audit       *capturingRecorder
	owner       uuid.UUID
	svc         *ParticipantMutationService
}

func newMutationFixture() *mutationFixture {
	repo := newMemRepo()
	invitations := &memInvitationsRepo{removalScope: repo.inManageScope}
	owners := staticResolver{}
	statuses := fakeStatuses{}
	policy := NewMembershipPolicy(owners, repo)
	lookup := newFakeLookup()
	emails := fakeEmailResolver{}
	titles := fakePropertyTitles{}
	mailer := &fakeAccessMailer{}
	clk := &fixedClock{now: time.Now()}
	limiter := newFakeRecipientLimiter()
	audit := &capturingRecorder{}
	coordinator := NewSlotCoordinator(repo, owners, limiter, newFakeOwnedProps(),
		audit, noopBeginner{})
	access := NewAccessService(repo, owners, statuses, lookup, policy, coordinator,
		newTestFactory(repo, invitations, audit), nil)
	svc := NewParticipantMutationService(access, owners, statuses, lookup, emails, policy,
		coordinator, mailer, titles, newTestFactory(repo, invitations, audit), clk, nil)
	return &mutationFixture{
		repo:        repo,
		invitations: invitations,
		owners:      owners,
		statuses:    statuses,
		lookup:      lookup,
		emails:      emails,
		titles:      titles,
		mailer:      mailer,
		clk:         clk,
		limiter:     limiter,
		audit:       audit,
		owner:       uuid.Must(uuid.NewV7()),
		svc:         svc,
	}
}

// addProperty registers an owned property with the owners resolvers and a
// display title.
func (f *mutationFixture) addProperty(propertyID, ownerID uuid.UUID, title string) {
	f.owners[propertyID] = ownerID
	f.repo.SetOwner(propertyID, ownerID)
	f.titles[propertyID] = title
}

func TestParticipantMutation_Invite_PendingGetsInvitationsAndOneEmail(t *testing.T) {
	t.Parallel()
	f := newMutationFixture()
	p1, p2 := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	f.addProperty(p1, f.owner, testFirstTitle)
	f.addProperty(p2, f.owner, testSecondTitle)

	results, err := f.svc.Invite(t.Context(), f.owner, testNewUserEmail, domain.RoleViewer,
		[]uuid.UUID{p1, p2})
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %+v, want two", results)
	}
	for i, want := range []uuid.UUID{p1, p2} {
		r := results[i]
		if r.PropertyID != want || r.Outcome != ParticipantGrantPending || r.InvitationID == uuid.Nil {
			t.Errorf("results[%d] = %+v, want pending for %s", i, r, want)
		}
	}

	assertBatchInvitations(t, f, []uuid.UUID{p1, p2})

	// Exactly one invite email listing every object of the batch.
	mail := f.mailer.only(t, "invite")
	if mail.to != testNewUserEmail {
		t.Errorf("mail to = %q", mail.to)
	}
	if !slices.Equal(mail.titles, []string{testFirstTitle, testSecondTitle}) {
		t.Errorf("mail titles = %v", mail.titles)
	}

	// An audit entry per created invitation row.
	if got := len(f.audit.actions(auditdomain.ActionPropertyMemberInvitationInvited)); got != 2 {
		t.Errorf("invitation audit entries = %d, want 2", got)
	}
}

// assertBatchInvitations checks that exactly one pending invitation per wanted
// property exists, carrying the batch role, the inviter and the send anchor.
func assertBatchInvitations(t *testing.T, f *mutationFixture, wantProps []uuid.UUID) {
	t.Helper()
	if len(f.invitations.rows) != len(wantProps) {
		t.Fatalf("invitations = %+v, want %d", f.invitations.rows, len(wantProps))
	}
	byProp := map[uuid.UUID]domain.Invitation{}
	for _, inv := range f.invitations.rows {
		byProp[inv.PropertyID] = inv
	}
	for _, prop := range wantProps {
		inv, ok := byProp[prop]
		if !ok {
			t.Errorf("no invitation for %s", prop)
			continue
		}
		if inv.Email != testNewUserEmail || inv.Role != domain.RoleViewer || inv.InvitedBy != f.owner {
			t.Errorf("invitation %s = %+v", prop, inv)
		}
		if !inv.LastSentAt.Equal(f.clk.now) {
			t.Errorf("last_sent_at = %v, want %v", inv.LastSentAt, f.clk.now)
		}
	}
}

func TestParticipantMutation_Invite_RegisteredMixedSlotResults(t *testing.T) {
	t.Parallel()
	f := newMutationFixture()
	p1, p2 := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	f.addProperty(p1, f.owner, testFirstTitle)
	f.addProperty(p2, f.owner, testSecondTitle)
	member := uuid.Must(uuid.NewV7())
	f.lookup.add(testMemberEmail, member)
	f.limiter.set(member, 1) // One free slot: the first grant fits, the second suspends.

	results, err := f.svc.Invite(t.Context(), f.owner, testMemberEmail, domain.RoleFullAccess,
		[]uuid.UUID{p1, p2})
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	if results[0].Outcome != ParticipantGrantActive || results[0].MembershipID == uuid.Nil {
		t.Errorf("results[0] = %+v, want active", results[0])
	}
	if results[1].Outcome != ParticipantGrantSuspended || results[1].MembershipID == uuid.Nil {
		t.Errorf("results[1] = %+v, want suspended", results[1])
	}
	first, err := f.repo.GetByPropertyAndUser(t.Context(), p1, member)
	if err != nil || first.Status != domain.MemberStatusActive {
		t.Errorf("p1 membership = %+v, %v", first, err)
	}
	second, err := f.repo.GetByPropertyAndUser(t.Context(), p2, member)
	if err != nil || second.Status != domain.MemberStatusSuspended {
		t.Errorf("p2 membership = %+v, %v", second, err)
	}
	if len(f.mailer.sent) != 0 {
		t.Errorf("registered invitee must not get the invite email, sent %+v", f.mailer.sent)
	}
	if got := len(f.audit.actions(auditdomain.ActionPropertyMemberAdded)); got != 2 {
		t.Errorf("membership audit entries = %d, want 2", got)
	}
}

func TestParticipantMutation_Invite_SkipsDuplicateArchivedAndForeign(t *testing.T) {
	t.Parallel()
	f := newMutationFixture()
	pDup, pArch, pForeign, pOk := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()),
		uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	f.addProperty(pDup, f.owner, "Дубль")
	f.addProperty(pArch, f.owner, "Архив")
	f.addProperty(pOk, f.owner, "Новый")
	f.statuses[pArch] = true
	// An existing pending invitation on pDup makes it a duplicate.
	if _, err := f.invitations.Create(t.Context(), domain.Invitation{
		ID: uuid.Must(uuid.NewV7()), PropertyID: pDup, Email: testNewUserEmail,
		Role: domain.RoleViewer, InvitedBy: f.owner, LastSentAt: f.clk.now,
		CreatedAt: f.clk.now,
	}); err != nil {
		t.Fatalf("seed invitation: %v", err)
	}

	results, err := f.svc.Invite(t.Context(), f.owner, testNewUserEmail, domain.RoleViewer,
		[]uuid.UUID{pDup, pDup, pArch, pForeign, pOk})
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	// Duplicate request ids collapse; every requested id reports its outcome.
	if len(results) != 4 {
		t.Fatalf("results = %+v, want four (duplicate id collapsed)", results)
	}
	wantOutcomes := []ParticipantGrantOutcome{
		ParticipantGrantDuplicate, ParticipantGrantArchived,
		ParticipantGrantUnavailable, ParticipantGrantPending,
	}
	for i, want := range wantOutcomes {
		if results[i].Outcome != want {
			t.Errorf("results[%d].outcome = %q, want %q", i, results[i].Outcome, want)
		}
	}
	// Only the non-duplicate object got a new invitation row.
	if len(f.invitations.rows) != 2 {
		t.Fatalf("invitations = %+v, want two (seed + new)", f.invitations.rows)
	}
	mail := f.mailer.only(t, "invite")
	if !slices.Equal(mail.titles, []string{"Новый"}) {
		t.Errorf("mail titles = %v, want only the granted object", mail.titles)
	}
}

func TestParticipantMutation_Invite_SelfEmailRejected(t *testing.T) {
	t.Parallel()
	f := newMutationFixture()
	p1 := uuid.Must(uuid.NewV7())
	f.addProperty(p1, f.owner, testFirstTitle)
	f.lookup.add(testOwnerEmail, f.owner)

	if _, err := f.svc.Invite(t.Context(), f.owner, testOwnerEmail, domain.RoleViewer,
		[]uuid.UUID{p1}); !errors.Is(err, domain.ErrCannotAddSelf) {
		t.Errorf("self invite: expected ErrCannotAddSelf, got %v", err)
	}
	if len(f.repo.rows)+len(f.invitations.rows) != 0 {
		t.Errorf("nothing must be created, got memberships %+v invitations %+v", f.repo.rows, f.invitations.rows)
	}
}

func TestParticipantMutation_Invite_ManagerTargetingOwnerSkipsAndSelfRejects(t *testing.T) {
	t.Parallel()
	f := newMutationFixture()
	p := uuid.Must(uuid.NewV7())
	other := uuid.Must(uuid.NewV7())
	f.addProperty(p, other, testForeignTitle)
	// The actor manages the object as an active full_access member.
	if _, err := f.repo.Create(t.Context(), domain.Membership{
		ID: uuid.Must(uuid.NewV7()), PropertyID: p, UserID: f.owner,
		Role: domain.RoleFullAccess, GrantedBy: other,
	}); err != nil {
		t.Fatalf("seed manager: %v", err)
	}
	f.lookup.add(testOwnerEmail, other) // The property owner's email.
	f.lookup.add(testRecipientEmail, f.owner)

	results, err := f.svc.Invite(t.Context(), f.owner, testOwnerEmail, domain.RoleViewer, []uuid.UUID{p})
	if err != nil {
		t.Fatalf("invite owner: %v", err)
	}
	if results[0].Outcome != ParticipantGrantOwner {
		t.Errorf("outcome = %q, want %q", results[0].Outcome, ParticipantGrantOwner)
	}
	if _, err := f.svc.Invite(t.Context(), f.owner, testRecipientEmail, domain.RoleViewer,
		[]uuid.UUID{p}); !errors.Is(err, domain.ErrCannotAddSelf) {
		t.Errorf("manager self invite: expected ErrCannotAddSelf, got %v", err)
	}
}

func TestParticipantMutation_AddProperties_RegisteredUser(t *testing.T) {
	t.Parallel()
	f := newMutationFixture()
	p1, p2 := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	f.addProperty(p1, f.owner, testFirstTitle)
	f.addProperty(p2, f.owner, testSecondTitle)
	member := uuid.Must(uuid.NewV7())
	// The participant already holds full access on p1: a new grant there is a
	// duplicate; p2 is a fresh active membership.
	if _, err := f.repo.Create(t.Context(), domain.Membership{
		ID: uuid.Must(uuid.NewV7()), PropertyID: p1, UserID: member,
		Role: domain.RoleFullAccess, GrantedBy: f.owner,
	}); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	f.limiter.set(member, 2)

	results, err := f.svc.AddProperties(t.Context(), f.owner, member.String(), domain.RoleViewer,
		[]uuid.UUID{p1, p2})
	if err != nil {
		t.Fatalf("add properties: %v", err)
	}
	if results[0].Outcome != ParticipantGrantDuplicate {
		t.Errorf("results[0] = %+v, want duplicate", results[0])
	}
	if results[1].Outcome != ParticipantGrantActive || results[1].MembershipID == uuid.Nil {
		t.Errorf("results[1] = %+v, want active", results[1])
	}
	if len(f.mailer.sent) != 0 {
		t.Errorf("registered target must not get the invite email, sent %+v", f.mailer.sent)
	}
}

func TestParticipantMutation_AddProperties_PendingEmailGetsInvitationsAndOneEmail(t *testing.T) {
	t.Parallel()
	f := newMutationFixture()
	p1, p2 := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	f.addProperty(p1, f.owner, testFirstTitle)
	f.addProperty(p2, f.owner, testSecondTitle)
	// The person is an existing participant: a pending invitation on p1.
	if _, err := f.invitations.Create(t.Context(), domain.Invitation{
		ID: uuid.Must(uuid.NewV7()), PropertyID: p1, Email: testPendingEmail,
		Role: domain.RoleViewer, InvitedBy: f.owner, LastSentAt: f.clk.now, CreatedAt: f.clk.now,
	}); err != nil {
		t.Fatalf("seed invitation: %v", err)
	}

	results, err := f.svc.AddProperties(t.Context(), f.owner, testPendingEmail, domain.RoleFullAccess,
		[]uuid.UUID{p1, p2})
	if err != nil {
		t.Fatalf("add properties: %v", err)
	}
	if results[0].Outcome != ParticipantGrantDuplicate {
		t.Errorf("results[0] = %+v, want duplicate", results[0])
	}
	if results[1].Outcome != ParticipantGrantPending || results[1].InvitationID == uuid.Nil {
		t.Errorf("results[1] = %+v, want pending", results[1])
	}
	mail := f.mailer.only(t, "invite")
	if mail.to != testPendingEmail {
		t.Errorf("mail to = %q", mail.to)
	}
	if !slices.Equal(mail.titles, []string{testSecondTitle}) {
		t.Errorf("mail titles = %v, want only the newly granted object", mail.titles)
	}
}

func TestParticipantMutation_AddProperties_EmailOfRegisteredUserActivates(t *testing.T) {
	t.Parallel()
	f := newMutationFixture()
	p1 := uuid.Must(uuid.NewV7())
	f.addProperty(p1, f.owner, testFirstTitle)
	member := uuid.Must(uuid.NewV7())
	f.lookup.add(testMemberEmail, member)
	f.limiter.set(member, 1)
	// The person exists as a pending invitation only; the email already
	// belongs to a registered user — the grant lands on the user.
	if _, err := f.invitations.Create(t.Context(), domain.Invitation{
		ID: uuid.Must(uuid.NewV7()), PropertyID: p1, Email: testMemberEmail,
		Role: domain.RoleViewer, InvitedBy: f.owner, LastSentAt: f.clk.now, CreatedAt: f.clk.now,
	}); err != nil {
		t.Fatalf("seed invitation: %v", err)
	}

	results, err := f.svc.AddProperties(t.Context(), f.owner, testMemberEmail, domain.RoleViewer,
		[]uuid.UUID{p1})
	if err != nil {
		t.Fatalf("add properties: %v", err)
	}
	if results[0].Outcome != ParticipantGrantActive || results[0].MembershipID == uuid.Nil {
		t.Errorf("results[0] = %+v, want active (the email resolved to the registered user)", results[0])
	}
	if len(f.invitations.rows) != 1 || len(f.mailer.sent) != 0 {
		t.Errorf("registered-by-email grant must not touch the invitation or send mail: %+v / %+v",
			f.invitations.rows, f.mailer.sent)
	}
}

func TestParticipantMutation_AddProperties_UnknownPersonNotFound(t *testing.T) {
	t.Parallel()
	f := newMutationFixture()
	p1 := uuid.Must(uuid.NewV7())
	f.addProperty(p1, f.owner, testFirstTitle)

	if _, err := f.svc.AddProperties(t.Context(), f.owner, uuid.Must(uuid.NewV7()).String(),
		domain.RoleViewer, []uuid.UUID{p1}); !errors.Is(err, domain.ErrParticipantNotFound) {
		t.Errorf("unknown uuid: expected ErrParticipantNotFound, got %v", err)
	}
	if _, err := f.svc.AddProperties(t.Context(), f.owner, "ghost@x.ru",
		domain.RoleViewer, []uuid.UUID{p1}); !errors.Is(err, domain.ErrParticipantNotFound) {
		t.Errorf("unknown email: expected ErrParticipantNotFound, got %v", err)
	}
	if _, err := f.svc.AddProperties(t.Context(), f.owner, "not-an-email",
		domain.RoleViewer, []uuid.UUID{p1}); !errors.Is(err, domain.ErrParticipantNotFound) {
		t.Errorf("malformed id: expected ErrParticipantNotFound, got %v", err)
	}
}

func TestParticipantMutation_AddProperties_PersonWithoutManagedLegsNotFound(t *testing.T) {
	t.Parallel()
	f := newMutationFixture()
	p1 := uuid.Must(uuid.NewV7())
	other := uuid.Must(uuid.NewV7())
	f.addProperty(p1, other, testForeignTitle)
	member := uuid.Must(uuid.NewV7())
	// The person's only leg is on another owner's property — outside the
	// actor's scope, so the aggregate (and this mutation) does not know them.
	if _, err := f.repo.Create(t.Context(), domain.Membership{
		ID: uuid.Must(uuid.NewV7()), PropertyID: p1, UserID: member,
		Role: domain.RoleViewer, GrantedBy: other,
	}); err != nil {
		t.Fatalf("seed membership: %v", err)
	}

	pMine := uuid.Must(uuid.NewV7())
	f.addProperty(pMine, f.owner, testOwnTitle)
	if _, err := f.svc.AddProperties(t.Context(), f.owner, member.String(), domain.RoleViewer,
		[]uuid.UUID{pMine}); !errors.Is(err, domain.ErrParticipantNotFound) {
		t.Errorf("expected ErrParticipantNotFound, got %v", err)
	}
}

func TestParticipantMutation_Remove_RegisteredAcrossOwnedProperties(t *testing.T) {
	t.Parallel()
	f := newMutationFixture()
	p1, p3 := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	other := uuid.Must(uuid.NewV7())
	f.addProperty(p1, f.owner, testOwnTitle)
	f.addProperty(p3, other, testForeignTitle)
	member := uuid.Must(uuid.NewV7())
	f.emails[member] = testMemberEmail
	// Active leg on the actor's property; active leg and pending invitation
	// on another owner's property.
	if _, err := f.repo.Create(t.Context(), domain.Membership{
		ID: uuid.Must(uuid.NewV7()), PropertyID: p1, UserID: member,
		Role: domain.RoleViewer, GrantedBy: f.owner,
	}); err != nil {
		t.Fatalf("seed p1: %v", err)
	}
	if _, err := f.repo.Create(t.Context(), domain.Membership{
		ID: uuid.Must(uuid.NewV7()), PropertyID: p3, UserID: member,
		Role: domain.RoleViewer, GrantedBy: other,
	}); err != nil {
		t.Fatalf("seed p3: %v", err)
	}
	if _, err := f.invitations.Create(t.Context(), domain.Invitation{
		ID: uuid.Must(uuid.NewV7()), PropertyID: p1, Email: testMemberEmail,
		Role: domain.RoleViewer, InvitedBy: f.owner, LastSentAt: f.clk.now, CreatedAt: f.clk.now,
	}); err != nil {
		t.Fatalf("seed invitation: %v", err)
	}
	if _, err := f.invitations.Create(t.Context(), domain.Invitation{
		ID: uuid.Must(uuid.NewV7()), PropertyID: p3, Email: testMemberEmail,
		Role: domain.RoleViewer, InvitedBy: other, LastSentAt: f.clk.now, CreatedAt: f.clk.now,
	}); err != nil {
		t.Fatalf("seed foreign invitation: %v", err)
	}

	if err := f.svc.Remove(t.Context(), f.owner, member.String()); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := f.repo.GetByPropertyAndUser(t.Context(), p1, member); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("p1 membership must be removed, got %+v (%v)", firstRow(f.repo.rows, p1), err)
	}
	if _, err := f.repo.GetByPropertyAndUser(t.Context(), p3, member); err != nil {
		t.Errorf("foreign-owner membership must stay, got %v", err)
	}
	if got := len(f.invitations.rows); got != 1 || f.invitations.rows[0].PropertyID != p3 {
		t.Errorf("invitations = %+v, want only the foreign-owner one", f.invitations.rows)
	}
	if got := len(f.audit.actions(auditdomain.ActionPropertyMemberRemoved)); got != 1 {
		t.Errorf("membership audit entries = %d, want 1", got)
	}
	if got := len(f.audit.actions(auditdomain.ActionPropertyMemberInvitationCancelled)); got != 1 {
		t.Errorf("invitation audit entries = %d, want 1", got)
	}
	// The removal itself sends nothing; the FIFO recovery emails come from the
	// shared slot coordinator (cut with the lifecycle emails, issue #695).
	if f.mailer.count("access_revoked") != 0 || f.mailer.count("invite") != 0 {
		t.Errorf("removal must not send revoke/invite emails, sent %+v", f.mailer.sent)
	}
}

func TestParticipantMutation_Remove_FreesSlotAndRecoversFIFO(t *testing.T) {
	t.Parallel()
	f := newMutationFixture()
	p1, p4 := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	other := uuid.Must(uuid.NewV7())
	f.addProperty(p1, f.owner, testOwnTitle)
	f.addProperty(p4, other, testForeignTitle)
	member := uuid.Must(uuid.NewV7())
	f.emails[member] = testMemberEmail
	f.limiter.set(member, 1)
	// An active leg on the actor's object occupies the member's only slot; a
	// suspended leg on another owner's object waits in the FIFO queue.
	if _, err := f.repo.Create(t.Context(), domain.Membership{
		ID: uuid.Must(uuid.NewV7()), PropertyID: p1, UserID: member,
		Role: domain.RoleViewer, GrantedBy: f.owner,
	}); err != nil {
		t.Fatalf("seed p1: %v", err)
	}
	if _, err := f.repo.CreateWithStatus(t.Context(), domain.Membership{
		ID: uuid.Must(uuid.NewV7()), PropertyID: p4, UserID: member,
		Role: domain.RoleViewer, GrantedBy: other, Status: domain.MemberStatusSuspended,
	}); err != nil {
		t.Fatalf("seed p4: %v", err)
	}

	if err := f.svc.Remove(t.Context(), f.owner, member.String()); err != nil {
		t.Fatalf("remove: %v", err)
	}
	recovered, err := f.repo.GetByPropertyAndUser(t.Context(), p4, member)
	if err != nil || recovered.Status != domain.MemberStatusActive {
		t.Errorf("suspended leg must be recovered FIFO, got %+v (%v)", recovered, err)
	}
	// The removal itself sends nothing; «access_restored» comes from the
	// shared slot coordinator's recovery (cut with the lifecycle emails, #695).
	if f.mailer.count("access_revoked") != 0 || f.mailer.count("invite") != 0 {
		t.Errorf("removal must not send revoke/invite emails, sent %+v", f.mailer.sent)
	}
}

func TestParticipantMutation_Remove_PendingEmail(t *testing.T) {
	t.Parallel()
	f := newMutationFixture()
	p1, p2 := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	other := uuid.Must(uuid.NewV7())
	f.addProperty(p1, f.owner, testOwnTitle)
	f.addProperty(p2, other, testForeignTitle)
	for _, tc := range []struct {
		prop uuid.UUID
		by   uuid.UUID
	}{
		{p1, f.owner},
		{p2, other},
	} {
		if _, err := f.invitations.Create(t.Context(), domain.Invitation{
			ID: uuid.Must(uuid.NewV7()), PropertyID: tc.prop, Email: testPendingEmail,
			Role: domain.RoleViewer, InvitedBy: tc.by, LastSentAt: f.clk.now, CreatedAt: f.clk.now,
		}); err != nil {
			t.Fatalf("seed invitation: %v", err)
		}
	}

	if err := f.svc.Remove(t.Context(), f.owner, testPendingEmail); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if len(f.invitations.rows) != 1 || f.invitations.rows[0].PropertyID != p2 {
		t.Errorf("invitations = %+v, want only the foreign-owner one", f.invitations.rows)
	}
}

func TestParticipantMutation_Remove_NotFound(t *testing.T) {
	t.Parallel()
	f := newMutationFixture()
	p1 := uuid.Must(uuid.NewV7())
	f.addProperty(p1, f.owner, testOwnTitle)

	for name, id := range map[string]string{
		"unknown uuid":   uuid.Must(uuid.NewV7()).String(),
		"unknown email":  "ghost@x.ru",
		"malformed id":   "not-an-email",
		"the actor self": f.owner.String(),
	} {
		if err := f.svc.Remove(t.Context(), f.owner, id); !errors.Is(err, domain.ErrParticipantNotFound) {
			t.Errorf("%s: expected ErrParticipantNotFound, got %v", name, err)
		}
	}
	// A registered person whose only legs are outside the actor's scope.
	other := uuid.Must(uuid.NewV7())
	p2 := uuid.Must(uuid.NewV7())
	f.addProperty(p2, other, testForeignTitle)
	member := uuid.Must(uuid.NewV7())
	if _, err := f.repo.Create(t.Context(), domain.Membership{
		ID: uuid.Must(uuid.NewV7()), PropertyID: p2, UserID: member,
		Role: domain.RoleViewer, GrantedBy: other,
	}); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	if err := f.svc.Remove(t.Context(), f.owner, member.String()); !errors.Is(err, domain.ErrParticipantNotFound) {
		t.Errorf("out-of-scope person: expected ErrParticipantNotFound, got %v", err)
	}
}

// firstRow returns the membership row for (property, any user) or nil.
func firstRow(rows []domain.Membership, propertyID uuid.UUID) *domain.Membership {
	for i := range rows {
		if rows[i].PropertyID == propertyID {
			return &rows[i]
		}
	}
	return nil
}

func TestParticipantMutation_AddProperties_ArchivedPropertySkipped(t *testing.T) {
	t.Parallel()
	f := newMutationFixture()
	pActive, pArchived := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	f.addProperty(pActive, f.owner, testFirstTitle)
	f.addProperty(pArchived, f.owner, testSecondTitle)
	f.statuses[pArchived] = true
	member := uuid.Must(uuid.NewV7())
	f.emails[member] = testMemberEmail
	// An existing participant (a pending invitation on the active property),
	// so the batch passes the existence gate.
	if _, err := f.invitations.Create(t.Context(), domain.Invitation{
		ID: uuid.Must(uuid.NewV7()), PropertyID: pActive, Email: testMemberEmail,
		Role: domain.RoleViewer, InvitedBy: f.owner, LastSentAt: f.clk.now, CreatedAt: f.clk.now,
	}); err != nil {
		t.Fatalf("seed invitation: %v", err)
	}
	f.lookup.add(testMemberEmail, member)
	f.limiter.set(member, 1)

	results, err := f.svc.AddProperties(t.Context(), f.owner, testMemberEmail, domain.RoleViewer,
		[]uuid.UUID{pArchived, pActive})
	if err != nil {
		t.Fatalf("add properties: %v", err)
	}
	// The archived object gets no grant (issue #163); the active one does.
	if results[0].Outcome != ParticipantGrantArchived || results[0].MembershipID != (uuid.UUID{}) {
		t.Errorf("results[0] = %+v, want skipped_archived", results[0])
	}
	if results[1].Outcome != ParticipantGrantActive || results[1].MembershipID == uuid.Nil {
		t.Errorf("results[1] = %+v, want active", results[1])
	}
}
