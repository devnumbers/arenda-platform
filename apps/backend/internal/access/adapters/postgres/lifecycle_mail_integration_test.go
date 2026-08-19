package postgres

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	accessemail "github.com/nambers/arenda-planform/apps/backend/internal/access/adapters/email"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	identitypg "github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/encryption"
	genpostgres "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/mailer"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// These integration tests run against a real Postgres via TEST_DATABASE_URL
// (skipped when unset, same convention as the other access integration tests).
// They exercise the sharing lifecycle emails (issue #162, T6) end to end: the
// real AccessService / InvitationService / SlotCoordinator / PropertyDeleteMailer
// over the real repositories, the real email renderer loading
// templates/email, and a capturing platform sender asserting which email goes
// out at every lifecycle event.

// lifecycleNoCommitTx wraps the test's pgx.Tx as a transaction.Tx whose
// commit/rollback are no-ops (the outer test transaction is rolled back in
// cleanup). The embedded pgx.Tx still satisfies postgres.DBTX, so real
// repositories bind to it via WithTx. Same pattern as the scheduler reminder
// integration tests.
type lifecycleNoCommitTx struct{ pgx.Tx }

func (lifecycleNoCommitTx) Commit(context.Context) error   { return nil }
func (lifecycleNoCommitTx) Rollback(context.Context) error { return nil }

// lifecycleBeginner always returns the test's already-open transaction.
type lifecycleBeginner struct{ tx pgx.Tx }

func (b lifecycleBeginner) Begin(context.Context) (transaction.Tx, error) {
	return lifecycleNoCommitTx{b.tx}, nil
}

// lifecycleUoW adapts lifecycleBeginner to the transaction.UoW port so the
// services under test open their transactions through runInTx (ADR 0033); the
// no-commit tx keeps the outer test transaction in charge of cleanup.
type lifecycleUoW struct{ beginner lifecycleBeginner }

