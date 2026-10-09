package application

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// newLogoutHarness wires a LogoutService to a fakeUoW + the shared identity
// fakes, returning the service and fakeStores for assertions.
func newLogoutHarness() (*LogoutService, *fakeStores) {
	stores := newFakeStores()
	svc := NewLogoutService(
		stores.factory(nil),
		LogoutServiceConfig{
			Hasher: fakeHasher{},
			Logger: slog.New(slog.DiscardHandler),
		},
	)
	return svc, stores
}

// testLogoutActor is the audit actor used across logout tests.
var testLogoutActor = auditdomain.Actor{Role: auditdomain.ActorRoleOwner}

// TestLogoutService_Logout_DeletesSessionByTokenHash proves Logout hashes the
// raw token, deletes the matching session through the transactional store, and
// the UoW commits.
func TestLogoutService_Logout_DeletesSessionByTokenHash(t *testing.T) {
	t.Parallel()
	svc, stores := newLogoutHarness()

	rawToken := "plain-test-value"
	tokenHash := fakeHasher{}.HashToken(rawToken)
	userID := uuid.Must(uuid.NewV7())
	stores.sessions.sessions[tokenHash] = domain.Session{UserID: userID, TokenHash: tokenHash}

	if err := svc.Logout(context.Background(), rawToken, testLogoutActor); err != nil {
		t.Fatalf("Logout error = %v, want nil", err)
	}

	if _, ok := stores.sessions.sessions[tokenHash]; ok {
		t.Fatal("session still present after Logout, want deleted")
	}
	if stores.beginner.committed != 1 {
		t.Errorf("committed = %d, want 1", stores.beginner.committed)
	}
	if stores.beginner.rolledBack != 0 {
		t.Errorf("rolledBack = %d, want 0", stores.beginner.rolledBack)
	}
}

// TestLogoutService_Logout_WrapsDeleteError proves a repository failure is
// returned and the transaction is rolled back.
func TestLogoutService_Logout_WrapsDeleteError(t *testing.T) {
	t.Parallel()
	users := newFakeUserRepo()
	codes := newFakeCodeRepo()
	attempts := newFakeAttemptRepo()
	sessions := &errorSessionRepo{err: errors.New("db down")}
	beginner := &fakeBeginner{}
	factory := NewTxStoreFactory(users, codes, attempts, sessions, newFakeGrantRepo(), nil, nil, &fakeUoW{beginner: beginner})
	svc := NewLogoutService(
		factory,
		LogoutServiceConfig{
			Hasher: fakeHasher{},
		},
	)

	err := svc.Logout(context.Background(), "any-token", testLogoutActor)
	if err == nil {
		t.Fatal("Logout error = nil, want delete error")
	}
	if !errors.Is(err, sessions.err) {
		t.Fatalf("Logout error = %v, want wrap of %v", err, sessions.err)
	}
	if beginner.committed != 0 {
		t.Errorf("committed = %d, want 0 on error", beginner.committed)
	}
	if beginner.rolledBack != 1 {
		t.Errorf("rolledBack = %d, want 1 on error", beginner.rolledBack)
	}
}

// TestLogoutService_UsesRunInTx proves Logout goes through the UoW seam: the
// session repository is bound to the transaction and the UoW commits. This is
// the core assertion of the ADR 0033 smoke test.
func TestLogoutService_UsesRunInTx(t *testing.T) {
	t.Parallel()
	users := newFakeUserRepo()
	codes := newFakeCodeRepo()
	attempts := newFakeAttemptRepo()
	sessions := &countingSessionRepo{fakeSessionRepo: newFakeSessionRepo()}
	beginner := &fakeBeginner{}
	factory := NewTxStoreFactory(users, codes, attempts, sessions, newFakeGrantRepo(), nil, nil, &fakeUoW{beginner: beginner})
	svc := NewLogoutService(
		factory,
		LogoutServiceConfig{
			Hasher: fakeHasher{},
			Logger: slog.New(slog.DiscardHandler),
		},
	)

	if err := svc.Logout(context.Background(), "token", testLogoutActor); err != nil {
		t.Fatalf("Logout error = %v, want nil", err)
	}

	if sessions.withTxCalls != 1 {
		t.Errorf("sessions.WithTx calls = %d, want 1 (must bind inside runInTx)", sessions.withTxCalls)
	}
	if beginner.committed != 1 {
		t.Errorf("committed = %d, want 1", beginner.committed)
	}
}

