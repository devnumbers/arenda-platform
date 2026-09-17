package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// seedOwnerEmail is the current address of the seeded user, reused across the
// email-change service tests.
const seedOwnerEmail = "owner@example.com"

// mutableStepClock is a unit-test clock whose Now can be advanced mid-test, so
// the email-change flow can exercise the 1-minute send throttle and the
// 10-minute grant TTL deterministically. Unlike the integration harness's
// mutableClock it lives in the internal test package.
type mutableStepClock struct{ now time.Time }

func (c *mutableStepClock) Now() time.Time          { return c.now }
func (c *mutableStepClock) advance(d time.Duration) { c.now = c.now.Add(d) }
func newMutableStepClock() *mutableStepClock        { return &mutableStepClock{now: testNow} }

func (c *mutableStepClock) verifiedEqualsNow(t *testing.T, at *time.Time) {
	t.Helper()
	if at == nil {
		t.Fatal("verified-at stamp is nil, want the clock instant")
	}
	if !at.Equal(c.now) {
		t.Fatalf("verified-at = %s, want the current clock instant %s", at, c.now)
	}
}

// emailChangeHarness wires an EmailChangeService (with its own LoginCodeService)
// to the shared fakes plus a recordingRecorder, so the three-step flow can be
// exercised end-to-end through fake repositories (issue #721).
type emailChangeHarness struct {
	*fakeStores
	svc    *EmailChangeService
	sender *fakeCodeSender
	audit  *recordingRecorder
	clock  *mutableStepClock
	hasher fakeHasher
	// BudgetLeft caps how many new-address sends the service may make; nil
	// means unlimited. Decremented on every allowed send.
	budgetLeft *int
}

func newEmailChangeHarness() *emailChangeHarness {
	stores := newFakeStores()
	audit := &recordingRecorder{}
	sender := &fakeCodeSender{}
	clock := newMutableStepClock()
	factory := stores.factory(audit)
	loginCodes := NewLoginCodeService(factory, LoginCodeServiceConfig{
		CodeSender: sender,
		Clock:      clock,
		Hasher:     fakeHasher{},
		Logger:     discardLogger(),
	})
	h := &emailChangeHarness{
		fakeStores: stores,
		sender:     sender,
		audit:      audit,
		clock:      clock,
		hasher:     fakeHasher{},
	}
	h.svc = NewEmailChangeService(factory, EmailChangeServiceConfig{
		LoginCodes: loginCodes,
		Clock:      clock,
		Hasher:     fakeHasher{},
		Logger:     discardLogger(),
		AllowNewAddressSend: func(uuid.UUID) bool {
			if h.budgetLeft == nil {
				return true
			}
			if *h.budgetLeft <= 0 {
				return false
			}
			*h.budgetLeft--
			return true
		},
	})
	return h
}

// seedEmailChangeUser creates an owner with an email in the fake repos so the
// flow has a user to act on. VerifiedAt may be nil to model an unverified
// current address (the flow must still accept it, grilling decision #720-6).
func (h *emailChangeHarness) seedEmailChangeUser(
	t *testing.T,
	phone domain.Phone,
	email domain.Email,
	verifiedAt *time.Time,
) domain.User {
	t.Helper()
	user, err := domain.NewOwner(phone)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	user.Email = &email
	user.EmailVerifiedAt = verifiedAt
	h.users.byPhone[phone.String()] = user
	return user
}

// seedOtherEmailOwner creates a second user owning the given email, so
// taken-email guards can fire.
func (h *emailChangeHarness) seedOtherEmailOwner(t *testing.T, email domain.Email) {
	t.Helper()
	other, err := domain.NewOwner(mustPhone(t, "+79160000900"))
	if err != nil {
		t.Fatalf("create other user: %v", err)
	}
	other.Email = &email
	other.EmailVerifiedAt = &testNow
	h.users.byPhone[other.Phone.String()] = other
}

// driveStep runs steps 1 and 2 and returns the grant token with the code sent
// to the new address, propagating errors so tests can drive partial flows.
func (h *emailChangeHarness) driveStep(
	ctx context.Context,
	t *testing.T,
	userID uuid.UUID,
	newEmail domain.Email,
) (grantToken, newCode string, err error) {
	t.Helper()
	if err := h.svc.SendCurrentEmailCode(ctx, userID); err != nil {
		return "", "", err
	}
	currentCode := h.sender.sent[len(h.sender.sent)-1].code
	grantToken, err = h.svc.ConfirmCurrentEmail(ctx, userID, currentCode, newEmail)
	if err != nil {
		return "", "", err
	}
	return grantToken, h.sender.sent[len(h.sender.sent)-1].code, nil
}

// runToStepThree drives the flow through steps 1 and 2 and returns the grant
// token with the plaintext code delivered to the new address — the two secrets
// the final confirmation consumes. It fails the test on any step error.
func (h *emailChangeHarness) runToStepThree(
	t *testing.T,
	userID uuid.UUID,
	newEmail domain.Email,
) (grantToken, newCode string) {
	t.Helper()
	grantToken, newCode, err := h.driveStep(t.Context(), t, userID, newEmail)
	if err != nil {
		t.Fatalf("steps 1-2: %v", err)
	}
	if grantToken == "" {
		t.Fatal("step 2 returned an empty grant token")
	}
	return grantToken, newCode
}

