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

// PhoneChangeService handles phone number change for authenticated users. It
// reuses LoginCodeService with purpose = phone_change instead of duplicating the
// login-code flow (ADR 0033, migration step 5). Code issuance runs through
// LoginCodeService.Send; verification runs inside this service's runInTx so
// verify → mark-used → update-phone → session/code cleanup → audit share one
// commit.
type PhoneChangeService struct {
	txStoreFactory
	loginCodes *LoginCodeService
	clock      clock.Clock
	hasher     TokenHasher
	logger     *slog.Logger
}

// PhoneChangeServiceConfig carries the non-transactional dependencies for
// PhoneChangeService. The transactional repositories, audit recorder, and UoW
// live in the shared txStoreFactory passed to NewPhoneChangeService.
type PhoneChangeServiceConfig struct {
	LoginCodes *LoginCodeService
	Clock      clock.Clock
	Hasher     TokenHasher
	Logger     *slog.Logger
}

// NewPhoneChangeService creates a PhoneChangeService. It embeds the shared
// identity txStoreFactory so ChangePhone runs through runInTx; login-code
// issuance and verification delegate to the shared LoginCodeService
// (ADR 0033 γ-factory).
func NewPhoneChangeService(
	factory txStoreFactory,
	cfg PhoneChangeServiceConfig,
) *PhoneChangeService {
	if cfg.Clock == nil {
		cfg.Clock = clock.Real{}
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &PhoneChangeService{
		txStoreFactory: factory,
		loginCodes:     cfg.LoginCodes,
		clock:          cfg.Clock,
		hasher:         cfg.Hasher,
		logger:         logger,
	}
}

// SendChangeCode sends a verification code to the user's email to confirm a new
// phone number. It validates the new phone against existing users before
// delegating issuance to LoginCodeService with purpose = phone_change.
func (s *PhoneChangeService) SendChangeCode(ctx context.Context, userID uuid.UUID, newPhone domain.Phone) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	if user.Phone == newPhone {
		return ErrPhoneUnchanged
	}

	existing, err := s.users.GetByPhone(ctx, newPhone)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("check phone: %w", err)
	}
	if existing.ID != uuid.Nil && existing.ID != userID {
		return ErrPhoneAlreadyTaken
	}

	email, err := userEmail(user)
	if err != nil {
		return err
	}

	return s.loginCodes.Send(ctx, newPhone, email, domain.LoginCodePurposePhoneChange, &user.ID)
}

// ChangePhone verifies the code and updates the user's phone number.
// currentToken is the raw session token of the current session; it is used to
// keep the current session alive when deleting all other sessions.
//
// The whole flow runs inside a single runInTx: verify → mark-used → update-phone
// → delete other sessions → clear codes/attempts → audit. A verification failure
// rolls back, then a separate short runInTx records the failed attempt so
// rate-limiting survives the rollback — mirroring AuthenticationService.
func (s *PhoneChangeService) ChangePhone(ctx context.Context, userID uuid.UUID, newPhone domain.Phone, code, currentToken string) (domain.User, error) {
	if err := checkNotBlocked(ctx, s.attempts, s.clock, newPhone); err != nil {
		return domain.User{}, err
	}

	var updated domain.User
	var oldPhone domain.Phone

	err := s.runInTx(ctx, func(stores *txStores) error {
		if err := checkNotBlocked(ctx, stores.attempts, s.clock, newPhone); err != nil {
			return err
		}

		user, err := stores.users.GetByIDForUpdate(ctx, userID)
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}
		if user.Phone == newPhone {
			return ErrPhoneUnchanged
		}
		oldPhone = user.Phone

		existing, err := stores.users.GetByPhoneForUpdate(ctx, newPhone)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("check phone: %w", err)
		}
		if existing.ID != uuid.Nil && existing.ID != userID {
			return ErrPhoneAlreadyTaken
		}

		email, err := userEmail(user)
		if err != nil {
			return err
		}

		loginCode, vErr := s.loginCodes.Verify(ctx, stores, newPhone, email, domain.LoginCodePurposePhoneChange, code)
		if vErr != nil {
			return vErr
		}

		if err := stores.codes.MarkUsedByID(ctx, loginCode.ID); err != nil {
			return fmt.Errorf("mark code used: %w", err)
		}

		updated, err = stores.users.UpdatePhone(ctx, userID, newPhone)
		if err != nil {
			return fmt.Errorf("update phone: %w", err)
		}

		if err := stores.sessions.DeleteByUserIDExcept(ctx, userID, s.hasher.HashToken(currentToken)); err != nil {
			return fmt.Errorf("delete other sessions: %w", err)
		}

		if err := stores.codes.DeleteByUserID(ctx, userID); err != nil {
			return fmt.Errorf("clear login codes: %w", err)
		}

		if err := stores.attempts.DeleteByUserID(ctx, userID); err != nil {
			return fmt.Errorf("clear login attempts: %w", err)
		}
		if err := stores.attempts.DeleteByPhone(ctx, newPhone); err != nil {
			return fmt.Errorf("reset new phone attempts: %w", err)
		}
		if err := stores.attempts.DeleteByPhone(ctx, oldPhone); err != nil {
			return fmt.Errorf("reset old phone attempts: %w", err)
		}

		if err := stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &userID,
			ActorRole:  auditdomain.ActorRoleFromRole(updated.Role),
			Action:     auditdomain.ActionAuthPhoneChanged,
			EntityType: auditdomain.EntityUser,
			EntityID:   &userID,
		}); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})

	if errors.Is(err, domain.ErrLoginCodeInvalid) || errors.Is(err, domain.ErrTooManyAttempts) {
		// Verification failed: record the attempt and the phone-change-failed
		// audit in a separate transaction so the rate-limit mutation survives
		// the rolled-back success path. RecordFailureAndAudit surfaces
		// ErrTooManyAttempts via outErr in place of the plain invalid-code error.
		finalErr := err
		auditEntry := auditdomain.Entry{
			ActorID:   &userID,
			ActorRole: auditdomain.ActorRoleOwner,
			Action:    auditdomain.ActionAuthPhoneChangeFailed,
		}
		if recErr := s.loginCodes.RecordFailureAndAudit(ctx, newPhone, userID, auditEntry, &finalErr); recErr != nil {
			s.logger.ErrorContext(ctx, "failed to record phone-change failed attempt", slog.String("error", sanitize.Error(recErr)))
		}
		return domain.User{}, finalErr
	}
	if err != nil {
		return domain.User{}, err
	}

	return updated, nil
}
