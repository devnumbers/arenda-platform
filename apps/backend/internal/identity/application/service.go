package application

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

var (
	ErrUserBlocked         = errors.New("user is temporarily blocked")
	ErrCodeSentTooRecently = errors.New("code sent too recently")
)

const minSendInterval = 1 * time.Minute
const smsCodeSpace = 1_000_000

type txBeginner interface {
	Begin(ctx context.Context) (transaction.Tx, error)
}

type AuthService struct {
	users     UserRepository
	codes     SMSCodeRepository
	attempts  AttemptRepository
	sessions  SessionRepository
	sender    Sender
	clock     Clock
	onboarding OnboardingService
	db        txBeginner
}

func NewAuthService(users UserRepository, codes SMSCodeRepository, attempts AttemptRepository, sessions SessionRepository, sender Sender, clock Clock, onboarding OnboardingService, db txBeginner) *AuthService {
	return &AuthService{
		users:      users,
		codes:      codes,
		attempts:   attempts,
		sessions:   sessions,
		sender:     sender,
		clock:      clock,
		onboarding: onboarding,
		db:         db,
	}
}

func (s *AuthService) SendCode(ctx context.Context, phone domain.Phone) error {
	now := s.clock.Now()

	window, err := s.attempts.GetByPhone(ctx, phone)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("get attempts: %w", err)
	}
	if window.Blocked(now) {
		return ErrUserBlocked
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txCodes := s.codes.WithTx(tx)

	latest, err := txCodes.GetLatestByPhone(ctx, phone, now)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("get latest code: %w", err)
	}
	if !errors.Is(err, ErrNotFound) && !latest.Used && latest.ExpiresAt.After(now) && now.Sub(latest.CreatedAt) < minSendInterval {
		_ = tx.Rollback(ctx)
		return ErrCodeSentTooRecently
	}

	code, err := generateCode()
	if err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("generate code: %w", err)
	}
	sms, err := domain.NewSMSCode(phone, code, now)
	if err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("create sms code: %w", err)
	}

	if err := txCodes.Save(ctx, sms); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("save code: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	if err := s.sender.Send(ctx, phone, fmt.Sprintf("Код подтверждения: %s", code)); err != nil {
		if delErr := s.codes.DeleteByID(ctx, sms.ID); delErr != nil {
			return fmt.Errorf("send code failed and cleanup failed: send %w, cleanup %w", err, delErr)
		}
		return fmt.Errorf("send code: %w", err)
	}
	return nil
}

func (s *AuthService) VerifyCode(ctx context.Context, phone domain.Phone, code string) (domain.RawSession, domain.User, error) {
	now := s.clock.Now()

	window, err := s.attempts.GetByPhone(ctx, phone)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return domain.RawSession{}, domain.User{}, fmt.Errorf("get attempts: %w", err)
	}
	if window.Blocked(now) {
		return domain.RawSession{}, domain.User{}, ErrUserBlocked
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.RawSession{}, domain.User{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	codes := s.codes.WithTx(tx)
	users := s.users.WithTx(tx)
	sessions := s.sessions.WithTx(tx)
	attempts := s.attempts.WithTx(tx)

	sms, err := codes.GetLatestByPhone(ctx, phone, now)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			if recErr := s.recordFailure(ctx, attempts, phone, window, now); recErr != nil {
				return domain.RawSession{}, domain.User{}, fmt.Errorf("verify code: %w", recErr)
			}
			return domain.RawSession{}, domain.User{}, domain.ErrSMSCodeInvalid
		}
		return domain.RawSession{}, domain.User{}, fmt.Errorf("get code: %w", err)
	}

	if err := sms.Verify(code, now); err != nil {
		if recErr := s.recordFailure(ctx, attempts, phone, window, now); recErr != nil {
			return domain.RawSession{}, domain.User{}, fmt.Errorf("verify code: %w", recErr)
		}
		return domain.RawSession{}, domain.User{}, err
	}

	if err := codes.MarkUsedByID(ctx, sms.ID); err != nil {
		return domain.RawSession{}, domain.User{}, fmt.Errorf("mark code used: %w", err)
	}

	user, err := users.GetByPhone(ctx, phone)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return domain.RawSession{}, domain.User{}, fmt.Errorf("get user: %w", err)
		}
		newUser, createErr := domain.NewOwner(phone)
		if createErr != nil {
			return domain.RawSession{}, domain.User{}, fmt.Errorf("create user: %w", createErr)
		}
		user, createErr = users.Create(ctx, newUser)
		if createErr != nil {
			return domain.RawSession{}, domain.User{}, fmt.Errorf("save user: %w", createErr)
		}
	}

	if err := s.onboarding.SetupDefaultSubscription(ctx, tx, user.ID); err != nil {
		return domain.RawSession{}, domain.User{}, fmt.Errorf("create subscription: %w", err)
	}

	raw, err := domain.NewSession(user.ID, now)
	if err != nil {
		return domain.RawSession{}, domain.User{}, fmt.Errorf("create session: %w", err)
	}
	if err := sessions.Create(ctx, raw.Session); err != nil {
		return domain.RawSession{}, domain.User{}, fmt.Errorf("save session: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.RawSession{}, domain.User{}, fmt.Errorf("commit tx: %w", err)
	}

	return raw, user, nil
}

func (s *AuthService) Logout(ctx context.Context, tokenHash string) error {
	return s.sessions.DeleteByTokenHash(ctx, tokenHash)
}

func (s *AuthService) Me(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return domain.User{}, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

func (s *AuthService) recordFailure(ctx context.Context, attempts AttemptRepository, phone domain.Phone, window domain.AttemptWindow, now time.Time) error {
	if err := window.RecordFailure(now); err != nil {
		// Still save the window so the block is persisted
		if saveErr := attempts.Save(ctx, phone, window); saveErr != nil {
			return fmt.Errorf("save attempts: %w", saveErr)
		}
		return err
	}
	return attempts.Save(ctx, phone, window)
}

func generateCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(smsCodeSpace))
	if err != nil {
		return "", fmt.Errorf("generate random code: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