// Step 1: SendCurrentEmailCode.

// stepOneHappyPath covers the SendCurrentEmailCode success path: the code goes
// to the user's current email over their phone, is persisted with purpose
// email_change, and the delivery itself marks the current address verified.
func stepOneHappyPath(t *testing.T) {
	t.Helper()
	h := newEmailChangeHarness()
	phone := mustPhone(t, "+79160000100")
	email := mustEmail(t, seedOwnerEmail)
	user := h.seedEmailChangeUser(t, phone, email, nil) // Deliberately unverified: the address starts unconfirmed.

	err := h.svc.SendCurrentEmailCode(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("SendCurrentEmailCode error = %v", err)
	}
	if len(h.sender.sent) != 1 {
		t.Fatalf("sender calls = %d, want 1", len(h.sender.sent))
	}
	sent := h.sender.sent[0]
	if sent.email != email {
		t.Fatalf("sent email = %s, want %s (current email)", sent.email, email)
	}
	if sent.phone != phone {
		t.Fatalf("sent phone = %s, want %s", sent.phone, phone)
	}
	if len(h.codes.codes) != 1 {
		t.Fatalf("codes persisted = %d, want 1", len(h.codes.codes))
	}
	for _, c := range h.codes.codes {
		if c.Purpose != domain.LoginCodePurposeEmailChange {
			t.Fatalf("code purpose = %s, want email_change", c.Purpose)
		}
	}
	// Verified semantics (#720-6): a delivered step-1 code confirms the current
	// address even though the flow is abandoned right here.
	stored, err := h.users.GetByID(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	h.clock.verifiedEqualsNow(t, stored.EmailVerifiedAt)
}

func TestEmailChangeService_SendCurrentEmailCode(t *testing.T) {
	t.Parallel()

	t.Run("happy path sends to the current email and marks it verified", func(t *testing.T) {
		t.Parallel()
		stepOneHappyPath(t)
	})

	t.Run("user without email returns ErrEmailDoesNotMatch and sends nothing", func(t *testing.T) {
		t.Parallel()
		h := newEmailChangeHarness()
		phone := mustPhone(t, "+79160000101")
		user, err := domain.NewOwner(phone)
		if err != nil {
			t.Fatalf("create user: %v", err)
		}
		h.users.byPhone[phone.String()] = user

		err = h.svc.SendCurrentEmailCode(t.Context(), user.ID)
		if !errors.Is(err, ErrEmailDoesNotMatch) {
			t.Fatalf("error = %v, want ErrEmailDoesNotMatch", err)
		}
		if len(h.sender.sent) != 0 {
			t.Fatalf("sender calls = %d, want 0", len(h.sender.sent))
		}
	})

	t.Run("unknown user returns ErrNotFound", func(t *testing.T) {
		t.Parallel()
		h := newEmailChangeHarness()
		err := h.svc.SendCurrentEmailCode(t.Context(), uuid.Must(uuid.NewV7()))
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("error = %v, want ErrNotFound", err)
		}
	})

	t.Run("re-issue within a minute is throttled without invalidating the live code", func(t *testing.T) {
		t.Parallel()
		h := newEmailChangeHarness()
		phone := mustPhone(t, "+79160000102")
		email := mustEmail(t, seedOwnerEmail)
		user := h.seedEmailChangeUser(t, phone, email, &testNow)

		if err := h.svc.SendCurrentEmailCode(t.Context(), user.ID); err != nil {
			t.Fatalf("first send: %v", err)
		}
		h.clock.advance(30 * time.Second)
		err := h.svc.SendCurrentEmailCode(t.Context(), user.ID)
		if !errors.Is(err, ErrCodeSentTooRecently) {
			t.Fatalf("error = %v, want ErrCodeSentTooRecently", err)
		}
		// The live code survives the throttled re-issue.
		if len(h.codes.codes) != 1 {
			t.Fatalf("codes persisted = %d, want 1 (live code intact)", len(h.codes.codes))
		}
		h.clock.advance(31 * time.Second)
		if err := h.svc.SendCurrentEmailCode(t.Context(), user.ID); err != nil {
			t.Fatalf("send after the throttle window: %v", err)
		}
	})
}

// assertStoredGrant checks exactly one grant lives for the user, bound to the
// new address, with a hash matching the presented plaintext token.
func assertStoredGrant(t *testing.T, h *emailChangeHarness, userID uuid.UUID, newEmail domain.Email, grantToken string) {
	t.Helper()
	grant, ok := h.grants.grants[userID]
	if !ok {
		t.Fatal("no grant stored for the user")
	}
	if grant.Email != newEmail {
		t.Fatalf("grant email = %s, want %s", grant.Email, newEmail)
	}
	if want := h.hasher.HashToken("email_change_grant:" + grantToken); grant.TokenHash != want {
		t.Fatalf("grant token hash mismatch: got %q, want %q", grant.TokenHash, want)
	}
}

// assertNewAddressCode checks a live email_change code was persisted for the
// new address.
func assertNewAddressCode(t *testing.T, h *emailChangeHarness, newEmail domain.Email) {
	t.Helper()
	found := false
	for _, c := range h.codes.codes {
		if c.Email == newEmail {
			found = true
			if c.Purpose != domain.LoginCodePurposeEmailChange {
				t.Fatalf("new code purpose = %s, want email_change", c.Purpose)
			}
			if c.Used {
				t.Fatal("new code already used")
			}
		}
	}
	if !found {
		t.Fatal("no code persisted for the new address")
	}
}

