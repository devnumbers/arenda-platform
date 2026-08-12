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
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

const (
	// minSendInterval throttles repeated code issuance for the same
	// phone+email+purpose triple. A code requested sooner is rejected with
	// ErrCodeSentTooRecently without invalidating the in-flight code.
	minSendInterval = 1 * time.Minute
	// codeSpace is the size of the numeric code space (000000–999999).
	codeSpace = 1_000_000
)

// LoginCodeService is the deep module owning login-code issuance, verification,
// the attempt window, and TTL (ADR 0033, migration step 4). It consolidates the
// rule "a code is valid for LoginCodeTTL and the window admits MaxLoginFailures
// attempts" in one place, replacing the loginCodeFlow helper that previously
// lived inside AuthenticationService and was shared ad hoc with PhoneChangeService.
//
// Two transactional shapes coexist by design:
//
//   - Send opens its own runInTx: code issuance is a self-contained write that
//     commits before the code is delivered, so a delivery failure can clean up
//     the unsent code without leaving a dangling row.
//   - Verify and RecordFailure operate on a *txStores handed in by the calling
//     orchestrator (AuthenticationService, PhoneChangeService), so code
//     verification, mark-used, session issuance, and audit can share one
//     transaction. A failed verification returns ErrLoginCodeInvalid or
//     ErrTooManyAttempts after recording the attempt inside the same stores;
//     the orchestrator then rolls back the success-path writes and records the
//     failed-login audit in a separate short transaction so the rate-limit
//     mutation survives the rollback.
type LoginCodeService struct {
	txStoreFactory
	sender LoginCodeSender
	clock  clock.Clock
	hasher TokenHasher
	logger *slog.Logger
}

// LoginCodeServiceConfig carries the non-repository dependencies for LoginCodeService.
type LoginCodeServiceConfig struct {
	CodeSender LoginCodeSender
	Clock      clock.Clock
	Hasher     TokenHasher
	Logger     *slog.Logger
	Audit      auditapp.Recorder
	UoW        transaction.UoW
}

