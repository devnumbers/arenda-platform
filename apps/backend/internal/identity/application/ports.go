package application

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

var (
	ErrNotFound = errors.New("not found")
)

// Clock is re-exported from the shared clock package for backwards compatibility.
type Clock = clock.Clock

type SMSSender interface {
	Send(ctx context.Context, phone domain.Phone, message string) error
}

type EmailSender interface {
	Send(ctx context.Context, email domain.Email, code string) error
}

type UserRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (domain.User, error)
	GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.User, error)
	GetByPhone(ctx context.Context, phone domain.Phone) (domain.User, error)
	GetByEmail(ctx context.Context, email string) (domain.User, error)
	GetPhoneByID(ctx context.Context, id uuid.UUID) (string, error)
	Create(ctx context.Context, user domain.User) (domain.User, error)
	Update(ctx context.Context, user domain.User) (domain.User, error)
	UpdatePhone(ctx context.Context, id uuid.UUID, phone domain.Phone) (domain.User, error)
	UpdateEmailVerified(ctx context.Context, id uuid.UUID, email string, verifiedAt *time.Time) (domain.User, error)
	WithTx(tx transaction.Tx) UserRepository
}

type LoginCodeRepository interface {
	Save(ctx context.Context, code domain.LoginCode) error
	GetLatestByPhoneAndEmail(ctx context.Context, phone domain.Phone, email domain.Email, purpose string, now time.Time) (domain.LoginCode, error)
	GetLatestByPhoneAndUserID(ctx context.Context, phone domain.Phone, purpose string, userID uuid.UUID, now time.Time) (domain.LoginCode, error)
	MarkUsedByID(ctx context.Context, id uuid.UUID) error
	DeleteByID(ctx context.Context, id uuid.UUID) error
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
	DeleteExpiredBefore(ctx context.Context, before time.Time) error
	DeleteExpiredBeforeBatch(ctx context.Context, before time.Time, batchSize int32) (int64, error)
	WithTx(tx transaction.Tx) LoginCodeRepository
}

type AttemptRepository interface {
	GetByPhone(ctx context.Context, phone domain.Phone) (domain.AttemptWindow, error)
	Save(ctx context.Context, phone domain.Phone, userID uuid.UUID, window domain.AttemptWindow) error
	IncrementFailures(ctx context.Context, phone domain.Phone, userID uuid.UUID, now time.Time) (domain.AttemptWindow, error)
	DeleteByPhone(ctx context.Context, phone domain.Phone) error
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
	DeleteStaleBefore(ctx context.Context, before time.Time) error
	DeleteStaleBeforeBatch(ctx context.Context, before time.Time, batchSize int32) (int64, error)
	WithTx(tx transaction.Tx) AttemptRepository
}

type SessionRepository interface {
	Create(ctx context.Context, session domain.Session) error
	GetByTokenHash(ctx context.Context, tokenHash string, now time.Time) (domain.Session, domain.User, error)
	Update(ctx context.Context, session domain.Session) error
	DeleteByTokenHash(ctx context.Context, tokenHash string) error
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
	DeleteByUserIDExcept(ctx context.Context, userID uuid.UUID, tokenHash string) error
	DeleteExpiredBefore(ctx context.Context, before time.Time) error
	DeleteExpiredBeforeBatch(ctx context.Context, before time.Time, batchSize int32) (int64, error)
	WithTx(tx transaction.Tx) SessionRepository
}

type OnboardingService interface {
	SetupDefaultSubscription(ctx context.Context, tx transaction.Tx, userID uuid.UUID) error
}