func (u lifecycleUoW) Do(ctx context.Context, work func(tx transaction.Tx) error) (err error) {
	tx, err := u.beginner.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		// Same defer shape as the production UoW (rollback, then recover and
		// re-panic). Unlike prod's `_ =` discard, the rollback error is folded
		// into the named return — and only when work succeeded, so it never
		// masks the work error or the re-panicked value. The wrapped
		// lifecycleNoCommitTx rollback is a no-op (the outer test transaction
		// owns cleanup), so the fold never fires.
		rollbackErr := tx.Rollback(ctx)
		if r := recover(); r != nil {
			panic(r)
		}
		if rollbackErr != nil && err == nil {
			err = rollbackErr
		}
	}()
	if err := work(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// lifecycleCapturingSender is a platform mailer.Sender recording the messages.
type lifecycleCapturingSender struct {
	messages []mailer.Message
}

func (s *lifecycleCapturingSender) Send(_ context.Context, msg mailer.Message) error {
	s.messages = append(s.messages, msg)
	return nil
}

func (s *lifecycleCapturingSender) reset() { s.messages = nil }

// withSubject returns the recorded messages with the given subject.
func (s *lifecycleCapturingSender) withSubject(subject string) []mailer.Message {
	var out []mailer.Message
	for _, m := range s.messages {
		if m.Subject == subject {
			out = append(out, m)
		}
	}
	return out
}

// onlyWithSubject returns the single recorded message with the given subject,
// failing the test otherwise.
func (s *lifecycleCapturingSender) onlyWithSubject(t *testing.T, subject string) mailer.Message {
	t.Helper()
	msgs := s.withSubject(subject)
	if len(msgs) != 1 {
		t.Fatalf("expected exactly one email with subject %q, got %d (all: %+v)", subject, len(msgs), s.messages)
	}
	return msgs[0]
}

// lifecycleFakeLimiter is an in-memory access RecipientLimiter.
type lifecycleFakeLimiter struct {
	limits map[uuid.UUID]int
}

func (f *lifecycleFakeLimiter) set(recipientID uuid.UUID, limit int) {
	f.limits[recipientID] = limit
}

func (f *lifecycleFakeLimiter) ActivePropertyLimit(_ context.Context, recipientID uuid.UUID) (int, error) {
	return f.limits[recipientID], nil
}

func (f *lifecycleFakeLimiter) WithTx(_ transaction.Tx) (accessapp.RecipientLimiter, error) {
	return f, nil
}

// lifecycleNoOccupancy reports no open leases.
type lifecycleNoOccupancy struct{}

func (lifecycleNoOccupancy) OccupiedPropertyIDs(context.Context, uuid.UUID) (map[uuid.UUID]bool, error) {
	return map[uuid.UUID]bool{}, nil
}

// lifecycleNoOwnedProps reports no own active properties.
type lifecycleNoOwnedProps struct{}

func (lifecycleNoOwnedProps) ListActiveWithMeta(context.Context, uuid.UUID) ([]accessapp.OwnedPropertyMeta, error) {
	return nil, nil
}

func (lifecycleNoOwnedProps) WithTx(_ transaction.Tx) (accessapp.OwnedActivePropertiesPort, error) {
	return lifecycleNoOwnedProps{}, nil
}

var (
	_ accessapp.RecipientLimiter          = (*lifecycleFakeLimiter)(nil)
	_ accessapp.OccupancyPort             = lifecycleNoOccupancy{}
	_ accessapp.OwnedActivePropertiesPort = lifecycleNoOwnedProps{}
	_ mailer.Sender                       = (*lifecycleCapturingSender)(nil)
)

// Subjects asserted against the access email sender (kept in sync with
// internal/access/adapters/email/sender.go).
const (
	subjectInvite     = "Приглашение к совместному доступу в Рентли"
	subjectRevoked    = "Ваш доступ к объекту в Рентли отозван"
	subjectDeleted    = "Объект в Рентли удалён владельцем"
	subjectWaiting    = "Доступ к объекту в Рентли ждёт свободного слота"
	subjectDowngrade  = "Часть доступов в Рентли приостановлена по вашему тарифу"
	subjectRestored   = "Доступ к объекту в Рентли восстановлен"
	subjectActivated  = "Приглашение к совместному доступу в Рентли принято"
	subjectMemberLeft = "Участник вышел из объекта в Рентли"
)

// lifecycleMailFixture bundles the real access services wired to the test
// transaction with the capturing sender and the real template renderer.
type lifecycleMailFixture struct {
	tx      pgx.Tx
	q       *genpostgres.Queries
	sender  *lifecycleCapturingSender
	limiter *lifecycleFakeLimiter
	access  *accessapp.AccessService
	invites *accessapp.InvitationService
	slots   *accessapp.SlotCoordinator
	deleter *accessapp.PropertyDeleteMailer
}

// bg returns a background context (kept out of the struct per containedctx).
func (f *lifecycleMailFixture) bg() context.Context { return context.Background() }

func newLifecycleMailFixture(t *testing.T) *lifecycleMailFixture {
	t.Helper()
	pool := setupAccessDB(t)
	_, tx, cleanup := beginAccessTx(t, pool)
	t.Cleanup(cleanup)

	renderer, err := mailer.NewRenderer("../../../../templates/email")
	if err != nil {
		t.Fatalf("load email templates: %v", err)
	}
	encryptor, err := encryption.NewEncryptor("")
	if err != nil {
		t.Fatalf("noop encryptor: %v", err)
	}

	memberRepo := NewMembershipRepository(tx)
	invitationRepo := NewInvitationRepository(tx)
	ownerResolver := NewOwnerResolver(tx)
	userRepo := identitypg.NewUserRepository(tx, encryptor)
	userLookup := NewUserLookup(userRepo)
	emailResolver := NewUserEmailResolver(userRepo)
	policy := accessapp.NewMembershipPolicy(ownerResolver, memberRepo)

	sender := &lifecycleCapturingSender{}
	accessMailer := accessemail.NewSender(sender, renderer, "https://app.example")
	lifecycle := accessapp.NewLifecycleMailer(accessMailer, emailResolver, ownerResolver, nil)
	limiter := &lifecycleFakeLimiter{limits: map[uuid.UUID]int{}}
	beginner := lifecycleBeginner{tx: tx}

	slots := accessapp.NewSlotCoordinator(memberRepo, ownerResolver, limiter,
		lifecycleNoOccupancy{}, lifecycleNoOwnedProps{}, lifecycle, auditapp.Noop{}, beginner)
	factory := accessapp.NewTxStoreFactory(memberRepo, invitationRepo, auditapp.Noop{}, lifecycleUoW{beginner})
	access := accessapp.NewAccessService(memberRepo, ownerResolver, ownerResolver, userLookup, policy, slots, lifecycle, factory, nil)
	invites := accessapp.NewInvitationService(access, memberRepo, invitationRepo, ownerResolver, ownerResolver,
		userLookup, policy, slots, accessMailer, lifecycle, ownerResolver, factory, nil, nil)
	deleter := accessapp.NewPropertyDeleteMailer(memberRepo, emailResolver, accessMailer, nil)

	return &lifecycleMailFixture{
		tx: tx, q: genpostgres.New(tx),
		sender: sender, limiter: limiter,
		access: access, invites: invites, slots: slots, deleter: deleter,
	}
}

// addUserWithEmail creates a user and sets their email.
func (f *lifecycleMailFixture) addUserWithEmail(t *testing.T, email string) uuid.UUID {
	t.Helper()
	id := createAccessTestUser(t, f.bg(), f.q)
	_, err := f.q.UpdateUserEmailVerified(f.bg(), genpostgres.UpdateUserEmailVerifiedParams{
		ID:    pgUUID(id),
		Email: pgtype.Text{String: email, Valid: true},
	})
	if err != nil {
		t.Fatalf("set user email: %v", err)
	}
	return id
}

// addProperty creates an active property with the given display name.
func (f *lifecycleMailFixture) addProperty(t *testing.T, owner uuid.UUID, name string) uuid.UUID {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}
	_, err = f.q.CreateProperty(f.bg(), genpostgres.CreatePropertyParams{
		ID:          pgUUID(id),
		OwnerID:     pgUUID(owner),
		Name:        name,
		Type:        propertyTypeApartment,
		Address:     "",
		Description: pgtype.Text{},
		Attributes:  []byte("{}"),
		Status:      statusActive,
	})
	if err != nil {
		t.Fatalf("create property: %v", err)
	}
	return id
}

