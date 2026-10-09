package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
)

// AuthenticationService is a thin orchestrator over LoginCodeService and
// SessionService.Issue (ADR 0033, migration step 4). It owns no login-code,
// attempt-window, TTL, or session-creation logic itself: it composes the two
// deep modules inside a single runInTx so verification, mark-used, session
// issuance, and audit share one commit.
//
// The early-Commit-on-error that the pre-refactor VerifyCode used to record the
// failed-login audit is gone. On a verification failure the success-path
// transaction rolls back, and a separate short runInTx records the attempt and
// the failed-login audit so the rate-limit mutation survives the rollback.
type AuthenticationService struct {
	txStoreFactory
	loginCodes *LoginCodeService
	sessions   SessionService
	publisher  EventPublisher
	clock      clock.Clock
	logger     *slog.Logger
}

// AuthenticationServiceConfig carries the deep-module dependencies the
// orchestrator composes. The transactional repositories, audit recorder, and
// UoW live in the shared txStoreFactory passed to NewAuthenticationService.
type AuthenticationServiceConfig struct {
	LoginCodes *LoginCodeService
	Sessions   SessionService
	Publisher  EventPublisher
	Clock      clock.Clock
	Logger     *slog.Logger
}

// NewAuthenticationService creates an AuthenticationService. It embeds the
// shared identity txStoreFactory so its runInTx calls bind the repositories and
// audit recorder to the transaction; the login-code and session deep modules are
// passed in already constructed (ADR 0033 γ-factory).
func NewAuthenticationService(
	factory txStoreFactory,
	cfg AuthenticationServiceConfig,
) *AuthenticationService {
	if cfg.Clock == nil {
		cfg.Clock = clock.Real{}
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &AuthenticationService{
		txStoreFactory: factory,
		loginCodes:     cfg.LoginCodes,
		sessions:       cfg.Sessions,
		publisher:      cfg.Publisher,
		clock:          cfg.Clock,
		logger:         logger,
	}
}

// SendCode generates a login code, persists it, and sends it by email after the
// transaction commits. It validates the email preconditions for the phone
// before delegating issuance to LoginCodeService, which authoritatively checks
// the not-blocked rule inside its own transaction (#239).
func (s *AuthenticationService) SendCode(
	ctx context.Context,
	phone domain.Phone,
	email domain.Email,
	purpose domain.LoginCodePurpose,
) error {
	user, err := s.users.GetByPhone(ctx, phone)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("get user: %w", err)
	}
	if errors.Is(err, ErrNotFound) {
		// New phone: reject the registration early when the email already
		// belongs to another account instead of failing later at verify.
		if _, emailErr := s.users.GetByEmail(ctx, email); emailErr == nil {
			return ErrEmailAlreadyTaken
		} else if !errors.Is(emailErr, ErrNotFound) {
			return fmt.Errorf("get user by email: %w", emailErr)
		}
	}
	if err == nil && user.Email != nil && *user.Email != email {
		return ErrEmailDoesNotMatch
	}
	var userID *uuid.UUID
	if err == nil {
		userID = &user.ID
	}

	return s.loginCodes.Send(ctx, phone, email, purpose, domain.LoginCodeStepCurrentEmail, userID)
}

// SendCodeByPhone sends a login code to the email stored for the given
// phone. It returns sent=false when the user does not exist or has no
// email on file; the caller should then ask the user for an email.
func (s *AuthenticationService) SendCodeByPhone(ctx context.Context, phone domain.Phone) (bool, error) {
	user, err := s.users.GetByPhone(ctx, phone)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get user: %w", err)
	}
	if user.Email == nil {
		return false, nil
	}
	if err := s.SendCode(ctx, phone, *user.Email, domain.LoginCodePurposeLogin); err != nil {
		return false, err
	}
	return true, nil
}

