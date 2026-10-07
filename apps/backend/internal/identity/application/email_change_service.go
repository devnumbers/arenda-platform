package application

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
)

// grantTokenSpace is the byte length of the random grant token handed to the
// client once: 256 bits of entropy, base64url-encoded for JSON transport.
const grantTokenSpace = 32

// grantTokenPrefix namespaces the grant token inside the token hasher, mirroring
// the "login_code:" prefix LoginCodeService.hashCode uses for codes.
const grantTokenPrefix = "email_change_grant:"

// hashGrantToken is the single hashing point for grant tokens: the hash a
// grant is created with and the hash a live-grant check compares against
// must stay byte-identical, or every step-3 verification silently fails.
func (s *EmailChangeService) hashGrantToken(token string) string {
	return s.hasher.HashToken(grantTokenPrefix + token)
}

// EmailChangeService handles confirmed email change for authenticated users
// (issue #721). It reuses LoginCodeService with purpose = email_change instead
// of duplicating the login-code flow, mirroring PhoneChangeService.
//
// The five-step protocol (grilling decisions in #720, reworked around the
// server-side step-1 check — protocol #1202):
//
//  1. SendCurrentEmailCode delivers a code to the user's current email; the
//     delivery itself confirms that address, so it is stamped verified.
//  2. VerifyCurrentEmail verifies the step-1 code, burns it, and issues the
//     grant — addressless: the grant proves the current address, before any
//     new address is named.
//  3. RequestNewEmailCode validates the grant, binds the new address to it,
//     and issues + delivers the code for the new address. The binding commits
//     before the budget-guarded issuance, so a budget refusal leaves the
//     grant and its address alive — the retry repeats only this step, not
//     steps 1–2.
//  4. ResendNewEmailCode (#732) re-issues the code against the still-live
//     grant bound to the new address.
//  5. ChangeEmail verifies the code against the grant, applies the new
//     address as verified, consumes the grant, revokes every other session,
//     audits, and schedules the change letter to the old address — all in
//     one commit (решение #1207: other devices are force-logged-out and the
//     old address learns the change the moment it commits).
//
// The grant is server-side state: the client sees a one-time plaintext token,
// the store keeps only its hash; a user holds at most one live grant. An
// addressless grant spans only steps 2→3: it cannot anchor a delivery, so
// resend and change require the bound address (#1202) — otherwise the NULL
// address would leak into the domain.
type EmailChangeService struct {
	txStoreFactory
	loginCodes *LoginCodeService
	clock      clock.Clock
	hasher     TokenHasher
	logger     *slog.Logger
	sendBudget func(userID uuid.UUID) bool
}

// EmailChangeServiceConfig carries the non-transactional dependencies for
// EmailChangeService. The transactional repositories, audit recorder, and UoW
// live in the shared txStoreFactory passed to NewEmailChangeService.
type EmailChangeServiceConfig struct {
	LoginCodes *LoginCodeService
	Clock      clock.Clock
	Hasher     TokenHasher
	Logger     *slog.Logger
	// AllowNewAddressSend gates each delivery of a code to a NEW address —
	// the per-user 5/hour budget from decision #720-3. It is consulted on
	// the sends that actually deliver to a new address — step 3
	// (RequestNewEmailCode) and resend; a denial there cannot undo the
	// address binding committed just before, so the retry repeats only the
	// send. Nil means "allow" (tests).
	AllowNewAddressSend func(userID uuid.UUID) bool
}

