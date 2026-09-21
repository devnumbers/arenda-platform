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
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// This file covers the email invitation lifecycle (issue #161, T5) through the
// real InvitationService + AccessService + MembershipPolicy + SlotCoordinator
// on in-memory ports — the same seam as service_integration_test.go and
// slot_coordinator_test.go.

// Shared fixture strings across the access application test files: the
// invitee/new-user email, the participant emails, and the property titles.
const (
	testNewUserEmail   = "new@example.com"
	testMemberEmail    = "member@example.com"
	testOwnerEmail     = "owner@example.com"
	testRecipientEmail = "recipient@example.com"
	testNevskyTitle    = "Квартира на Невском"
	testApartmentTitle = "Квартира"
)

// In-memory port stubs for the invitation dependencies.

// memInvitationsRepo is an in-memory InvitationRepository.
type memInvitationsRepo struct {
	rows []domain.Invitation
}

func (r *memInvitationsRepo) Create(_ context.Context, inv domain.Invitation) (domain.Invitation, error) {
	for _, e := range r.rows {
		if e.PropertyID == inv.PropertyID && e.Email == inv.Email {
			return domain.Invitation{}, domain.ErrInvitationAlreadyExists
		}
	}
	if inv.CreatedAt.IsZero() {
		inv.CreatedAt = time.Now()
	}
	inv.UpdatedAt = inv.CreatedAt
	r.rows = append(r.rows, inv)
	return inv, nil
}

func (r *memInvitationsRepo) GetByID(_ context.Context, id, propertyID uuid.UUID) (domain.Invitation, error) {
	for _, e := range r.rows {
		if e.ID == id && e.PropertyID == propertyID {
			return e, nil
		}
	}
	return domain.Invitation{}, domain.ErrInvitationNotFound
}

func (r *memInvitationsRepo) GetByPropertyAndEmail(_ context.Context, propertyID uuid.UUID, email string) (domain.Invitation, error) {
	for _, e := range r.rows {
		if e.PropertyID == propertyID && e.Email == email {
			return e, nil
		}
	}
	return domain.Invitation{}, domain.ErrInvitationNotFound
}