// assertMessage checks the recipient and that both bodies carry the fragment.
func assertMessage(t *testing.T, msg mailer.Message, to string, fragments ...string) {
	t.Helper()
	if len(msg.To) != 1 || msg.To[0] != to {
		t.Errorf("message To = %v, want [%s]", msg.To, to)
	}
	for _, fragment := range fragments {
		if !strings.Contains(msg.TextBody, fragment) {
			t.Errorf("text body missing %q:\n%s", fragment, msg.TextBody)
		}
		if !strings.Contains(msg.HTMLBody, fragment) {
			t.Errorf("html body missing %q", fragment)
		}
	}
}

// suspendedTitle returns the title of the recipient's single suspended
// membership and the title of the other property, reading the membership rows
// directly. Used where the eviction tie-break is not deterministic in the test
// transaction, so the expectation follows the actual suspended row.
func (f *lifecycleMailFixture) suspendedTitle(t *testing.T, recipient uuid.UUID, titles map[uuid.UUID]string) (suspended, other string) {
	t.Helper()
	rows, err := f.q.ListSuspendedMembersByUser(f.bg(), pgUUID(recipient))
	if err != nil {
		t.Fatalf("list suspended memberships: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected exactly one suspended membership, got %d", len(rows))
	}
	suspendedID := pgconv.UUIDFromPgtype(rows[0].PropertyID)
	for propertyID, title := range titles {
		if propertyID == suspendedID {
			suspended = title
		} else {
			other = title
		}
	}
	return suspended, other
}

// TestLifecycleMail_ActiveMembershipLifecycle covers: adding a registered
// member sends nothing (T5 behaviour preserved), revoking an active membership
// emails the former member.
func TestLifecycleMail_ActiveMembershipLifecycle(t *testing.T) {
	f := newLifecycleMailFixture(t)
	owner := f.addUserWithEmail(t, "owner@example.com")
	member := f.addUserWithEmail(t, "member@example.com")
	property := f.addProperty(t, owner, "Квартира на Невском")
	f.limiter.set(member, 10)

	m, err := f.access.AddMember(f.bg(), owner, property, member, domain.RoleViewer)
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if m.Status != domain.MemberStatusActive {
		t.Fatalf("member must be active, got %v", m.Status)
	}
	if len(f.sender.messages) != 0 {
		t.Fatalf("instant activation of a registered user must not send email, got %+v", f.sender.messages)
	}

	if err := f.access.RevokeMember(f.bg(), owner, property, m.ID); err != nil {
		t.Fatalf("RevokeMember: %v", err)
	}
	msg := f.sender.onlyWithSubject(t, subjectRevoked)
	assertMessage(t, msg, "member@example.com", "Квартира на Невском")
}