// NewEmailChangeService creates an EmailChangeService. It embeds the shared
// identity txStoreFactory so ConfirmCurrentEmail and ChangeEmail run through
// runInTx; login-code issuance and verification delegate to the shared
// LoginCodeService.
func NewEmailChangeService(
	factory txStoreFactory,
	cfg EmailChangeServiceConfig,
) *EmailChangeService {
	if cfg.Clock == nil {
		cfg.Clock = clock.Real{}
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &EmailChangeService{
		txStoreFactory: factory,
		loginCodes:     cfg.LoginCodes,
		clock:          cfg.Clock,
		hasher:         cfg.Hasher,
		logger:         logger,
		sendBudget:     cfg.AllowNewAddressSend,
	}
}

// SendCurrentEmailCode implements step 1: a code to the current email. A user
// without an email is refused up front — the flow has nowhere to deliver. On
// success the current address is marked verified: a code delivered to it is
// itself the proof of ownership (decision #720-6), even if the flow is
// abandoned here.
func (s *EmailChangeService) SendCurrentEmailCode(ctx context.Context, userID uuid.UUID) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	email, err := userEmail(user)
	if err != nil {
		return err
	}

	if err := s.loginCodes.Send(
		ctx, user.Phone, email, domain.LoginCodePurposeEmailChange, domain.LoginCodeStepCurrentEmail, &user.ID,
	); err != nil {
		return err
	}

	if _, err := s.users.MarkEmailVerified(ctx, user.ID, s.clock.Now()); err != nil {
		return fmt.Errorf("mark current email verified: %w", err)
	}
	return nil
}

// VerifyCurrentEmail implements step 2: verify the code sent to the current
// email, burn it, and issue the grant — without any new address (protocol
// #1202: the grant proves the current address; the new address binds to it at
// step 3). No delivery happens here.
//
// A verification failure rolls back, then a separate short runInTx records the
// failed attempt into the phone's window so rate-limiting survives the
// rollback — the same recovery PhoneChangeService and AuthenticationService use.
func (s *EmailChangeService) VerifyCurrentEmail(
	ctx context.Context,
	userID uuid.UUID,
	code string,
) (string, error) {
	var phone domain.Phone
	var grantToken string

	err := s.runInTx(ctx, func(stores *txStores) error {
		user, err := stores.users.GetByIDForUpdate(ctx, userID)
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}
		phone = user.Phone
		currentEmail, err := userEmail(user)
		if err != nil {
			return err
		}

		loginCode, err := s.loginCodes.Verify(ctx, stores, user.Phone, currentEmail, domain.LoginCodePurposeEmailChange, code)
		if err != nil {
			return err
		}
		if err := stores.codes.MarkUsedByID(ctx, loginCode.ID); err != nil {
			return fmt.Errorf("mark code used: %w", err)
		}

		token, err := s.replaceGrant(ctx, stores, user.ID)
		if err != nil {
			return err
		}
		grantToken = token
		return nil
	})

	if errors.Is(err, domain.ErrLoginCodeInvalid) {
		finalErr := s.recordVerifyFailure(ctx, phone, userID)
		return "", finalErr
	}
	if err != nil {
		return "", err
	}
	return grantToken, nil
}

