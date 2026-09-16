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
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
)

const (
	// The minSendInterval throttles repeated code issuance for the same
	// phone+email+purpose triple. A code requested sooner is rejected with
	// ErrCodeSentTooRecently without invalidating the in-flight code.
	minSendInterval = 1 * time.Minute
	// The codeSpace is the size of the numeric code space (000000–999999).
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
//   - Send and RecordFailureAndAudit open their own runInTx: Send commits a
//     code before delivery so a delivery failure can clean up the unsent row;
//     RecordFailureAndAudit commits the rate-limit mutation and the failure
//     audit in a short transaction that survives the rolled-back success path.
//   - Verify and RecordFailure operate on a *txStores handed in by the calling
//     orchestrator (AuthenticationService, PhoneChangeService), so code
//     verification, mark-used, session issuance, and audit can share one
//     transaction. A failed verification returns ErrLoginCodeInvalid or
//     ErrTooManyAttempts after recording the attempt inside the same stores;
//     the orchestrator then rolls back the success-path writes and calls
//     RecordFailureAndAudit in a separate short transaction so the rate-limit
//     mutation survives the rollback.
type LoginCodeService struct {
	txStoreFactory
	sender LoginCodeSender
	clock  clock.Clock
	hasher TokenHasher
	logger *slog.Logger
}

// LoginCodeServiceConfig carries the non-transactional dependencies for
// LoginCodeService. The transactional repositories, audit recorder, and UoW
// live in the shared txStoreFactory passed to NewLoginCodeService.
type LoginCodeServiceConfig struct {
	CodeSender LoginCodeSender
	Clock      clock.Clock
	Hasher     TokenHasher
	Logger     *slog.Logger
}

// NewLoginCodeService creates a LoginCodeService. It embeds the shared identity
// txStoreFactory so Send runs through runInTx; Verify and RecordFailure receive
// their stores from the calling orchestrator's runInTx (ADR 0033 γ-factory).
func NewLoginCodeService(
	factory txStoreFactory,
	cfg LoginCodeServiceConfig,
) *LoginCodeService {
	if cfg.Clock == nil {
		cfg.Clock = clock.Real{}
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &LoginCodeService{
		txStoreFactory: factory,
		sender:         cfg.CodeSender,
		clock:          cfg.Clock,
		hasher:         cfg.Hasher,
		logger:         logger,
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
func (s *LoginCodeService) Send(
	ctx context.Context,
	phone domain.Phone,
	email domain.Email,
	purpose domain.LoginCodePurpose,
	userID *uuid.UUID,
) error {
	var loginCode domain.LoginCode
	var plaintextCode string

	err := s.runInTx(ctx, func(stores *txStores) error {
		var err error
		loginCode, plaintextCode, err = s.IssueInTx(ctx, stores, phone, email, purpose, userID)
		return err
	})
	if err != nil {
		return err
	}

	return s.Deliver(ctx, phone, email, purpose, loginCode, plaintextCode)
}

// IssueInTx is the transactional half of Send: it guards the phone against
// the attempt-window block, clears the way past superseded codes and the send
// throttle, and persists a freshly generated code for the triple. It returns
// the persisted code together with its plaintext for post-commit delivery.
// Orchestrators that verify another code in the same transaction (email-change
// step 2, issue #721) call it directly so verify, burn, and issue share one
// commit, and deliver the returned plaintext themselves after that commit.
func (s *LoginCodeService) IssueInTx(
	ctx context.Context,
	stores *txStores,
	phone domain.Phone,
	email domain.Email,
	purpose domain.LoginCodePurpose,
	userID *uuid.UUID,
) (domain.LoginCode, string, error) {
	now := s.clock.Now()

	if err := s.purgeAndThrottle(ctx, stores, phone, email, purpose, now); err != nil {
		return domain.LoginCode{}, "", err
	}
	return s.persistNewCode(ctx, stores, phone, email, purpose, userID, now)
}

// purgeAndThrottle clears the way for a new code inside the caller's
// transaction: a blocked phone is refused outright, expired codes are purged
// first, the send throttle then rejects a live code issued within
// minSendInterval, and only then are the remaining unused codes deleted. The
// order matters: the latest code must still be readable when the throttle is
// checked (CONTEXT.md, "Throttle отправки").
func (s *LoginCodeService) purgeAndThrottle(
	ctx context.Context,
	stores *txStores,
	phone domain.Phone,
	email domain.Email,
	purpose domain.LoginCodePurpose,
	now time.Time,
) error {
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
	if sendThrottled(latest, !errors.Is(err, ErrNotFound), now) {
		return ErrCodeSentTooRecently
	}

	if err := stores.codes.DeleteUnusedByPhoneAndEmail(ctx, phone, email, purpose); err != nil {
		return fmt.Errorf("delete unused login codes: %w", err)
	}
	return nil
}

// sendThrottled reports whether the latest stored code is still live and was
// issued within minSendInterval, so re-issuance must wait. Found is false when
// no code is stored for the triple.
func sendThrottled(latest domain.LoginCode, found bool, now time.Time) bool {
	return found && !latest.Used && latest.ExpiresAt.After(now) && now.Sub(latest.CreatedAt) < minSendInterval
}

// persistNewCode generates the plaintext code, builds the domain LoginCode for
// the triple, and saves it inside the caller's transaction.
func (s *LoginCodeService) persistNewCode(
	ctx context.Context,
	stores *txStores,
	phone domain.Phone,
	email domain.Email,
	purpose domain.LoginCodePurpose,
	userID *uuid.UUID,
	now time.Time,
) (domain.LoginCode, string, error) {
	plaintextCode, err := generateCode()
	if err != nil {
		return domain.LoginCode{}, "", fmt.Errorf("generate code: %w", err)
	}

	loginCode, err := domain.NewLoginCode(phone, email, s.hashCode(purpose, phone, email, plaintextCode), purpose, userID, now)
	if err != nil {
		return domain.LoginCode{}, "", fmt.Errorf("create login code: %w", err)
	}

	if err := stores.codes.Save(ctx, loginCode); err != nil {
		return domain.LoginCode{}, "", fmt.Errorf("save code: %w", err)
	}
	return loginCode, plaintextCode, nil
}

// Deliver sends the plaintext after the issuing transaction has committed.
// Delivery runs post-commit: the code is already persisted, so a delivery
// failure cleans up the unsent row rather than leaving a code the user never
// received. Send calls it itself; an orchestrator that issued through IssueInTx
// inside its own transaction calls it after that transaction commits.
func (s *LoginCodeService) Deliver(
	ctx context.Context,
	phone domain.Phone,
	email domain.Email,
	purpose domain.LoginCodePurpose,
	loginCode domain.LoginCode,
	plaintextCode string,
) error {
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
// code is absent, expired, used, or wrong. It returns ErrUserBlocked when the
// phone is currently blocked by the attempt window — checked first, before any
// code is read, so a blocked phone cannot be verified.
//
// Verify deliberately does NOT record the attempt-window failure: the success
// path transaction will roll back on this error, and recording inside it would
// be rolled back with it. The orchestrator records the failure via RecordFailure
// in a separate transaction so the rate-limit mutation survives (ADR 0033).
func (s *LoginCodeService) Verify(
	ctx context.Context,
	stores *txStores,
	phone domain.Phone,
	email domain.Email,
	purpose domain.LoginCodePurpose,
	code string,
) (domain.LoginCode, error) {
	if err := checkNotBlocked(ctx, stores.attempts, s.clock, phone); err != nil {
		return domain.LoginCode{}, err
	}

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

// RecordFailureAndAudit records a failed verification attempt and the
// corresponding audit entry in a single short transaction, so they outlive the
// rolled-back success path of the calling orchestrator. It is the shared deep
// method that both AuthenticationService (login) and PhoneChangeService
// (phone-change) invoke after a verification failure, deduplicating the two
// previously separate inline copies.
//
// It opens its own runInTx (not the caller's stores) so the rate-limit mutation
// and audit entry survive the rollback of the success-path transaction. Inside,
// it runs RecordFailure first so the audit reason reflects the final outcome:
// "too_many_attempts" once the threshold is reached, otherwise "invalid_code".
// The computed reason is merged into auditEntry.Context before recording.
//
// RecordFailure returns domain.ErrTooManyAttempts once the threshold is reached;
// when that happens, *outErr is updated to it so the caller surfaces the block
// in place of the plain invalid-code error. Returns only infrastructure errors;
// domain errors from RecordFailure are captured via outErr.
func (s *LoginCodeService) RecordFailureAndAudit(
	ctx context.Context,
	phone domain.Phone,
	userID uuid.UUID,
	auditEntry auditdomain.Entry,
	outErr *error,
) error {
	return s.runInTx(ctx, func(stores *txStores) error {
		if err := s.RecordFailure(ctx, stores, phone, userID); err != nil {
			if errors.Is(err, domain.ErrTooManyAttempts) {
				*outErr = err
			} else if !errors.Is(err, domain.ErrLoginCodeInvalid) {
				return fmt.Errorf("record failed attempt: %w", err)
			}
		}

		reason := "invalid_code"
		if errors.Is(*outErr, domain.ErrTooManyAttempts) {
			reason = "too_many_attempts"
		}
		if auditEntry.Context == nil {
			auditEntry.Context = map[string]any{}
		}
		auditEntry.Context["reason"] = reason

		if err := stores.audit.Record(ctx, auditEntry); err != nil {
			return fmt.Errorf("record audit: %w", err)
		}
		return nil
	})
}

// attemptDelta derives the persistence hint for AttemptRepository.Save from the
// pre- and post-mutation windows. It returns 0 (reset: write the absolute
// counter) when the window was just created or restarted after its TTL, and a
// positive delta (increment the existing counter) otherwise. The reset-vs-
// increment rule lives in domain.WasReset; this wrapper keeps call sites
// readable.
func attemptDelta(prev, next domain.AttemptWindow) int {
	if next.WasReset(prev) {
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
func checkNotBlocked(ctx context.Context, attempts AttemptRepository, clk clock.Clock, phone domain.Phone) error {
	now := clk.Now()

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
