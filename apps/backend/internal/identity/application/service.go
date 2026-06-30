package application

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

var (
	ErrUserBlocked          = errors.New("user is temporarily blocked")
	ErrCodeSentTooRecently  = errors.New("code sent too recently")
	ErrPhoneAlreadyTaken    = errors.New("phone already taken")
	ErrPhoneUnchanged       = errors.New("new phone must differ from current phone")
	ErrEmailAlreadyTaken    = errors.New("email already taken")
	ErrEmailDoesNotMatch    = errors.New("email does not match the phone number")
	ErrPhoneLoginDeprecated = errors.New("phone login is no longer supported")
)

const minSendInterval = 1 * time.Minute
const codeSpace = 1_000_000

type txBeginner interface {
	Begin(ctx context.Context) (transaction.Tx, error)
}

type AuthService struct {
	users       UserRepository
	codes       LoginCodeRepository
	attempts    AttemptRepository
	sessions    SessionRepository
	smsSender   SMSSender
	emailSender EmailSender
	clock       Clock
	onboarding  OnboardingService
	db          txBeginner
	logger      *slog.Logger
}

func NewAuthService(users UserRepository, codes LoginCodeRepository, attempts AttemptRepository, sessions SessionRepository, smsSender SMSSender, emailSender EmailSender, clock Clock, onboarding OnboardingService, db txBeginner, logger *slog.Logger) *AuthService {
	if logger == nil {
		logger = slog.Default()
	}
	return &AuthService{
		users:       users,
		codes:       codes,
		attempts:    attempts,
		sessions:    sessions,
		smsSender:   smsSender,
		emailSender: emailSender,
		clock:       clock,
		onboarding:  onboarding,
		db:          db,
		logger:      logger,
	}
}

func (s *AuthService) SendCode(ctx context.Context, phone domain.Phone) error {
	return ErrPhoneLoginDeprecated
}

func (s *AuthService) sendCode(ctx context.Context, phone domain.Phone, purpose string, userID *uuid.UUID) error {
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

	txAttempts := s.attempts.WithTx(tx)
	reload, err := txAttempts.GetByPhone(ctx, phone)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("get attempts: %w", err)
	}
	if reload.Blocked(now) {
		return ErrUserBlocked
	}

	txCodes := s.codes.WithTx(tx)

	latest, err := txCodes.GetLatestByPhoneAndUserID(ctx, phone, purpose, *userID, now)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("get latest code: %w", err)
	}
	if !errors.Is(err, ErrNotFound) && !latest.Used && latest.ExpiresAt.After(now) && now.Sub(latest.CreatedAt) < minSendInterval {
		return ErrCodeSentTooRecently
	}

	code, err := generateCode()
	if err != nil {
		return fmt.Errorf("generate code: %w", err)
	}
	loginCode, err := domain.NewLoginCode(phone, domain.EmailFrom(""), code, purpose, userID, now)
	if err != nil {
		return fmt.Errorf("create login code: %w", err)
	}

	if err := txCodes.Save(ctx, loginCode); err != nil {
		return fmt.Errorf("save code: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	if err := s.smsSender.Send(ctx, phone, fmt.Sprintf("Код подтверждения: %s", code)); err != nil {
		if delErr := s.codes.DeleteByID(ctx, loginCode.ID); delErr != nil {
			return fmt.Errorf("send code failed and cleanup failed: send %w, cleanup %w", err, delErr)
		}
		return fmt.Errorf("send code: %w", err)
	}
	return nil
}

func (s *AuthService) VerifyCode(ctx context.Context, phone domain.Phone, code string) (domain.RawSession, domain.User, error) {
	return domain.RawSession{}, domain.User{}, ErrPhoneLoginDeprecated
}

func (s *AuthService) SendEmailCode(ctx context.Context, phone domain.Phone, email domain.Email) error {
	now := s.clock.Now()

	window, err := s.attempts.GetByPhone(ctx, phone)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("get attempts: %w", err)
	}
	if window.Blocked(now) {
		return ErrUserBlocked
	}

	user, userErr := s.users.GetByPhone(ctx, phone)
	if userErr != nil && !errors.Is(userErr, ErrNotFound) {
		return fmt.Errorf("get user: %w", userErr)
	}
	if userErr == nil && user.Email != nil && *user.Email != email.String() {
		return ErrEmailDoesNotMatch
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txAttempts := s.attempts.WithTx(tx)
	reload, err := txAttempts.GetByPhone(ctx, phone)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("get attempts: %w", err)
	}
	if reload.Blocked(now) {
		return ErrUserBlocked
	}

	txCodes := s.codes.WithTx(tx)
	latest, err := txCodes.GetLatestByPhoneAndEmail(ctx, phone, email, domain.LoginCodePurposeLogin, now)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("get latest code: %w", err)
	}
	if !errors.Is(err, ErrNotFound) && !latest.Used && latest.ExpiresAt.After(now) && now.Sub(latest.CreatedAt) < minSendInterval {
		return ErrCodeSentTooRecently
	}

	code, err := generateCode()
	if err != nil {
		return fmt.Errorf("generate code: %w", err)
	}

	var userID *uuid.UUID
	if userErr == nil {
		userID = &user.ID
	}

	loginCode, err := domain.NewLoginCode(phone, email, code, domain.LoginCodePurposeLogin, userID, now)
	if err != nil {
		return fmt.Errorf("create login code: %w", err)
	}

	if err := txCodes.Save(ctx, loginCode); err != nil {
		return fmt.Errorf("save code: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	if err := s.emailSender.Send(ctx, email, code); err != nil {
		if delErr := s.codes.DeleteByID(ctx, loginCode.ID); delErr != nil {
			return fmt.Errorf("send code failed and cleanup failed: send %w, cleanup %w", err, delErr)
		}
		return fmt.Errorf("send code: %w", err)
	}
	return nil
}

