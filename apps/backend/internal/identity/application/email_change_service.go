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
// The three-step protocol (grilling decisions in #720):
//
//  1. SendCurrentEmailCode delivers a code to the user's current email; the
//     delivery itself confirms that address, so it is stamped verified.
//  2. ConfirmCurrentEmail verifies the step-1 code and — in one transaction —
//     issues the grant and the code for the new address, after prechecks:
//     the address must differ from the current one and be free.
//  3. ChangeEmail verifies the step-2 code against the grant, applies the new
//     address as verified, consumes the grant, and audits — sessions are
//     untouched (the email is a delivery channel, not the login).
//
// ResendNewEmailCode (#732) re-issues the step-2 code against the still-live
// grant: ConfirmCurrentEmail burned the step-1 code, so the grant is the
// resend's anchor — same prechecks, budget and send throttle; the grant
// itself survives for step 3.
//
// The grant is server-side state: the client sees a one-time plaintext token,
// the store keeps only its hash; a user holds at most one live grant.
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
	// the per-user 5/hour budget from decision #720-3. It is consulted only
	// after the current-address code verified, so a denial costs the user no
	// burned code (the transaction rolls back). Nil means "allow" (tests).
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

	if err := s.loginCodes.Send(ctx, user.Phone, email, domain.LoginCodePurposeEmailChange, &user.ID); err != nil {
		return err
	}

	if _, err := s.users.MarkEmailVerified(ctx, user.ID, s.clock.Now()); err != nil {
		return fmt.Errorf("mark current email verified: %w", err)
	}
	return nil
}

// ConfirmCurrentEmail implements step 2: verify the code from the current
// email, then — in one transaction — store the grant and issue the code for
// the new address, so the order of steps is enforced server-side (decision
// #720-2). The code for the new address is delivered after the commit.
//
// A verification failure rolls back, then a separate short runInTx records the
// failed attempt into the phone's window so rate-limiting survives the
// rollback — the same recovery PhoneChangeService and AuthenticationService use.
func (s *EmailChangeService) ConfirmCurrentEmail(
	ctx context.Context,
	userID uuid.UUID,
	code string,
	newEmail domain.Email,
) (string, error) {
	var phone domain.Phone
	var issued domain.LoginCode
	var plaintextCode string
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

		// The current-address code is verified before the address prechecks:
		// without the code a probe must see the plain 401, not which
		// addresses are taken (enumeration).
		loginCode, err := s.loginCodes.Verify(ctx, stores, user.Phone, currentEmail, domain.LoginCodePurposeEmailChange, code)
		if err != nil {
			return err
		}

		if err := s.checkNewEmailFree(ctx, stores, user.ID, currentEmail, newEmail); err != nil {
			return err
		}

		// The budget guards actual sends to the new address: a denial here
		// rolls the transaction back, so the verified code survives for a
		// retry once the hourly budget clears.
		if s.sendBudget != nil && !s.sendBudget(user.ID) {
			return ErrEmailChangeBudgetExhausted
		}

		if err := stores.codes.MarkUsedByID(ctx, loginCode.ID); err != nil {
			return fmt.Errorf("mark code used: %w", err)
		}

		token, err := s.replaceGrant(ctx, stores, user.ID, newEmail)
		if err != nil {
			return err
		}
		grantToken = token

		issued, plaintextCode, err = s.loginCodes.IssueInTx(ctx, stores, user.Phone, newEmail, domain.LoginCodePurposeEmailChange, &user.ID)
		return err
	})

	if errors.Is(err, domain.ErrLoginCodeInvalid) {
		finalErr := s.recordConfirmFailure(ctx, phone, userID)
		return "", finalErr
	}
	if err != nil {
		return "", err
	}

	if err := s.loginCodes.Deliver(ctx, phone, newEmail, domain.LoginCodePurposeEmailChange, issued, plaintextCode); err != nil {
		return "", err
	}
	return grantToken, nil
}

