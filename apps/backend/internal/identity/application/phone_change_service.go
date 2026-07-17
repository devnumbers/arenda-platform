package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// PhoneChangeService handles phone number change for authenticated users.
type PhoneChangeService struct {
	users    UserRepository
	codes    LoginCodeRepository
	attempts AttemptRepository
	sessions SessionRepository
	sender   LoginCodeSender
	clock    clock.Clock
	db       transaction.Beginner
	hasher   TokenHasher
	audit    auditapp.Recorder
}

// PhoneChangeServiceConfig carries optional dependencies for PhoneChangeService.
type PhoneChangeServiceConfig struct {
	Sender LoginCodeSender
	Clock  clock.Clock
	DB     transaction.Beginner
	Hasher TokenHasher
	Audit  auditapp.Recorder
}

// NewPhoneChangeService creates a PhoneChangeService.
func NewPhoneChangeService(users UserRepository, codes LoginCodeRepository, attempts AttemptRepository, sessions SessionRepository, cfg PhoneChangeServiceConfig) *PhoneChangeService {
	if cfg.Clock == nil {
		cfg.Clock = clock.Real{}
	}
	audit := cfg.Audit
	if audit == nil {
		audit = auditapp.Noop{}
	}
	return &PhoneChangeService{
		users:    users,
		codes:    codes,
		attempts: attempts,
		sessions: sessions,
		sender:   cfg.Sender,
		clock:    cfg.Clock,
		db:       cfg.DB,
		hasher:   cfg.Hasher,
		audit:    audit,
	}
}

// SendChangeCode sends a verification code to the user's email to confirm a new
// phone number.
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

	flow := newLoginCodeFlow(s.codes, s.attempts, s.sender, s.clock, s.db, s.hasher, nil)
	return flow.sendCode(ctx, newPhone, email, domain.LoginCodePurposePhoneChange, &user.ID)
}

// ChangePhone verifies the code and updates the user's phone number.
// currentToken is the raw session token of the current session; it is used to
// keep the current session alive when deleting all other sessions.
func (s *PhoneChangeService) ChangePhone(ctx context.Context, userID uuid.UUID, newPhone domain.Phone, code, currentToken string) (domain.User, error) {
	if _, err := checkNotBlocked(ctx, s.attempts, s.clock, newPhone); err != nil {
		return domain.User{}, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.User{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txAttempts, err := s.attempts.WithTx(tx)
	if err != nil {
		return domain.User{}, fmt.Errorf("bind attempt repository to tx: %w", err)
	}
	if _, err := checkNotBlocked(ctx, txAttempts, s.clock, newPhone); err != nil {
		return domain.User{}, err
	}

	users, err := s.users.WithTx(tx)
	if err != nil {
		return domain.User{}, fmt.Errorf("bind user repository to tx: %w", err)
	}
	user, err := users.GetByIDForUpdate(ctx, userID)
	if err != nil {
		return domain.User{}, fmt.Errorf("get user: %w", err)
	}
	if user.Phone == newPhone {
		return domain.User{}, ErrPhoneUnchanged
	}

	existing, err := users.GetByPhoneForUpdate(ctx, newPhone)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return domain.User{}, fmt.Errorf("check phone: %w", err)
	}
	if existing.ID != uuid.Nil && existing.ID != userID {
		return domain.User{}, ErrPhoneAlreadyTaken
	}

	email, err := userEmail(user)
	if err != nil {
		return domain.User{}, err
	}

	flow := newLoginCodeFlow(s.codes, s.attempts, s.sender, s.clock, s.db, s.hasher, nil)
	loginCode, err := flow.verifyCode(ctx, tx, newPhone, email, domain.LoginCodePurposePhoneChange, code, userID)
	if err != nil {
		if errors.Is(err, domain.ErrLoginCodeInvalid) || errors.Is(err, domain.ErrTooManyAttempts) {
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return domain.User{}, fmt.Errorf("commit attempts: %w", commitErr)
			}
		}
		return domain.User{}, err
	}

	txCodes, err := s.codes.WithTx(tx)
	if err != nil {
		return domain.User{}, fmt.Errorf("bind login code repository to tx: %w", err)
	}
	if err := txCodes.MarkUsedByID(ctx, loginCode.ID); err != nil {
		return domain.User{}, fmt.Errorf("mark code used: %w", err)
	}

	txSessions, err := s.sessions.WithTx(tx)
	if err != nil {
		return domain.User{}, fmt.Errorf("bind session repository to tx: %w", err)
	}

	updated, err := users.UpdatePhone(ctx, userID, newPhone)
	if err != nil {
		return domain.User{}, fmt.Errorf("update phone: %w", err)
	}

	if err := txSessions.DeleteByUserIDExcept(ctx, userID, s.hasher.HashToken(currentToken)); err != nil {
		return domain.User{}, fmt.Errorf("delete other sessions: %w", err)
	}

	if err := txCodes.DeleteByUserID(ctx, userID); err != nil {
		return domain.User{}, fmt.Errorf("clear login codes: %w", err)
	}

	if err := txAttempts.DeleteByUserID(ctx, userID); err != nil {
		return domain.User{}, fmt.Errorf("clear login attempts: %w", err)
	}
	if err := txAttempts.DeleteByPhone(ctx, newPhone); err != nil {
		return domain.User{}, fmt.Errorf("reset new phone attempts: %w", err)
	}
	if err := txAttempts.DeleteByPhone(ctx, user.Phone); err != nil {
		return domain.User{}, fmt.Errorf("reset old phone attempts: %w", err)
	}

	if err := s.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &userID,
		ActorRole:  AuditActorRole(user.Role),
		Action:     auditdomain.ActionAuthPhoneChanged,
		EntityType: auditdomain.EntityUser,
		EntityID:   &userID,
	}); err != nil {
		return domain.User{}, fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.User{}, fmt.Errorf("commit tx: %w", err)
	}

	return updated, nil
}