func (r *memInvitationsRepo) ListByProperty(_ context.Context, propertyID uuid.UUID) ([]domain.Invitation, error) {
	var out []domain.Invitation
	for _, e := range r.rows {
		if e.PropertyID == propertyID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (r *memInvitationsRepo) ListPendingByEmail(_ context.Context, email string) ([]domain.Invitation, error) {
	var out []domain.Invitation
	for _, e := range r.rows {
		if e.Email == email {
			out = append(out, e)
		}
	}
	// FIFO activation: oldest first (matches the SQL ORDER BY created_at ASC).
	slices.SortFunc(out, func(a, b domain.Invitation) int {
		return a.CreatedAt.Compare(b.CreatedAt)
	})
	return out, nil
}

func (r *memInvitationsRepo) UpdateRole(_ context.Context, id, propertyID uuid.UUID, role domain.Role) (domain.Invitation, error) {
	for i := range r.rows {
		if r.rows[i].ID == id && r.rows[i].PropertyID == propertyID {
			r.rows[i].Role = role
			return r.rows[i], nil
		}
	}
	return domain.Invitation{}, domain.ErrInvitationNotFound
}

func (r *memInvitationsRepo) UpdateLastSentAt(_ context.Context, id, propertyID uuid.UUID, sentAt time.Time) error {
	for i := range r.rows {
		if r.rows[i].ID == id && r.rows[i].PropertyID == propertyID {
			r.rows[i].LastSentAt = sentAt
			return nil
		}
	}
	return nil
}

func (r *memInvitationsRepo) Delete(_ context.Context, id, propertyID uuid.UUID) error {
	for i := range r.rows {
		if r.rows[i].ID == id && r.rows[i].PropertyID == propertyID {
			r.rows = append(r.rows[:i], r.rows[i+1:]...)
			return nil
		}
	}
	return nil
}

func (r *memInvitationsRepo) WithTx(transaction.Tx) InvitationRepository { return r }

// fakeLookup is a configurable UserLookup keyed by normalized email.
type fakeLookup struct {
	byEmail map[string]MemberUser
}

func newFakeLookup() *fakeLookup {
	return &fakeLookup{byEmail: map[string]MemberUser{}}
}

func (f *fakeLookup) add(email string, userID uuid.UUID) {
	f.byEmail[email] = MemberUser{ID: userID, HasEmail: true}
}

func (f *fakeLookup) GetByID(_ context.Context, id uuid.UUID) (MemberUser, error) {
	name := "Member"
	return MemberUser{ID: id, Name: &name}, nil
}

func (f *fakeLookup) GetByEmail(_ context.Context, email string) (MemberUser, error) {
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}
	return MemberUser{}, domain.ErrUserNotFound
}

// fakeTitles resolves a fixed property title.
type fakeTitles string

func (t fakeTitles) GetTitle(context.Context, uuid.UUID) (string, error) { return string(t), nil }

// fixedClock is a settable clock.Clock for cooldown tests.
type fixedClock struct {
	now time.Time
}

func (c *fixedClock) Now() time.Time          { return c.now }
func (c *fixedClock) advance(d time.Duration) { c.now = c.now.Add(d) }

var (
	_ InvitationRepository  = (*memInvitationsRepo)(nil)
	_ UserLookup            = (*fakeLookup)(nil)
	_ PropertyTitleResolver = fakeTitles("")
)

// invitationFixture bundles the invitation service with its in-memory
// dependencies.
type invitationFixture struct {
	repo        *memRepo
	invitations *memInvitationsRepo
	owners      staticResolver
	statuses    fakeStatuses
	lookup      *fakeLookup
	mailer      *fakeAccessMailer
	clk         *fixedClock
	limiter     *fakeRecipientLimiter
	access      *AccessService
	svc         *InvitationService
}

func newInvitationFixture() *invitationFixture {
	repo := newMemRepo()
	invitations := &memInvitationsRepo{}
	owners := staticResolver{}
	statuses := fakeStatuses{}
	policy := NewMembershipPolicy(owners, repo)
	lookup := newFakeLookup()
	mailer := &fakeAccessMailer{}
	clk := &fixedClock{now: time.Now()}
	limiter := newFakeRecipientLimiter()
	// The lifecycle events are not wired here: the T5 scenarios assert the
	// invite email only, no event publication is part of them.
	coordinator := NewSlotCoordinator(repo, owners, limiter, newFakeOwnedProps(),
		nil, auditapp.Noop{}, noopBeginner{}, nil, nil)
	access := NewAccessService(repo, owners, statuses, lookup, policy, coordinator, nil,
		newTestFactory(repo, invitations, auditapp.Noop{}), nil)
	svc := NewInvitationService(access, repo, invitations, owners, statuses, lookup, policy,
		coordinator, mailer, nil, fakeTitles(testNevskyTitle),
		newTestFactory(repo, invitations, auditapp.Noop{}), clk, nil)
	return &invitationFixture{
		repo:        repo,
		invitations: invitations,
		owners:      owners,
		statuses:    statuses,
		lookup:      lookup,
		mailer:      mailer,
		clk:         clk,
		limiter:     limiter,
		access:      access,
		svc:         svc,
	}
}

// addProperty registers a property with its owner in the resolver and the
// membership repo owner map.
func (f *invitationFixture) addProperty(ownerID uuid.UUID) uuid.UUID {
	propertyID := uuid.Must(uuid.NewV7())
	f.owners[propertyID] = ownerID
	f.repo.SetOwner(propertyID, ownerID)
	return propertyID
}

// Invite: registered email → instant activation.

func TestInvitationService_InviteRegisteredEmailActivatesInstantly(t *testing.T) {
	t.Parallel()
	f := newInvitationFixture()
	owner := uuid.Must(uuid.NewV7())
	property := f.addProperty(owner)
	invitee := uuid.Must(uuid.NewV7())
	f.lookup.add("friend@example.com", invitee)
	f.limiter.set(invitee, 10)

	outcome, err := f.svc.InviteByEmail(t.Context(), owner, property, " Friend@Example.com ", domain.RoleViewer)
	if err != nil {
		t.Fatalf("InviteByEmail: %v", err)
	}
	if outcome.Member == nil || outcome.Invitation != nil {
		t.Fatalf("expected instant membership, got %+v", outcome)
	}
	if outcome.Member.UserID != invitee || outcome.Member.Role != domain.RoleViewer {
		t.Errorf("membership = %+v, want user %s viewer", outcome.Member, invitee)
	}
	if outcome.Member.Status != domain.MemberStatusActive {
		t.Errorf("status = %v, want active", outcome.Member.Status)
	}
	if len(f.mailer.sent) != 0 {
		t.Errorf("instant activation must not send the invite email, sent %d", len(f.mailer.sent))
	}
	if len(f.invitations.rows) != 0 {
		t.Errorf("no pending invitation expected, got %d", len(f.invitations.rows))
	}

	// Re-inviting an active member is a conflict.
	if _, err := f.svc.InviteByEmail(t.Context(), owner, property, "friend@example.com",
		domain.RoleViewer); !errors.Is(err, domain.ErrMemberAlreadyExists) {
		t.Errorf("re-invite member: expected ErrMemberAlreadyExists, got %v", err)
	}
}

func TestInvitationService_InviteOwnerOrSelfEmailRejected(t *testing.T) {
	t.Parallel()
	f := newInvitationFixture()
	owner := uuid.Must(uuid.NewV7())
	property := f.addProperty(owner)
	f.lookup.add(testOwnerEmail, owner)

	if _, err := f.svc.InviteByEmail(t.Context(), owner, property, testOwnerEmail,
		domain.RoleViewer); !errors.Is(err, domain.ErrCannotAddOwner) {
		t.Errorf("owner email: expected ErrCannotAddOwner, got %v", err)
	}
}

func TestInvitationService_InviteInvalidEmailRejected(t *testing.T) {
	t.Parallel()
	f := newInvitationFixture()
	owner := uuid.Must(uuid.NewV7())
	property := f.addProperty(owner)

	for _, raw := range []string{"", "not-an-email", "a@b"} {
		if _, err := f.svc.InviteByEmail(t.Context(), owner, property, raw, domain.RoleViewer); !errors.Is(err, domain.ErrInvalidEmail) {
			t.Errorf("email %q: expected ErrInvalidEmail, got %v", raw, err)
		}
	}
}

// Invite: unregistered email → pending invitation + single invite email.

func TestInvitationService_InviteUnregisteredCreatesPending(t *testing.T) {
	t.Parallel()
	f := newInvitationFixture()
	owner := uuid.Must(uuid.NewV7())
	property := f.addProperty(owner)

	outcome, err := f.svc.InviteByEmail(t.Context(), owner, property, "New@Example.com", domain.RoleFullAccess)
	if err != nil {
		t.Fatalf("InviteByEmail: %v", err)
	}
	if outcome.Invitation == nil || outcome.Member != nil {
		t.Fatalf("expected pending invitation, got %+v", outcome)
	}
	inv := *outcome.Invitation
	if inv.Email != testNewUserEmail {
		t.Errorf("email = %q, want normalized new@example.com", inv.Email)
	}
	if inv.Role != domain.RoleFullAccess || inv.InvitedBy != owner {
		t.Errorf("invitation = %+v", inv)
	}
	if inv.LastSentAt.IsZero() {
		t.Error("last_sent_at must be set at creation")
	}
	if len(f.mailer.sent) != 1 || f.mailer.sent[0].to != testNewUserEmail {
		t.Fatalf("expected exactly one invite email, got %+v", f.mailer.sent)
	}
	if f.mailer.sent[0].title != testNevskyTitle {
		t.Errorf("mail title = %q", f.mailer.sent[0].title)
	}

	// A duplicate pending invite is a conflict and sends no second email.
	if _, err := f.svc.InviteByEmail(t.Context(), owner, property, testNewUserEmail,
		domain.RoleViewer); !errors.Is(err, domain.ErrInvitationAlreadyExists) {
		t.Errorf("duplicate invite: expected ErrInvitationAlreadyExists, got %v", err)
	}
	if len(f.mailer.sent) != 1 {
		t.Errorf("duplicate invite must not send, sent %d", len(f.mailer.sent))
	}
}

func TestInvitationService_InviteMailFailureKeepsInvitation(t *testing.T) {
	t.Parallel()
	f := newInvitationFixture()
	owner := uuid.Must(uuid.NewV7())
	property := f.addProperty(owner)
	f.mailer.err = errors.New("smtp down")

	outcome, err := f.svc.InviteByEmail(t.Context(), owner, property, testNewUserEmail, domain.RoleViewer)
	if err != nil {
		t.Fatalf("mail failure must not fail the invite: %v", err)
	}
	if outcome.Invitation == nil {
		t.Fatal("invitation must stay after a mail failure")
	}
}

// Resend with the 24h cooldown.

func TestInvitationService_ResendCooldown(t *testing.T) {
	t.Parallel()
	f := newInvitationFixture()
	owner := uuid.Must(uuid.NewV7())
	property := f.addProperty(owner)

	outcome, err := f.svc.InviteByEmail(t.Context(), owner, property, testNewUserEmail, domain.RoleViewer)
	if err != nil {
		t.Fatalf("InviteByEmail: %v", err)
	}
	invID := outcome.Invitation.ID

	// Immediate resend hits the cooldown; no second email.
	err = f.svc.ResendInvitation(t.Context(), owner, property, invID)
	if !errors.Is(err, domain.ErrInvitationResendCooldown) {
		t.Fatalf("resend: expected ErrInvitationResendCooldown, got %v", err)
	}
	var cooldown *domain.ResendCooldownError
	if !errors.As(err, &cooldown) || cooldown.RetryAfter <= 0 || cooldown.RetryAfter > domain.InvitationResendCooldown {
		t.Errorf("expected positive RetryAfter within 24h, got %+v", cooldown)
	}
	if len(f.mailer.sent) != 1 {
		t.Errorf("cooldown resend must not send, sent %d", len(f.mailer.sent))
	}

	// After the cooldown the resend goes through and bumps last_sent_at.
	f.clk.advance(domain.InvitationResendCooldown + time.Hour)
	if err := f.svc.ResendInvitation(t.Context(), owner, property, invID); err != nil {
		t.Fatalf("resend after cooldown: %v", err)
	}
	if len(f.mailer.sent) != 2 {
		t.Fatalf("expected 2 sent emails, got %d", len(f.mailer.sent))
	}
	updated, err := f.invitations.GetByID(t.Context(), invID, property)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if !updated.LastSentAt.Equal(f.clk.Now()) {
		t.Errorf("last_sent_at = %v, want %v", updated.LastSentAt, f.clk.Now())
	}

	// Unknown invitation id is not found.
	if err := f.svc.ResendInvitation(t.Context(), owner, property, uuid.Must(uuid.NewV7())); !errors.Is(err, domain.ErrInvitationNotFound) {
		t.Errorf("unknown invitation: expected ErrInvitationNotFound, got %v", err)
	}
}

// Role change and cancellation of a pending invitation.

func TestInvitationService_ChangeRoleAndCancel(t *testing.T) {
	t.Parallel()
	f := newInvitationFixture()
	owner := uuid.Must(uuid.NewV7())
	property := f.addProperty(owner)

	outcome, err := f.svc.InviteByEmail(t.Context(), owner, property, testNewUserEmail, domain.RoleViewer)
	if err != nil {
		t.Fatalf("InviteByEmail: %v", err)
	}
	invID := outcome.Invitation.ID

	updated, err := f.svc.ChangeInvitationRole(t.Context(), owner, property, invID, domain.RoleFullAccess)
	if err != nil {
		t.Fatalf("ChangeInvitationRole: %v", err)
	}
	if updated.Role != domain.RoleFullAccess {
		t.Errorf("role = %v, want full_access", updated.Role)
	}
	if len(f.mailer.sent) != 1 {
		t.Errorf("role change must not send a new email, sent %d", len(f.mailer.sent))
	}

	if err := f.svc.CancelInvitation(t.Context(), owner, property, invID); err != nil {
		t.Fatalf("CancelInvitation: %v", err)
	}
	if len(f.mailer.sent) != 1 {
		t.Errorf("cancel must not send an email, sent %d", len(f.mailer.sent))
	}
	if _, err := f.invitations.GetByID(t.Context(), invID, property); !errors.Is(err, domain.ErrInvitationNotFound) {
		t.Errorf("invitation must be deleted after cancel, got %v", err)
	}

	// Re-inviting a cancelled email is free.
	if _, err := f.svc.InviteByEmail(t.Context(), owner, property, testNewUserEmail, domain.RoleViewer); err != nil {
		t.Errorf("re-invite after cancel: %v", err)
	}

	// A viewer cannot manage invitations.
	viewer := uuid.Must(uuid.NewV7())
	if _, err := f.access.AddMember(t.Context(), owner, property, viewer, domain.RoleViewer); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if _, err := f.svc.InviteByEmail(t.Context(), viewer, property, "other@example.com",
		domain.RoleViewer); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("viewer invite: expected ErrMemberNotFound, got %v", err)
	}
}