// RequestNewEmailCode implements step 3: validate the grant, bind the new
// address to it, and issue + deliver the code for the new address (protocol
// #1202). The check order — grant → format / not-same / taken → budget →
// issue — puts the address probes behind the grant gate: reaching them
// requires the burned step-1 code, a stronger proof than the authenticated
// session, so the old "verify the code before the prechecks" enumeration
// guard is obsolete.
//
// Two transactions, in this order: the first validates the grant and commits
// the address binding; the second spends the new-address budget and issues
// the code. A budget refusal then rolls back only the issuance — the grant
// and its bound address survive, so the retry after the hourly budget clears
// repeats just this step, not steps 1–2. There is no code check here: the
// attempt window and the failure audit stay untouched.
func (s *EmailChangeService) RequestNewEmailCode(
	ctx context.Context,
	userID uuid.UUID,
	grantToken string,
	newEmail domain.Email,
) error {
	var phone domain.Phone

	// Transaction 1: the grant gate plus the address binding. The grant may
	// still be addressless — binding one is this step's whole point.
	if err := s.runInTx(ctx, func(stores *txStores) error {
		user, err := stores.users.GetByIDForUpdate(ctx, userID)
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}
		phone = user.Phone

		grant, err := s.liveGrant(ctx, stores, userID, grantToken)
		if err != nil {
			return err
		}

		currentEmail, err := userEmail(user)
		if err != nil {
			return err
		}
		if err := s.checkNewEmailFree(ctx, stores, user.ID, currentEmail, newEmail); err != nil {
			return err
		}

		return stores.grants.UpdateEmail(ctx, grant.ID, newEmail)
	}); err != nil {
		return err
	}

	// Transaction 2: the budget plus the issuance, re-checked against the
	// same grant — the flow may have been restarted or finished between the
	// two transactions, and the binding may have moved: issuing for an
	// address the grant no longer carries would spend the budget on a dead
	// flow.
	var issued domain.LoginCode
	var plaintextCode string
	err := s.runInTx(ctx, func(stores *txStores) error {
		user, err := stores.users.GetByIDForUpdate(ctx, userID)
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}
		grant, err := s.liveGrant(ctx, stores, userID, grantToken)
		if err != nil {
			return err
		}
		if grant.Email == nil || *grant.Email != newEmail {
			return ErrEmailChangeGrantInvalid
		}

		if s.sendBudget != nil && !s.sendBudget(user.ID) {
			return ErrEmailChangeBudgetExhausted
		}

		issued, plaintextCode, err = s.loginCodes.IssueInTx(ctx, stores, phone, newEmail, domain.LoginCodePurposeEmailChange, &user.ID)
		return err
	})
	if err != nil {
		return err
	}

	return s.loginCodes.Deliver(ctx, phone, newEmail, domain.LoginCodePurposeEmailChange, domain.LoginCodeStepNewEmail, issued, plaintextCode)
}

// ChangeEmail implements the final step: verify the code sent to the new
// address against the grant, apply the change, and consume the grant — one
// commit covers verify → mark-used → apply → session revocation → grant
// deletion → audit → change-letter scheduling. The grant check runs before
// the code check so an expired or replayed grant fails without burning a
// still-live code or polluting the attempt window. The grant must carry its
// bound address: an addressless one (step 2 just passed, step 3 never ran)
// is the same "start over" refusal (#1202).
//
// Every session except the one making the change is revoked (решение #1207,
// the same rule the phone change follows — overriding the old "email is only
// a delivery channel" stance of #720-5), and the change letter is scheduled
// in this same transaction: the old address learns the change the moment it
// commits, or not at all.
func (s *EmailChangeService) ChangeEmail(
	ctx context.Context,
	userID uuid.UUID,
	code, grantToken, currentToken string,
) (domain.User, error) {
	var updated domain.User
	var phone domain.Phone
	var oldEmail domain.Email

	err := s.runInTx(ctx, func(stores *txStores) error {
		user, err := stores.users.GetByIDForUpdate(ctx, userID)
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}
		phone = user.Phone

		grant, err := s.liveBoundGrant(ctx, stores, userID, grantToken)
		if err != nil {
			return err
		}

		currentEmail, err := userEmail(user)
		if err != nil {
			return err
		}
		oldEmail = currentEmail
		if err := s.checkNewEmailFree(ctx, stores, user.ID, currentEmail, *grant.Email); err != nil {
			return err
		}

		loginCode, err := s.loginCodes.Verify(ctx, stores, user.Phone, *grant.Email, domain.LoginCodePurposeEmailChange, code)
		if err != nil {
			return err
		}
		if err := stores.codes.MarkUsedByID(ctx, loginCode.ID); err != nil {
			return fmt.Errorf("mark code used: %w", err)
		}

		if err := stores.grants.DeleteByID(ctx, grant.ID); err != nil {
			return fmt.Errorf("consume grant: %w", err)
		}

		updated, err = stores.users.UpdateEmailVerified(ctx, userID, grant.Email, new(s.clock.Now()))
		if err != nil {
			return fmt.Errorf("update email: %w", err)
		}

		if _, err := stores.sessions.DeleteByUserIDExcept(ctx, userID, s.hasher.HashToken(currentToken)); err != nil {
			return fmt.Errorf("delete other sessions: %w", err)
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &userID,
			ActorRole:  auditdomain.ActorRoleFromRole(updated.Role),
			Action:     auditdomain.ActionAuthEmailChanged,
			EntityType: auditdomain.EntityUser,
			EntityID:   &userID,
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}

		// The change letter commits with the change (решение #1207): the old
		// address is announced the moment the new one takes effect. The new
		// address never enters the event — the recipient is the old one.
		if err := stores.notifier.ScheduleChanged(ctx, ContactChangedEvent{
			Kind:      ContactChangedEmail,
			UserID:    userID,
			Recipient: oldEmail,
			ChangedAt: s.clock.Now(),
		}); err != nil {
			return fmt.Errorf("schedule change letter: %w", err)
		}
		return nil
	})

	if errors.Is(err, domain.ErrLoginCodeInvalid) {
		finalErr := s.recordVerifyFailure(ctx, phone, userID)
		return domain.User{}, finalErr
	}
	if err != nil {
		return domain.User{}, err
	}
	return updated, nil
}