// TestLogoutService_RecordsAuditInTx proves Logout records the logout audit
// entry inside the same transaction that deletes the session.
func TestLogoutService_RecordsAuditInTx(t *testing.T) {
	t.Parallel()
	users := newFakeUserRepo()
	codes := newFakeCodeRepo()
	attempts := newFakeAttemptRepo()
	sessions := newFakeSessionRepo()
	audit := &recordingRecorder{}
	beginner := &fakeBeginner{}
	factory := NewTxStoreFactory(users, codes, attempts, sessions, newFakeGrantRepo(), audit, nil, &fakeUoW{beginner: beginner})
	svc := NewLogoutService(
		factory,
		LogoutServiceConfig{
			Hasher: fakeHasher{},
			Logger: slog.New(slog.DiscardHandler),
		},
	)

	userID := uuid.Must(uuid.NewV7())
	actor := auditdomain.Actor{ID: userID, Role: auditdomain.ActorRoleOwner}

	if err := svc.Logout(context.Background(), "token", actor); err != nil {
		t.Fatalf("Logout error = %v, want nil", err)
	}

	if len(audit.entries) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(audit.entries))
	}
	entry := audit.entries[0]
	if entry.Action != auditdomain.ActionAuthLogout {
		t.Errorf("audit action = %s, want %s", entry.Action, auditdomain.ActionAuthLogout)
	}
	if entry.ActorRole != auditdomain.ActorRoleOwner {
		t.Errorf("audit actor role = %s, want %s", entry.ActorRole, auditdomain.ActorRoleOwner)
	}
	if entry.EntityType != auditdomain.EntityUser {
		t.Errorf("audit entity type = %s, want %s", entry.EntityType, auditdomain.EntityUser)
	}
	if entry.ActorID == nil || *entry.ActorID != userID {
		t.Errorf("audit actor id = %v, want %v", entry.ActorID, userID)
	}
	if entry.EntityID == nil || *entry.EntityID != userID {
		t.Errorf("audit entity id = %v, want %v", entry.EntityID, userID)
	}
	if beginner.committed != 1 {
		t.Errorf("committed = %d, want 1", beginner.committed)
	}
}

// TestLogoutService_AuditFailOpen proves an audit failure does not block
// logout: the session delete still commits. This is the documented fail-open
// exception — logout must always succeed regardless of the audit write.
func TestLogoutService_AuditFailOpen(t *testing.T) {
	t.Parallel()
	users := newFakeUserRepo()
	codes := newFakeCodeRepo()
	attempts := newFakeAttemptRepo()
	sessions := newFakeSessionRepo()
	audit := &recordingRecorder{err: errors.New("audit db down")}
	beginner := &fakeBeginner{}
	factory := NewTxStoreFactory(users, codes, attempts, sessions, newFakeGrantRepo(), audit, nil, &fakeUoW{beginner: beginner})
	svc := NewLogoutService(
		factory,
		LogoutServiceConfig{
			Hasher: fakeHasher{},
			Logger: slog.New(slog.DiscardHandler),
		},
	)

	userID := uuid.Must(uuid.NewV7())
	actor := auditdomain.Actor{ID: userID, Role: auditdomain.ActorRoleOwner}

	if err := svc.Logout(context.Background(), "token", actor); err != nil {
		t.Fatalf("Logout error = %v, want nil (fail-open: audit error must not block logout)", err)
	}
	if beginner.committed != 1 {
		t.Errorf("committed = %d, want 1 (fail-open: session delete must commit despite audit error)", beginner.committed)
	}
	if beginner.rolledBack != 0 {
		t.Errorf("rolledBack = %d, want 0 (fail-open: must not roll back on audit error)", beginner.rolledBack)
	}
}

// errorSessionRepo is a SessionRepository whose DeleteByTokenHash always fails,
// so the error/rollback path can be exercised.
type errorSessionRepo struct{ err error }

func (r *errorSessionRepo) Create(context.Context, domain.Session) error { return nil }
func (r *errorSessionRepo) GetByTokenHash(context.Context, string, time.Time) (domain.Session, domain.User, error) {
	return domain.Session{}, domain.User{}, ErrNotFound
}
func (r *errorSessionRepo) Update(context.Context, domain.Session) error    { return nil }
func (r *errorSessionRepo) DeleteByTokenHash(context.Context, string) error { return r.err }
func (r *errorSessionRepo) DeleteByUserIDExcept(context.Context, uuid.UUID, string) (int64, error) {
	return 0, r.err
}
func (r *errorSessionRepo) Touch(context.Context, domain.Session) error { return nil }
func (r *errorSessionRepo) Rotate(context.Context, domain.Session, string) (bool, error) {
	return true, nil
}

func (r *errorSessionRepo) ListByUserID(_ context.Context, _ uuid.UUID, _ time.Time) ([]domain.Session, error) {
	return nil, nil
}

func (r *errorSessionRepo) GetByID(context.Context, uuid.UUID) (domain.Session, error) {
	return domain.Session{}, ErrNotFound
}

func (r *errorSessionRepo) DeleteByIDForUser(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, r.err
}
func (r *errorSessionRepo) WithTx(transaction.Tx) (SessionRepository, error) { return r, nil }