// Step 2: ConfirmCurrentEmail.

// confirmHappyPath covers the step-2 success path: the current code is burned,
// a grant bound to the new address is stored, and the code for the new address
// is issued with the email_change purpose.
func confirmHappyPath(t *testing.T) {
	t.Helper()
	h := newEmailChangeHarness()
	phone := mustPhone(t, "+79160000200")
	currentEmail := mustEmail(t, seedOwnerEmail)
	newEmail := mustEmail(t, "new@example.com")
	user := h.seedEmailChangeUser(t, phone, currentEmail, &testNow)

	if err := h.svc.SendCurrentEmailCode(t.Context(), user.ID); err != nil {
		t.Fatalf("step 1: %v", err)
	}
	grantToken, err := h.svc.ConfirmCurrentEmail(t.Context(), user.ID, h.sender.sent[0].code, newEmail)
	if err != nil {
		t.Fatalf("step 2: %v", err)
	}
	if grantToken == "" {
		t.Fatal("grant token is empty")
	}

	// Both deliveries happened: current email first, then the new one.
	if len(h.sender.sent) != 2 {
		t.Fatalf("sender calls = %d, want 2", len(h.sender.sent))
	}
	if h.sender.sent[0].email != currentEmail {
		t.Fatalf("first delivery email = %s, want %s", h.sender.sent[0].email, currentEmail)
	}
	if h.sender.sent[1].email != newEmail {
		t.Fatalf("second delivery email = %s, want %s (new address)", h.sender.sent[1].email, newEmail)
	}

	// The step-1 code is burned.
	for _, c := range h.codes.codes {
		if c.Email == currentEmail && !c.Used {
			t.Fatal("current-email code still unused, want burned")
		}
	}

	assertStoredGrant(t, h, user.ID, newEmail, grantToken)
	assertNewAddressCode(t, h, newEmail)

	// The email itself is untouched until step 3.
	stored, err := h.users.GetByID(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.Email == nil || stored.Email.String() != currentEmail.String() {
		t.Fatalf("email = %v, want unchanged %s", stored.Email, currentEmail)
	}
}

// confirmWrongCodeRecordsWindow proves a wrong step-1-code entry lands in the
// phone's attempt window, delivers nothing, stores no grant, and is audited.
func confirmWrongCodeRecordsWindow(t *testing.T) {
	t.Helper()
	h := newEmailChangeHarness()
	phone := mustPhone(t, "+79160000201")
	user := h.seedEmailChangeUser(t, phone, mustEmail(t, seedOwnerEmail), &testNow)
	newEmail := mustEmail(t, "new@example.com")

	if err := h.svc.SendCurrentEmailCode(t.Context(), user.ID); err != nil {
		t.Fatalf("step 1: %v", err)
	}
	_, err := h.svc.ConfirmCurrentEmail(t.Context(), user.ID, "000000", newEmail)
	if !errors.Is(err, domain.ErrLoginCodeInvalid) {
		t.Fatalf("error = %v, want ErrLoginCodeInvalid", err)
	}
	window, ok := h.attempts.windows[phone.String()]
	if !ok {
		t.Fatal("no attempt window recorded for the phone")
	}
	if window.Failures != 1 {
		t.Fatalf("window failures = %d, want 1", window.Failures)
	}
	if len(h.sender.sent) != 1 {
		t.Fatalf("sender calls = %d, want 1 (no code for the new address)", len(h.sender.sent))
	}
	if len(h.grants.grants) != 0 {
		t.Fatalf("grants = %d, want 0", len(h.grants.grants))
	}
	// The failed attempt is audited (parity with the phone change).
	if len(h.audit.entries) != 1 || h.audit.entries[0].Action != auditdomain.ActionAuthEmailChangeFailed {
		t.Fatalf("audit entries = %+v, want one email_change_failed", h.audit.entries)
	}
}

// confirmSameAsCurrentRejects proves same-as-current is refused before any
// code is issued for the "new" address.
func confirmSameAsCurrentRejects(t *testing.T) {
	t.Helper()
	h := newEmailChangeHarness()
	user := h.seedEmailChangeUser(t, mustPhone(t, "+79160000202"), mustEmail(t, seedOwnerEmail), &testNow)

	if err := h.svc.SendCurrentEmailCode(t.Context(), user.ID); err != nil {
		t.Fatalf("step 1: %v", err)
	}
	_, err := h.svc.ConfirmCurrentEmail(t.Context(), user.ID, h.sender.sent[0].code, mustEmail(t, seedOwnerEmail))
	if !errors.Is(err, ErrEmailUnchanged) {
		t.Fatalf("error = %v, want ErrEmailUnchanged", err)
	}
	if len(h.sender.sent) != 1 {
		t.Fatalf("sender calls = %d, want 1 (no code for the new address)", len(h.sender.sent))
	}
}

// confirmTakenRejects proves an address owned by another user is refused at
// step 2 without a delivery.
func confirmTakenRejects(t *testing.T) {
	t.Helper()
	h := newEmailChangeHarness()
	user := h.seedEmailChangeUser(t, mustPhone(t, "+79160000203"), mustEmail(t, seedOwnerEmail), &testNow)
	taken := mustEmail(t, "taken@example.com")
	h.seedOtherEmailOwner(t, taken)

	if err := h.svc.SendCurrentEmailCode(t.Context(), user.ID); err != nil {
		t.Fatalf("step 1: %v", err)
	}
	_, err := h.svc.ConfirmCurrentEmail(t.Context(), user.ID, h.sender.sent[0].code, taken)
	if !errors.Is(err, ErrEmailAlreadyTaken) {
		t.Fatalf("error = %v, want ErrEmailAlreadyTaken", err)
	}
	if len(h.sender.sent) != 1 {
		t.Fatalf("sender calls = %d, want 1 (no code to the taken address)", len(h.sender.sent))
	}
}

// confirmGrantReplaced proves a second step 2 replaces the previous grant.
func confirmGrantReplaced(t *testing.T) {
	t.Helper()
	h := newEmailChangeHarness()
	user := h.seedEmailChangeUser(t, mustPhone(t, "+79160000204"), mustEmail(t, seedOwnerEmail), &testNow)
	first := mustEmail(t, "first@example.com")
	second := mustEmail(t, "second@example.com")
	ctx := t.Context()

	if _, _, err := h.driveStep(ctx, t, user.ID, first); err != nil {
		t.Fatalf("first flow: %v", err)
	}
	firstGrant := h.grants.grants[user.ID]

	// Past the 1-minute throttle: different triples never collide, but the
	// step-1 re-send shares the current triple, so advance past it.
	h.clock.advance(time.Minute)
	if _, _, err := h.driveStep(ctx, t, user.ID, second); err != nil {
		t.Fatalf("second flow: %v", err)
	}
	secondGrant := h.grants.grants[user.ID]

	if firstGrant.ID == secondGrant.ID {
		t.Fatal("the first grant survived, want replaced")
	}
	if secondGrant.Email != second {
		t.Fatalf("live grant email = %s, want %s", secondGrant.Email, second)
	}
}

func TestEmailChangeService_ConfirmCurrentEmail(t *testing.T) {
	t.Parallel()

	t.Run("happy path burns the current code, stores a grant, and sends the new code", func(t *testing.T) {
		t.Parallel()
		confirmHappyPath(t)
	})

	t.Run("wrong current code records the failure on the phone window", func(t *testing.T) {
		t.Parallel()
		confirmWrongCodeRecordsWindow(t)
	})

	t.Run("same-as-current email is rejected before any code is issued", func(t *testing.T) {
		t.Parallel()
		confirmSameAsCurrentRejects(t)
	})

	t.Run("email taken by another user returns ErrEmailAlreadyTaken", func(t *testing.T) {
		t.Parallel()
		confirmTakenRejects(t)
	})

	t.Run("a new grant replaces the previous one", func(t *testing.T) {
		t.Parallel()
		confirmGrantReplaced(t)
	})

	t.Run("exhausted budget denies the send without burning the code", func(t *testing.T) {
		t.Parallel()
		confirmBudgetExhaustedKeepsCode(t)
	})

	t.Run("wrong code hides a taken address from probing", func(t *testing.T) {
		t.Parallel()
		confirmWrongCodeHidesTakenAddress(t)
	})
}

// Step 3: ChangeEmail.

// changeEmailHappyPath drives the full three-step flow and returns the
// handles the success-path assertions need.
func changeEmailHappyPath(t *testing.T) (h *emailChangeHarness, user domain.User, newEmail domain.Email, currentHash, otherHash string) {
	t.Helper()
	h = newEmailChangeHarness()
	phone := mustPhone(t, "+79160000300")
	currentEmail := mustEmail(t, "change@example.com")
	newEmail = mustEmail(t, "new-change@example.com")
	user = h.seedEmailChangeUser(t, phone, currentEmail, nil) // Unverified current address: yet to be confirmed.

	// Two sessions: a successful email change must leave both intact —
	// sessions are not part of this flow (decision #720-5).
	currentHash = h.hasher.HashToken("current-session")
	h.sessions.sessions[currentHash] = domain.Session{UserID: user.ID, TokenHash: currentHash}
	otherHash = h.hasher.HashToken("other-session")
	h.sessions.sessions[otherHash] = domain.Session{UserID: user.ID, TokenHash: otherHash}

	grantToken, newCode := h.runToStepThree(t, user.ID, newEmail)

	updated, err := h.svc.ChangeEmail(t.Context(), user.ID, newCode, grantToken)
	if err != nil {
		t.Fatalf("step 3 ChangeEmail: %v", err)
	}
	if updated.Email == nil || updated.Email.String() != newEmail.String() {
		t.Fatalf("updated email = %v, want %s", updated.Email, newEmail)
	}
	return h, user, newEmail, currentHash, otherHash
}

// changeEmailAppliesAndConsumes asserts the success-path side effects: the new
// address stored verified, the grant consumed, sessions intact, the code
// burned, and the success audit recorded.
func changeEmailAppliesAndConsumes(t *testing.T) {
	t.Helper()
	h, user, newEmail, currentHash, otherHash := changeEmailHappyPath(t)
	ctx := t.Context()

	// The change is visible through the repository.
	stored, err := h.users.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.Email == nil || stored.Email.String() != newEmail.String() {
		t.Fatalf("stored email = %v, want %s", stored.Email, newEmail)
	}
	// Verified semantics (#720-6): the new address is confirmed by the
	// code the user just entered.
	h.clock.verifiedEqualsNow(t, stored.EmailVerifiedAt)

	// The grant is consumed — the change is one-time.
	if len(h.grants.grants) != 0 {
		t.Fatalf("grants remaining = %d, want 0", len(h.grants.grants))
	}

	// Sessions untouched.
	if _, ok := h.sessions.sessions[currentHash]; !ok {
		t.Fatal("current session deleted, want retained")
	}
	if _, ok := h.sessions.sessions[otherHash]; !ok {
		t.Fatal("other session deleted, want retained")
	}

	// The step-2 code is burned.
	for _, c := range h.codes.codes {
		if c.Email == newEmail && !c.Used {
			t.Fatal("new-email code still unused, want burned")
		}
	}

	// Audit: exactly one success entry.
	if len(h.audit.entries) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(h.audit.entries))
	}
	entry := h.audit.entries[0]
	if entry.Action != auditdomain.ActionAuthEmailChanged {
		t.Fatalf("audit action = %s, want %s", entry.Action, auditdomain.ActionAuthEmailChanged)
	}
	if entry.EntityID == nil || *entry.EntityID != user.ID {
		t.Fatalf("audit entity = %v, want the user", entry.EntityID)
	}
}