// VerifyCode verifies a login code and creates a session for the user.
// A nil email is resolved from the stored user record for the phone.
// A nil or invalid timezone keeps the creation default (Europe/Moscow,
// migration 000088); a valid one applies only when this verify creates the
// account — an existing user's saved zone is never overwritten (#451).
//
// The success path runs inside a single runInTx: verify → mark-used →
// SessionService.Issue → apply registration timezone → reset attempts → audit.
// A verification failure rolls that transaction back, then a separate short
// runInTx records the failed attempt and the failed-login audit so
// rate-limiting survives the rollback — replacing the pre-refactor
// early-Commit-on-error (ADR 0033).
func (s *AuthenticationService) VerifyCode(
	ctx context.Context,
	phone domain.Phone,
	email *domain.Email,
	code string,
	timezone *string,
	device DeviceContext,
) (domain.RawSession, domain.User, error) {
	now := s.clock.Now()

	resolvedEmail, err := s.resolveEmail(ctx, phone, email)
	if err != nil {
		return domain.RawSession{}, domain.User{}, err
	}

	// Validate before the transaction: an unparsable value is not an error —
	// the account is simply born on the default zone.
	registrationTZ := registrationTimezone(timezone)

	// The not-blocked check is authoritative inside LoginCodeService.Verify,
	// which runs within the runInTx below and returns ErrUserBlocked before any
	// code is read (#237, #239). A blocked phone falls through to the
	// recovery-free `case err != nil` branch — the attempt was already recorded
	// when the block took effect, so no recovery is needed.
	var raw domain.RawSession
	var user domain.User
	isNewUser := false

	err = s.runInTx(ctx, func(stores *txStores) error {
		loginCode, vErr := s.loginCodes.Verify(ctx, stores, phone, resolvedEmail, domain.LoginCodePurposeLogin, code)
		if vErr != nil {
			return vErr
		}

		if err := stores.codes.MarkUsedByID(ctx, loginCode.ID); err != nil {
			return fmt.Errorf("mark code used: %w", err)
		}

		issued, u, isNew, iErr := s.sessions.Issue(ctx, stores, phone, resolvedEmail, device, now)
		if iErr != nil {
			return iErr
		}
		raw, user, isNewUser = issued, u, isNew

		updatedUser, tzErr := applyRegistrationTimezone(ctx, stores, isNew, user, registrationTZ)
		if tzErr != nil {
			return tzErr
		}
		user = updatedUser

		if err := stores.attempts.DeleteByPhone(ctx, phone); err != nil {
			return fmt.Errorf("reset login attempts: %w", err)
		}

		action := auditdomain.ActionAuthLogin
		if isNewUser {
			action = auditdomain.ActionAuthRegistered
		}
		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &user.ID,
			ActorRole:  auditdomain.ActorRoleFromRole(user.Role),
			Action:     action,
			EntityType: auditdomain.EntityUser,
			EntityID:   &user.ID,
			Context:    map[string]any{"method": "email_code"},
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})

	switch {
	case errors.Is(err, domain.ErrLoginCodeInvalid):
		// The success path rolled back. Record the failed attempt and the
		// failed-login audit in a separate transaction so the rate-limit
		// mutation survives. RecordFailureAndAudit opens its own runInTx,
		// runs RecordFailure, and records the audit with the failure reason;
		// it surfaces ErrTooManyAttempts via outErr so callers and tests
		// observe the block. Infrastructure errors are logged but do not mask
		// the original verification error.
		//
		// Only an invalid code triggers recovery: ErrUserBlocked (now surfaced
		// by LoginCodeService.Verify for an already-blocked phone) passes
		// through the recovery-free branch below, since the attempt that
		// triggered the block was recorded when the block took effect (#239).
		//
		// The 3-line wrapper (finalErr + auditEntry + RecordFailureAndAudit)
		// is intentionally NOT extracted into a LoginCodeService.OnVerifyFailure
		// method. The deep logic already lives in RecordFailureAndAudit
		// (ADR 0033). What remains here is orchestrator-specific data: the
		// auditEntry carries an anonymous actor and a login-failed action that
		// differ from PhoneChangeService, the return type is (RawSession, User,
		// error), and the log message names this context. Folding those into
		// OnVerifyFailure would move — not concentrate — the duplication, add a
		// log-message parameter, and widen LoginCodeService's responsibility past
		// login-code management. See issue #240 (re-evaluated, rejected).
		finalErr := err
		auditEntry := auditdomain.Entry{
			ActorRole: auditdomain.ActorRoleAnonymous,
			Action:    auditdomain.ActionAuthLoginFailed,
		}
		if recErr := s.loginCodes.RecordFailureAndAudit(ctx, phone, uuid.Nil, auditEntry, &finalErr); recErr != nil {
			s.logger.ErrorContext(ctx, "failed to record failed-login audit", slog.String("error", sanitize.Error(recErr)))
		}
		return domain.RawSession{}, domain.User{}, finalErr
	case err != nil:
		return domain.RawSession{}, domain.User{}, err
	}

	if isNewUser {
		if err := s.publisher.PublishUserRegistered(ctx,
			UserRegistered{UserID: user.ID, Phone: phone, Email: resolvedEmail, At: now}); err != nil {
			s.logger.ErrorContext(ctx, "failed to publish user registered event", slog.String("error", sanitize.Error(err)))
		}
	}

	return raw, user, nil
}

// applyRegistrationTimezone fixes the device zone on a brand-new account
// (#451): the zone travels with the verify request and is persisted in the
// same transaction that creates the user, so the verify response already
// carries it. A race that resolved to an existing row (isNew=false) keeps its
// saved zone untouched, and a nil zone leaves the Europe/Moscow creation
// default. Returns the user as the caller should see it.
func applyRegistrationTimezone(
	ctx context.Context,
	stores *txStores,
	isNew bool,
	user domain.User,
	tz *domain.Timezone,
) (domain.User, error) {
	if !isNew || tz == nil {
		return user, nil
	}
	user.Timezone = *tz
	if _, err := stores.users.Update(ctx, user); err != nil {
		return domain.User{}, fmt.Errorf("apply registration timezone: %w", err)
	}
	return user, nil
}

// registrationTimezone validates the browser-detected zone carried by the
// verify request (#451). An absent or unparsable value is not an error — the
// account is simply born on the Europe/Moscow default (migration 000088).
func registrationTimezone(raw *string) *domain.Timezone {
	if raw == nil {
		return nil
	}
	tz, err := domain.NewTimezone(*raw)
	if err != nil {
		return nil
	}
	return &tz
}

// resolveEmail returns the given email or, when it is nil, the email stored
// for the phone. ErrNotFound maps to 401 in the HTTP handler.
func (s *AuthenticationService) resolveEmail(ctx context.Context, phone domain.Phone, email *domain.Email) (domain.Email, error) {
	if email != nil {
		return *email, nil
	}
	user, err := s.users.GetByPhone(ctx, phone)
	if err != nil {
		return domain.Email{}, err
	}
	if user.Email == nil {
		return domain.Email{}, ErrNotFound
	}
	return *user.Email, nil
}
