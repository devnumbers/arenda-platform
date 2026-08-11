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
)

// This file covers the sharing lifecycle emails (issue #162, T6) through the
// real AccessService + InvitationService + SlotCoordinator on in-memory ports
// (the same seam as service_integration_test.go and slot_coordinator_test.go),
// with a recording fake at the AccessMailer port. The emails are post-commit
// fire-and-forget: a send failure is logged and never fails the operation.

// ---------------------------------------------------------------------------
// Fakes for the lifecycle mail dependencies.
// ---------------------------------------------------------------------------

// sentMail records one email passed through the AccessMailer port. kind is the
// template name (invite, access_revoked, property_deleted, access_suspended,
// downgrade_summary, access_restored, invitation_activated, member_left).
type sentMail struct {
	kind        string
	to          string
	title       string
	titles      []string
	role        domain.Role
	memberEmail string
	memberName  string
}

// fakeAccessMailer records sent lifecycle emails; err simulates a send failure.
type fakeAccessMailer struct {
	sent []sentMail
	err  error
}

func (f *fakeAccessMailer) record(m sentMail) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, m)
	return nil
}

func (f *fakeAccessMailer) SendInvite(_ context.Context, to, title string, role domain.Role) error {
	return f.record(sentMail{kind: "invite", to: to, title: title, role: role})
}

func (f *fakeAccessMailer) SendAccessRevoked(_ context.Context, to, title string) error {
	return f.record(sentMail{kind: "access_revoked", to: to, title: title})
}

func (f *fakeAccessMailer) SendPropertyDeleted(_ context.Context, to, title string) error {
	return f.record(sentMail{kind: "property_deleted", to: to, title: title})
}

func (f *fakeAccessMailer) SendAccessSuspended(_ context.Context, to, title string) error {
	return f.record(sentMail{kind: "access_suspended", to: to, title: title})
}

func (f *fakeAccessMailer) SendDowngradeSummary(_ context.Context, to string, titles []string) error {
	return f.record(sentMail{kind: "downgrade_summary", to: to, titles: titles})
}

func (f *fakeAccessMailer) SendAccessRestored(_ context.Context, to, title string) error {
	return f.record(sentMail{kind: "access_restored", to: to, title: title})
}

func (f *fakeAccessMailer) SendInvitationActivated(_ context.Context, to, title, memberEmail string) error {
	return f.record(sentMail{kind: "invitation_activated", to: to, title: title, memberEmail: memberEmail})
}

func (f *fakeAccessMailer) SendMemberLeft(_ context.Context, to, title, memberName string) error {
	return f.record(sentMail{kind: "member_left", to: to, title: title, memberName: memberName})
}

// count returns how many emails of the given kind were sent.
func (f *fakeAccessMailer) count(kind string) int {
	n := 0
	for _, m := range f.sent {
		if m.kind == kind {
			n++
		}
	}
	return n
}

// only returns the single email of the given kind, failing the test otherwise.
func (f *fakeAccessMailer) only(t *testing.T, kind string) sentMail {
	t.Helper()
	if got := f.count(kind); got != 1 {
		t.Fatalf("expected exactly one %s email, got %d (all: %+v)", kind, got, f.sent)
	}
	for _, m := range f.sent {
		if m.kind == kind {
			return m
		}
	}
	panic("unreachable")
}

// reset clears the recorded sends (used between setup and the act step).
func (f *fakeAccessMailer) reset() { f.sent = nil }

// fakeEmailResolver is an in-memory UserEmailResolver keyed by user id; an
// absent user reads as "no email" and the send is skipped.
type fakeEmailResolver map[uuid.UUID]string

func (f fakeEmailResolver) GetEmail(_ context.Context, id uuid.UUID) (string, error) {
	return f[id], nil
}

// fakePropertyTitles resolves per-property titles for lifecycle emails.
type fakePropertyTitles map[uuid.UUID]string

func (f fakePropertyTitles) GetTitle(_ context.Context, id uuid.UUID) (string, error) {
	if title, ok := f[id]; ok {
		return title, nil
	}
	return "", domain.ErrMemberNotFound
}

var (
	_ AccessMailer          = (*fakeAccessMailer)(nil)
	_ UserEmailResolver     = fakeEmailResolver(nil)
	_ PropertyTitleResolver = fakePropertyTitles(nil)
)