// TestLifecycleMail_SuspendedGrantAndRevoke covers: a grant created without a
// free slot emails "access waits for a free slot"; revoking a suspended
// membership is silent.
func TestLifecycleMail_SuspendedGrantAndRevoke(t *testing.T) {
	f := newLifecycleMailFixture(t)
	owner := f.addUserWithEmail(t, "owner@example.com")
	member := f.addUserWithEmail(t, "member@example.com")
	property := f.addProperty(t, owner, "Дача у моря")
	f.limiter.set(member, 0)

	m, err := f.access.AddMember(f.bg(), owner, property, member, domain.RoleViewer)
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if m.Status != domain.MemberStatusSuspended {
		t.Fatalf("member must be suspended, got %v", m.Status)
	}
	msg := f.sender.onlyWithSubject(t, subjectWaiting)
	assertMessage(t, msg, "member@example.com", "Дача у моря")

	f.sender.reset()
	if err := f.access.RevokeMember(f.bg(), owner, property, m.ID); err != nil {
		t.Fatalf("RevokeMember: %v", err)
	}
	if len(f.sender.messages) != 0 {
		t.Errorf("revoking a suspended member must not send email, got %+v", f.sender.messages)
	}
}

// TestLifecycleMail_LeaveNotifiesOwner covers: self-exit emails the owner and
// never the leaving member.
func TestLifecycleMail_LeaveNotifiesOwner(t *testing.T) {
	f := newLifecycleMailFixture(t)
	owner := f.addUserWithEmail(t, "owner@example.com")
	member := f.addUserWithEmail(t, "member@example.com")
	property := f.addProperty(t, owner, "Квартира на Невском")
	f.limiter.set(member, 10)

	if _, err := f.access.AddMember(f.bg(), owner, property, member, domain.RoleFullAccess); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	f.sender.reset()

	if err := f.access.LeaveProperty(f.bg(), member, property); err != nil {
		t.Fatalf("LeaveProperty: %v", err)
	}
	msg := f.sender.onlyWithSubject(t, subjectMemberLeft)
	assertMessage(t, msg, "owner@example.com", "Квартира на Невском")
	for _, m := range f.sender.messages {
		if slices.Contains(m.To, "member@example.com") {
			t.Errorf("the leaving member must not receive email, got %+v", m)
		}
	}
}

// TestLifecycleMail_InvitationLifecycle covers: the invite email (T5), the
// silent cancellation of a pending invitation, the owner notice on activation
// at registration, and the "waiting for a slot" email on a suspended
// activation.
func TestLifecycleMail_InvitationLifecycle(t *testing.T) {
	f := newLifecycleMailFixture(t)
	owner := f.addUserWithEmail(t, "owner@example.com")
	property := f.addProperty(t, owner, "Квартира на Невском")
	other := f.addProperty(t, owner, "Дача у моря")

	// Pending invitation: the single invite email.
	if _, err := f.invites.InviteByEmail(f.bg(), owner, property, "new@example.com", domain.RoleViewer); err != nil {
		t.Fatalf("InviteByEmail: %v", err)
	}
	msg := f.sender.onlyWithSubject(t, subjectInvite)
	assertMessage(t, msg, "new@example.com", "Квартира на Невском")

	// Cancelling a pending invitation sends nothing.
	outcome, err := f.invites.InviteByEmail(f.bg(), owner, other, "cancel@example.com", domain.RoleViewer)
	if err != nil {
		t.Fatalf("InviteByEmail (cancel): %v", err)
	}
	f.sender.reset()
	if err := f.invites.CancelInvitation(f.bg(), owner, other, outcome.Invitation.ID); err != nil {
		t.Fatalf("CancelInvitation: %v", err)
	}
	if len(f.sender.messages) != 0 {
		t.Errorf("cancelling a pending invitation must not send email, got %+v", f.sender.messages)
	}

	// Registration activates the pending invitation: the owner is notified.
	f.sender.reset()
	user := f.addUserWithEmail(t, "new@example.com")
	f.limiter.set(user, 10)
	if err := f.invites.ActivatePendingInvitations(f.bg(), user, "new@example.com"); err != nil {
		t.Fatalf("ActivatePendingInvitations: %v", err)
	}
	msg = f.sender.onlyWithSubject(t, subjectActivated)
	assertMessage(t, msg, "owner@example.com", "Квартира на Невском", "new@example.com")
	if got := len(f.sender.withSubject(subjectWaiting)); got != 0 {
		t.Errorf("activation with a free slot must not send the waiting email, got %d", got)
	}

	// A suspended activation additionally emails the new member.
	f.sender.reset()
	if _, err := f.invites.InviteByEmail(f.bg(), owner, other, "late@example.com", domain.RoleViewer); err != nil {
		t.Fatalf("InviteByEmail (late): %v", err)
	}
	late := f.addUserWithEmail(t, "late@example.com")
	f.limiter.set(late, 0)
	f.sender.reset()
	if err := f.invites.ActivatePendingInvitations(f.bg(), late, "late@example.com"); err != nil {
		t.Fatalf("ActivatePendingInvitations (late): %v", err)
	}
	msg = f.sender.onlyWithSubject(t, subjectWaiting)
	assertMessage(t, msg, "late@example.com", "Дача у моря")
	if got := len(f.sender.withSubject(subjectActivated)); got != 1 {
		t.Errorf("the owner must be notified about the suspended activation too, got %d", got)
	}
}