func (s *AuthService) VerifyEmailCode(ctx context.Context, phone domain.Phone, email domain.Email, code string) (domain.RawSession, domain.User, error) {
	now := s.clock.Now()

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.RawSession{}, domain.User{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txAttempts := s.attempts.WithTx(tx)
	window, err := txAttempts.GetByPhone(ctx, phone)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return domain.RawSession{}, domain.User{}, fmt.Errorf("get attempts: %w", err)
	}

	txCodes := s.codes.WithTx(tx)
	txUsers := s.users.WithTx(tx)
	txSessions := s.sessions.WithTx(tx)

	loginCode, err := txCodes.GetLatestByPhoneAndEmail(ctx, phone, email, domain.LoginCodePurposeLogin, now)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			if recErr := s.recordFailure(ctx, txAttempts, phone, uuid.Nil, window, now); recErr != nil {
				if errors.Is(recErr, domain.ErrTooManyAttempts) || errors.Is(recErr, ErrUserBlocked) {
					return domain.RawSession{}, domain.User{}, recErr
				}
				return domain.RawSession{}, domain.User{}, fmt.Errorf("verify code: %w", recErr)
			}
			return domain.RawSession{}, domain.User{}, domain.ErrLoginCodeInvalid
		}
		return domain.RawSession{}, domain.User{}, fmt.Errorf("get code: %w", err)
	}

	if err := loginCode.Verify(code, now); err != nil {
		if recErr := s.recordFailure(ctx, txAttempts, phone, uuid.Nil, window, now); recErr != nil {
			if errors.Is(recErr, domain.ErrTooManyAttempts) || errors.Is(recErr, ErrUserBlocked) {
				return domain.RawSession{}, domain.User{}, recErr
			}
			return domain.RawSession{}, domain.User{}, fmt.Errorf("verify code: %w", recErr)
		}
		return domain.RawSession{}, domain.User{}, err
	}

	if err := txCodes.MarkUsedByID(ctx, loginCode.ID); err != nil {
		return domain.RawSession{}, domain.User{}, fmt.Errorf("mark code used: %w", err)
	}

	user, err := txUsers.GetByPhone(ctx, phone)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return domain.RawSession{}, domain.User{}, fmt.Errorf("get user: %w", err)
		}
		newUser, createErr := domain.NewOwner(phone)
		if createErr != nil {
			return domain.RawSession{}, domain.User{}, fmt.Errorf("create user: %w", createErr)
		}
		emailStr := email.String()
		newUser.Email = &emailStr
		verifiedAt := now
		newUser.EmailVerifiedAt = &verifiedAt
		user, createErr = txUsers.Create(ctx, newUser)
		if createErr != nil {
			return domain.RawSession{}, domain.User{}, fmt.Errorf("save user: %w", createErr)
		}
	} else {
		if user.Email == nil || *user.Email != email.String() || user.EmailVerifiedAt == nil {
			verifiedAt := now
			updated, updateErr := txUsers.UpdateEmailVerified(ctx, user.ID, email.String(), &verifiedAt)
			if updateErr != nil {
				return domain.RawSession{}, domain.User{}, fmt.Errorf("update user email: %w", updateErr)
			}
			user = updated
		}
	}

	if err := s.onboarding.SetupDefaultSubscription(ctx, tx, user.ID); err != nil {
		return domain.RawSession{}, domain.User{}, fmt.Errorf("create subscription: %w", err)
	}

	raw, err := domain.NewSession(user.ID, now)
	if err != nil {
		return domain.RawSession{}, domain.User{}, fmt.Errorf("create session: %w", err)
	}
	if err := txSessions.Create(ctx, raw.Session); err != nil {
		return domain.RawSession{}, domain.User{}, fmt.Errorf("save session: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.RawSession{}, domain.User{}, fmt.Errorf("commit tx: %w", err)
	}

	if err := s.attempts.DeleteByPhone(ctx, phone); err != nil {
		s.logger.ErrorContext(ctx, "failed to reset login attempts", slog.String("error", err.Error()))
	}

	return raw, user, nil
}

func (s *AuthService) Logout(ctx context.Context, tokenHash string) error {
	return s.sessions.DeleteByTokenHash(ctx, tokenHash)
}

func (s *AuthService) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	return s.sessions.DeleteByUserID(ctx, userID)
}

