package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/encryption"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// UserRepository persists users.
type UserRepository struct {
	db  postgres.DBTX
	enc encryption.Encryptor
}

// NewUserRepository creates a new user repository.
func NewUserRepository(db postgres.DBTX, enc encryption.Encryptor) *UserRepository {
	return &UserRepository{db: db, enc: enc}
}

func (r *UserRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *UserRepository) WithTx(tx transaction.Tx) application.UserRepository {
	return NewUserRepository(tx.(postgres.DBTX), r.enc)
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	row, err := r.q().GetUserByID(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, application.ErrNotFound
		}
		return domain.User{}, err
	}
	return r.mapUser(ctx, row)
}

func (r *UserRepository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.User, error) {
	row, err := r.q().GetUserByIDForUpdate(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, application.ErrNotFound
		}
		return domain.User{}, err
	}
	return r.mapUser(ctx, row)
}

func (r *UserRepository) GetPhoneByID(ctx context.Context, id uuid.UUID) (domain.Phone, error) {
	row, err := r.q().GetUserPhoneByID(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Phone{}, application.ErrNotFound
		}
		return domain.Phone{}, err
	}
	phone, err := decryptPhone(ctx, r.enc, row.Phone, row.PhoneEncrypted)
	if err != nil {
		return domain.Phone{}, err
	}
	parsed, err := domain.NewPhone(phone)
	if err != nil {
		return domain.Phone{}, err
	}
	return parsed, nil
}

func (r *UserRepository) GetByPhone(ctx context.Context, phone domain.Phone) (domain.User, error) {
	encryptedPhone, err := encryptPhone(ctx, r.enc, phone.String())
	if err != nil {
		return domain.User{}, err
	}
	row, err := r.q().GetUserByPhone(ctx, encryptedPhone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, application.ErrNotFound
		}
		return domain.User{}, err
	}
	return r.mapUser(ctx, row)
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	row, err := r.q().GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, application.ErrNotFound
		}
		return domain.User{}, err
	}
	return r.mapUser(ctx, row)
}

func (r *UserRepository) Create(ctx context.Context, user domain.User) (domain.User, error) {
	encryptedPhone, err := encryptPhone(ctx, r.enc, user.Phone.String())
	if err != nil {
		return domain.User{}, err
	}
	row, err := r.q().CreateUser(ctx, postgres.CreateUserParams{
		ID:              pgconv.UUIDToPgtype(user.ID),
		Phone:           encryptedPhone,
		Role:            string(user.Role),
		PhoneEncrypted:  !r.enc.IsNoop(),
		Email:           pgconv.StringPtrToPgtype(user.Email),
		EmailVerifiedAt: pgconv.TimePtrToPgtype(user.EmailVerifiedAt),
	})
	if err != nil {
		return domain.User{}, err
	}
	return r.mapUser(ctx, postgres.User{
		ID:              row.ID,
		Phone:           row.Phone,
		Role:            row.Role,
		Name:            row.Name,
		Surname:         row.Surname,
		Patronymic:      row.Patronymic,
		Email:           row.Email,
		EmailVerifiedAt: row.EmailVerifiedAt,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		PhoneEncrypted:  row.PhoneEncrypted,
	})
}

func (r *UserRepository) Update(ctx context.Context, user domain.User) (domain.User, error) {
	row, err := r.q().UpdateUser(ctx, postgres.UpdateUserParams{
		ID:         pgconv.UUIDToPgtype(user.ID),
		Name:       pgconv.StringPtrToPgtype(user.Name),
		Surname:    pgconv.StringPtrToPgtype(user.Surname),
		Patronymic: pgconv.StringPtrToPgtype(user.Patronymic),
		Email:      pgconv.StringPtrToPgtype(user.Email),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, application.ErrNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return domain.User{}, application.ErrEmailAlreadyTaken
		}
		return domain.User{}, err
	}
	return r.mapUser(ctx, row)
}

func (r *UserRepository) UpdatePhone(ctx context.Context, id uuid.UUID, phone domain.Phone) (domain.User, error) {
	encryptedPhone, err := encryptPhone(ctx, r.enc, phone.String())
	if err != nil {
		return domain.User{}, err
	}
	row, err := r.q().UpdateUserPhone(ctx, postgres.UpdateUserPhoneParams{
		ID:             pgconv.UUIDToPgtype(id),
		Phone:          encryptedPhone,
		PhoneEncrypted: !r.enc.IsNoop(),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, application.ErrNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return domain.User{}, application.ErrPhoneAlreadyTaken
		}
		return domain.User{}, err
	}
	return r.mapUser(ctx, row)
}

