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
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/pgerr"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

const (
	minSendInterval = 1 * time.Minute
	codeSpace       = 1_000_000
)

// AuthenticationService handles login code issuance and verification.
type AuthenticationService struct {
	users      UserRepository
	codes      LoginCodeRepository
	attempts   AttemptRepository
	sessions   SessionRepository
	codeSender LoginCodeSender
	clock      clock.Clock
	publisher  EventPublisher
	db         transaction.Beginner
	logger     *slog.Logger
	hasher     TokenHasher
}

// NewAuthenticationService creates an AuthenticationService.
func NewAuthenticationService(users UserRepository, codes LoginCodeRepository, attempts AttemptRepository, sessions SessionRepository, codeSender LoginCodeSender, clock clock.Clock, publisher EventPublisher, db transaction.Beginner, logger *slog.Logger, hasher TokenHasher) *AuthenticationService {
	if logger == nil {
		logger = slog.Default()
	}
	return &AuthenticationService{
		users:      users,
		codes:      codes,
		attempts:   attempts,
		sessions:   sessions,
		codeSender: codeSender,
		clock:      clock,
		publisher:  publisher,
		db:         db,
		logger:     logger,
		hasher:     hasher,
	}
}

// SendCode generates a login code, persists it, and sends it by email after the
// transaction commits.
func (s *AuthenticationService) SendCode(ctx context.Context, phone domain.Phone, email domain.Email, purpose domain.LoginCodePurpose) error {
	if _, err := checkNotBlocked(ctx, s.attempts, s.clock, phone); err != nil {
		return err
	}

	user, err := s.users.GetByPhone(ctx, phone)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("get user: %w", err)
	}
	if err == nil && user.Email != nil && *user.Email != email {
		return ErrEmailDoesNotMatch
	}
	var userID *uuid.UUID
	if err == nil {
		userID = &user.ID
	}

	flow := newLoginCodeFlow(s.codes, s.attempts, s.codeSender, s.clock, s.db, s.hasher, s.logger)
	return flow.sendCode(ctx, phone, email, purpose, userID)
}

// VerifyCode verifies a login code and creates a session for the user.
func (s *AuthenticationService) VerifyCode(ctx context.Context, phone domain.Phone, email domain.Email, code string) (domain.RawSession, domain.User, error) {
	now := s.clock.Now()

	if _, err := checkNotBlocked(ctx, s.attempts, s.clock, phone); err != nil {
		return domain.RawSession{}, domain.User{}, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.RawSession{}, domain.User{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txAttempts, err := s.attempts.WithTx(tx)
	if err != nil {
		return domain.RawSession{}, domain.User{}, fmt.Errorf("bind attempt repository to tx: %w", err)
	}
	if _, err := checkNotBlocked(ctx, txAttempts, s.clock, phone); err != nil {
		return domain.RawSession{}, domain.User{}, err
	}

	flow := newLoginCodeFlow(s.codes, s.attempts, s.codeSender, s.clock, s.db, s.hasher, s.logger)

	loginCode, err := flow.verifyCode(ctx, tx, phone, email, domain.LoginCodePurposeLogin, code, uuid.Nil)
	if err != nil {
		if errors.Is(err, domain.ErrLoginCodeInvalid) || errors.Is(err, domain.ErrTooManyAttempts) {
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return domain.RawSession{}, domain.User{}, fmt.Errorf("commit attempts: %w", commitErr)
			}
		}
		return domain.RawSession{}, domain.User{}, err
	}

	txCodes, err := s.codes.WithTx(tx)
	if err != nil {
		return domain.RawSession{}, domain.User{}, fmt.Errorf("bind login code repository to tx: %w", err)
	}
	if err := txCodes.MarkUsedByID(ctx, loginCode.ID); err != nil {
		return domain.RawSession{}, domain.User{}, fmt.Errorf("mark code used: %w", err)
	}

	raw, user, isNewUser, err := s.authenticate(ctx, tx, phone, email, now)
	if err != nil {
		return domain.RawSession{}, domain.User{}, err
	}

	if err := txAttempts.DeleteByPhone(ctx, phone); err != nil {
		return domain.RawSession{}, domain.User{}, fmt.Errorf("reset login attempts: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.RawSession{}, domain.User{}, fmt.Errorf("commit tx: %w", err)
	}

	if isNewUser {
		if err := s.publisher.PublishUserRegistered(ctx, UserRegistered{UserID: user.ID, Phone: phone, Email: email, At: now}); err != nil {
			s.logger.ErrorContext(ctx, "failed to publish user registered event", slog.String("error", sanitize.Error(err)))
		}
	}

	return raw, user, nil
}

func (s *AuthenticationService) authenticate(ctx context.Context, tx transaction.Tx, phone domain.Phone, email domain.Email, now time.Time) (domain.RawSession, domain.User, bool, error) {
	txUsers, err := s.users.WithTx(tx)
	if err != nil {
		return domain.RawSession{}, domain.User{}, false, fmt.Errorf("bind user repository to tx: %w", err)
	}
	txSessions, err := s.sessions.WithTx(tx)
	if err != nil {
		return domain.RawSession{}, domain.User{}, false, fmt.Errorf("bind session repository to tx: %w", err)
	}

	user, err := txUsers.GetByPhone(ctx, phone)
	isNewUser := false
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return domain.RawSession{}, domain.User{}, false, fmt.Errorf("get user: %w", err)
		}
		isNewUser = true
		newUser, createErr := domain.NewOwner(phone)
		if createErr != nil {
			return domain.RawSession{}, domain.User{}, false, fmt.Errorf("create user: %w", createErr)
		}
		newUser.Email = &email
		newUser.EmailVerifiedAt = &now
		user, createErr = txUsers.Create(ctx, newUser)
		if createErr != nil {
			return domain.RawSession{}, domain.User{}, false, fmt.Errorf("save user: %w", createErr)
		}
		if user.ID != newUser.ID {
			isNewUser = false
		}
	} else {
		if user.Email == nil || *user.Email != email || user.EmailVerifiedAt == nil {
			updated, updateErr := txUsers.UpdateEmailVerified(ctx, user.ID, &email, &now)
			if updateErr != nil {
				return domain.RawSession{}, domain.User{}, false, fmt.Errorf("verify user email: %w", updateErr)
			}
			user = updated
		}
	}

	raw, err := domain.NewSession(user.ID, now)
	if err != nil {
		return domain.RawSession{}, domain.User{}, false, fmt.Errorf("create session: %w", err)
	}
	raw.Session.TokenHash = s.hasher.HashToken(raw.Token)

	if err := txSessions.Create(ctx, raw.Session); err != nil {
		return domain.RawSession{}, domain.User{}, false, fmt.Errorf("save session: %w", err)
	}

	return raw, user, isNewUser, nil
}

