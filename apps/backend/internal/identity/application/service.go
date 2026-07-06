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
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/pgerr"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

type tokenHasher interface {
	HashToken(plaintext string) string
}

var (
	ErrUserBlocked         = errors.New("user is temporarily blocked")
	ErrCodeSentTooRecently = errors.New("code sent too recently")
	ErrPhoneAlreadyTaken   = errors.New("phone already taken")
	ErrPhoneUnchanged      = errors.New("new phone must differ from current phone")
	ErrEmailAlreadyTaken   = errors.New("email already taken")
	ErrEmailDoesNotMatch   = errors.New("email does not match the phone number")
)

const minSendInterval = 1 * time.Minute
const codeSpace = 1_000_000

type txBeginner interface {
	Begin(ctx context.Context) (transaction.Tx, error)
}

type AuthService struct {
	users         UserRepository
	codes         LoginCodeRepository
	attempts      AttemptRepository
	sessions      SessionRepository
	codeSender    LoginCodeSender
	clock         Clock
	authenticator UserAuthenticator
	publisher     EventPublisher
	db            txBeginner
	logger        *slog.Logger
	hasher        tokenHasher
}

func NewAuthService(users UserRepository, codes LoginCodeRepository, attempts AttemptRepository, sessions SessionRepository, codeSender LoginCodeSender, clock Clock, authenticator UserAuthenticator, publisher EventPublisher, db txBeginner, logger *slog.Logger, hasher tokenHasher) *AuthService {
	if logger == nil {
		logger = slog.Default()
	}
	return &AuthService{
		users:         users,
		codes:         codes,
		attempts:      attempts,
		sessions:      sessions,
		codeSender:    codeSender,
		clock:         clock,
		authenticator: authenticator,
		publisher:     publisher,
		db:            db,
		logger:        logger,
		hasher:        hasher,
	}
}

func (s *AuthService) hashCode(purpose string, phone domain.Phone, email domain.Email, code string) string {
	return s.hasher.HashToken("login_code:" + purpose + ":" + phone.String() + ":" + email.String() + ":" + code)
}

func (s *AuthService) SendCode(ctx context.Context, phone domain.Phone, email domain.Email, purpose string) error {
	if _, err := s.checkNotBlocked(ctx, phone); err != nil {
		return err
	}

	user, userErr := s.users.GetByPhone(ctx, phone)
	if userErr != nil && !errors.Is(userErr, ErrNotFound) {
		return fmt.Errorf("get user: %w", userErr)
	}

	var userID *uuid.UUID
	if userErr == nil {
		userID = &user.ID
	}

	return s.sendCode(ctx, phone, email, purpose, userID)
}