func (r *UserRepository) UpdateEmailVerified(ctx context.Context, id uuid.UUID, email string, verifiedAt *time.Time) (domain.User, error) {
	row, err := r.q().UpdateUserEmailVerified(ctx, postgres.UpdateUserEmailVerifiedParams{
		ID:              pgconv.UUIDToPgtype(id),
		Email:           pgconv.StringPtrToPgtype(&email),
		EmailVerifiedAt: pgconv.TimePtrToPgtype(verifiedAt),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, application.ErrNotFound
		}
		return domain.User{}, err
	}
	return r.mapUser(ctx, postgres.User{
		ID:              row.ID,
		Phone:           row.Phone,
		Role:            row.Role,
		Name:            row.Name,
		Surname:         row.Surname,
		Patronymic:      row.Patronymic,
		Email:           row.Email,
		EmailVerifiedAt: row.EmailVerifiedAt,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		PhoneEncrypted:  row.PhoneEncrypted,
	})
}

func (r *UserRepository) mapUser(ctx context.Context, row postgres.User) (domain.User, error) {
	phone, err := decryptPhone(ctx, r.enc, row.Phone, row.PhoneEncrypted)
	if err != nil {
		return domain.User{}, err
	}
	return domain.User{
		ID:              pgconv.UUIDFromPgtype(row.ID),
		Phone:           domain.PhoneFrom(phone),
		Role:            domain.Role(row.Role),
		Name:            pgconv.TextToPtrString(row.Name),
		Surname:         pgconv.TextToPtrString(row.Surname),
		Patronymic:      pgconv.TextToPtrString(row.Patronymic),
		Email:           pgconv.TextToPtrString(row.Email),
		EmailVerifiedAt: pgconv.TimestamptzToPtrTime(row.EmailVerifiedAt),
	}, nil
}

// LoginCodeRepository persists login codes.
type LoginCodeRepository struct {
	db  postgres.DBTX
	enc encryption.Encryptor
}

// NewLoginCodeRepository creates a new login code repository.
func NewLoginCodeRepository(db postgres.DBTX, enc encryption.Encryptor) *LoginCodeRepository {
	return &LoginCodeRepository{db: db, enc: enc}
}

func (r *LoginCodeRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *LoginCodeRepository) WithTx(tx transaction.Tx) application.LoginCodeRepository {
	return NewLoginCodeRepository(tx.(postgres.DBTX), r.enc)
}

func (r *LoginCodeRepository) Save(ctx context.Context, code domain.LoginCode) error {
	encryptedPhone, err := encryptPhone(ctx, r.enc, code.Phone.String())
	if err != nil {
		return err
	}
	return r.q().CreateLoginCode(ctx, postgres.CreateLoginCodeParams{
		ID:             pgconv.UUIDToPgtype(code.ID),
		Phone:          pgtype.Text{String: encryptedPhone, Valid: true},
		Email:          pgtype.Text{String: code.Email.String(), Valid: code.Email.String() != ""},
		CodeHash:       code.CodeHash,
		ExpiresAt:      pgtype.Timestamptz{Time: code.ExpiresAt, Valid: true},
		UserID:         pgconv.UUIDToPgtypePtr(code.UserID),
		Purpose:        code.Purpose,
		PhoneEncrypted: !r.enc.IsNoop(),
	})
}

func (r *LoginCodeRepository) GetLatestByPhoneAndEmail(ctx context.Context, phone domain.Phone, email domain.Email, purpose string, now time.Time) (domain.LoginCode, error) {
	encryptedPhone, err := encryptPhone(ctx, r.enc, phone.String())
	if err != nil {
		return domain.LoginCode{}, err
	}
	row, err := r.q().GetLatestLoginCodeByPhoneAndEmailAndPurpose(ctx, postgres.GetLatestLoginCodeByPhoneAndEmailAndPurposeParams{
		Phone:     pgtype.Text{String: encryptedPhone, Valid: true},
		Email:     pgtype.Text{String: email.String(), Valid: true},
		Purpose:   purpose,
		ExpiresAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.LoginCode{}, application.ErrNotFound
		}
		return domain.LoginCode{}, err
	}
	return r.mapLoginCode(ctx, toLoginCodeRowPhoneEmail(row))
}

