package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// LogoutService terminates sessions.
//
// It is the first identity use case migrated to runInTx/txStores (ADR 0033,
// migration step 2) and serves as the smoke test for the Unit-of-Work apparatus
// before the more complex use cases (Profile, Authenticate, PhoneChange) follow.
// Although each method performs a single delete and does not require atomicity,
// routing it through runInTx proves the seam works end-to-end on a live use
// case: the session repository is bound to the transaction, work runs, and UoW
// commits on success or rolls back on error.
type LogoutService struct {
	txStoreFactory
	hasher TokenHasher
}

// LogoutServiceConfig carries the non-repository dependencies for LogoutService.
type LogoutServiceConfig struct {
	Hasher TokenHasher
	Audit  auditapp.Recorder
	UoW    transaction.UoW
}

// NewLogoutService creates a LogoutService. It embeds the identity
// txStoreFactory so Logout/LogoutAll run through runInTx; the repositories and
// audit recorder are shared by every identity service (ADR 0033 γ-factory).
func NewLogoutService(
	users UserRepository,
	codes LoginCodeRepository,
	attempts AttemptRepository,
	sessions SessionRepository,
	cfg LogoutServiceConfig,
) *LogoutService {
	audit := cfg.Audit
	if audit == nil {
		audit = auditapp.Noop{}
	}
	return &LogoutService{
		txStoreFactory: txStoreFactory{
			users:    users,
			codes:    codes,
			attempts: attempts,
			sessions: sessions,
			audit:    audit,
			uow:      cfg.UoW,
		},
		hasher: cfg.Hasher,
	}
}

// Logout deletes the session associated with the raw token.
func (s *LogoutService) Logout(ctx context.Context, rawToken string) error {
	tokenHash := s.hasher.HashToken(rawToken)
	return s.runInTx(ctx, func(stores *txStores) error {
		if err := stores.sessions.DeleteByTokenHash(ctx, tokenHash); err != nil {
			return fmt.Errorf("delete session: %w", err)
		}
		return nil
	})
}

// LogoutAll deletes all sessions for the given user.
func (s *LogoutService) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	return s.runInTx(ctx, func(stores *txStores) error {
		if err := stores.sessions.DeleteByUserID(ctx, userID); err != nil {
			return fmt.Errorf("delete sessions: %w", err)
		}
		return nil
	})
}