// Activation at registration.

// seedInvitation stores a pending invitation fixture with explicit timestamps
// so the FIFO activation order is deterministic.
func (f *invitationFixture) seedInvitation(
	t *testing.T, propertyID uuid.UUID, email string,
	role domain.Role, invitedBy uuid.UUID, lastSentAt, createdAt time.Time,
) domain.Invitation {
	t.Helper()
	inv, err := f.invitations.Create(t.Context(), domain.Invitation{
		ID: uuid.Must(uuid.NewV7()), PropertyID: propertyID, Email: email,
		Role: role, InvitedBy: invitedBy, LastSentAt: lastSentAt, CreatedAt: createdAt,
	})
	if err != nil {
		t.Fatalf("create invitation on %s: %v", propertyID, err)
	}
	return inv
}

// assertMembership loads the membership on the property and checks its role,
// grantor and active status.
func (f *invitationFixture) assertMembership(t *testing.T, propertyID, userID, grantedBy uuid.UUID, wantRole domain.Role) {
	t.Helper()
	m, err := f.repo.GetByPropertyAndUser(t.Context(), propertyID, userID)
	if err != nil {
		t.Fatalf("membership on %s: %v", propertyID, err)
	}
	if m.Role != wantRole || m.GrantedBy != grantedBy || m.Status != domain.MemberStatusActive {
		t.Errorf("membership on %s = %+v, want role %s granted by %s active", propertyID, m, wantRole, grantedBy)
	}
}

