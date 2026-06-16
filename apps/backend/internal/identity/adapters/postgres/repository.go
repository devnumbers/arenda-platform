package postgres

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// UserRepository persists users.
type UserRepository struct {
	db postgres.DBTX
}

// NewUserRepository creates a new user repository.
func NewUserRepository(db postgres.DBTX) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *UserRepository) WithTx(tx transaction.Tx) application.UserRepository {
	return NewUserRepository(tx.(postgres.DBTX))
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	row, err := r.q().GetUserByID(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, application.ErrNotFound
		}
		return domain.User{}, err
	}
	return domain.User{
		ID:         uuidFromPgtype(row.ID),
		Phone:      domain.Phone(row.Phone),
		Role:       domain.Role(row.Role),
		Name:       pgtypeTextPtr(row.Name),
		Surname:    pgtypeTextPtr(row.Surname),
		Patronymic: pgtypeTextPtr(row.Patronymic),
		Email:      pgtypeTextPtr(row.Email),
	}, nil
}

func (r *UserRepository) GetByPhone(ctx context.Context, phone domain.Phone) (domain.User, error) {
	row, err := r.q().GetUserByPhone(ctx, phone.String())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, application.ErrNotFound
		}
		return domain.User{}, err
	}
	return domain.User{
		ID:         uuidFromPgtype(row.ID),
		Phone:      domain.Phone(row.Phone),
		Role:       domain.Role(row.Role),
		Name:       pgtypeTextPtr(row.Name),
		Surname:    pgtypeTextPtr(row.Surname),
		Patronymic: pgtypeTextPtr(row.Patronymic),
		Email:      pgtypeTextPtr(row.Email),
	}, nil
}

func (r *UserRepository) Create(ctx context.Context, user domain.User) (domain.User, error) {
	row, err := r.q().CreateUser(ctx, postgres.CreateUserParams{
		ID:    pgtype.UUID{Bytes: user.ID, Valid: true},
		Phone: user.Phone.String(),
		Role:  string(user.Role),
	})
	if err != nil {
		return domain.User{}, err
	}
	return domain.User{
		ID:         uuidFromPgtype(row.ID),
		Phone:      domain.Phone(row.Phone),
		Role:       domain.Role(row.Role),
		Name:       pgtypeTextPtr(row.Name),
		Surname:    pgtypeTextPtr(row.Surname),
		Patronymic: pgtypeTextPtr(row.Patronymic),
		Email:      pgtypeTextPtr(row.Email),
	}, nil
}

// SMSCodeRepository persists SMS codes.
type SMSCodeRepository struct {
	db postgres.DBTX
}

// NewSMSCodeRepository creates a new SMS code repository.
func NewSMSCodeRepository(db postgres.DBTX) *SMSCodeRepository {
	return &SMSCodeRepository{db: db}
}

func (r *SMSCodeRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *SMSCodeRepository) WithTx(tx transaction.Tx) application.SMSCodeRepository {
	return NewSMSCodeRepository(tx.(postgres.DBTX))
}

func (r *SMSCodeRepository) Save(ctx context.Context, code domain.SMSCode) error {
	_, err := r.q().CreateSMSCode(ctx, postgres.CreateSMSCodeParams{
		ID:        pgtype.UUID{Bytes: code.ID, Valid: true},
		Phone:     code.Phone.String(),
		CodeHash:  code.CodeHash,
		ExpiresAt: pgtype.Timestamptz{Time: code.ExpiresAt, Valid: true},
	})
	return err
}

func (r *SMSCodeRepository) GetLatestByPhone(ctx context.Context, phone domain.Phone, now time.Time) (domain.SMSCode, error) {
	row, err := r.q().GetLatestSMSCodeByPhone(ctx, postgres.GetLatestSMSCodeByPhoneParams{
		Phone:     phone.String(),
		ExpiresAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SMSCode{}, application.ErrNotFound
		}
		return domain.SMSCode{}, err
	}
	return domain.SMSCode{
		ID:        uuidFromPgtype(row.ID),
		Phone:     domain.Phone(row.Phone),
		CodeHash:  row.CodeHash,
		ExpiresAt: row.ExpiresAt.Time,
		Used:      row.Used,
		CreatedAt: row.CreatedAt.Time,
	}, nil
}

func (r *SMSCodeRepository) MarkUsedByID(ctx context.Context, id uuid.UUID) error {
	return r.q().MarkSMSCodeUsed(ctx, pgtype.UUID{Bytes: id, Valid: true})
}

func (r *SMSCodeRepository) DeleteByID(ctx context.Context, id uuid.UUID) error {
	return r.q().DeleteSMSCodeByID(ctx, pgtype.UUID{Bytes: id, Valid: true})
}