func (s *AuthService) Me(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return domain.User{}, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

type UpdateUserCommand struct {
	Name       domain.Optional[string]
	Surname    domain.Optional[string]
	Patronymic domain.Optional[string]
	Email      domain.Optional[string]
}

func (s *AuthService) UpdateUser(ctx context.Context, userID uuid.UUID, cmd UpdateUserCommand) (domain.User, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return domain.User{}, fmt.Errorf("get user: %w", err)
	}

	if err := user.UpdatePersonalData(cmd.Name, cmd.Surname, cmd.Patronymic, cmd.Email); err != nil {
		return domain.User{}, err
	}

	if user.Email != nil {
		existing, err := s.users.GetByEmail(ctx, *user.Email)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return domain.User{}, fmt.Errorf("check email: %w", err)
		}
		if existing.ID != uuid.Nil && existing.ID != userID {
			return domain.User{}, ErrEmailAlreadyTaken
		}
	}

	updated, err := s.users.Update(ctx, user)
	if err != nil {
		return domain.User{}, fmt.Errorf("update user: %w", err)
	}
	return updated, nil
}

func (s *AuthService) SendPhoneChangeCode(ctx context.Context, userID uuid.UUID, newPhone domain.Phone) error {
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

	return s.sendCode(ctx, newPhone, domain.LoginCodePurposePhoneChange, &userID)
}

func (s *AuthService) ChangePhone(ctx context.Context, userID uuid.UUID, newPhone domain.Phone, code, tokenHash string) (domain.User, error) {
	now := s.clock.Now()

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.User{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	users := s.users.WithTx(tx)
	txCodes := s.codes.WithTx(tx)
	txAttempts := s.attempts.WithTx(tx)
	sessions := s.sessions.WithTx(tx)

	user, err := users.GetByIDForUpdate(ctx, userID)
	if err != nil {
		return domain.User{}, fmt.Errorf("get user: %w", err)
	}
	if user.Phone == newPhone {
		return domain.User{}, ErrPhoneUnchanged
	}

	existing, err := users.GetByPhone(ctx, newPhone)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return domain.User{}, fmt.Errorf("check phone: %w", err)
	}
	if existing.ID != uuid.Nil && existing.ID != userID {
		return domain.User{}, ErrPhoneAlreadyTaken
	}

	window, err := txAttempts.GetByPhone(ctx, newPhone)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return domain.User{}, fmt.Errorf("get attempts: %w", err)
	}

	loginCode, err := txCodes.GetLatestByPhoneAndUserID(ctx, newPhone, domain.LoginCodePurposePhoneChange, userID, now)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			if recErr := s.recordFailure(ctx, txAttempts, newPhone, userID, window, now); recErr != nil {
				if errors.Is(recErr, domain.ErrTooManyAttempts) || errors.Is(recErr, ErrUserBlocked) {
					return domain.User{}, recErr
				}
				return domain.User{}, fmt.Errorf("verify code: %w", recErr)
			}
			return domain.User{}, domain.ErrLoginCodeInvalid
		}
		return domain.User{}, fmt.Errorf("get code: %w", err)
	}

	if err := loginCode.Verify(code, now); err != nil {
		if recErr := s.recordFailure(ctx, txAttempts, newPhone, userID, window, now); recErr != nil {
			if errors.Is(recErr, domain.ErrTooManyAttempts) || errors.Is(recErr, ErrUserBlocked) {
				return domain.User{}, recErr
			}
			return domain.User{}, fmt.Errorf("verify code: %w", recErr)
		}
		return domain.User{}, err
	}

	if err := txCodes.MarkUsedByID(ctx, loginCode.ID); err != nil {
		return domain.User{}, fmt.Errorf("mark code used: %w", err)
	}

	updated, err := users.UpdatePhone(ctx, userID, newPhone)
	if err != nil {
		return domain.User{}, fmt.Errorf("update phone: %w", err)
	}

	if err := sessions.DeleteByUserIDExcept(ctx, userID, tokenHash); err != nil {
		return domain.User{}, fmt.Errorf("delete other sessions: %w", err)
	}

	if err := txCodes.DeleteByUserID(ctx, userID); err != nil {
		return domain.User{}, fmt.Errorf("clear login codes: %w", err)
	}

	if err := txAttempts.DeleteByUserID(ctx, userID); err != nil {
		return domain.User{}, fmt.Errorf("clear login attempts: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.User{}, fmt.Errorf("commit tx: %w", err)
	}

	return updated, nil
}

func (s *AuthService) recordFailure(ctx context.Context, attempts AttemptRepository, phone domain.Phone, userID uuid.UUID, window domain.AttemptWindow, now time.Time) error {
	if err := window.RecordFailure(now); err != nil {
		// Still save the window so the block is persisted
		if saveErr := attempts.Save(ctx, phone, userID, window); saveErr != nil {
			return fmt.Errorf("save attempts: %w", saveErr)
		}
		return err
	}
	return attempts.Save(ctx, phone, userID, window)
}

func generateCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(codeSpace))
	if err != nil {
		return "", fmt.Errorf("generate random code: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