// assertInvitationConsumed checks that an activation consumed the invitation.
func (f *invitationFixture) assertInvitationConsumed(t *testing.T, invitationID, propertyID uuid.UUID) {
	t.Helper()
	if _, err := f.invitations.GetByID(t.Context(), invitationID, propertyID); !errors.Is(err, domain.ErrInvitationNotFound) {
		t.Errorf("invitation %s must be deleted, got %v", invitationID, err)
	}
}

func TestInvitationService_ActivatePendingInvitations(t *testing.T) {
	t.Parallel()
	f := newInvitationFixture()
	owner := uuid.Must(uuid.NewV7())
	propA := f.addProperty(owner)
	propB := f.addProperty(owner)
	other := f.addProperty(owner)

	// Two pending invitations on the same email (propA is older), one on
	// another email.
	base := time.Now().Add(-time.Hour)
	invA := f.seedInvitation(t, propA, testNewUserEmail, domain.RoleViewer, owner, base, base)
	invB := f.seedInvitation(t, propB, testNewUserEmail, domain.RoleFullAccess, owner, base, base.Add(time.Minute))
	f.seedInvitation(t, other, "someone-else@example.com", domain.RoleViewer, owner, base, base)

	user := uuid.Must(uuid.NewV7())
	f.limiter.set(user, 10)
	if err := f.svc.ActivatePendingInvitations(t.Context(), user, "New@Example.com"); err != nil {
		t.Fatalf("ActivatePendingInvitations: %v", err)
	}

	// Both invitations activated with their stored roles.
	f.assertMembership(t, propA, user, owner, domain.RoleViewer)
	mB, err := f.repo.GetByPropertyAndUser(t.Context(), propB, user)
	if err != nil {
		t.Fatalf("membership B: %v", err)
	}
	if mB.Role != domain.RoleFullAccess {
		t.Errorf("membership B role = %v, want full_access", mB.Role)
	}

	// Invitations are gone; the other email's invitation is untouched.
	f.assertInvitationConsumed(t, invA.ID, propA)
	f.assertInvitationConsumed(t, invB.ID, propB)
	remaining, err := f.invitations.ListPendingByEmail(t.Context(), "someone-else@example.com")
	if err != nil || len(remaining) != 1 {
		t.Errorf("other email invitation must stay, got %v %v", remaining, err)
	}

	// A non-matching email activates nothing.
	if err := f.svc.ActivatePendingInvitations(t.Context(), uuid.Must(uuid.NewV7()), "nobody@example.com"); err != nil {
		t.Fatalf("activate non-matching: %v", err)
	}
	// An empty email is a no-op, not an error.
	if err := f.svc.ActivatePendingInvitations(t.Context(), uuid.Must(uuid.NewV7()), ""); err != nil {
		t.Fatalf("activate empty email: %v", err)
	}
}