// NewLoginCodeService creates a LoginCodeService. It embeds the identity
// txStoreFactory so Send runs through runInTx; Verify and RecordFailure receive
// their stores from the calling orchestrator's runInTx (ADR 0033 γ-factory).
func NewLoginCodeService(
	users UserRepository,
	codes LoginCodeRepository,
	attempts AttemptRepository,
	sessions SessionRepository,
	cfg LoginCodeServiceConfig,
) *LoginCodeService {
	if cfg.Clock == nil {
		cfg.Clock = clock.Real{}
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	audit := cfg.Audit
	if audit == nil {
		audit = auditapp.Noop{}
	}
	return &LoginCodeService{
		txStoreFactory: txStoreFactory{
			users:    users,
			codes:    codes,
			attempts: attempts,
			sessions: sessions,
			audit:    audit,
			uow:      cfg.UoW,
		},
		sender: cfg.CodeSender,
		clock:  cfg.Clock,
		hasher: cfg.Hasher,
		logger: logger,
	}
}

// hashCode derives the stored hash for a login code. The hash covers purpose,
// phone, email, and the plaintext code so a code is only valid for the exact
// triple it was issued for.
func (s *LoginCodeService) hashCode(purpose domain.LoginCodePurpose, phone domain.Phone, email domain.Email, code string) string {
	return s.hasher.HashToken("login_code:" + purpose.String() + ":" + phone.String() + ":" + email.String() + ":" + code)
}

// Send issues a login code for the phone+email+purpose triple, persists it, and
// delivers it after the transaction commits. It enforces the not-blocked check,
// purges expired and unused prior codes, and throttles re-issuance within
// minSendInterval of a live code.
func (s *LoginCodeService) Send(ctx context.Context, phone domain.Phone, email domain.Email, purpose domain.LoginCodePurpose, userID *uuid.UUID) error {
	var loginCode domain.LoginCode
	var plaintextCode string

	err := s.runInTx(ctx, func(stores *txStores) error {
		now := s.clock.Now()

		if err := checkNotBlocked(ctx, stores.attempts, s.clock, phone); err != nil {
			return err
		}

		if err := stores.codes.DeleteExpiredByPhoneAndEmail(ctx, phone, email, purpose, now); err != nil {
			return fmt.Errorf("delete expired login codes: %w", err)
		}

		latest, err := stores.codes.GetLatestByPhoneAndEmail(ctx, phone, email, purpose, now)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("get latest code: %w", err)
		}
		if !errors.Is(err, ErrNotFound) && !latest.Used && latest.ExpiresAt.After(now) && now.Sub(latest.CreatedAt) < minSendInterval {
			return ErrCodeSentTooRecently
		}

		if err := stores.codes.DeleteUnusedByPhoneAndEmail(ctx, phone, email, purpose); err != nil {
			return fmt.Errorf("delete unused login codes: %w", err)
		}

		var genErr error
		plaintextCode, genErr = generateCode()
		if genErr != nil {
			return fmt.Errorf("generate code: %w", genErr)
		}

		loginCode, genErr = domain.NewLoginCode(phone, email, s.hashCode(purpose, phone, email, plaintextCode), purpose, userID, now)
		if genErr != nil {
			return fmt.Errorf("create login code: %w", genErr)
		}

		if err := stores.codes.Save(ctx, loginCode); err != nil {
			return fmt.Errorf("save code: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Delivery runs post-commit: the code is already persisted, so a delivery
	// failure cleans up the unsent row rather than leaving a code the user
	// never received.
	s.logger.InfoContext(ctx, "sending login code", slog.String("purpose", purpose.String()))
	if err := s.sender.Send(ctx, phone, email, plaintextCode); err != nil {
		s.logger.ErrorContext(ctx, "failed to send login code", slog.String("error", sanitize.Error(err)))
		if delErr := s.codes.DeleteByID(ctx, loginCode.ID); delErr != nil {
			s.logger.ErrorContext(ctx, "failed to delete unsent login code", slog.String("error", sanitize.Error(delErr)))
		}
		return fmt.Errorf("send code: %w", err)
	}

	s.logger.InfoContext(ctx, "login code sent", slog.String("purpose", purpose.String()))
	return nil
}

// Verify checks the code for the phone+email+purpose triple against the latest
// stored code. It operates inside the caller's transaction via stores and
// returns the verified code on success, or domain.ErrLoginCodeInvalid when the
// code is absent, expired, used, or wrong.
//
// Verify deliberately does NOT record the attempt-window failure: the success
// path transaction will roll back on this error, and recording inside it would
// be rolled back with it. The orchestrator records the failure via RecordFailure
// in a separate transaction so the rate-limit mutation survives (ADR 0033).
func (s *LoginCodeService) Verify(ctx context.Context, stores *txStores, phone domain.Phone, email domain.Email, purpose domain.LoginCodePurpose, code string) (domain.LoginCode, error) {
	now := s.clock.Now()

	loginCode, err := stores.codes.GetLatestByPhoneAndEmail(ctx, phone, email, purpose, now)
	if errors.Is(err, ErrNotFound) {
		return domain.LoginCode{}, domain.ErrLoginCodeInvalid
	}
	if err != nil {
		return domain.LoginCode{}, fmt.Errorf("get code: %w", err)
	}

	if err := loginCode.Verify(s.hashCode(purpose, phone, email, code), now); err != nil {
		return domain.LoginCode{}, domain.ErrLoginCodeInvalid
	}

	return loginCode, nil
}

// RecordFailure records a single failed attempt into the window bound to phone,
// operating inside the caller's transaction via stores. It is the canonical
// place the orchestrators call from a separate short transaction after a
// verification failure rolls back the success path, so the rate-limit mutation
// survives the rollback (ADR 0033). Returns domain.ErrTooManyAttempts once the
// window threshold is reached; otherwise returns nil. Callers treat both as
// expected outcomes and only surface infrastructure errors.
//
// The failure counter is persisted atomically: a plain increment delegates to
// an atomic database-side add (Issue #215), and a TTL reset writes the absolute
// counter. The ForUpdate lock acquired here is retained for correctness of the
// read-modify-write of the window's timestamps, but the counter itself no
// longer depends on it to avoid a lost update.
func (s *LoginCodeService) RecordFailure(ctx context.Context, stores *txStores, phone domain.Phone, userID uuid.UUID) error {
	now := s.clock.Now()
	window, err := stores.attempts.GetByPhoneForUpdate(ctx, phone)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("get attempts: %w", err)
	}
	if errors.Is(err, ErrNotFound) {
		window = domain.NewAttemptWindow(now)
	}

	// Snapshot the pre-mutation window so the repository can tell a TTL reset
	// (FirstFailureAt changed, counter restarted at 1) from a plain increment.
	// On a reset the counter must be written as an absolute; otherwise it is
	// atomically incremented by the delta, which survives concurrent upserts
	// even without a ForUpdate lock (issue #215).
	prev := window
	if recErr := window.RecordFailure(now); recErr != nil {
		// Persist the threshold-reaching failure before returning so the block
		// takes effect even though the caller treats this as an expected error.
		if saveErr := stores.attempts.Save(ctx, phone, userID, window, attemptDelta(prev, window)); saveErr != nil {
			return fmt.Errorf("save attempts: %w", saveErr)
		}
		return recErr
	}
	if err := stores.attempts.Save(ctx, phone, userID, window, attemptDelta(prev, window)); err != nil {
		return fmt.Errorf("save attempts: %w", err)
	}
	return nil
}

// attemptDelta derives the persistence hint for AttemptRepository.Save from the
// pre- and post-mutation windows. It returns 0 (reset: write the absolute
// counter) when the window was just created or restarted after its TTL, and a
// positive delta (increment the existing counter) otherwise.
func attemptDelta(prev, next domain.AttemptWindow) int {
	if !next.FirstFailureAt.Equal(prev.FirstFailureAt) {
		// A fresh or TTL-reset window writes an absolute counter.
		return 0
	}
	return next.Failures - prev.Failures
}

// userEmail returns the email of a user, or ErrEmailDoesNotMatch when the user
// has none. It is shared by PhoneChangeService, which must resolve the stored
// email before issuing or verifying a phone-change code.
func userEmail(user domain.User) (domain.Email, error) {
	if user.Email == nil {
		return domain.Email{}, ErrEmailDoesNotMatch
	}
	return *user.Email, nil
}

// checkNotBlocked reports whether the phone is currently blocked by the attempt
// window. A blocked phone yields ErrUserBlocked before any code work begins.
func checkNotBlocked(ctx context.Context, attempts AttemptRepository, clock clock.Clock, phone domain.Phone) error {
	now := clock.Now()

	window, err := attempts.GetByPhone(ctx, phone)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("get attempts: %w", err)
	}
	if window.Blocked(now) {
		return ErrUserBlocked
	}
	return nil
}

// generateCode returns a cryptographically random 6-digit zero-padded code.
func generateCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(codeSpace))
	if err != nil {
		return "", fmt.Errorf("generate random code: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
