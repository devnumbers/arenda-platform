package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
)

// LogoutService terminates sessions by logout: the current session by its
// token, or every session of the user at once. Logout keeps the fail-open
// audit policy of ADR 0020 — a recording error is never returned, unlike the
// fail-safe revocations of SessionsService.
//
// Each method performs a single delete and does not require atomicity on its
// own, but routing it through runInTx (ADR 0033) keeps the audit entry and
// the delete in one transaction: the session repository is bound to the
// transaction, work runs, and the UoW commits on success or rolls back on
// error.
type LogoutService struct {
	txStoreFactory
	hasher TokenHasher
	logger *slog.Logger
}

// LogoutServiceConfig carries the non-transactional dependencies for
// LogoutService. The transactional repositories, audit recorder, and UoW
// live in the shared txStoreFactory passed to NewLogoutService.
type LogoutServiceConfig struct {
	Hasher TokenHasher
	Logger *slog.Logger
}

// NewLogoutService creates a LogoutService. It embeds the shared identity
// txStoreFactory so Logout/LogoutAll run through runInTx; the repositories and
// audit recorder are shared by every identity service (ADR 0033 γ-factory).
func NewLogoutService(
	factory txStoreFactory,
	cfg LogoutServiceConfig,
) *LogoutService {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &LogoutService{
		txStoreFactory: factory,
		hasher:         cfg.Hasher,
		logger:         logger,
	}
}

// Logout deletes the session associated with the raw token and records the
// logout audit entry inside the same transaction.
func (s *LogoutService) Logout(ctx context.Context, rawToken string, actor auditdomain.Actor) error {
	tokenHash := s.hasher.HashToken(rawToken)
	return s.runInTx(ctx, func(stores *txStores) error {
		if err := stores.sessions.DeleteByTokenHash(ctx, tokenHash); err != nil {
			return fmt.Errorf("delete session: %w", err)
		}
		s.recordLogoutAudit(ctx, stores, actor, auditdomain.ActionAuthLogout)
		return nil
	})
}

// LogoutAll deletes all sessions for the given user and records the logout-all
// audit entry inside the same transaction.
func (s *LogoutService) LogoutAll(ctx context.Context, userID uuid.UUID, actor auditdomain.Actor) error {
	return s.runInTx(ctx, func(stores *txStores) error {
		if err := stores.sessions.DeleteByUserID(ctx, userID); err != nil {
			return fmt.Errorf("delete sessions: %w", err)
		}
		s.recordLogoutAudit(ctx, stores, actor, auditdomain.ActionAuthLogoutAll)
		return nil
	})
}

// recordLogoutAudit writes the logout audit entry inside the transaction. This
// is a deliberate fail-open exception: unlike the other identity use cases,
// where an audit failure rolls back the transaction (fail-closed), a logout
// must always succeed — the session is already deleted and the user must not be
// blocked by an audit write. The audit error is logged but never returned, so
// the UoW commits the session delete regardless of the audit outcome.
func (s *LogoutService) recordLogoutAudit(ctx context.Context, stores *txStores, actor auditdomain.Actor, action auditdomain.Action) {
	if err := stores.audit.Record(ctx, auditdomain.Entry{
		ActorID:    &actor.ID,
		ActorRole:  actor.Role,
		Action:     action,
		EntityType: auditdomain.EntityUser,
		EntityID:   &actor.ID,
	}); err != nil {
		s.logger.ErrorContext(ctx, "failed to record logout audit", slog.String("error", sanitize.Error(err)))
	}
}