func TestInvitationService_ActivationFIFOWhenSlotShort(t *testing.T) {
	t.Parallel()
	f := newInvitationFixture()
	owner := uuid.Must(uuid.NewV7())
	propA := f.addProperty(owner)
	propB := f.addProperty(owner)

	base := time.Now().Add(-time.Hour)
	if _, err := f.invitations.Create(t.Context(), domain.Invitation{
		ID: uuid.Must(uuid.NewV7()), PropertyID: propA, Email: testNewUserEmail,
		Role: domain.RoleViewer, InvitedBy: owner, LastSentAt: base, CreatedAt: base,
	}); err != nil {
		t.Fatalf("create A: %v", err)
	}
	if _, err := f.invitations.Create(t.Context(), domain.Invitation{
		ID: uuid.Must(uuid.NewV7()), PropertyID: propB, Email: testNewUserEmail,
		Role: domain.RoleViewer, InvitedBy: owner,
		LastSentAt: base, CreatedAt: base.Add(time.Minute),
	}); err != nil {
		t.Fatalf("create B: %v", err)
	}

	// One free slot only: the earlier invitation activates active, the later
	// one is created suspended (T4 mechanics via the real SlotCoordinator).
	user := uuid.Must(uuid.NewV7())
	f.limiter.set(user, 1)

	if err := f.svc.ActivatePendingInvitations(t.Context(), user, testNewUserEmail); err != nil {
		t.Fatalf("ActivatePendingInvitations: %v", err)
	}

	mA, err := f.repo.GetByPropertyAndUser(t.Context(), propA, user)
	if err != nil {
		t.Fatalf("membership A: %v", err)
	}
	if mA.Status != domain.MemberStatusActive {
		t.Errorf("earlier invitation must activate active, got %v", mA.Status)
	}
	mB, err := f.repo.GetByPropertyAndUser(t.Context(), propB, user)
	if err != nil {
		t.Fatalf("membership B: %v", err)
	}
	if mB.Status != domain.MemberStatusSuspended {
		t.Errorf("later invitation must activate suspended, got %v", mB.Status)
	}
}

