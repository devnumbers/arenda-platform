//go:build integration

package application_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	auditpg "github.com/nambers/arenda-planform/apps/backend/internal/audit/adapters/postgres"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/adapters/postgres"
	identityapp "github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	pgdb "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/encryption"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// integrationBaseTime anchors every fake-clock advance against a fixed instant
// so sliding-window and TTL behaviour is deterministic across tests.
var integrationBaseTime = time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)

// mutableClock is a fake clock.Clock whose Now can be advanced mid-test. It is
// shared by every service in an integration harness so that the attempt window,
// login-code TTL, and session sliding window all observe the same fake time.
type mutableClock struct{ now time.Time }

func (c *mutableClock) Now() time.Time { return c.now }

func (c *mutableClock) advance(d time.Duration) { c.now = c.now.Add(d) }

// captureSender is a LoginCodeSender that records every delivered code instead
// of sending email, so a test can feed the plaintext code back into VerifyCode.
type captureSender struct {
	codes []sentLoginCode
}

type sentLoginCode struct {
	phone domain.Phone
	email domain.Email
	code  string
}

func (s *captureSender) Send(_ context.Context, phone domain.Phone, email domain.Email, code string) error {
	s.codes = append(s.codes, sentLoginCode{phone: phone, email: email, code: code})
	return nil
}

// lastCode returns the most recently delivered code, failing the test if none
// was captured.
func (s *captureSender) lastCode(t *testing.T) string {
	t.Helper()
	if len(s.codes) == 0 {
		t.Fatal("captureSender: no login code was delivered")
	}
	return s.codes[len(s.codes)-1].code
}

// capturePublisher is an EventPublisher that records UserRegistered events so a
// test can assert registration produced the domain event.
type capturePublisher struct {
	registered []identityapp.UserRegistered
}

func (p *capturePublisher) PublishUserRegistered(_ context.Context, event identityapp.UserRegistered) error {
	p.registered = append(p.registered, event)
	return nil
}

// captureRescheduler is a sharedtz.ReminderRescheduler that records calls so a
// test can assert the profile service triggered a post-commit reschedule.
type captureRescheduler struct {
	calls []rescheduleCall
}

type rescheduleCall struct {
	userID uuid.UUID
	oldTZ  string
	newTZ  string
}

func (r *captureRescheduler) RescheduleForTimezoneChange(_ context.Context, userID uuid.UUID, oldTZ, newTZ string) error {
	r.calls = append(r.calls, rescheduleCall{userID: userID, oldTZ: oldTZ, newTZ: newTZ})
	return nil
}

// integrationHarness wires every identity service to real PostgreSQL
// repositories through a postgres-backed Unit-of-Work, sharing one fake clock,
// one noop encryptor, and capture fakes for the email sender, event publisher,
// and reminder rescheduler. Each test builds a fresh harness over a clean
// (truncated) database via testdb.Setup.
type integrationHarness struct {
	t           *testing.T
	pool        *pgxpool.Pool
	enc         encryption.Encryptor
	clock       *mutableClock
	sender      *captureSender
	publisher   *capturePublisher
	rescheduler *captureRescheduler

	users    *postgres.UserRepository
	codes    *postgres.LoginCodeRepository
	attempts *postgres.AttemptRepository
	sessions *postgres.SessionRepository
	uow      transaction.UoW
	audit    auditapp.Recorder

	auth       *identityapp.AuthenticationService
	phone      *identityapp.PhoneChangeService
	profile    *identityapp.ProfileService
	logout     *identityapp.LogoutService
	sessionsvc identityapp.SessionService
}

// newIntegrationHarness builds a fresh harness over a clean database. The
// returned clock starts at integrationBaseTime; advance it with h.clock.advance.
func newIntegrationHarness(t *testing.T) *integrationHarness {
	t.Helper()

	pool := testdb.Setup(t)
	enc, err := encryption.NewEncryptor("")
	if err != nil {
		t.Fatalf("create noop encryptor: %v", err)
	}

	clk := &mutableClock{now: integrationBaseTime}
	sender := &captureSender{}
	publisher := &capturePublisher{}
	rescheduler := &captureRescheduler{}
	logger := slog.New(slog.DiscardHandler)

	users := postgres.NewUserRepository(pool, enc)
	codes := postgres.NewLoginCodeRepository(pool, enc)
	attempts := postgres.NewAttemptRepository(pool, enc)
	sessions := postgres.NewSessionRepository(pool, enc)
	uow := pgdb.NewUoW(pool, logger)
	auditWriter := auditpg.NewWriter(pool)
	audit := auditapp.NewService(auditWriter, clk)

	factory := identityapp.NewTxStoreFactory(users, codes, attempts, sessions, audit, uow)

	loginCodes := identityapp.NewLoginCodeService(factory, identityapp.LoginCodeServiceConfig{
		CodeSender: sender,
		Clock:      clk,
		Hasher:     enc,
		Logger:     logger,
	})
	sessionSvc := identityapp.NewSessionService(factory, identityapp.SessionServiceConfig{
		Hasher: enc,
	})

	return &integrationHarness{
		t:           t,
		pool:        pool,
		enc:         enc,
		clock:       clk,
		sender:      sender,
		publisher:   publisher,
		rescheduler: rescheduler,
		users:       users,
		codes:       codes,
		attempts:    attempts,
		sessions:    sessions,
		uow:         uow,
		audit:       audit,
		auth: identityapp.NewAuthenticationService(factory, identityapp.AuthenticationServiceConfig{
			LoginCodes: loginCodes,
			Sessions:   sessionSvc,
			Publisher:  publisher,
			Clock:      clk,
			Logger:     logger,
		}),
		phone: identityapp.NewPhoneChangeService(factory, identityapp.PhoneChangeServiceConfig{
			LoginCodes: loginCodes,
			Clock:      clk,
			Hasher:     enc,
			Logger:     logger,
		}),
		profile: identityapp.NewProfileService(factory, identityapp.ProfileServiceConfig{
			ReminderRescheduler: rescheduler,
		}),
		logout: identityapp.NewLogoutService(factory, identityapp.LogoutServiceConfig{
			Hasher: enc,
		}),
		sessionsvc: sessionSvc,
	}
}