func (r *LoginCodeRepository) GetLatestByPhoneAndUserID(ctx context.Context, phone domain.Phone, purpose string, userID uuid.UUID, now time.Time) (domain.LoginCode, error) {
	encryptedPhone, err := encryptPhone(ctx, r.enc, phone.String())
	if err != nil {
		return domain.LoginCode{}, err
	}
	row, err := r.q().GetLatestLoginCodeByPhoneAndPurposeAndUserID(ctx, postgres.GetLatestLoginCodeByPhoneAndPurposeAndUserIDParams{
		Phone:     pgtype.Text{String: encryptedPhone, Valid: true},
		Purpose:   purpose,
		UserID:    pgconv.UUIDToPgtype(userID),
		ExpiresAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.LoginCode{}, application.ErrNotFound
		}
		return domain.LoginCode{}, err
	}
	return r.mapLoginCode(ctx, toLoginCodeRowUserID(row))
}

func (r *LoginCodeRepository) MarkUsedByID(ctx context.Context, id uuid.UUID) error {
	return r.q().MarkLoginCodeUsed(ctx, pgconv.UUIDToPgtype(id))
}

func (r *LoginCodeRepository) DeleteByID(ctx context.Context, id uuid.UUID) error {
	return r.q().DeleteLoginCodeByID(ctx, pgconv.UUIDToPgtype(id))
}

func (r *LoginCodeRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	return r.q().DeleteLoginCodesByUserID(ctx, pgconv.UUIDToPgtype(userID))
}

func (r *LoginCodeRepository) DeleteExpiredBefore(ctx context.Context, before time.Time) error {
	return r.q().DeleteExpiredLoginCodes(ctx, pgtype.Timestamptz{Time: before, Valid: true})
}

func (r *LoginCodeRepository) DeleteExpiredBeforeBatch(ctx context.Context, before time.Time, batchSize int32) (int64, error) {
	return r.q().DeleteExpiredLoginCodesBatch(ctx, postgres.DeleteExpiredLoginCodesBatchParams{
		ExpiresAt: pgtype.Timestamptz{Time: before, Valid: true},
		Limit:     batchSize,
	})
}

type loginCodeRow struct {
	id             pgtype.UUID
	userID         pgtype.UUID
	phone          pgtype.Text
	email          pgtype.Text
	codeHash       string
	expiresAt      pgtype.Timestamptz
	used           bool
	createdAt      pgtype.Timestamptz
	purpose        string
	phoneEncrypted bool
}

func toLoginCodeRowPhoneEmail(row postgres.GetLatestLoginCodeByPhoneAndEmailAndPurposeRow) loginCodeRow {
	return loginCodeRow{
		id:             row.ID,
		userID:         row.UserID,
		phone:          row.Phone,
		email:          row.Email,
		codeHash:       row.CodeHash,
		expiresAt:      row.ExpiresAt,
		used:           row.Used,
		createdAt:      row.CreatedAt,
		purpose:        row.Purpose,
		phoneEncrypted: row.PhoneEncrypted,
	}
}

func toLoginCodeRowUserID(row postgres.GetLatestLoginCodeByPhoneAndPurposeAndUserIDRow) loginCodeRow {
	return loginCodeRow{
		id:             row.ID,
		userID:         row.UserID,
		phone:          row.Phone,
		email:          row.Email,
		codeHash:       row.CodeHash,
		expiresAt:      row.ExpiresAt,
		used:           row.Used,
		createdAt:      row.CreatedAt,
		purpose:        row.Purpose,
		phoneEncrypted: row.PhoneEncrypted,
	}
}

func (r *LoginCodeRepository) mapLoginCode(ctx context.Context, row loginCodeRow) (domain.LoginCode, error) {
	phone, err := decryptPhone(ctx, r.enc, row.phone.String, row.phoneEncrypted)
	if err != nil {
		return domain.LoginCode{}, err
	}
	email := domain.Email{}
	if row.email.Valid {
		parsed, err := domain.EmailFrom(row.email.String)
		if err != nil {
			return domain.LoginCode{}, fmt.Errorf("invalid email in DB: %w", err)
		}
		email = parsed
	}
	return domain.LoginCode{
		ID:        pgconv.UUIDFromPgtype(row.id),
		UserID:    pgconv.UUIDFromPgtypePtr(row.userID),
		Phone:     domain.PhoneFrom(phone),
		Email:     email,
		Purpose:   row.purpose,
		CodeHash:  row.codeHash,
		ExpiresAt: row.expiresAt.Time,
		Used:      row.used,
		CreatedAt: row.createdAt.Time,
	}, nil
}

// AttemptRepository persists login attempt windows.
type AttemptRepository struct {
	db  postgres.DBTX
	enc encryption.Encryptor
}

