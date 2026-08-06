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

// ---------------------------------------------------------------------------
// In-memory port stubs for the invitation dependencies.
// ---------------------------------------------------------------------------

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

// sentInvite records one invite email passed through the mailer port.
type sentInvite struct {
	to    string
	title string
	role  domain.Role
}

// fakeInvitationMailer records sent invite emails; err simulates a send
// failure.
type fakeInvitationMailer struct {
	sent []sentInvite
	err  error
}

func (f *fakeInvitationMailer) SendInvite(_ context.Context, to, title string, role domain.Role) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, sentInvite{to: to, title: title, role: role})
	return nil
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
	_ InvitationMailer      = (*fakeInvitationMailer)(nil)
	_ PropertyTitleResolver = fakeTitles("")
)

// invitationFixture bundles the invitation service with its in-memory
// dependencies.
type invitationFixture struct {
	repo        *memRepo
	invitations *memInvitationsRepo
	owners      staticResolver
	lookup      *fakeLookup
	mailer      *fakeInvitationMailer
	clk         *fixedClock
	limiter     *fakeRecipientLimiter
	access      *AccessService
	svc         *InvitationService
}

func newInvitationFixture() *invitationFixture {
	repo := newMemRepo()
	invitations := &memInvitationsRepo{}
	owners := staticResolver{}
	policy := NewMembershipPolicy(owners, repo)
	lookup := newFakeLookup()
	mailer := &fakeInvitationMailer{}
	clk := &fixedClock{now: time.Now()}
	limiter := newFakeRecipientLimiter()
	coordinator := NewSlotCoordinator(repo, owners, limiter, newFakeOccupancy(), newFakeOwnedProps(), auditapp.Noop{}, noopBeginner{})
	access := NewAccessService(repo, owners, lookup, policy, coordinator, noopBeginner{}, auditapp.Noop{}, nil)
	svc := NewInvitationService(access, repo, invitations, owners, lookup, policy, coordinator, mailer, fakeTitles("Квартира на Невском"), noopBeginner{}, auditapp.Noop{}, clk, nil)
	return &invitationFixture{
		repo:        repo,
		invitations: invitations,
		owners:      owners,
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
	propertyID := uuid.New()
	f.owners[propertyID] = ownerID
	f.repo.SetOwner(propertyID, ownerID)
	return propertyID
}

// ---------------------------------------------------------------------------
// Invite: registered email → instant activation.
// ---------------------------------------------------------------------------

func TestInvitationService_InviteRegisteredEmailActivatesInstantly(t *testing.T) {
	f := newInvitationFixture()
	owner := uuid.New()
	property := f.addProperty(owner)
	invitee := uuid.New()
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
	if _, err := f.svc.InviteByEmail(t.Context(), owner, property, "friend@example.com", domain.RoleViewer); !errors.Is(err, domain.ErrMemberAlreadyExists) {
		t.Errorf("re-invite member: expected ErrMemberAlreadyExists, got %v", err)
	}
}

func TestInvitationService_InviteOwnerOrSelfEmailRejected(t *testing.T) {
	f := newInvitationFixture()
	owner := uuid.New()
	property := f.addProperty(owner)
	f.lookup.add("owner@example.com", owner)

	if _, err := f.svc.InviteByEmail(t.Context(), owner, property, "owner@example.com", domain.RoleViewer); !errors.Is(err, domain.ErrCannotAddOwner) {
		t.Errorf("owner email: expected ErrCannotAddOwner, got %v", err)
	}
}

func TestInvitationService_InviteInvalidEmailRejected(t *testing.T) {
	f := newInvitationFixture()
	owner := uuid.New()
	property := f.addProperty(owner)

	for _, raw := range []string{"", "not-an-email", "a@b"} {
		if _, err := f.svc.InviteByEmail(t.Context(), owner, property, raw, domain.RoleViewer); !errors.Is(err, domain.ErrInvalidEmail) {
			t.Errorf("email %q: expected ErrInvalidEmail, got %v", raw, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Invite: unregistered email → pending invitation + single invite email.
// ---------------------------------------------------------------------------

func TestInvitationService_InviteUnregisteredCreatesPending(t *testing.T) {
	f := newInvitationFixture()
	owner := uuid.New()
	property := f.addProperty(owner)

	outcome, err := f.svc.InviteByEmail(t.Context(), owner, property, "New@Example.com", domain.RoleFullAccess)
	if err != nil {
		t.Fatalf("InviteByEmail: %v", err)
	}
	if outcome.Invitation == nil || outcome.Member != nil {
		t.Fatalf("expected pending invitation, got %+v", outcome)
	}
	inv := *outcome.Invitation
	if inv.Email != "new@example.com" {
		t.Errorf("email = %q, want normalized new@example.com", inv.Email)
	}
	if inv.Role != domain.RoleFullAccess || inv.InvitedBy != owner {
		t.Errorf("invitation = %+v", inv)
	}
	if inv.LastSentAt.IsZero() {
		t.Error("last_sent_at must be set at creation")
	}
	if len(f.mailer.sent) != 1 || f.mailer.sent[0].to != "new@example.com" {
		t.Fatalf("expected exactly one invite email, got %+v", f.mailer.sent)
	}
	if f.mailer.sent[0].title != "Квартира на Невском" {
		t.Errorf("mail title = %q", f.mailer.sent[0].title)
	}

	// A duplicate pending invite is a conflict and sends no second email.
	if _, err := f.svc.InviteByEmail(t.Context(), owner, property, "new@example.com", domain.RoleViewer); !errors.Is(err, domain.ErrInvitationAlreadyExists) {
		t.Errorf("duplicate invite: expected ErrInvitationAlreadyExists, got %v", err)
	}
	if len(f.mailer.sent) != 1 {
		t.Errorf("duplicate invite must not send, sent %d", len(f.mailer.sent))
	}
}

func TestInvitationService_InviteMailFailureKeepsInvitation(t *testing.T) {
	f := newInvitationFixture()
	owner := uuid.New()
	property := f.addProperty(owner)
	f.mailer.err = errors.New("smtp down")

	outcome, err := f.svc.InviteByEmail(t.Context(), owner, property, "new@example.com", domain.RoleViewer)
	if err != nil {
		t.Fatalf("mail failure must not fail the invite: %v", err)
	}
	if outcome.Invitation == nil {
		t.Fatal("invitation must stay after a mail failure")
	}
}

// ---------------------------------------------------------------------------
// Resend with the 24h cooldown.
// ---------------------------------------------------------------------------

func TestInvitationService_ResendCooldown(t *testing.T) {
	f := newInvitationFixture()
	owner := uuid.New()
	property := f.addProperty(owner)

	outcome, err := f.svc.InviteByEmail(t.Context(), owner, property, "new@example.com", domain.RoleViewer)
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
	if err := f.svc.ResendInvitation(t.Context(), owner, property, uuid.New()); !errors.Is(err, domain.ErrInvitationNotFound) {
		t.Errorf("unknown invitation: expected ErrInvitationNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Role change and cancellation of a pending invitation.
// ---------------------------------------------------------------------------

func TestInvitationService_ChangeRoleAndCancel(t *testing.T) {
	f := newInvitationFixture()
	owner := uuid.New()
	property := f.addProperty(owner)

	outcome, err := f.svc.InviteByEmail(t.Context(), owner, property, "new@example.com", domain.RoleViewer)
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
	if _, err := f.svc.InviteByEmail(t.Context(), owner, property, "new@example.com", domain.RoleViewer); err != nil {
		t.Errorf("re-invite after cancel: %v", err)
	}

	// A viewer cannot manage invitations.
	viewer := uuid.New()
	if _, err := f.access.AddMember(t.Context(), owner, property, viewer, domain.RoleViewer); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if _, err := f.svc.InviteByEmail(t.Context(), viewer, property, "other@example.com", domain.RoleViewer); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("viewer invite: expected ErrMemberNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Activation at registration.
// ---------------------------------------------------------------------------

func TestInvitationService_ActivatePendingInvitations(t *testing.T) {
	f := newInvitationFixture()
	owner := uuid.New()
	propA := f.addProperty(owner)
	propB := f.addProperty(owner)
	other := f.addProperty(owner)

	// Two pending invitations on the same email (propA is older), one on
	// another email.
	base := time.Now().Add(-time.Hour)
	invA, err := f.invitations.Create(t.Context(), domain.Invitation{ID: uuid.New(), PropertyID: propA, Email: "new@example.com", Role: domain.RoleViewer, InvitedBy: owner, LastSentAt: base, CreatedAt: base})
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	invB, err := f.invitations.Create(t.Context(), domain.Invitation{ID: uuid.New(), PropertyID: propB, Email: "new@example.com", Role: domain.RoleFullAccess, InvitedBy: owner, LastSentAt: base, CreatedAt: base.Add(time.Minute)})
	if err != nil {
		t.Fatalf("create B: %v", err)
	}
	if _, err := f.invitations.Create(t.Context(), domain.Invitation{ID: uuid.New(), PropertyID: other, Email: "someone-else@example.com", Role: domain.RoleViewer, InvitedBy: owner, LastSentAt: base, CreatedAt: base}); err != nil {
		t.Fatalf("create other: %v", err)
	}

	user := uuid.New()
	f.limiter.set(user, 10)
	if err := f.svc.ActivatePendingInvitations(t.Context(), user, "New@Example.com"); err != nil {
		t.Fatalf("ActivatePendingInvitations: %v", err)
	}

	// Both invitations activated with their stored roles.
	mA, err := f.repo.GetByPropertyAndUser(t.Context(), propA, user)
	if err != nil {
		t.Fatalf("membership A: %v", err)
	}
	if mA.Role != domain.RoleViewer || mA.GrantedBy != owner || mA.Status != domain.MemberStatusActive {
		t.Errorf("membership A = %+v", mA)
	}
	mB, err := f.repo.GetByPropertyAndUser(t.Context(), propB, user)
	if err != nil {
		t.Fatalf("membership B: %v", err)
	}
	if mB.Role != domain.RoleFullAccess {
		t.Errorf("membership B role = %v, want full_access", mB.Role)
	}

	// Invitations are gone; the other email's invitation is untouched.
	if _, err := f.invitations.GetByID(t.Context(), invA.ID, propA); !errors.Is(err, domain.ErrInvitationNotFound) {
		t.Errorf("invitation A must be deleted, got %v", err)
	}
	if _, err := f.invitations.GetByID(t.Context(), invB.ID, propB); !errors.Is(err, domain.ErrInvitationNotFound) {
		t.Errorf("invitation B must be deleted, got %v", err)
	}
	remaining, err := f.invitations.ListPendingByEmail(t.Context(), "someone-else@example.com")
	if err != nil || len(remaining) != 1 {
		t.Errorf("other email invitation must stay, got %v %v", remaining, err)
	}

	// A non-matching email activates nothing.
	if err := f.svc.ActivatePendingInvitations(t.Context(), uuid.New(), "nobody@example.com"); err != nil {
		t.Fatalf("activate non-matching: %v", err)
	}
	// An empty email is a no-op, not an error.
	if err := f.svc.ActivatePendingInvitations(t.Context(), uuid.New(), ""); err != nil {
		t.Fatalf("activate empty email: %v", err)
	}
}

func TestInvitationService_ActivationFIFOWhenSlotShort(t *testing.T) {
	f := newInvitationFixture()
	owner := uuid.New()
	propA := f.addProperty(owner)
	propB := f.addProperty(owner)

	base := time.Now().Add(-time.Hour)
	if _, err := f.invitations.Create(t.Context(), domain.Invitation{ID: uuid.New(), PropertyID: propA, Email: "new@example.com", Role: domain.RoleViewer, InvitedBy: owner, LastSentAt: base, CreatedAt: base}); err != nil {
		t.Fatalf("create A: %v", err)
	}
	if _, err := f.invitations.Create(t.Context(), domain.Invitation{ID: uuid.New(), PropertyID: propB, Email: "new@example.com", Role: domain.RoleViewer, InvitedBy: owner, LastSentAt: base, CreatedAt: base.Add(time.Minute)}); err != nil {
		t.Fatalf("create B: %v", err)
	}

	// One free slot only: the earlier invitation activates active, the later
	// one is created suspended (T4 mechanics via the real SlotCoordinator).
	user := uuid.New()
	f.limiter.set(user, 1)

	if err := f.svc.ActivatePendingInvitations(t.Context(), user, "new@example.com"); err != nil {
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
	f := newInvitationFixture()
	owner := uuid.New()
	property := f.addProperty(owner)

	outcome, err := f.svc.InviteByEmail(t.Context(), owner, property, "new@example.com", domain.RoleViewer)
	if err != nil {
		t.Fatalf("InviteByEmail: %v", err)
	}
	// The role is changed after the invite; no new email, and the role current
	// at registration time is applied.
	if _, err := f.svc.ChangeInvitationRole(t.Context(), owner, property, outcome.Invitation.ID, domain.RoleFullAccess); err != nil {
		t.Fatalf("ChangeInvitationRole: %v", err)
	}

	user := uuid.New()
	if err := f.svc.ActivatePendingInvitations(t.Context(), user, "new@example.com"); err != nil {
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
	f := newInvitationFixture()
	owner := uuid.New()
	property := f.addProperty(owner)

	user := uuid.New()
	// The user is already a (suspended) member of the property; a pending
	// invitation for the same email must be dropped silently without creating
	// a duplicate membership.
	if _, err := f.repo.CreateWithStatus(t.Context(), domain.Membership{ID: uuid.New(), PropertyID: property, UserID: user, Role: domain.RoleViewer, GrantedBy: owner, Status: domain.MemberStatusSuspended}); err != nil {
		t.Fatalf("CreateWithStatus: %v", err)
	}
	inv, err := f.invitations.Create(t.Context(), domain.Invitation{ID: uuid.New(), PropertyID: property, Email: "new@example.com", Role: domain.RoleFullAccess, InvitedBy: owner, LastSentAt: time.Now()})
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}

	if err := f.svc.ActivatePendingInvitations(t.Context(), user, "new@example.com"); err != nil {
		t.Fatalf("ActivatePendingInvitations: %v", err)
	}
	if _, err := f.invitations.GetByID(t.Context(), inv.ID, property); !errors.Is(err, domain.ErrInvitationNotFound) {
		t.Errorf("invitation must be dropped silently, got %v", err)
	}
	if got := len(f.repo.rows); got != 1 {
		t.Errorf("exactly one membership expected, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// Member list: pending invitations are manager-only.
// ---------------------------------------------------------------------------

func TestInvitationService_ListMembersIncludesPendingForManagers(t *testing.T) {
	f := newInvitationFixture()
	owner := uuid.New()
	property := f.addProperty(owner)
	viewer := uuid.New()
	f.limiter.set(viewer, 10)
	if _, err := f.access.AddMember(t.Context(), owner, property, viewer, domain.RoleViewer); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if _, err := f.svc.InviteByEmail(t.Context(), owner, property, "new@example.com", domain.RoleFullAccess); err != nil {
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
	if !pending.Pending || pending.Email == nil || *pending.Email != "new@example.com" {
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