// ---------------------------------------------------------------------------
// Fixture.
// ---------------------------------------------------------------------------

// lifecycleFixture bundles the access, invitation and slot coordinator services
// wired with the recording mailer, the email resolver and per-property titles.
type lifecycleFixture struct {
	repo        *memRepo
	invitations *memInvitationsRepo
	owners      staticResolver
	lookup      *fakeLookup
	mailer      *fakeAccessMailer
	emails      fakeEmailResolver
	titles      fakePropertyTitles
	limiter     *fakeRecipientLimiter
	access      *AccessService
	invites     *InvitationService
	slots       *SlotCoordinator
}

func newLifecycleFixture() *lifecycleFixture {
	repo := newMemRepo()
	invitations := &memInvitationsRepo{}
	owners := staticResolver{}
	policy := NewMembershipPolicy(owners, repo)
	lookup := newFakeLookup()
	mailer := &fakeAccessMailer{}
	emails := fakeEmailResolver{}
	titles := fakePropertyTitles{}
	limiter := newFakeRecipientLimiter()
	lifecycle := NewLifecycleMailer(mailer, emails, titles, nil)
	slots := NewSlotCoordinator(repo, owners, limiter, newFakeOccupancy(), newFakeOwnedProps(), lifecycle, auditapp.Noop{}, noopBeginner{})
	access := NewAccessService(repo, owners, nil, lookup, policy, slots, lifecycle, noopBeginner{}, auditapp.Noop{}, nil)
	invites := NewInvitationService(access, repo, invitations, owners, nil, lookup, policy, slots, mailer, lifecycle, titles, noopBeginner{}, auditapp.Noop{}, nil, nil)
	return &lifecycleFixture{
		repo:        repo,
		invitations: invitations,
		owners:      owners,
		lookup:      lookup,
		mailer:      mailer,
		emails:      emails,
		titles:      titles,
		limiter:     limiter,
		access:      access,
		invites:     invites,
		slots:       slots,
	}
}

// addProperty registers a property with its owner in every in-memory port and
// records its display title for the lifecycle emails.
func (f *lifecycleFixture) addProperty(ownerID uuid.UUID, title string) uuid.UUID {
	propertyID := uuid.New()
	f.owners[propertyID] = ownerID
	f.repo.SetOwner(propertyID, ownerID)
	f.titles[propertyID] = title
	return propertyID
}

// addMember inserts an active membership directly (bypassing the service) with
// a deterministic UpdatedAt for the eviction comparator.
func (f *lifecycleFixture) addMember(t *testing.T, propertyID, ownerID, userID uuid.UUID, status domain.MemberStatus, updatedAt time.Time) {
	t.Helper()
	memberID := uuid.New()
	m := domain.Membership{
		ID:         memberID,
		PropertyID: propertyID,
		UserID:     userID,
		Role:       domain.RoleViewer,
		GrantedBy:  ownerID,
		Status:     status,
	}
	var err error
	if status == domain.MemberStatusSuspended {
		m.SuspendedAt = &updatedAt
		_, err = f.repo.CreateWithStatus(t.Context(), m)
	} else {
		_, err = f.repo.Create(t.Context(), m)
	}
	if err != nil {
		t.Fatalf("create member: %v", err)
	}
	f.repo.setUpdatedAt(memberID, propertyID, updatedAt)
}

// ---------------------------------------------------------------------------
// Revoke: an active member gets the "access revoked" email; a suspended one
// does not.
// ---------------------------------------------------------------------------

func TestAccessService_RevokeActiveMemberSendsRevokedEmail(t *testing.T) {
	f := newLifecycleFixture()
	owner := uuid.New()
	property := f.addProperty(owner, "Квартира на Невском")
	member := uuid.New()
	f.emails[member] = "member@example.com"
	f.limiter.set(member, 10)

	m, err := f.access.AddMember(t.Context(), owner, property, member, domain.RoleViewer)
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if m.Status != domain.MemberStatusActive {
		t.Fatalf("member must be active, got %v", m.Status)
	}
	f.mailer.reset()

	if err := f.access.RevokeMember(t.Context(), owner, property, m.ID); err != nil {
		t.Fatalf("RevokeMember: %v", err)
	}

	mail := f.mailer.only(t, "access_revoked")
	if mail.to != "member@example.com" {
		t.Errorf("revoked mail to = %q, want member@example.com", mail.to)
	}
	if mail.title != "Квартира на Невском" {
		t.Errorf("revoked mail title = %q", mail.title)
	}
}