// TestLifecycleMail_DowngradeAndRecovery covers: a tariff downgrade sends one
// summary email with the suspended titles; a recovery sends "access restored".
func TestLifecycleMail_DowngradeAndRecovery(t *testing.T) {
	f := newLifecycleMailFixture(t)
	owner := f.addUserWithEmail(t, "owner@example.com")
	recipient := f.addUserWithEmail(t, "recipient@example.com")
	p1 := f.addProperty(t, owner, "Квартира")
	p2 := f.addProperty(t, owner, "Дача")
	f.limiter.set(recipient, 10)

	if _, err := f.access.AddMember(f.bg(), owner, p1, recipient, domain.RoleViewer); err != nil {
		t.Fatalf("AddMember p1: %v", err)
	}
	if _, err := f.access.AddMember(f.bg(), owner, p2, recipient, domain.RoleViewer); err != nil {
		t.Fatalf("AddMember p2: %v", err)
	}
	f.sender.reset()

	// Downgrade to one slot: one of the memberships is suspended and exactly
	// one summary email goes out, listing only the suspended object. The
	// eviction comparator ranks tied memberships by updated_at, which the
	// set_updated_at trigger pins to the transaction time for both rows, so
	// the test must not depend on which object is evicted — it is derived from
	// the membership rows after the enforcement.
	f.limiter.set(recipient, 1)
	if err := f.slots.EnforceRecipientLimit(f.bg(), lifecycleNoCommitTx{f.tx}, owner, "downgrade"); err != nil {
		t.Fatalf("EnforceRecipientLimit: %v", err)
	}
	suspendedTitle, otherTitle := f.suspendedTitle(t, recipient, map[uuid.UUID]string{p1: "Квартира", p2: "Дача"})
	msg := f.sender.onlyWithSubject(t, subjectDowngrade)
	assertMessage(t, msg, "recipient@example.com", suspendedTitle)
	if strings.Contains(msg.TextBody, otherTitle) {
		t.Errorf("summary must list only the suspended object, body:\n%s", msg.TextBody)
	}
	if got := len(f.sender.withSubject(subjectWaiting)); got != 0 {
		t.Errorf("the downgrade path must not send per-membership waiting emails, got %d", got)
	}

	// The tariff grows again: the suspended membership is restored FIFO and the
	// recipient is emailed.
	f.sender.reset()
	f.limiter.set(recipient, 2)
	if err := f.slots.RecoverSuspended(f.bg(), lifecycleNoCommitTx{f.tx}, recipient); err != nil {
		t.Fatalf("RecoverSuspended: %v", err)
	}
	msg = f.sender.onlyWithSubject(t, subjectRestored)
	assertMessage(t, msg, "recipient@example.com", suspendedTitle)
}