func TestInvitationService_ActivationAppliesCurrentRole(t *testing.T) {
	t.Parallel()
	f := newInvitationFixture()
	owner := uuid.Must(uuid.NewV7())
	property := f.addProperty(owner)

	outcome, err := f.svc.InviteByEmail(t.Context(), owner, property, testNewUserEmail, domain.RoleViewer)
	if err != nil {
		t.Fatalf("InviteByEmail: %v", err)
	}
	// The role is changed after the invite; no new email, and the role current
	// at registration time is applied.
	if _, err := f.svc.ChangeInvitationRole(t.Context(), owner, property, outcome.Invitation.ID, domain.RoleFullAccess); err != nil {
		t.Fatalf("ChangeInvitationRole: %v", err)
	}

	user := uuid.Must(uuid.NewV7())
	if err := f.svc.ActivatePendingInvitations(t.Context(), user, testNewUserEmail); err != nil {
		t.Fatalf("ActivatePendingInvitations: %v", err)
	}
	m, err := f.repo.GetByPropertyAndUser(t.Context(), property, user)
	if err != nil {
		t.Fatalf("membership: %v", err)
	}
	if m.Role != domain.RoleFullAccess {
		t.Errorf("role = %v, want full_access (current at activation)", m.Role)
	}
}

func TestInvitationService_ActivationSkipsExistingMembership(t *testing.T) {
	t.Parallel()
	f := newInvitationFixture()
	owner := uuid.Must(uuid.NewV7())
	property := f.addProperty(owner)

	user := uuid.Must(uuid.NewV7())
	// The user is already a (suspended) member of the property; a pending
	// invitation for the same email must be dropped silently without creating
	// a duplicate membership.
	if _, err := f.repo.CreateWithStatus(t.Context(), domain.Membership{
		ID: uuid.Must(uuid.NewV7()), PropertyID: property, UserID: user,
		Role: domain.RoleViewer, GrantedBy: owner, Status: domain.MemberStatusSuspended,
	}); err != nil {
		t.Fatalf("CreateWithStatus: %v", err)
	}
	inv, err := f.invitations.Create(t.Context(), domain.Invitation{
		ID: uuid.Must(uuid.NewV7()), PropertyID: property, Email: testNewUserEmail,
		Role: domain.RoleFullAccess, InvitedBy: owner, LastSentAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}

	if err := f.svc.ActivatePendingInvitations(t.Context(), user, testNewUserEmail); err != nil {
		t.Fatalf("ActivatePendingInvitations: %v", err)
	}
	if _, err := f.invitations.GetByID(t.Context(), inv.ID, property); !errors.Is(err, domain.ErrInvitationNotFound) {
		t.Errorf("invitation must be dropped silently, got %v", err)
	}
	if got := len(f.repo.rows); got != 1 {
		t.Errorf("exactly one membership expected, got %d", got)
	}
}

// Member list: pending invitations are manager-only.

func TestInvitationService_ListMembersIncludesPendingForManagers(t *testing.T) {
	t.Parallel()
	f := newInvitationFixture()
	owner := uuid.Must(uuid.NewV7())
	property := f.addProperty(owner)
	viewer := uuid.Must(uuid.NewV7())
	f.limiter.set(viewer, 10)
	if _, err := f.access.AddMember(t.Context(), owner, property, viewer, domain.RoleViewer); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if _, err := f.svc.InviteByEmail(t.Context(), owner, property, testNewUserEmail, domain.RoleFullAccess); err != nil {
		t.Fatalf("InviteByEmail: %v", err)
	}

	// The owner (manager) sees owner + member + pending invitation.
	members, err := f.svc.ListMembers(t.Context(), owner, property)
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(members) != 3 {
		t.Fatalf("expected 3 rows, got %+v", members)
	}
	pending := members[2]
	if !pending.Pending || pending.Email == nil || *pending.Email != testNewUserEmail {
		t.Errorf("pending row = %+v", pending)
	}
	if pending.UserID != uuid.Nil || pending.Role != sharedpolicy.RoleFullAccess {
		t.Errorf("pending row user/role = %v %v", pending.UserID, pending.Role)
	}
	if pending.LastSentAt == nil {
		t.Error("pending row must carry last_sent_at")
	}

	// A viewer does not see pending invitations (they expose the invitee email).
	members, err = f.svc.ListMembers(t.Context(), viewer, property)
	if err != nil {
		t.Fatalf("viewer ListMembers: %v", err)
	}
	for _, m := range members {
		if m.Pending {
			t.Errorf("viewer must not see pending invitations, got %+v", members)
		}
	}
	if len(members) != 2 {
		t.Errorf("viewer rows = %d, want 2 (owner + viewer)", len(members))
	}
}

// Archived property: invites are rejected, pending invitations activate
// read-only without a slot (issue #163).

func TestInvitationService_InviteArchivedPropertyRejected(t *testing.T) {
	t.Parallel()
	f := newInvitationFixture()
	owner := uuid.Must(uuid.NewV7())
	property := f.addProperty(owner)
	f.statuses[property] = true

	// Unregistered email path.
	if _, err := f.svc.InviteByEmail(t.Context(), owner, property, testNewUserEmail,
		domain.RoleViewer); !errors.Is(err, domain.ErrPropertyArchived) {
		t.Errorf("unregistered invite to archived: expected ErrPropertyArchived, got %v", err)
	}
	if len(f.invitations.rows) != 0 {
		t.Errorf("no invitation must be stored, got %d", len(f.invitations.rows))
	}
	if len(f.mailer.sent) != 0 {
		t.Errorf("no invite email must be sent, sent %d", len(f.mailer.sent))
	}

	// Registered email path.
	invitee := uuid.Must(uuid.NewV7())
	f.lookup.add("friend@example.com", invitee)
	f.limiter.set(invitee, 10)
	if _, err := f.svc.InviteByEmail(t.Context(), owner, property, "friend@example.com",
		domain.RoleViewer); !errors.Is(err, domain.ErrPropertyArchived) {
		t.Errorf("registered invite to archived: expected ErrPropertyArchived, got %v", err)
	}
	if len(f.repo.rows) != 0 {
		t.Errorf("no membership must be created, got %d", len(f.repo.rows))
	}
}

func TestInvitationService_ActivationToArchivedPropertySkipsSlotCheck(t *testing.T) {
	t.Parallel()
	f := newInvitationFixture()
	owner := uuid.Must(uuid.NewV7())
	property := f.addProperty(owner)
	f.statuses[property] = true

	if _, err := f.invitations.Create(t.Context(), domain.Invitation{
		ID: uuid.Must(uuid.NewV7()), PropertyID: property, Email: testNewUserEmail,
		Role: domain.RoleViewer, InvitedBy: owner, LastSentAt: time.Now(),
	}); err != nil {
		t.Fatalf("create invitation: %v", err)
	}

	// Zero free slots: without the archived skip the membership would be
	// created suspended.
	user := uuid.Must(uuid.NewV7())
	f.limiter.set(user, 0)

	if err := f.svc.ActivatePendingInvitations(t.Context(), user, testNewUserEmail); err != nil {
		t.Fatalf("ActivatePendingInvitations: %v", err)
	}
	m, err := f.repo.GetByPropertyAndUser(t.Context(), property, user)
	if err != nil {
		t.Fatalf("membership: %v", err)
	}
	if m.Status != domain.MemberStatusActive {
		t.Errorf("activation to an archived property must be active (no slot needed), got %v", m.Status)
	}
}

func TestInvitationService_ActivationToActivePropertyWithoutSlotSuspends(t *testing.T) {
	t.Parallel()
	f := newInvitationFixture()
	owner := uuid.Must(uuid.NewV7())
	property := f.addProperty(owner)

	if _, err := f.invitations.Create(t.Context(), domain.Invitation{
		ID: uuid.Must(uuid.NewV7()), PropertyID: property, Email: testNewUserEmail,
		Role: domain.RoleViewer, InvitedBy: owner, LastSentAt: time.Now(),
	}); err != nil {
		t.Fatalf("create invitation: %v", err)
	}

	// Same setup as the archived case but the property stays active: the
	// regular T4 slot enforcement applies and the membership is suspended.
	user := uuid.Must(uuid.NewV7())
	f.limiter.set(user, 0)

	if err := f.svc.ActivatePendingInvitations(t.Context(), user, testNewUserEmail); err != nil {
		t.Fatalf("ActivatePendingInvitations: %v", err)
	}
	m, err := f.repo.GetByPropertyAndUser(t.Context(), property, user)
	if err != nil {
		t.Fatalf("membership: %v", err)
	}
	if m.Status != domain.MemberStatusSuspended {
		t.Errorf("activation without a free slot must be suspended, got %v", m.Status)
	}
}