// ChangeEmail implements step 3: verify the code sent to the new address
// against the grant, apply the change, and consume the grant — one commit
// covers verify → mark-used → apply → grant deletion → audit. The grant check
// runs before the code check so an expired or replayed grant fails without
// burning a still-live code or polluting the attempt window.
//
// Sessions are deliberately untouched: the email is the delivery channel, not
// the login (decision #720-5).
func (s *EmailChangeService) ChangeEmail(
	ctx context.Context,
	userID uuid.UUID,
	code, grantToken string,
) (domain.User, error) {
	var updated domain.User
	var phone domain.Phone

	err := s.runInTx(ctx, func(stores *txStores) error {
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
		if err := s.checkNewEmailFree(ctx, stores, user.ID, currentEmail, grant.Email); err != nil {
			return err
		}

		loginCode, err := s.loginCodes.Verify(ctx, stores, user.Phone, grant.Email, domain.LoginCodePurposeEmailChange, code)
		if err != nil {
			return err
		}
		if err := stores.codes.MarkUsedByID(ctx, loginCode.ID); err != nil {
			return fmt.Errorf("mark code used: %w", err)
		}

		if err := stores.grants.DeleteByID(ctx, grant.ID); err != nil {
			return fmt.Errorf("consume grant: %w", err)
		}

		updated, err = stores.users.UpdateEmailVerified(ctx, userID, &grant.Email, new(s.clock.Now()))
		if err != nil {
			return fmt.Errorf("update email: %w", err)
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
		return nil
	})

	if errors.Is(err, domain.ErrLoginCodeInvalid) {
		finalErr := s.recordConfirmFailure(ctx, phone, userID)
		return domain.User{}, finalErr
	}
	if err != nil {
		return domain.User{}, err
	}
	return updated, nil
}

// ResendNewEmailCode re-issues and re-delivers the code for the pending new
// address (issue #732, resend button on the step-2 screen). The grant is the
// anchor: the step-1 code was burned by ConfirmCurrentEmail, so what proves
// the passed step 1 — and pins the delivery address — is the still-live grant.
//
// One runInTx covers lock user → check grant → re-check the address is still
// free (it may have been snatched while the user waited; failing before the
// budget means a dead flow costs nothing) → spend the new-address budget
// (every delivery counts, #720-3) → issue through IssueInTx, whose send
// throttle rejects a re-issuance within a minute of the live code. The stale
// step-2 code is deleted by that same issuance path, so only the fresh code
// verifies at step 3. The grant itself survives — step 3 consumes it. No
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

		grant, err := s.liveGrant(ctx, stores, userID, grantToken)
		if err != nil {
			return err
		}

		currentEmail, err := userEmail(user)
		if err != nil {
			return err
		}
		if err := s.checkNewEmailFree(ctx, stores, user.ID, currentEmail, grant.Email); err != nil {
			return err
		}

		if s.sendBudget != nil && !s.sendBudget(user.ID) {
			return ErrEmailChangeBudgetExhausted
		}

		issued, plaintextCode, err = s.loginCodes.IssueInTx(ctx, stores, phone, grant.Email, domain.LoginCodePurposeEmailChange, &user.ID)
		return err
	})
	if err != nil {
		return err
	}

	return s.loginCodes.Deliver(ctx, phone, issued.Email, domain.LoginCodePurposeEmailChange, issued, plaintextCode)
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

// replaceGrant issues a fresh grant for the user and the new address, removing
// any previous grant first — a user holds at most one live grant.
// It returns the one-time plaintext token; only the hash is persisted.
func (s *EmailChangeService) replaceGrant(
	ctx context.Context,
	stores *txStores,
	userID uuid.UUID,
	newEmail domain.Email,
) (string, error) {
	token, err := generateGrantToken()
	if err != nil {
		return "", err
	}
	grant := domain.NewEmailChangeGrant(userID, newEmail, s.hashGrantToken(token), s.clock.Now())
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

// recordConfirmFailure records a failed verification attempt into the phone's
// window plus the failure audit in a short transaction that survives the
// rolled-back success path — the recovery wrapper PhoneChangeService uses,
// specialized to the email-change audit action.
func (s *EmailChangeService) recordConfirmFailure(ctx context.Context, phone domain.Phone, userID uuid.UUID) error {
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