func (s *AuthService) sendCode(ctx context.Context, phone domain.Phone, email domain.Email, purpose string, userID *uuid.UUID) error {
	now := s.clock.Now()

	if _, err := s.checkNotBlocked(ctx, phone); err != nil {
		return err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := s.checkNotBlockedTx(ctx, tx, phone); err != nil {
		return err
	}

	txCodes := s.codes.WithTx(tx)
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

	loginCode, err := domain.NewLoginCode(phone, email, s.hashCode(purpose, phone, email, plaintextCode), purpose, userID, now)
	if err != nil {
		return fmt.Errorf("create login code: %w", err)
	}

	if err := txCodes.Save(ctx, loginCode); err != nil {
		if isUniqueViolation(err) {
			return ErrCodeSentTooRecently
		}
		return fmt.Errorf("save code: %w", err)
	}

	s.logger.InfoContext(ctx, "sending login code", slog.String("purpose", purpose))
	if err := s.codeSender.Send(ctx, phone, email, plaintextCode); err != nil {
		_ = tx.Rollback(ctx)
		s.logger.ErrorContext(ctx, "failed to send login code", slog.String("error", sanitize.Error(err)))
		return fmt.Errorf("send code: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	s.logger.InfoContext(ctx, "login code sent", slog.String("purpose", purpose))
	return nil
}

func (s *AuthService) VerifyLoginCode(ctx context.Context, phone domain.Phone, email domain.Email, code string) (domain.RawSession, domain.User, error) {
	now := s.clock.Now()

	if _, err := s.checkNotBlocked(ctx, phone); err != nil {
		return domain.RawSession{}, domain.User{}, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.RawSession{}, domain.User{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := s.verifyCode(ctx, tx, phone, email, code, domain.LoginCodePurposeLogin, uuid.Nil); err != nil {
		return domain.RawSession{}, domain.User{}, err
	}

	raw, user, isNewUser, err := s.authenticator.Authenticate(ctx, tx, phone, email, now)
	if err != nil {
		return domain.RawSession{}, domain.User{}, err
	}

	if err := s.attempts.WithTx(tx).DeleteByPhone(ctx, phone); err != nil {
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

func (s *AuthService) verifyCode(ctx context.Context, tx transaction.Tx, phone domain.Phone, email domain.Email, code, purpose string, failureUserID uuid.UUID) error {
	now := s.clock.Now()

	txCodes := s.codes.WithTx(tx)
	loginCode, err := txCodes.GetLatestByPhoneAndEmail(ctx, phone, email, purpose, now)
	if errors.Is(err, ErrNotFound) {
		return s.recordVerifyFailure(ctx, phone, failureUserID, now)
	}
	if err != nil {
		return fmt.Errorf("get code: %w", err)
	}

	if err := loginCode.Verify(s.hashCode(purpose, phone, email, code), now); err != nil {
		return s.recordVerifyFailure(ctx, phone, failureUserID, now)
	}

	if err := txCodes.MarkUsedByID(ctx, loginCode.ID); err != nil {
		return fmt.Errorf("mark code used: %w", err)
	}

	return nil
}

func (s *AuthService) recordVerifyFailure(ctx context.Context, phone domain.Phone, userID uuid.UUID, now time.Time) error {
	if _, err := s.attempts.IncrementFailures(ctx, phone, userID, now); err != nil {
		return s.mapAttemptError(err, "increment failures")
	}
	return domain.ErrLoginCodeInvalid
}

func (s *AuthService) mapAttemptError(err error, op string) error {
	if errors.Is(err, domain.ErrTooManyAttempts) || errors.Is(err, ErrUserBlocked) {
		return err
	}
	return fmt.Errorf("%s: %w", op, err)
}

func (s *AuthService) checkNotBlocked(ctx context.Context, phone domain.Phone) (domain.AttemptWindow, error) {
	now := s.clock.Now()

	window, err := s.attempts.GetByPhone(ctx, phone)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return domain.AttemptWindow{}, fmt.Errorf("get attempts: %w", err)
	}
	if window.Blocked(now) {
		return domain.AttemptWindow{}, ErrUserBlocked
	}
	return window, nil
}

func (s *AuthService) checkNotBlockedTx(ctx context.Context, tx transaction.Tx, phone domain.Phone) (domain.AttemptWindow, error) {
	now := s.clock.Now()

	window, err := s.attempts.WithTx(tx).GetByPhone(ctx, phone)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return domain.AttemptWindow{}, fmt.Errorf("get attempts: %w", err)
	}
	if window.Blocked(now) {
		return domain.AttemptWindow{}, ErrUserBlocked
	}
	return window, nil
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

	email, err := s.userEmail(user)
	if err != nil {
		return err
	}

	return s.sendCode(ctx, newPhone, email, domain.LoginCodePurposePhoneChange, &user.ID)
}

func (s *AuthService) ChangePhone(ctx context.Context, userID uuid.UUID, newPhone domain.Phone, code, tokenHash string) (domain.User, error) {
	if _, err := s.checkNotBlocked(ctx, newPhone); err != nil {
		return domain.User{}, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return domain.User{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	users := s.users.WithTx(tx)
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

	email, err := s.userEmail(user)
	if err != nil {
		return domain.User{}, err
	}

	if err := s.verifyCode(ctx, tx, newPhone, email, code, domain.LoginCodePurposePhoneChange, userID); err != nil {
		return domain.User{}, err
	}

	updated, err := users.UpdatePhone(ctx, userID, newPhone)
	if err != nil {
		return domain.User{}, fmt.Errorf("update phone: %w", err)
	}

	sessions := s.sessions.WithTx(tx)
	if err := sessions.DeleteByUserIDExcept(ctx, userID, tokenHash); err != nil {
		return domain.User{}, fmt.Errorf("delete other sessions: %w", err)
	}

	txCodes := s.codes.WithTx(tx)
	if err := txCodes.DeleteByUserID(ctx, userID); err != nil {
		return domain.User{}, fmt.Errorf("clear login codes: %w", err)
	}

	txAttempts := s.attempts.WithTx(tx)
	if err := txAttempts.DeleteByUserID(ctx, userID); err != nil {
		return domain.User{}, fmt.Errorf("clear login attempts: %w", err)
	}
	if err := txAttempts.DeleteByPhone(ctx, newPhone); err != nil {
		return domain.User{}, fmt.Errorf("reset new phone attempts: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.User{}, fmt.Errorf("commit tx: %w", err)
	}

	return updated, nil
}

func (s *AuthService) userEmail(user domain.User) (domain.Email, error) {
	if user.Email == nil {
		return domain.Email{}, ErrEmailDoesNotMatch
	}
	email, err := domain.EmailFrom(*user.Email)
	if err != nil {
		return domain.Email{}, fmt.Errorf("parse user email: %w", err)
	}
	return email, nil
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
