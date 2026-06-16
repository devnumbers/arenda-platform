package application

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

var (
	ErrNotFound = errors.New("not found")
)

type Clock interface {
	Now() time.Time
}

type Sender interface {
	Send(ctx context.Context, phone domain.Phone, message string) error
}

type UserRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (domain.User, error)
	GetByPhone(ctx context.Context, phone domain.Phone) (domain.User, error)
	Create(ctx context.Context, user domain.User) (domain.User, error)
	WithTx(tx transaction.Tx) UserRepository
}

type SMSCodeRepository interface {
	Save(ctx context.Context, code domain.SMSCode) error
	GetLatestByPhone(ctx context.Context, phone domain.Phone, now time.Time) (domain.SMSCode, error)
	MarkUsedByID(ctx context.Context, id uuid.UUID) error
	DeleteByID(ctx context.Context, id uuid.UUID) error
	DeleteExpiredBefore(ctx context.Context, before time.Time) error
	WithTx(tx transaction.Tx) SMSCodeRepository
}

type AttemptRepository interface {
	GetByPhone(ctx context.Context, phone domain.Phone) (domain.AttemptWindow, error)
	Save(ctx context.Context, phone domain.Phone, window domain.AttemptWindow) error
	DeleteStaleBefore(ctx context.Context, before time.Time) error
	WithTx(tx transaction.Tx) AttemptRepository
}

type SessionRepository interface {
	Create(ctx context.Context, session domain.Session) error
	GetByTokenHash(ctx context.Context, tokenHash string, now time.Time) (domain.Session, domain.User, error)
	DeleteByTokenHash(ctx context.Context, tokenHash string) error
	DeleteExpiredBefore(ctx context.Context, before time.Time) error
	WithTx(tx transaction.Tx) SessionRepository
}

type OnboardingService interface {
	SetupDefaultSubscription(ctx context.Context, tx transaction.Tx, userID uuid.UUID) error
}
