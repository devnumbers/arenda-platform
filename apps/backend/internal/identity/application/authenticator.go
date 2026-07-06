package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// UserAuthenticator encapsulates the invariant: given a verified phone/email pair,
// return an authenticated session and the corresponding user.
type UserAuthenticator interface {
	Authenticate(ctx context.Context, tx transaction.Tx, phone domain.Phone, email domain.Email, now time.Time) (domain.RawSession, domain.User, bool, error)
}

type userAuthenticator struct {
	users    UserRepository
	sessions SessionRepository
	clock    Clock
}

func NewUserAuthenticator(users UserRepository, sessions SessionRepository, clock Clock) UserAuthenticator {
	return &userAuthenticator{users: users, sessions: sessions, clock: clock}
}

func (a *userAuthenticator) Authenticate(ctx context.Context, tx transaction.Tx, phone domain.Phone, email domain.Email, now time.Time) (domain.RawSession, domain.User, bool, error) {
	txUsers := a.users.WithTx(tx)
	txSessions := a.sessions.WithTx(tx)

	user, err := txUsers.GetByPhone(ctx, phone)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return domain.RawSession{}, domain.User{}, false, fmt.Errorf("get user: %w", err)
		}
		newUser, createErr := domain.NewOwner(phone)
		if createErr != nil {
			return domain.RawSession{}, domain.User{}, false, fmt.Errorf("create user: %w", createErr)
		}
		emailStr := email.String()
		newUser.Email = &emailStr
		verifiedAt := now
		newUser.EmailVerifiedAt = &verifiedAt
		user, createErr = txUsers.Create(ctx, newUser)
		if createErr != nil {
			return domain.RawSession{}, domain.User{}, false, fmt.Errorf("save user: %w", createErr)
		}
		return a.createSession(ctx, txSessions, user, true, now)
	}

	if user.Email == nil {
		return domain.RawSession{}, domain.User{}, false, ErrEmailDoesNotMatch
	}
	if *user.Email != email.String() {
		return domain.RawSession{}, domain.User{}, false, ErrEmailDoesNotMatch
	}
	if user.EmailVerifiedAt == nil {
		verifiedAt := now
		updated, updateErr := txUsers.UpdateEmailVerified(ctx, user.ID, email.String(), &verifiedAt)
		if updateErr != nil {
			return domain.RawSession{}, domain.User{}, false, fmt.Errorf("update user email: %w", updateErr)
		}
		user = updated
	}

	return a.createSession(ctx, txSessions, user, false, now)
}

func (a *userAuthenticator) createSession(ctx context.Context, txSessions SessionRepository, user domain.User, isNewUser bool, now time.Time) (domain.RawSession, domain.User, bool, error) {
	raw, err := domain.NewSession(user.ID, now)
	if err != nil {
		return domain.RawSession{}, domain.User{}, false, fmt.Errorf("create session: %w", err)
	}
	if err := txSessions.Create(ctx, raw.Session); err != nil {
		return domain.RawSession{}, domain.User{}, false, fmt.Errorf("save session: %w", err)
	}
	return raw, user, isNewUser, nil
}