// ctx returns a background context for the harness.
func (h *integrationHarness) ctx() context.Context { return context.Background() }

// mustPhone parses a raw phone, failing the test on error.
func mustPhone(t *testing.T, raw string) domain.Phone {
	t.Helper()
	phone, err := domain.NewPhone(raw)
	if err != nil {
		t.Fatalf("parse phone %q: %v", raw, err)
	}
	return phone
}

// mustEmail parses a raw email, failing the test on error.
func mustEmail(t *testing.T, raw string) domain.Email {
	t.Helper()
	email, err := domain.NewEmail(raw)
	if err != nil {
		t.Fatalf("parse email %q: %v", raw, err)
	}
	return email
}

// registerAndLogin is a convenience that drives the full send → verify happy
// path for a brand-new phone+email and returns the raw session token and user.
// It fails the test if any step errors.
func (h *integrationHarness) registerAndLogin(t *testing.T, phone domain.Phone, email domain.Email) (string, domain.User) {
	t.Helper()
	ctx := h.ctx()
	if err := h.auth.SendCode(ctx, phone, email, domain.LoginCodePurposeLogin); err != nil {
		t.Fatalf("SendCode: %v", err)
	}
	code := h.sender.lastCode(t)
	raw, user, err := h.auth.VerifyCode(ctx, phone, &email, code)
	if err != nil {
		t.Fatalf("VerifyCode: %v", err)
	}
	if raw.Token == "" {
		t.Fatal("VerifyCode returned empty token")
	}
	return raw.Token, user
}

// countSessionsForUser runs a raw count query against the sessions table so a
// test can assert how many sessions a user currently owns.
func (h *integrationHarness) countSessionsForUser(t *testing.T, userID uuid.UUID) int {
	t.Helper()
	ctx := h.ctx()
	var n int
	if err := h.pool.QueryRow(ctx,
		"SELECT count(*) FROM sessions WHERE user_id = $1", userID,
	).Scan(&n); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	return n
}

// countFailedAttempts returns the recorded failure count for a phone, or 0 when
// no attempt window row exists yet. It reads through the real AttemptRepository
// so the assertion uses the same mapping the services rely on.
func (h *integrationHarness) countFailedAttempts(t *testing.T, phone domain.Phone) int {
	t.Helper()
	window, err := h.attempts.GetByPhone(h.ctx(), phone)
	if err != nil {
		if errors.Is(err, identityapp.ErrNotFound) {
			return 0
		}
		t.Fatalf("get attempt window: %v", err)
	}
	return window.Failures
}

// auditActionExists reports whether at least one audit_log row with the given
// action exists, proving the recorder wrote the entry through the real UoW.
func (h *integrationHarness) auditActionExists(t *testing.T, action string) bool {
	t.Helper()
	ctx := h.ctx()
	var n int
	if err := h.pool.QueryRow(ctx,
		"SELECT count(*) FROM audit_log WHERE action = $1", action,
	).Scan(&n); err != nil {
		t.Fatalf("count audit %q: %v", action, err)
	}
	return n > 0
}

// seedVerifiedUser inserts a verified owner directly through the user repository
// so a test can exercise the login (not registration) path. The email is marked
// verified at the harness's current fake-clock time.
func (h *integrationHarness) seedVerifiedUser(t *testing.T, phone domain.Phone, email domain.Email) domain.User {
	t.Helper()
	ctx := h.ctx()
	user, err := domain.NewOwner(phone)
	if err != nil {
		t.Fatalf("new owner: %v", err)
	}
	user.Email = &email
	now := h.clock.Now()
	user.EmailVerifiedAt = &now
	created, err := h.users.Create(ctx, user)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return created
}

// userIDForPhone resolves the user ID for a phone, failing the test if the row
// is absent. It is a convenience for tests that need the user ID after seeding.
func (h *integrationHarness) userIDForPhone(t *testing.T, phone domain.Phone) uuid.UUID {
	t.Helper()
	ctx := h.ctx()
	user, err := h.users.GetByPhone(ctx, phone)
	if err != nil {
		t.Fatalf("GetByPhone %s: %v", phone, err)
	}
	return user.ID
}