func TestAccessService_RevokeSuspendedMemberSendsNoEmail(t *testing.T) {
	f := newLifecycleFixture()
	owner := uuid.New()
	property := f.addProperty(owner, "Квартира на Невском")
	member := uuid.New()
	f.emails[member] = "member@example.com"
	f.limiter.set(member, 0) // no free slot: the grant is created suspended

	m, err := f.access.AddMember(t.Context(), owner, property, member, domain.RoleViewer)
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if m.Status != domain.MemberStatusSuspended {
		t.Fatalf("member must be suspended, got %v", m.Status)
	}
	f.mailer.reset()

	if err := f.access.RevokeMember(t.Context(), owner, property, m.ID); err != nil {
		t.Fatalf("RevokeMember: %v", err)
	}
	if got := f.mailer.count("access_revoked"); got != 0 {
		t.Errorf("revoking a suspended member must not send access_revoked, got %d", got)
	}
}

func TestAccessService_RevokeMailFailureDoesNotFailRevoke(t *testing.T) {
	f := newLifecycleFixture()
	owner := uuid.New()
	property := f.addProperty(owner, "Квартира на Невском")
	member := uuid.New()
	f.emails[member] = "member@example.com"
	f.limiter.set(member, 10)

	m, err := f.access.AddMember(t.Context(), owner, property, member, domain.RoleViewer)
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	f.mailer.err = errors.New("smtp down")

	if err := f.access.RevokeMember(t.Context(), owner, property, m.ID); err != nil {
		t.Fatalf("mail failure must not fail the revoke: %v", err)
	}
	if _, err := f.repo.GetByID(t.Context(), m.ID, property); !errors.Is(err, domain.ErrMemberNotFound) {
		t.Errorf("membership must be deleted despite the mail failure, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// AddMember / activation without a free slot: the "waiting for a slot" email.
// ---------------------------------------------------------------------------

func TestAccessService_AddMemberWithoutSlotSendsWaitingEmail(t *testing.T) {
	f := newLifecycleFixture()
	owner := uuid.New()
	property := f.addProperty(owner, "Квартира на Невском")

	member := uuid.New()
	f.emails[member] = "member@example.com"
	f.limiter.set(member, 0) // pool full: suspended grant

	if _, err := f.access.AddMember(t.Context(), owner, property, member, domain.RoleViewer); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	mail := f.mailer.only(t, "access_suspended")
	if mail.to != "member@example.com" || mail.title != "Квартира на Невском" {
		t.Errorf("waiting mail = %+v", mail)
	}

	// An active grant sends nothing.
	other := uuid.New()
	f.emails[other] = "other@example.com"
	f.limiter.set(other, 10)
	if _, err := f.access.AddMember(t.Context(), owner, property, other, domain.RoleViewer); err != nil {
		t.Fatalf("AddMember other: %v", err)
	}
	if got := f.mailer.count("access_suspended"); got != 1 {
		t.Errorf("active add must not send access_suspended, total %d", got)
	}
}

// ---------------------------------------------------------------------------
// Self-exit: the owner is notified; the leaving member gets nothing.
// ---------------------------------------------------------------------------

func TestAccessService_LeavePropertyNotifiesOwner(t *testing.T) {
	f := newLifecycleFixture()
	owner := uuid.New()
	property := f.addProperty(owner, "Квартира на Невском")
	f.emails[owner] = "owner@example.com"
	member := uuid.New()
	f.emails[member] = "member@example.com"
	f.limiter.set(member, 10)

	if _, err := f.access.AddMember(t.Context(), owner, property, member, domain.RoleViewer); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	f.mailer.reset()

	if err := f.access.LeaveProperty(t.Context(), member, property); err != nil {
		t.Fatalf("LeaveProperty: %v", err)
	}

	mail := f.mailer.only(t, "member_left")
	if mail.to != "owner@example.com" {
		t.Errorf("member_left mail to = %q, want the owner", mail.to)
	}
	if mail.title != "Квартира на Невском" {
		t.Errorf("member_left mail title = %q", mail.title)
	}
	if mail.memberName == "" {
		t.Error("member_left mail must carry the member display name")
	}
	for _, m := range f.mailer.sent {
		if m.to == "member@example.com" {
			t.Errorf("the leaving member must not receive any email, got %+v", m)
		}
	}
}

// ---------------------------------------------------------------------------
// Activation at registration: the owner is notified; a suspended activation
// additionally sends the "waiting for a slot" email to the new member.
// ---------------------------------------------------------------------------

func TestInvitationService_ActivationAtRegistrationNotifiesOwner(t *testing.T) {
	f := newLifecycleFixture()
	owner := uuid.New()
	property := f.addProperty(owner, "Квартира на Невском")
	f.emails[owner] = "owner@example.com"

	if _, err := f.invitations.Create(t.Context(), domain.Invitation{
		ID: uuid.New(), PropertyID: property, Email: "new@example.com",
		Role: domain.RoleViewer, InvitedBy: owner, LastSentAt: time.Now(),
	}); err != nil {
		t.Fatalf("create invitation: %v", err)
	}

	user := uuid.New()
	f.emails[user] = "new@example.com"
	f.limiter.set(user, 10)
	if err := f.invites.ActivatePendingInvitations(t.Context(), user, "new@example.com"); err != nil {
		t.Fatalf("ActivatePendingInvitations: %v", err)
	}

	mail := f.mailer.only(t, "invitation_activated")
	if mail.to != "owner@example.com" {
		t.Errorf("invitation_activated mail to = %q, want the owner", mail.to)
	}
	if mail.memberEmail != "new@example.com" {
		t.Errorf("invitation_activated memberEmail = %q", mail.memberEmail)
	}
	if mail.title != "Квартира на Невском" {
		t.Errorf("invitation_activated title = %q", mail.title)
	}
	if got := f.mailer.count("access_suspended"); got != 0 {
		t.Errorf("activation with a free slot must not send access_suspended, got %d", got)
	}
}

func TestInvitationService_ActivationWithoutSlotSendsWaitingEmail(t *testing.T) {
	f := newLifecycleFixture()
	owner := uuid.New()
	property := f.addProperty(owner, "Квартира на Невском")
	f.emails[owner] = "owner@example.com"

	if _, err := f.invitations.Create(t.Context(), domain.Invitation{
		ID: uuid.New(), PropertyID: property, Email: "new@example.com",
		Role: domain.RoleViewer, InvitedBy: owner, LastSentAt: time.Now(),
	}); err != nil {
		t.Fatalf("create invitation: %v", err)
	}

	user := uuid.New()
	f.emails[user] = "new@example.com"
	f.limiter.set(user, 0) // no free slot: the membership is created suspended
	if err := f.invites.ActivatePendingInvitations(t.Context(), user, "new@example.com"); err != nil {
		t.Fatalf("ActivatePendingInvitations: %v", err)
	}

	mail := f.mailer.only(t, "access_suspended")
	if mail.to != "new@example.com" || mail.title != "Квартира на Невском" {
		t.Errorf("waiting mail = %+v", mail)
	}
	// The owner is still notified about the activation.
	if got := f.mailer.count("invitation_activated"); got != 1 {
		t.Errorf("owner must be notified about the activation, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// Downgrade: one summary email per recipient with the titles suspended in
// this enforcement call.
// ---------------------------------------------------------------------------

func TestSlotCoordinator_DowngradeSendsSingleSummaryEmail(t *testing.T) {
	f := newLifecycleFixture()
	owner := uuid.New()
	p1 := f.addProperty(owner, "Квартира")
	p2 := f.addProperty(owner, "Дача")
	recipient := uuid.New()
	f.emails[recipient] = "recipient@example.com"

	// m1 is the earlier-updated membership — the eviction candidate.
	f.addMember(t, p1, owner, recipient, domain.MemberStatusActive, t1Old)
	f.addMember(t, p2, owner, recipient, domain.MemberStatusActive, t2New)
	f.limiter.set(recipient, 1)

	if err := f.slots.EnforceRecipientLimit(t.Context(), noopTx{}, owner, "downgrade"); err != nil {
		t.Fatalf("EnforceRecipientLimit: %v", err)
	}

	mail := f.mailer.only(t, "downgrade_summary")
	if mail.to != "recipient@example.com" {
		t.Errorf("summary mail to = %q", mail.to)
	}
	if !slices.Equal(mail.titles, []string{"Квартира"}) {
		t.Errorf("summary titles = %v, want [Квартира]", mail.titles)
	}
	// The per-membership waiting email is NOT sent on a downgrade: the summary
	// replaces it.
	if got := f.mailer.count("access_suspended"); got != 0 {
		t.Errorf("downgrade must not send per-membership access_suspended, got %d", got)
	}
}

func TestSlotCoordinator_DowngradeWithoutExcessSendsNoEmail(t *testing.T) {
	f := newLifecycleFixture()
	owner := uuid.New()
	p1 := f.addProperty(owner, "Квартира")
	recipient := uuid.New()
	f.emails[recipient] = "recipient@example.com"

	f.addMember(t, p1, owner, recipient, domain.MemberStatusActive, t1Old)
	f.limiter.set(recipient, 10) // within the limit

	if err := f.slots.EnforceRecipientLimit(t.Context(), noopTx{}, owner, "downgrade"); err != nil {
		t.Fatalf("EnforceRecipientLimit: %v", err)
	}
	if got := f.mailer.count("downgrade_summary"); got != 0 {
		t.Errorf("no excess → no summary email, got %d", got)
	}
}

// The downgrading user himself (billing passes sub.UserID) receives the single
// summary for his suspended shared memberships on other owners' objects.
func TestSlotCoordinator_DowngradingRecipientGetsSummaryEmail(t *testing.T) {
	f := newLifecycleFixture()
	foreignOwner := uuid.New()
	p1 := f.addProperty(foreignOwner, "Квартира")
	p2 := f.addProperty(foreignOwner, "Дача")
	recipient := uuid.New()
	f.emails[recipient] = "recipient@example.com"

	// p1 is the earlier-updated membership — the eviction candidate.
	f.addMember(t, p1, foreignOwner, recipient, domain.MemberStatusActive, t1Old)
	f.addMember(t, p2, foreignOwner, recipient, domain.MemberStatusActive, t2New)
	f.limiter.set(recipient, 1)

	// Billing calls EnforceRecipientLimit with the downgrading user's own id.
	if err := f.slots.EnforceRecipientLimit(t.Context(), noopTx{}, recipient, "downgrade"); err != nil {
		t.Fatalf("EnforceRecipientLimit: %v", err)
	}

	mail := f.mailer.only(t, "downgrade_summary")
	if mail.to != "recipient@example.com" {
		t.Errorf("summary mail to = %q", mail.to)
	}
	if !slices.Equal(mail.titles, []string{"Квартира"}) {
		t.Errorf("summary titles = %v, want [Квартира]", mail.titles)
	}
}

// The downgrading user who is both an owner with members and a recipient on a
// foreign object is processed exactly once: one summary to him, one to the
// member — no duplicates.
func TestSlotCoordinator_DowngradeOwnerAndRecipientNoDuplicateSummary(t *testing.T) {
	f := newLifecycleFixture()
	user := uuid.New()
	f.emails[user] = "user@example.com"
	ownProp := f.addProperty(user, "Своя квартира")
	foreignOwner := uuid.New()
	foreignProp := f.addProperty(foreignOwner, "Чужая дача")
	member := uuid.New()
	f.emails[member] = "member@example.com"

	f.addMember(t, ownProp, user, member, domain.MemberStatusActive, t1Old)
	f.addMember(t, foreignProp, foreignOwner, user, domain.MemberStatusActive, t2New)
	f.limiter.set(member, 0) // the member's pool exceeds
	f.limiter.set(user, 0)   // the user's own pool (1 shared) exceeds

	if err := f.slots.EnforceRecipientLimit(t.Context(), noopTx{}, user, "downgrade"); err != nil {
		t.Fatalf("EnforceRecipientLimit: %v", err)
	}

	if got := f.mailer.count("downgrade_summary"); got != 2 {
		t.Fatalf("expected exactly 2 summaries (member + user), got %d (all: %+v)", got, f.mailer.sent)
	}
	userSummaries := 0
	for _, m := range f.mailer.sent {
		if m.kind == "downgrade_summary" && m.to == "user@example.com" {
			userSummaries++
			if !slices.Equal(m.titles, []string{"Чужая дача"}) {
				t.Errorf("user summary titles = %v, want [Чужая дача]", m.titles)
			}
		}
	}
	if userSummaries != 1 {
		t.Errorf("the downgrading user must get exactly one summary, got %d", userSummaries)
	}
}

// ---------------------------------------------------------------------------
// Recovery: the "access restored" email per reactivated membership.
// ---------------------------------------------------------------------------

func TestSlotCoordinator_RecoverSendsRestoredEmail(t *testing.T) {
	f := newLifecycleFixture()
	owner := uuid.New()
	p1 := f.addProperty(owner, "Квартира")
	recipient := uuid.New()
	f.emails[recipient] = "recipient@example.com"

	f.addMember(t, p1, owner, recipient, domain.MemberStatusSuspended, t1Old)
	f.limiter.set(recipient, 1) // one free slot (pool empty)

	if err := f.slots.RecoverSuspended(t.Context(), noopTx{}, recipient); err != nil {
		t.Fatalf("RecoverSuspended: %v", err)
	}

	mail := f.mailer.only(t, "access_restored")
	if mail.to != "recipient@example.com" || mail.title != "Квартира" {
		t.Errorf("restored mail = %+v", mail)
	}
}

// ---------------------------------------------------------------------------
// Unarchive without a free slot: the "waiting for a slot" email.
// ---------------------------------------------------------------------------

func TestSlotCoordinator_UnarchiveWithoutSlotSendsWaitingEmail(t *testing.T) {
	f := newLifecycleFixture()
	owner := uuid.New()
	property := f.addProperty(owner, "Квартира")
	recipient := uuid.New()
	f.emails[recipient] = "recipient@example.com"

	f.addMember(t, property, owner, recipient, domain.MemberStatusActive, t1Old)
	f.limiter.set(recipient, 0) // the re-entering object exceeds the pool

	if err := f.slots.EnforceOnUnarchiveForProperty(t.Context(), noopTx{}, property); err != nil {
		t.Fatalf("EnforceOnUnarchiveForProperty: %v", err)
	}

	mail := f.mailer.only(t, "access_suspended")
	if mail.to != "recipient@example.com" || mail.title != "Квартира" {
		t.Errorf("waiting mail = %+v", mail)
	}
}

// ---------------------------------------------------------------------------
// Property deletion: former members (active + suspended) are collected inside
// the delete transaction and notified after commit; members without an email
// are skipped.
// ---------------------------------------------------------------------------

func TestPropertyDeleteMailer_CollectsFormerMembersAndSends(t *testing.T) {
	f := newLifecycleFixture()
	owner := uuid.New()
	property := f.addProperty(owner, "Квартира")

	active := uuid.New()
	suspended := uuid.New()
	noEmail := uuid.New()
	f.emails[active] = "active@example.com"
	f.emails[suspended] = "suspended@example.com"
	f.addMember(t, property, owner, active, domain.MemberStatusActive, t1Old)
	f.addMember(t, property, owner, suspended, domain.MemberStatusSuspended, t2New)
	f.addMember(t, property, owner, noEmail, domain.MemberStatusActive, t3Newer)

	deleter := NewPropertyDeleteMailer(f.repo, f.emails, f.mailer, nil)
	emails, err := deleter.CollectFormerMemberEmails(t.Context(), noopTx{}, property)
	if err != nil {
		t.Fatalf("CollectFormerMemberEmails: %v", err)
	}
	slices.Sort(emails)
	if !slices.Equal(emails, []string{"active@example.com", "suspended@example.com"}) {
		t.Errorf("collected emails = %v, want active+suspended only", emails)
	}

	for _, to := range emails {
		if err := deleter.SendPropertyDeleted(t.Context(), to, "Квартира"); err != nil {
			t.Fatalf("SendPropertyDeleted: %v", err)
		}
	}
	if got := f.mailer.count("property_deleted"); got != 2 {
		t.Fatalf("expected 2 property_deleted emails, got %d", got)
	}
	for _, m := range f.mailer.sent {
		if m.title != "Квартира" {
			t.Errorf("property_deleted title = %q", m.title)
		}
	}
}