// NewAttemptRepository creates a new attempt repository.
func NewAttemptRepository(db postgres.DBTX, enc encryption.Encryptor) *AttemptRepository {
	return &AttemptRepository{db: db, enc: enc}
}

func (r *AttemptRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *AttemptRepository) WithTx(tx transaction.Tx) application.AttemptRepository {
	return NewAttemptRepository(tx.(postgres.DBTX), r.enc)
}

func (r *AttemptRepository) GetByPhone(ctx context.Context, phone domain.Phone) (domain.AttemptWindow, error) {
	encryptedPhone, err := encryptPhone(ctx, r.enc, phone.String())
	if err != nil {
		return domain.AttemptWindow{}, err
	}
	row, err := r.q().GetLoginAttemptByPhone(ctx, encryptedPhone)
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

func (r *AttemptRepository) Save(ctx context.Context, phone domain.Phone, userID uuid.UUID, window domain.AttemptWindow) error {
	failures := window.Failures
	if failures < 0 {
		failures = 0
	}
	if failures > math.MaxInt32 {
		failures = math.MaxInt32
	}
	encryptedPhone, err := encryptPhone(ctx, r.enc, phone.String())
	if err != nil {
		return err
	}
	return r.q().UpsertLoginAttempt(ctx, postgres.UpsertLoginAttemptParams{
		Phone:          encryptedPhone,
		Failures:       int32(failures),
		FirstFailureAt: pgtype.Timestamptz{Time: window.FirstFailureAt, Valid: true},
		LastFailureAt:  pgtype.Timestamptz{Time: window.LastFailureAt, Valid: true},
		UserID:         pgconv.UUIDToPgtype(userID),
		PhoneEncrypted: !r.enc.IsNoop(),
	})
}

func (r *AttemptRepository) IncrementFailures(ctx context.Context, phone domain.Phone, userID uuid.UUID, now time.Time) (domain.AttemptWindow, error) {
	b, ok := r.db.(interface {
		Begin(ctx context.Context) (transaction.Tx, error)
	})
	if !ok {
		return domain.AttemptWindow{}, fmt.Errorf("attempt repository not backed by a connection pool")
	}

	tx, err := b.Begin(ctx)
	if err != nil {
		return domain.AttemptWindow{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRepo := &AttemptRepository{db: tx.(postgres.DBTX), enc: r.enc}

	encryptedPhone, err := encryptPhone(ctx, r.enc, phone.String())
	if err != nil {
		return domain.AttemptWindow{}, err
	}

	row, err := txRepo.q().GetLoginAttemptByPhoneForUpdate(ctx, encryptedPhone)
	var window domain.AttemptWindow
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return domain.AttemptWindow{}, err
		}
		window = domain.NewAttemptWindow(now)
	} else {
		window = domain.AttemptWindow{
			Failures:       int(row.Failures),
			FirstFailureAt: row.FirstFailureAt.Time,
			LastFailureAt:  row.LastFailureAt.Time,
		}
	}

	recErr := window.RecordFailure(now)
	if saveErr := txRepo.Save(ctx, phone, userID, window); saveErr != nil {
		return domain.AttemptWindow{}, fmt.Errorf("save attempts: %w", saveErr)
	}
	if commitErr := tx.Commit(ctx); commitErr != nil {
		return domain.AttemptWindow{}, fmt.Errorf("commit tx: %w", commitErr)
	}
	return window, recErr
}

func (r *AttemptRepository) DeleteByPhone(ctx context.Context, phone domain.Phone) error {
	encryptedPhone, err := encryptPhone(ctx, r.enc, phone.String())
	if err != nil {
		return err
	}
	return r.q().DeleteLoginAttemptByPhone(ctx, encryptedPhone)
}

func (r *AttemptRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	return r.q().DeleteLoginAttemptsByUserID(ctx, pgconv.UUIDToPgtype(userID))
}

func (r *AttemptRepository) DeleteStaleBefore(ctx context.Context, before time.Time) error {
	return r.q().DeleteStaleLoginAttempts(ctx, pgtype.Timestamptz{Time: before, Valid: true})
}

func (r *AttemptRepository) DeleteStaleBeforeBatch(ctx context.Context, before time.Time, batchSize int32) (int64, error) {
	return r.q().DeleteStaleLoginAttemptsBatch(ctx, postgres.DeleteStaleLoginAttemptsBatchParams{
		LastFailureAt: pgtype.Timestamptz{Time: before, Valid: true},
		Limit:         batchSize,
	})
}

// SessionRepository persists sessions.
type SessionRepository struct {
	db  postgres.DBTX
	enc encryption.Encryptor
}

// NewSessionRepository creates a new session repository.
func NewSessionRepository(db postgres.DBTX, enc encryption.Encryptor) *SessionRepository {
	return &SessionRepository{db: db, enc: enc}
}

func (r *SessionRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *SessionRepository) WithTx(tx transaction.Tx) application.SessionRepository {
	return NewSessionRepository(tx.(postgres.DBTX), r.enc)
}

func (r *SessionRepository) Create(ctx context.Context, session domain.Session) error {
	_, err := r.q().CreateSession(ctx, postgres.CreateSessionParams{
		UserID:     pgconv.UUIDToPgtype(session.UserID),
		TokenHash:  session.TokenHash,
		ExpiresAt:  pgtype.Timestamptz{Time: session.ExpiresAt, Valid: true},
		LastUsedAt: pgtype.Timestamptz{Time: session.LastUsedAt, Valid: true},
	})
	return err
}

func (r *SessionRepository) Update(ctx context.Context, session domain.Session) error {
	return r.q().UpdateSession(ctx, postgres.UpdateSessionParams{
		ExpiresAt:  pgtype.Timestamptz{Time: session.ExpiresAt, Valid: true},
		LastUsedAt: pgtype.Timestamptz{Time: session.LastUsedAt, Valid: true},
		TokenHash:  session.TokenHash,
	})
}

func (r *SessionRepository) DeleteByTokenHash(ctx context.Context, tokenHash string) error {
	return r.q().DeleteSessionByTokenHash(ctx, tokenHash)
}

func (r *SessionRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	return r.q().DeleteSessionsByUserID(ctx, pgconv.UUIDToPgtype(userID))
}

func (r *SessionRepository) DeleteByUserIDExcept(ctx context.Context, userID uuid.UUID, tokenHash string) error {
	return r.q().DeleteSessionsByUserIDExcept(ctx, postgres.DeleteSessionsByUserIDExceptParams{
		UserID:    pgconv.UUIDToPgtype(userID),
		TokenHash: tokenHash,
	})
}

func (r *SessionRepository) DeleteExpiredBefore(ctx context.Context, before time.Time) error {
	return r.q().DeleteExpiredSessions(ctx, pgtype.Timestamptz{Time: before, Valid: true})
}

func (r *SessionRepository) DeleteExpiredBeforeBatch(ctx context.Context, before time.Time, batchSize int32) (int64, error) {
	return r.q().DeleteExpiredSessionsBatch(ctx, postgres.DeleteExpiredSessionsBatchParams{
		ExpiresAt: pgtype.Timestamptz{Time: before, Valid: true},
		Limit:     batchSize,
	})
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
	phone, err := decryptPhone(ctx, r.enc, row.Phone, row.PhoneEncrypted)
	if err != nil {
		return domain.Session{}, domain.User{}, err
	}
	return domain.Session{
			UserID:     pgconv.UUIDFromPgtype(row.UserID),
			TokenHash:  row.TokenHash,
			ExpiresAt:  row.ExpiresAt.Time,
			CreatedAt:  row.CreatedAt.Time,
			LastUsedAt: row.LastUsedAt.Time,
		}, domain.User{
			ID:              pgconv.UUIDFromPgtype(row.UserID),
			Phone:           domain.PhoneFrom(phone),
			Role:            domain.Role(row.Role),
			Name:            pgconv.TextToPtrString(row.Name),
			Surname:         pgconv.TextToPtrString(row.Surname),
			Patronymic:      pgconv.TextToPtrString(row.Patronymic),
			Email:           pgconv.TextToPtrString(row.Email),
			EmailVerifiedAt: pgconv.TimestamptzToPtrTime(row.EmailVerifiedAt),
		}, nil
}

func encryptPhone(ctx context.Context, enc encryption.Encryptor, phone string) (string, error) {
	encrypted, err := enc.DeterministicEncrypt(ctx, phone)
	if err != nil {
		return "", fmt.Errorf("encrypt phone: %w", err)
	}
	return encrypted, nil
}

func decryptPhone(ctx context.Context, enc encryption.Encryptor, phone string, encrypted bool) (string, error) {
	if !encrypted {
		return phone, nil
	}
	decrypted, err := enc.Decrypt(ctx, phone)
	if err != nil {
		return "", fmt.Errorf("decrypt phone: %w", err)
	}
	return decrypted, nil
}
