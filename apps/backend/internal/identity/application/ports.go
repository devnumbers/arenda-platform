// Package application holds the identity use cases and ports: authentication by phone and login code, profile
// and phone-change flows, session lifecycle and logout.
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

// LoginCodeSender delivers a login code by email. Phone is part of the
// binding triple (the code is hashed together with phone+email via
// LoginCodeService.hashCode); this channel only uses email and code.
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
	// GetByEmailForUpdate acquires a row-level pessimistic lock and must only be called inside a transaction.
	GetByEmailForUpdate(ctx context.Context, email domain.Email) (domain.User, error)
	Create(ctx context.Context, user domain.User) (domain.User, error)
	Update(ctx context.Context, user domain.User) (domain.User, error)
	UpdatePhone(ctx context.Context, id uuid.UUID, phone domain.Phone) (domain.User, error)
	UpdateEmailVerified(ctx context.Context, id uuid.UUID, email *domain.Email, verifiedAt *time.Time) (domain.User, error)
	// MarkEmailVerified stamps the user's stored email as verified without
	// touching the address itself (email-change step 1: a delivered code
	// confirms the current address, #721).
	MarkEmailVerified(ctx context.Context, id uuid.UUID, verifiedAt time.Time) (domain.User, error)
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

// DeviceInfoParser parses a raw User-Agent string into the device description
// stored on a session. Parsing happens once per session, at creation.
type DeviceInfoParser interface {
	Parse(userAgent string) domain.DeviceInfo
}

// GeoResolver resolves a public client IP to a city name in the ru locale
// (fallback en) from the offline GeoIP base. A lookup that finds nothing —
// unknown, private, or missing-from-base IP — reports found=false; resolution
// is a best-effort enrichment and never fails a session operation.
type GeoResolver interface {
	ResolveCity(ctx context.Context, ip string) (city string, found bool)
}

type SessionRepository interface {
	Create(ctx context.Context, session domain.Session) error
	// GetByTokenHash resolves a live (unexpired) session by the hash of the
	// presented token. It also accepts the session's previous token hash within
	// the rotation grace window (domain.SessionRotationGrace), so in-flight
	// requests survive a token rotation. The returned session always carries
	// the canonical current TokenHash.
	GetByTokenHash(ctx context.Context, tokenHash string, now time.Time) (domain.Session, domain.User, error)
	// Update persists the sliding-window fields (expiry and last seen) by token
	// hash. Session bookkeeping beyond the sliding window goes through Touch
	// and Rotate.
	Update(ctx context.Context, session domain.Session) error
	// Touch persists one request's activity by token hash: the sliding expiry,
	// the last-seen stamp, and the client IP with its resolved city (an empty
	// city never erases a stored one).
	Touch(ctx context.Context, session domain.Session) error
	// Rotate swaps the session's token in place: newTokenHash becomes the
	// canonical hash, the old hash moves to the grace slot and RotatedAt (plus
	// the sliding fields) restart. It reports whether the swap won the race —
	// a concurrent rotation of the same session makes it a no-op.
	Rotate(ctx context.Context, session domain.Session, newTokenHash string) (rotated bool, err error)
	// ListByUserID returns the user's sessions, most recently active first.
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Session, error)
	// GetByID loads one session row regardless of expiry (revocation needs the
	// row even near its end of life).
	GetByID(ctx context.Context, id uuid.UUID) (domain.Session, error)
	DeleteByTokenHash(ctx context.Context, tokenHash string) error
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
	// DeleteByUserIDExcept removes every session of the user except the one
	// carrying tokenHash (its previous hash within the grace window also
	// protects it) and reports how many rows were removed.
	DeleteByUserIDExcept(ctx context.Context, userID uuid.UUID, tokenHash string) (int64, error)
	// DeleteByIDForUser removes the session with id owned by userID and
	// reports whether a row was removed.
	DeleteByIDForUser(ctx context.Context, id, userID uuid.UUID) (bool, error)
	WithTx(tx transaction.Tx) (SessionRepository, error)
}

// EmailChangeGrantRepository stores the one grant binding a confirmed new
// email to a user between email-change steps 2 and 3 (issue #721). The grant
// is server-side state: the client only sees the plaintext token once, the
// store keeps the hash. A user holds at most one grant — issuing a new one
// replaces the previous (DeleteByUserID + Save in the issuing transaction).
type EmailChangeGrantRepository interface {
	Save(ctx context.Context, grant domain.EmailChangeGrant) error
	// GetByUserIDForUpdate acquires a row-level pessimistic lock and must only be called inside a transaction.
	GetByUserIDForUpdate(ctx context.Context, userID uuid.UUID) (domain.EmailChangeGrant, error)
	DeleteByID(ctx context.Context, id uuid.UUID) error
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
	WithTx(tx transaction.Tx) (EmailChangeGrantRepository, error)
}