func (r *SMSCodeRepository) DeleteExpiredBefore(ctx context.Context, before time.Time) error {
	return r.q().DeleteExpiredSMSCodes(ctx, pgtype.Timestamptz{Time: before, Valid: true})
}

// AttemptRepository persists login attempt windows.
type AttemptRepository struct {
	db postgres.DBTX
}

// NewAttemptRepository creates a new attempt repository.
func NewAttemptRepository(db postgres.DBTX) *AttemptRepository {
	return &AttemptRepository{db: db}
}

func (r *AttemptRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *AttemptRepository) WithTx(tx transaction.Tx) application.AttemptRepository {
	return NewAttemptRepository(tx.(postgres.DBTX))
}

func (r *AttemptRepository) GetByPhone(ctx context.Context, phone domain.Phone) (domain.AttemptWindow, error) {
	row, err := r.q().GetLoginAttemptByPhone(ctx, phone.String())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.AttemptWindow{}, application.ErrNotFound
		}
		return domain.AttemptWindow{}, err
	}
	return domain.AttemptWindow{
		Failures:       int(row.Failures),
		FirstFailureAt: row.FirstFailureAt.Time,
		LastFailureAt:  row.LastFailureAt.Time,
	}, nil
}

func (r *AttemptRepository) Save(ctx context.Context, phone domain.Phone, window domain.AttemptWindow) error {
	failures := window.Failures
	if failures < 0 {
		failures = 0
	}
	if failures > math.MaxInt32 {
		failures = math.MaxInt32
	}
	return r.q().UpsertLoginAttempt(ctx, postgres.UpsertLoginAttemptParams{
		Phone:          phone.String(),
		Failures:       int32(failures),
		FirstFailureAt: pgtype.Timestamptz{Time: window.FirstFailureAt, Valid: true},
		LastFailureAt:  pgtype.Timestamptz{Time: window.LastFailureAt, Valid: true},
	})
}

func (r *AttemptRepository) DeleteByPhone(ctx context.Context, phone domain.Phone) error {
	return r.q().DeleteLoginAttemptByPhone(ctx, phone.String())
}

func (r *AttemptRepository) DeleteStaleBefore(ctx context.Context, before time.Time) error {
	return r.q().DeleteStaleLoginAttempts(ctx, pgtype.Timestamptz{Time: before, Valid: true})
}

// SessionRepository persists sessions.
type SessionRepository struct {
	db postgres.DBTX
}

// NewSessionRepository creates a new session repository.
func NewSessionRepository(db postgres.DBTX) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *SessionRepository) WithTx(tx transaction.Tx) application.SessionRepository {
	return NewSessionRepository(tx.(postgres.DBTX))
}

func (r *SessionRepository) Create(ctx context.Context, session domain.Session) error {
	_, err := r.q().CreateSession(ctx, postgres.CreateSessionParams{
		UserID:    pgtype.UUID{Bytes: session.UserID, Valid: true},
		TokenHash: session.TokenHash,
		ExpiresAt: pgtype.Timestamptz{Time: session.ExpiresAt, Valid: true},
	})
	return err
}

func (r *SessionRepository) DeleteByTokenHash(ctx context.Context, tokenHash string) error {
	return r.q().DeleteSessionByTokenHash(ctx, tokenHash)
}

func (r *SessionRepository) DeleteExpiredBefore(ctx context.Context, before time.Time) error {
	return r.q().DeleteExpiredSessions(ctx, pgtype.Timestamptz{Time: before, Valid: true})
}

func (r *SessionRepository) GetByTokenHash(ctx context.Context, tokenHash string, now time.Time) (domain.Session, domain.User, error) {
	row, err := r.q().GetSessionByTokenHash(ctx, postgres.GetSessionByTokenHashParams{
		TokenHash: tokenHash,
		ExpiresAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Session{}, domain.User{}, application.ErrNotFound
		}
		return domain.Session{}, domain.User{}, err
	}
	return domain.Session{
			UserID:    uuidFromPgtype(row.UserID),
			TokenHash: row.TokenHash,
			ExpiresAt: row.ExpiresAt.Time,
		}, domain.User{
			ID:         uuidFromPgtype(row.UserID),
			Phone:      domain.Phone(row.Phone),
			Role:       domain.Role(row.Role),
			Name:       pgtypeTextPtr(row.Name),
			Surname:    pgtypeTextPtr(row.Surname),
			Patronymic: pgtypeTextPtr(row.Patronymic),
			Email:      pgtypeTextPtr(row.Email),
		}, nil
}

func uuidFromPgtype(u pgtype.UUID) uuid.UUID {
	if !u.Valid {
		return uuid.UUID{}
	}
	return uuid.UUID(u.Bytes)
}

func pgtypeTextPtr(t pgtype.Text) *string {
	if !t.Valid || t.String == "" {
		return nil
	}
	return &t.String
}