// loginCodeFlow contains the shared login-code logic used by AuthenticationService
// and PhoneChangeService.
type loginCodeFlow struct {
	codes    LoginCodeRepository
	attempts AttemptRepository
	sender   LoginCodeSender
	clock    clock.Clock
	db       transaction.Beginner
	hasher   TokenHasher
	logger   *slog.Logger
}

func newLoginCodeFlow(codes LoginCodeRepository, attempts AttemptRepository, sender LoginCodeSender, clock clock.Clock, db transaction.Beginner, hasher TokenHasher, logger *slog.Logger) loginCodeFlow {
	return loginCodeFlow{
		codes:    codes,
		attempts: attempts,
		sender:   sender,
		clock:    clock,
		db:       db,
		hasher:   hasher,
		logger:   logger,
	}
}

func (f *loginCodeFlow) hashCode(purpose domain.LoginCodePurpose, phone domain.Phone, email domain.Email, code string) string {
	return f.hasher.HashToken("login_code:" + purpose.String() + ":" + phone.String() + ":" + email.String() + ":" + code)
}

func (f *loginCodeFlow) sendCode(ctx context.Context, phone domain.Phone, email domain.Email, purpose domain.LoginCodePurpose, userID *uuid.UUID) error {
	now := f.clock.Now()

	tx, err := f.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txAttempts, err := f.attempts.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind attempt repository to tx: %w", err)
	}
	if _, err := checkNotBlocked(ctx, txAttempts, f.clock, phone); err != nil {
		return err
	}

	txCodes, err := f.codes.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind login code repository to tx: %w", err)
	}
	if err := txCodes.DeleteExpiredByPhoneAndEmail(ctx, phone, email, purpose, now); err != nil {
		return fmt.Errorf("delete expired login codes: %w", err)
	}

	latest, err := txCodes.GetLatestByPhoneAndEmail(ctx, phone, email, purpose, now)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("get latest code: %w", err)
	}
	if !errors.Is(err, ErrNotFound) && !latest.Used && latest.ExpiresAt.After(now) && now.Sub(latest.CreatedAt) < minSendInterval {
		return ErrCodeSentTooRecently
	}

	if err := txCodes.DeleteUnusedByPhoneAndEmail(ctx, phone, email, purpose); err != nil {
		return fmt.Errorf("delete unused login codes: %w", err)
	}

	plaintextCode, err := generateCode()
	if err != nil {
		return fmt.Errorf("generate code: %w", err)
	}

	loginCode, err := domain.NewLoginCode(phone, email, f.hashCode(purpose, phone, email, plaintextCode), purpose, userID, now)
	if err != nil {
		return fmt.Errorf("create login code: %w", err)
	}

	if err := txCodes.Save(ctx, loginCode); err != nil {
		if isUniqueViolation(err) {
			return ErrCodeSentTooRecently
		}
		return fmt.Errorf("save code: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	f.logger.InfoContext(ctx, "sending login code", slog.String("purpose", purpose.String()))
	if err := f.sender.Send(ctx, phone, email, plaintextCode); err != nil {
		f.logger.ErrorContext(ctx, "failed to send login code", slog.String("error", sanitize.Error(err)))
		if delErr := f.codes.DeleteByID(ctx, loginCode.ID); delErr != nil {
			f.logger.ErrorContext(ctx, "failed to delete unsent login code", slog.String("error", sanitize.Error(delErr)))
		}
		return fmt.Errorf("send code: %w", err)
	}

	f.logger.InfoContext(ctx, "login code sent", slog.String("purpose", purpose.String()))
	return nil
}

func (f *loginCodeFlow) verifyCode(ctx context.Context, tx transaction.Tx, phone domain.Phone, email domain.Email, purpose domain.LoginCodePurpose, code string, failureUserID uuid.UUID) (domain.LoginCode, error) {
	now := f.clock.Now()
	txCodes, err := f.codes.WithTx(tx)
	if err != nil {
		return domain.LoginCode{}, fmt.Errorf("bind login code repository to tx: %w", err)
	}
	txAttempts, err := f.attempts.WithTx(tx)
	if err != nil {
		return domain.LoginCode{}, fmt.Errorf("bind attempt repository to tx: %w", err)
	}

	loginCode, err := txCodes.GetLatestByPhoneAndEmail(ctx, phone, email, purpose, now)
	if errors.Is(err, ErrNotFound) {
		return domain.LoginCode{}, f.recordVerifyFailure(ctx, txAttempts, phone, failureUserID, now)
	}
	if err != nil {
		return domain.LoginCode{}, fmt.Errorf("get code: %w", err)
	}

	if err := loginCode.Verify(f.hashCode(purpose, phone, email, code), now); err != nil {
		return domain.LoginCode{}, f.recordVerifyFailure(ctx, txAttempts, phone, failureUserID, now)
	}

	return loginCode, nil
}

func (f *loginCodeFlow) recordVerifyFailure(ctx context.Context, attempts AttemptRepository, phone domain.Phone, userID uuid.UUID, now time.Time) error {
	window, err := attempts.GetByPhoneForUpdate(ctx, phone)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("get attempts: %w", err)
	}
	if errors.Is(err, ErrNotFound) {
		window = domain.NewAttemptWindow(now)
	}

	recErr := window.RecordFailure(now)
	if err := attempts.Save(ctx, phone, userID, window); err != nil {
		return fmt.Errorf("save attempts: %w", err)
	}
	if recErr != nil {
		return recErr
	}
	return domain.ErrLoginCodeInvalid
}

func checkNotBlocked(ctx context.Context, attempts AttemptRepository, clock clock.Clock, phone domain.Phone) (domain.AttemptWindow, error) {
	now := clock.Now()

	window, err := attempts.GetByPhone(ctx, phone)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return domain.AttemptWindow{}, fmt.Errorf("get attempts: %w", err)
	}
	if window.Blocked(now) {
		return domain.AttemptWindow{}, ErrUserBlocked
	}
	return window, nil
}

func userEmail(user domain.User) (domain.Email, error) {
	if user.Email == nil {
		return domain.Email{}, ErrEmailDoesNotMatch
	}
	return *user.Email, nil
}

func generateCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(codeSpace))
	if err != nil {
		return "", fmt.Errorf("generate random code: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// isUniqueViolation reports whether err is a PostgreSQL unique violation.
// It is used to convert concurrent code creation races into a user-friendly
// rate-limit response.
func isUniqueViolation(err error) bool {
	return pgerr.IsUniqueViolation(err)
}