// ResendNewEmailCode re-issues and re-delivers the code for the pending new
// address (issue #732, resend button on the code screen). The grant is the
// anchor: the step-1 code was burned by VerifyCurrentEmail, so what proves
// the passed step 1 — and pins the delivery address — is the still-live
// grant bound to the new address; an addressless grant cannot anchor a
// delivery and is refused (#1202).
//
// One runInTx covers lock user → check grant → re-check the address is still
// free (it may have been snatched while the user waited; failing before the
// budget means a dead flow costs nothing) → spend the new-address budget
// (every delivery counts, #720-3) → issue through IssueInTx, whose send
// throttle rejects a re-issuance within a minute of the live code. The stale
// code is deleted by that same issuance path, so only the fresh code verifies
// at the final step. The grant itself survives — the change consumes it. No
// audit: resend changes no state.
func (s *EmailChangeService) ResendNewEmailCode(ctx context.Context, userID uuid.UUID, grantToken string) error {
	var phone domain.Phone
	var issued domain.LoginCode
	var plaintextCode string

	err := s.runInTx(ctx, func(stores *txStores) error {
		user, err := stores.users.GetByIDForUpdate(ctx, userID)
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}
		phone = user.Phone

		grant, err := s.liveBoundGrant(ctx, stores, userID, grantToken)
		if err != nil {
			return err
		}

		currentEmail, err := userEmail(user)
		if err != nil {
			return err
		}
		if err := s.checkNewEmailFree(ctx, stores, user.ID, currentEmail, *grant.Email); err != nil {
			return err
		}

		if s.sendBudget != nil && !s.sendBudget(user.ID) {
			return ErrEmailChangeBudgetExhausted
		}

		issued, plaintextCode, err = s.loginCodes.IssueInTx(ctx, stores, phone, *grant.Email, domain.LoginCodePurposeEmailChange, &user.ID)
		return err
	})
	if err != nil {
		return err
	}

	return s.loginCodes.Deliver(
		ctx, phone, issued.Email, domain.LoginCodePurposeEmailChange, domain.LoginCodeStepNewEmail, issued, plaintextCode,
	)
}

// checkNewEmailFree guards the address switch: the new email must differ from
// the current one and must not belong to another user (decision #720-4). The
// transactional read narrows the window, but the durable backstop against two
// users ending up with one address is the unique index on LOWER(email)
// (migration 000049), which fails the second write at commit.
func (s *EmailChangeService) checkNewEmailFree(
	ctx context.Context,
	stores *txStores,
	userID uuid.UUID,
	currentEmail, newEmail domain.Email,
) error {
	if currentEmail.String() == newEmail.String() {
		return ErrEmailUnchanged
	}
	existing, err := stores.users.GetByEmailForUpdate(ctx, newEmail)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("check email: %w", err)
	}
	if existing.ID != uuid.Nil && existing.ID != userID {
		return ErrEmailAlreadyTaken
	}
	return nil
}