// changeEmailWrongCodeKeepsGrant proves a wrong step-3 code records the window
// failure, keeps the grant for a retry, and audits the failure.
func changeEmailWrongCodeKeepsGrant(t *testing.T) {
	t.Helper()
	h := newEmailChangeHarness()
	phone := mustPhone(t, "+79160000301")
	user := h.seedEmailChangeUser(t, phone, mustEmail(t, seedOwnerEmail), &testNow)
	newEmail := mustEmail(t, "new@example.com")
	grantToken, _ := h.runToStepThree(t, user.ID, newEmail)

	_, err := h.svc.ChangeEmail(t.Context(), user.ID, "000000", grantToken)
	if !errors.Is(err, domain.ErrLoginCodeInvalid) {
		t.Fatalf("error = %v, want ErrLoginCodeInvalid", err)
	}

	window, ok := h.attempts.windows[phone.String()]
	if !ok || window.Failures != 1 {
		t.Fatalf("window = %+v, want 1 failure on the phone", window)
	}
	// The grant survives so the user can retry the code.
	if len(h.grants.grants) != 1 {
		t.Fatalf("grants = %d, want 1 (grant kept for retry)", len(h.grants.grants))
	}
	// The email is unchanged; the code is not burned by the failure.
	stored, err := h.users.GetByID(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.Email.String() != seedOwnerEmail {
		t.Fatalf("email = %s, want unchanged", stored.Email)
	}
	// The failed attempt is audited.
	if len(h.audit.entries) != 1 || h.audit.entries[0].Action != auditdomain.ActionAuthEmailChangeFailed {
		t.Fatalf("audit entries = %+v, want one email_change_failed", h.audit.entries)
	}
}

// changeEmailExpiredGrant proves a grant past its TTL refuses the change.
func changeEmailExpiredGrant(t *testing.T) {
	t.Helper()
	h := newEmailChangeHarness()
	user := h.seedEmailChangeUser(t, mustPhone(t, "+79160000302"), mustEmail(t, seedOwnerEmail), &testNow)
	newEmail := mustEmail(t, "new@example.com")
	grantToken, newCode := h.runToStepThree(t, user.ID, newEmail)

	h.clock.advance(domain.EmailChangeGrantTTL + time.Second)
	_, err := h.svc.ChangeEmail(t.Context(), user.ID, newCode, grantToken)
	if !errors.Is(err, ErrEmailChangeGrantInvalid) {
		t.Fatalf("error = %v, want ErrEmailChangeGrantInvalid", err)
	}
	stored, err := h.users.GetByID(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.Email.String() != seedOwnerEmail {
		t.Fatalf("email = %s, want unchanged", stored.Email)
	}
}

// changeEmailReusedGrant proves a consumed grant cannot be replayed, while a
// fresh flow with a fresh grant still completes.
func changeEmailReusedGrant(t *testing.T) {
	t.Helper()
	h := newEmailChangeHarness()
	user := h.seedEmailChangeUser(t, mustPhone(t, "+79160000303"), mustEmail(t, seedOwnerEmail), &testNow)
	firstEmail := mustEmail(t, "first@example.com")

	// First change consumes the first grant.
	grantToken, newCode := h.runToStepThree(t, user.ID, firstEmail)
	if _, err := h.svc.ChangeEmail(t.Context(), user.ID, newCode, grantToken); err != nil {
		t.Fatalf("first change: %v", err)
	}

	// A second flow issues a new grant; presenting the OLD token must fail.
	secondEmail := mustEmail(t, "second@example.com")
	h.clock.advance(time.Minute)
	freshToken, freshCode := h.runToStepThree(t, user.ID, secondEmail)

	_, err := h.svc.ChangeEmail(t.Context(), user.ID, freshCode, grantToken)
	if !errors.Is(err, ErrEmailChangeGrantInvalid) {
		t.Fatalf("error = %v, want ErrEmailChangeGrantInvalid", err)
	}
	// And the fresh flow still completes.
	if _, err := h.svc.ChangeEmail(t.Context(), user.ID, freshCode, freshToken); err != nil {
		t.Fatalf("fresh change: %v", err)
	}
	stored, err := h.users.GetByID(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.Email.String() != secondEmail.String() {
		t.Fatalf("email = %s, want %s", stored.Email, secondEmail)
	}
}

// changeEmailTakenBetweenSteps proves the taken-guard re-runs at step 3.
func changeEmailTakenBetweenSteps(t *testing.T) {
	t.Helper()
	h := newEmailChangeHarness()
	user := h.seedEmailChangeUser(t, mustPhone(t, "+79160000304"), mustEmail(t, seedOwnerEmail), &testNow)
	newEmail := mustEmail(t, "snatched@example.com")
	grantToken, newCode := h.runToStepThree(t, user.ID, newEmail)

	h.seedOtherEmailOwner(t, newEmail)

	_, err := h.svc.ChangeEmail(t.Context(), user.ID, newCode, grantToken)
	if !errors.Is(err, ErrEmailAlreadyTaken) {
		t.Fatalf("error = %v, want ErrEmailAlreadyTaken", err)
	}
	stored, err := h.users.GetByID(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.Email.String() != seedOwnerEmail {
		t.Fatalf("email = %s, want unchanged", stored.Email)
	}
}

// confirmBudgetExhaustedKeepsCode proves the 5/hour budget on new-address
// sends is spent on the send itself: a denial rolls the transaction back, so
// the verified current-address code survives and a later send still works
// (decision #720-3).
func confirmBudgetExhaustedKeepsCode(t *testing.T) {
	t.Helper()
	h := newEmailChangeHarness()
	zero := 0
	h.budgetLeft = &zero
	phone := mustPhone(t, "+79160000205")
	user := h.seedEmailChangeUser(t, phone, mustEmail(t, seedOwnerEmail), &testNow)
	newEmail := mustEmail(t, "new@example.com")

	if err := h.svc.SendCurrentEmailCode(t.Context(), user.ID); err != nil {
		t.Fatalf("step 1: %v", err)
	}
	code := h.sender.sent[0].code

	_, err := h.svc.ConfirmCurrentEmail(t.Context(), user.ID, code, newEmail)
	if !errors.Is(err, ErrEmailChangeBudgetExhausted) {
		t.Fatalf("error = %v, want ErrEmailChangeBudgetExhausted", err)
	}
	// Nothing was sent, stored, or burned.
	if len(h.sender.sent) != 1 {
		t.Fatalf("sender calls = %d, want 1 (no delivery to the new address)", len(h.sender.sent))
	}
	if len(h.grants.grants) != 0 {
		t.Fatalf("grants = %d, want 0", len(h.grants.grants))
	}

	// The budget clears: the SAME code still verifies and completes step 2.
	one := 1
	h.budgetLeft = &one
	grantToken, err := h.svc.ConfirmCurrentEmail(t.Context(), user.ID, code, newEmail)
	if err != nil {
		t.Fatalf("retry after budget cleared: %v", err)
	}
	if grantToken == "" {
		t.Fatal("grant token is empty on the retry")
	}
}

// confirmWrongCodeHidesTakenAddress proves the enumeration guard: a wrong
// current-address code yields the plain 401 even when the requested address is
// taken — the prechecks run only after the code verified.
func confirmWrongCodeHidesTakenAddress(t *testing.T) {
	t.Helper()
	h := newEmailChangeHarness()
	user := h.seedEmailChangeUser(t, mustPhone(t, "+79160000206"), mustEmail(t, seedOwnerEmail), &testNow)
	taken := mustEmail(t, "taken@example.com")
	h.seedOtherEmailOwner(t, taken)

	if err := h.svc.SendCurrentEmailCode(t.Context(), user.ID); err != nil {
		t.Fatalf("step 1: %v", err)
	}
	_, err := h.svc.ConfirmCurrentEmail(t.Context(), user.ID, "000000", taken)
	if !errors.Is(err, domain.ErrLoginCodeInvalid) {
		t.Fatalf("error = %v, want ErrLoginCodeInvalid (no taken-address leak)", err)
	}
	if len(h.sender.sent) != 1 {
		t.Fatalf("sender calls = %d, want 1", len(h.sender.sent))
	}
}

// Resend: ResendNewEmailCode.

// resendHappyPath proves a resend re-arms step 3: a fresh code is delivered to
// the grant's address, the grant is kept, no audit is written (no state
// change), and the new code — not the stale one — completes the change.
func resendHappyPath(t *testing.T) {
	t.Helper()
	h := newEmailChangeHarness()
	user := h.seedEmailChangeUser(t, mustPhone(t, "+79160000400"), mustEmail(t, seedOwnerEmail), &testNow)
	newEmail := mustEmail(t, "resend@example.com")
	grantToken, oldCode := h.runToStepThree(t, user.ID, newEmail)

	// Past the 1-minute send throttle the step-2 send left on the triple.
	h.clock.advance(time.Minute)
	if err := h.svc.ResendNewEmailCode(t.Context(), user.ID, grantToken); err != nil {
		t.Fatalf("ResendNewEmailCode: %v", err)
	}

	if len(h.sender.sent) != 3 {
		t.Fatalf("sender calls = %d, want 3 (step 1, step 2, resend)", len(h.sender.sent))
	}
	if h.sender.sent[2].email != newEmail {
		t.Fatalf("resend delivery email = %s, want %s", h.sender.sent[2].email, newEmail)
	}
	assertStoredGrant(t, h, user.ID, newEmail, grantToken)
	if len(h.audit.entries) != 0 {
		t.Fatalf("audit entries = %d, want 0 (resend changes no state)", len(h.audit.entries))
	}

	// The stale step-2 code no longer verifies; the fresh one does.
	if _, err := h.svc.ChangeEmail(t.Context(), user.ID, oldCode, grantToken); !errors.Is(err, domain.ErrLoginCodeInvalid) {
		t.Fatalf("stale code error = %v, want ErrLoginCodeInvalid", err)
	}
	updated, err := h.svc.ChangeEmail(t.Context(), user.ID, h.sender.sent[2].code, grantToken)
	if err != nil {
		t.Fatalf("change with resent code: %v", err)
	}
	if updated.Email == nil || updated.Email.String() != newEmail.String() {
		t.Fatalf("updated email = %v, want %s", updated.Email, newEmail)
	}
}

// resendThrottledWithinMinute proves the send throttle covers resend: a second
// delivery to the new address within a minute is refused, nothing is delivered,
// and the original step-2 code stays live.
func resendThrottledWithinMinute(t *testing.T) {
	t.Helper()
	h := newEmailChangeHarness()
	user := h.seedEmailChangeUser(t, mustPhone(t, "+79160000401"), mustEmail(t, seedOwnerEmail), &testNow)
	newEmail := mustEmail(t, "throttle@example.com")
	grantToken, liveCode := h.runToStepThree(t, user.ID, newEmail)

	err := h.svc.ResendNewEmailCode(t.Context(), user.ID, grantToken)
	if !errors.Is(err, ErrCodeSentTooRecently) {
		t.Fatalf("error = %v, want ErrCodeSentTooRecently", err)
	}
	if len(h.sender.sent) != 2 {
		t.Fatalf("sender calls = %d, want 2 (no resend delivery)", len(h.sender.sent))
	}
	// The original code still completes step 3.
	if _, err := h.svc.ChangeEmail(t.Context(), user.ID, liveCode, grantToken); err != nil {
		t.Fatalf("original code after throttled resend: %v", err)
	}
}

// resendExpiredGrant proves a grant past its TTL refuses the resend — the flow
// restarts from step 1 (decision #720-2).
func resendExpiredGrant(t *testing.T) {
	t.Helper()
	h := newEmailChangeHarness()
	user := h.seedEmailChangeUser(t, mustPhone(t, "+79160000402"), mustEmail(t, seedOwnerEmail), &testNow)
	newEmail := mustEmail(t, "expired@example.com")
	grantToken, _ := h.runToStepThree(t, user.ID, newEmail)

	h.clock.advance(domain.EmailChangeGrantTTL + time.Second)
	err := h.svc.ResendNewEmailCode(t.Context(), user.ID, grantToken)
	if !errors.Is(err, ErrEmailChangeGrantInvalid) {
		t.Fatalf("error = %v, want ErrEmailChangeGrantInvalid", err)
	}
	if len(h.sender.sent) != 2 {
		t.Fatalf("sender calls = %d, want 2 (no resend delivery)", len(h.sender.sent))
	}
}

// resendWrongGrantToken proves an unknown token is refused without a delivery.
func resendWrongGrantToken(t *testing.T) {
	t.Helper()
	h := newEmailChangeHarness()
	user := h.seedEmailChangeUser(t, mustPhone(t, "+79160000403"), mustEmail(t, seedOwnerEmail), &testNow)
	newEmail := mustEmail(t, "wrongtoken@example.com")
	h.runToStepThree(t, user.ID, newEmail)

	h.clock.advance(time.Minute)
	err := h.svc.ResendNewEmailCode(t.Context(), user.ID, "not-a-grant")
	if !errors.Is(err, ErrEmailChangeGrantInvalid) {
		t.Fatalf("error = %v, want ErrEmailChangeGrantInvalid", err)
	}
	if len(h.sender.sent) != 2 {
		t.Fatalf("sender calls = %d, want 2", len(h.sender.sent))
	}
}

// resendTakenAddress proves the taken-guard re-runs before the budget is
// spent: an address snatched between steps 2 and the resend is refused and
// the hourly budget keeps its unit.
func resendTakenAddress(t *testing.T) {
	t.Helper()
	h := newEmailChangeHarness()
	// Two units: one for the step-2 send inside runToStepThree, one for the
	// resend attempt — the refusal must leave the second unit unspent.
	two := 2
	h.budgetLeft = &two
	user := h.seedEmailChangeUser(t, mustPhone(t, "+79160000404"), mustEmail(t, seedOwnerEmail), &testNow)
	newEmail := mustEmail(t, "snatched@example.com")
	grantToken, _ := h.runToStepThree(t, user.ID, newEmail)

	h.seedOtherEmailOwner(t, newEmail)
	h.clock.advance(time.Minute)
	err := h.svc.ResendNewEmailCode(t.Context(), user.ID, grantToken)
	if !errors.Is(err, ErrEmailAlreadyTaken) {
		t.Fatalf("error = %v, want ErrEmailAlreadyTaken", err)
	}
	if len(h.sender.sent) != 2 {
		t.Fatalf("sender calls = %d, want 2 (no delivery to the taken address)", len(h.sender.sent))
	}
	if *h.budgetLeft != 1 {
		t.Fatalf("budget left = %d, want 1 (spent nothing on the dead flow)", *h.budgetLeft)
	}
}

// resendBudgetExhausted proves the 5/hour budget covers resend: a denial sends
// nothing and keeps the grant for a later retry.
func resendBudgetExhausted(t *testing.T) {
	t.Helper()
	h := newEmailChangeHarness()
	user := h.seedEmailChangeUser(t, mustPhone(t, "+79160000405"), mustEmail(t, seedOwnerEmail), &testNow)
	newEmail := mustEmail(t, "budget@example.com")
	grantToken, _ := h.runToStepThree(t, user.ID, newEmail)

	zero := 0
	h.budgetLeft = &zero
	h.clock.advance(time.Minute)
	err := h.svc.ResendNewEmailCode(t.Context(), user.ID, grantToken)
	if !errors.Is(err, ErrEmailChangeBudgetExhausted) {
		t.Fatalf("error = %v, want ErrEmailChangeBudgetExhausted", err)
	}
	if len(h.sender.sent) != 2 {
		t.Fatalf("sender calls = %d, want 2", len(h.sender.sent))
	}
	assertStoredGrant(t, h, user.ID, newEmail, grantToken)

	// The budget clears: the resend goes through.
	one := 1
	h.budgetLeft = &one
	if err := h.svc.ResendNewEmailCode(t.Context(), user.ID, grantToken); err != nil {
		t.Fatalf("resend after budget cleared: %v", err)
	}
}

func TestEmailChangeService_ResendNewEmailCode(t *testing.T) {
	t.Parallel()

	t.Run("happy path re-arms step 3 with a fresh code and keeps the grant", func(t *testing.T) {
		t.Parallel()
		resendHappyPath(t)
	})

	t.Run("second resend within a minute is throttled, the live code survives", func(t *testing.T) {
		t.Parallel()
		resendThrottledWithinMinute(t)
	})

	t.Run("expired grant refuses the resend", func(t *testing.T) {
		t.Parallel()
		resendExpiredGrant(t)
	})

	t.Run("unknown grant token refuses the resend", func(t *testing.T) {
		t.Parallel()
		resendWrongGrantToken(t)
	})

	t.Run("taken address refuses the resend without spending the budget", func(t *testing.T) {
		t.Parallel()
		resendTakenAddress(t)
	})

	t.Run("exhausted budget refuses the resend until it clears", func(t *testing.T) {
		t.Parallel()
		resendBudgetExhausted(t)
	})

	t.Run("unknown user returns ErrNotFound", func(t *testing.T) {
		t.Parallel()
		h := newEmailChangeHarness()
		err := h.svc.ResendNewEmailCode(t.Context(), uuid.Must(uuid.NewV7()), "grant")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("error = %v, want ErrNotFound", err)
		}
	})

	t.Run("user without email never holds a live grant — the grant seam refuses first", func(t *testing.T) {
		t.Parallel()
		h := newEmailChangeHarness()
		phone := mustPhone(t, "+79160000406")
		user, err := domain.NewOwner(phone)
		if err != nil {
			t.Fatalf("create user: %v", err)
		}
		h.users.byPhone[phone.String()] = user

		// A user without an email cannot pass step 1, so no live grant can
		// exist: the grant seam answers before the email seam ever runs —
		// the handler-side no-email mapping (#720 Q9) stays defensive.
		err = h.svc.ResendNewEmailCode(t.Context(), user.ID, "grant")
		if !errors.Is(err, ErrEmailChangeGrantInvalid) {
			t.Fatalf("error = %v, want ErrEmailChangeGrantInvalid", err)
		}
		if len(h.sender.sent) != 0 {
			t.Fatalf("sender calls = %d, want 0", len(h.sender.sent))
		}
	})
}

func TestEmailChangeService_ChangeEmail(t *testing.T) {
	t.Parallel()

	t.Run("happy path applies the new email as verified and consumes the grant", func(t *testing.T) {
		t.Parallel()
		changeEmailAppliesAndConsumes(t)
	})

	t.Run("wrong new code records the failure and keeps the grant for a retry", func(t *testing.T) {
		t.Parallel()
		changeEmailWrongCodeKeepsGrant(t)
	})

	t.Run("expired grant forces a restart from step 1", func(t *testing.T) {
		t.Parallel()
		changeEmailExpiredGrant(t)
	})

	t.Run("reused grant is refused even with a fresh code", func(t *testing.T) {
		t.Parallel()
		changeEmailReusedGrant(t)
	})

	t.Run("email taken between steps 2 and 3 returns ErrEmailAlreadyTaken", func(t *testing.T) {
		t.Parallel()
		changeEmailTakenBetweenSteps(t)
	})

	t.Run("unknown user returns ErrNotFound", func(t *testing.T) {
		t.Parallel()
		h := newEmailChangeHarness()
		_, err := h.svc.ChangeEmail(t.Context(), uuid.Must(uuid.NewV7()), "123456", "grant")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("error = %v, want ErrNotFound", err)
		}
	})
}
