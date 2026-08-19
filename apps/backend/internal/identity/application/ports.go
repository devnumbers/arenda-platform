package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// TokenHasher hashes raw session tokens and login codes. It is implemented by
// encryption.Encryptor.
type TokenHasher interface {
	HashToken(plaintext string) string
}

// LoginCodeSender delivers a login code for the phone+email triple. Phone
// identifies the recipient (the code is bound to the triple via
// LoginCodeService.hashCode) and is reserved for a future SMS channel
// (ADR 0006); the email implementation only uses email and code.
type LoginCodeSender interface {
	Send(ctx context.Context, phone domain.Phone, email domain.Email, code string) error
}

type UserRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (domain.User, error)
	// GetByIDForUpdate acquires a row-level pessimistic lock and must only be called inside a transaction.
	GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.User, error)
	GetByPhone(ctx context.Context, phone domain.Phone) (domain.User, error)
	// GetByPhoneForUpdate acquires a row-level pessimistic lock and must only be called inside a transaction.
	GetByPhoneForUpdate(ctx context.Context, phone domain.Phone) (domain.User, error)
	GetByEmail(ctx context.Context, email domain.Email) (domain.User, error)
	Create(ctx context.Context, user domain.User) (domain.User, error)
	Update(ctx context.Context, user domain.User) (domain.User, error)
	UpdatePhone(ctx context.Context, id uuid.UUID, phone domain.Phone) (domain.User, error)
	UpdateEmailVerified(ctx context.Context, id uuid.UUID, email *domain.Email, verifiedAt *time.Time) (domain.User, error)
	WithTx(tx transaction.Tx) (UserRepository, error)
}

type LoginCodeRepository interface {
	Save(ctx context.Context, code domain.LoginCode) error
	GetLatestByPhoneAndEmail(
		ctx context.Context,
		phone domain.Phone,
		email domain.Email,
		purpose domain.LoginCodePurpose,
		now time.Time,
	) (domain.LoginCode, error)
	MarkUsedByID(ctx context.Context, id uuid.UUID) error
	DeleteByID(ctx context.Context, id uuid.UUID) error
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
	DeleteExpiredByPhoneAndEmail(
		ctx context.Context,
		phone domain.Phone,
		email domain.Email,
		purpose domain.LoginCodePurpose,
		before time.Time,
	) error
	DeleteUnusedByPhoneAndEmail(ctx context.Context, phone domain.Phone, email domain.Email, purpose domain.LoginCodePurpose) error
	WithTx(tx transaction.Tx) (LoginCodeRepository, error)
}

type AttemptRepository interface {
	GetByPhone(ctx context.Context, phone domain.Phone) (domain.AttemptWindow, error)
	// GetByPhoneForUpdate acquires a row-level pessimistic lock and must only be called inside a transaction.
	GetByPhoneForUpdate(ctx context.Context, phone domain.Phone) (domain.AttemptWindow, error)
	// Save persists the attempt window for phone. When delta > 0 the failure
	// counter is atomically incremented by delta on the database side so
	// concurrent upserts cannot lose an increment (issue #215); the window's
	// timestamps are written as absolutes. When delta <= 0 the counter is set
	// to the absolute window.Failures value — the reset path used when the
	// window is new or has expired (TTL reset).
	Save(ctx context.Context, phone domain.Phone, userID uuid.UUID, window domain.AttemptWindow, delta int) error
	DeleteByPhone(ctx context.Context, phone domain.Phone) error
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
	WithTx(tx transaction.Tx) (AttemptRepository, error)
}

type SessionRepository interface {
	Create(ctx context.Context, session domain.Session) error
	GetByTokenHash(ctx context.Context, tokenHash string, now time.Time) (domain.Session, domain.User, error)
	Update(ctx context.Context, session domain.Session) error
	DeleteByTokenHash(ctx context.Context, tokenHash string) error
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
	DeleteByUserIDExcept(ctx context.Context, userID uuid.UUID, tokenHash string) error
	WithTx(tx transaction.Tx) (SessionRepository, error)
}