// replaceGrant issues a fresh addressless grant for the user, removing any
// previous grant first — a user holds at most one live grant. It returns the
// one-time plaintext token; only the hash is persisted. The new address binds
// to the surviving grant at step 3 (RequestNewEmailCode).
func (s *EmailChangeService) replaceGrant(
	ctx context.Context,
	stores *txStores,
	userID uuid.UUID,
) (string, error) {
	token, err := generateGrantToken()
	if err != nil {
		return "", err
	}
	grant := domain.NewEmailChangeGrant(userID, s.hashGrantToken(token), s.clock.Now())
	if err := stores.grants.DeleteByUserID(ctx, userID); err != nil {
		return "", fmt.Errorf("clear previous grant: %w", err)
	}
	if err := stores.grants.Save(ctx, grant); err != nil {
		return "", fmt.Errorf("save grant: %w", err)
	}
	return token, nil
}

// liveGrant resolves the user's grant and validates it against the presented
// token and the clock. An absent, expired, or mismatched grant is the same
// outcome for the caller: the flow must restart from step 1 (decision #720-2).
func (s *EmailChangeService) liveGrant(
	ctx context.Context,
	stores *txStores,
	userID uuid.UUID,
	grantToken string,
) (domain.EmailChangeGrant, error) {
	grant, err := stores.grants.GetByUserIDForUpdate(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.EmailChangeGrant{}, ErrEmailChangeGrantInvalid
		}
		return domain.EmailChangeGrant{}, fmt.Errorf("get grant: %w", err)
	}
	if grant.Expired(s.clock.Now()) || grant.TokenHash != s.hashGrantToken(grantToken) {
		return domain.EmailChangeGrant{}, ErrEmailChangeGrantInvalid
	}
	return grant, nil
}

// liveBoundGrant resolves the user's grant for the delivery-anchored steps —
// resend and the final change: the grant must be live AND carry the address
// bound at step 3. An addressless grant (step 2 just passed, step 3 never
// ran) cannot anchor a delivery, so it is the same "start over" outcome for
// the caller (#1202) — the NULL address never leaks below this seam.
func (s *EmailChangeService) liveBoundGrant(
	ctx context.Context,
	stores *txStores,
	userID uuid.UUID,
	grantToken string,
) (domain.EmailChangeGrant, error) {
	grant, err := s.liveGrant(ctx, stores, userID, grantToken)
	if err != nil {
		return domain.EmailChangeGrant{}, err
	}
	if grant.Email == nil {
		return domain.EmailChangeGrant{}, ErrEmailChangeGrantInvalid
	}
	return grant, nil
}

// recordVerifyFailure records a failed verification attempt into the phone's
// window plus the failure audit in a short transaction that survives the
// rolled-back success path — the recovery wrapper PhoneChangeService uses,
// specialized to the email-change audit action.
func (s *EmailChangeService) recordVerifyFailure(ctx context.Context, phone domain.Phone, userID uuid.UUID) error {
	finalErr := domain.ErrLoginCodeInvalid
	auditEntry := auditdomain.Entry{
		ActorID:   &userID,
		ActorRole: auditdomain.ActorRoleOwner,
		Action:    auditdomain.ActionAuthEmailChangeFailed,
	}
	if recErr := s.loginCodes.RecordFailureAndAudit(ctx, phone, userID, auditEntry, &finalErr); recErr != nil {
		s.logger.ErrorContext(ctx, "failed to record email-change failed attempt", slog.String("error", sanitize.Error(recErr)))
	}
	return finalErr
}

// generateGrantToken returns a cryptographically random 256-bit base64url token.
func generateGrantToken() (string, error) {
	buf := make([]byte, grantTokenSpace)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate grant token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