// TestLifecycleMail_DowngradeOfRecipientOnForeignObjects covers the billing
// production path: EnforceRecipientLimit is called with the downgrading user's
// OWN id (sub.UserID), and his shared memberships on other owners' objects are
// suspended with exactly one summary email to him (issue #162 AC5, issue #158).
func TestLifecycleMail_DowngradeOfRecipientOnForeignObjects(t *testing.T) {
	f := newLifecycleMailFixture(t)
	foreignOwner := f.addUserWithEmail(t, "foreign-owner@example.com")
	recipient := f.addUserWithEmail(t, "recipient@example.com")
	p1 := f.addProperty(t, foreignOwner, "Квартира")
	p2 := f.addProperty(t, foreignOwner, "Дача")
	f.limiter.set(recipient, 10)

	if _, err := f.access.AddMember(f.bg(), foreignOwner, p1, recipient, domain.RoleViewer); err != nil {
		t.Fatalf("AddMember p1: %v", err)
	}
	if _, err := f.access.AddMember(f.bg(), foreignOwner, p2, recipient, domain.RoleViewer); err != nil {
		t.Fatalf("AddMember p2: %v", err)
	}
	f.sender.reset()

	// Downgrade the recipient to one slot, passing his own id as billing does:
	// one of his memberships on the foreign owner's objects is suspended and he
	// gets exactly one summary email. The eviction tie-break is not
	// deterministic in the test transaction, so the expectation follows the
	// actual suspended row.
	f.limiter.set(recipient, 1)
	if err := f.slots.EnforceRecipientLimit(f.bg(), lifecycleNoCommitTx{f.tx}, recipient, "downgrade"); err != nil {
		t.Fatalf("EnforceRecipientLimit: %v", err)
	}
	suspendedTitle, otherTitle := f.suspendedTitle(t, recipient, map[uuid.UUID]string{p1: "Квартира", p2: "Дача"})
	msg := f.sender.onlyWithSubject(t, subjectDowngrade)
	assertMessage(t, msg, "recipient@example.com", suspendedTitle)
	if strings.Contains(msg.TextBody, otherTitle) {
		t.Errorf("summary must list only the suspended object, body:\n%s", msg.TextBody)
	}
}

// TestLifecycleMail_PropertyDelete covers: the former members (active and
// suspended) are collected before the memberships are dropped and emailed
// after the delete; a pending invitation receives nothing.
func TestLifecycleMail_PropertyDelete(t *testing.T) {
	f := newLifecycleMailFixture(t)
	owner := f.addUserWithEmail(t, "owner@example.com")
	active := f.addUserWithEmail(t, "active@example.com")
	suspended := f.addUserWithEmail(t, "suspended@example.com")
	property := f.addProperty(t, owner, "Квартира на Невском")
	f.limiter.set(active, 10)
	f.limiter.set(suspended, 0)

	if _, err := f.access.AddMember(f.bg(), owner, property, active, domain.RoleViewer); err != nil {
		t.Fatalf("AddMember active: %v", err)
	}
	m, err := f.access.AddMember(f.bg(), owner, property, suspended, domain.RoleViewer)
	if err != nil {
		t.Fatalf("AddMember suspended: %v", err)
	}
	if m.Status != domain.MemberStatusSuspended {
		t.Fatalf("second member must be suspended, got %v", m.Status)
	}
	if _, err := f.invites.InviteByEmail(f.bg(), owner, property, "pending@example.com", domain.RoleViewer); err != nil {
		t.Fatalf("InviteByEmail: %v", err)
	}
	f.sender.reset()

	emails, err := f.deleter.CollectFormerMemberEmails(f.bg(), lifecycleNoCommitTx{f.tx}, property)
	if err != nil {
		t.Fatalf("CollectFormerMemberEmails: %v", err)
	}
	slices.Sort(emails)
	if !slices.Equal(emails, []string{"active@example.com", "suspended@example.com"}) {
		t.Fatalf("collected emails = %v, want active+suspended only", emails)
	}

	title, err := NewOwnerResolver(f.tx).GetTitle(f.bg(), property)
	if err != nil {
		t.Fatalf("GetTitle: %v", err)
	}
	for _, to := range emails {
		if err := f.deleter.SendPropertyDeleted(f.bg(), to, title); err != nil {
			t.Fatalf("SendPropertyDeleted: %v", err)
		}
	}
	msgs := f.sender.withSubject(subjectDeleted)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 property deleted emails, got %+v", f.sender.messages)
	}
	for _, msg := range msgs {
		if msg.To[0] == "pending@example.com" {
			t.Error("a pending (unregistered) invitee must not receive the deletion email")
		}
		assertMessage(t, msg, msg.To[0], "Квартира на Невском")
	}
}
