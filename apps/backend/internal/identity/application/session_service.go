package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

// SessionService is the application-layer facade for session lookups, updates,
// and issuance. It wraps the SessionRepository so that transport code does not
// depend directly on persistence details.
type SessionService interface {
	Load(ctx context.Context, rawToken string, now time.Time) (domain.Session, domain.User, error)
	Update(ctx context.Context, session domain.Session) error
	// Issue finds or creates the user for the phone+email pair and opens a new
	// session for them. It runs inside the caller's transaction via stores so
	// verification, mark-used, issuance, and audit share one commit (ADR 0033,
	// migration step 4). The returned isNew flag is true only when a brand-new
	// user row was inserted; a create that races an concurrent insert and ends
	// up loading the existing row reports isNew=false. Audit and the
	// UserRegistered event stay the orchestrator's responsibility.
	Issue(
		ctx context.Context,
		stores *txStores,
		phone domain.Phone,
		email domain.Email,
		now time.Time,
	) (domain.RawSession, domain.User, bool, error)
}

type sessionService struct {
	txStoreFactory
	hasher TokenHasher
}

// NewSessionService creates a SessionService backed by the provided repository.
// The hasher (used to look up sessions by the hashed value of the raw token and
// to hash freshly issued tokens before persisting them) is carried in cfg for
// uniformity with the other identity constructors. The shared identity
// txStoreFactory is embedded so Issue can run inside the caller's runInTx.
func NewSessionService(
	factory txStoreFactory,
	cfg SessionServiceConfig,
) *sessionService {
	return &sessionService{
		txStoreFactory: factory,
		hasher:         cfg.Hasher,
	}
}

// SessionServiceConfig carries the non-transactional dependencies for
// sessionService. The transactional repositories, audit recorder, and UoW
// live in the shared txStoreFactory passed to NewSessionService.
type SessionServiceConfig struct {
	Hasher TokenHasher
}

func (s *sessionService) Load(ctx context.Context, rawToken string, now time.Time) (domain.Session, domain.User, error) {
	return s.sessions.GetByTokenHash(ctx, s.hasher.HashToken(rawToken), now)
}

func (s *sessionService) Update(ctx context.Context, session domain.Session) error {
	return s.sessions.Update(ctx, session)
}

// Issue finds or creates the user for phone, marking email verified, then opens
// and persists a new session. It operates inside the caller's transaction via
// stores. Returns the raw session token, the user, and isNew (true only when a
// new user row was inserted).
func (s *sessionService) Issue(
	ctx context.Context,
	stores *txStores,
	phone domain.Phone,
	email domain.Email,
	now time.Time,
) (domain.RawSession, domain.User, bool, error) {
	user, err := stores.users.GetByPhone(ctx, phone)
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
		newUser.VerifyEmail(email, now)
		user, createErr = stores.users.Create(ctx, newUser)
		if createErr != nil {
			return domain.RawSession{}, domain.User{}, false, fmt.Errorf("save user: %w", createErr)
		}
		if user.ID != newUser.ID {
			isNewUser = false
		}
	} else if user.Email == nil || *user.Email != email || user.EmailVerifiedAt == nil {
		updated, updateErr := stores.users.UpdateEmailVerified(ctx, user.ID, &email, &now)
		if updateErr != nil {
			return domain.RawSession{}, domain.User{}, false, fmt.Errorf("verify user email: %w", updateErr)
		}
		user = updated
	}

	raw, err := domain.NewSession(user.ID, now)
	if err != nil {
		return domain.RawSession{}, domain.User{}, false, fmt.Errorf("create session: %w", err)
	}
	raw.Session.TokenHash = s.hasher.HashToken(raw.Token)

	if err := stores.sessions.Create(ctx, raw.Session); err != nil {
		return domain.RawSession{}, domain.User{}, false, fmt.Errorf("save session: %w", err)
	}

	return raw, user, isNewUser, nil
}
