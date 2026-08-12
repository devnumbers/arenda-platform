package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/encryption"
	pgen "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// notFound reports whether err is a pgx.ErrNoRows. Every repository maps a
// not-found row to application.ErrNotFound, so the check is shared here instead
// of repeating errors.Is(err, pgx.ErrNoRows) at each call site.
func notFound(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// repoBase is embedded by every identity repository. It owns the database
// handle and encryptor, exposes a sqlc Queries accessor (q), and centralizes the
// transaction type-assertion (withTxDB) so each repository's WithTx stays a
// thin wrapper that only selects its own concrete application.*Repository type.
type repoBase struct {
	db  pgen.DBTX
	enc encryption.Encryptor
}

func (r *repoBase) q() *pgen.Queries { return pgen.New(r.db) }

// assertTxDB asserts that tx is a postgres transaction (pgen.DBTX) and returns
// the underlying handle for binding a repository copy to the transaction.
func assertTxDB(tx transaction.Tx) (pgen.DBTX, error) {
	dbtx, ok := tx.(pgen.DBTX)
	if !ok {
		return nil, fmt.Errorf("%T is not a postgres.DBTX", tx)
	}
	return dbtx, nil
}

// userRow is the canonical shape of the user columns consumed by mapUser. It is
// the single mapping target for every sqlc row that carries user data.
type userRow struct {
	ID              pgtype.UUID
	Phone           string
	Role            string
	Name            pgtype.Text
	Surname         pgtype.Text
	Patronymic      pgtype.Text
	Email           pgtype.Text
	EmailVerifiedAt pgtype.Timestamptz
	Timezone        string
	PhoneEncrypted  bool
}

// userSourceFromUser adapts a pgen.User model row.
type userSourceFromUser pgen.User

func (s userSourceFromUser) toUserRow() userRow {
	return userRow{
		ID:              s.ID,
		Phone:           s.Phone,
		Role:            s.Role,
		Name:            s.Name,
		Surname:         s.Surname,
		Patronymic:      s.Patronymic,
		Email:           s.Email,
		EmailVerifiedAt: s.EmailVerifiedAt,
		Timezone:        s.Timezone,
		PhoneEncrypted:  s.PhoneEncrypted,
	}
}

// userSourceFromCreateUser adapts a CreateUser query row.
type userSourceFromCreateUser pgen.CreateUserRow

func (s userSourceFromCreateUser) toUserRow() userRow {
	return userRow{
		ID:              s.ID,
		Phone:           s.Phone,
		Role:            s.Role,
		Name:            s.Name,
		Surname:         s.Surname,
		Patronymic:      s.Patronymic,
		Email:           s.Email,
		EmailVerifiedAt: s.EmailVerifiedAt,
		Timezone:        s.Timezone,
		PhoneEncrypted:  s.PhoneEncrypted,
	}
}

// userSourceFromUpdateEmailVerified adapts an UpdateUserEmailVerified query row.
type userSourceFromUpdateEmailVerified pgen.UpdateUserEmailVerifiedRow

func (s userSourceFromUpdateEmailVerified) toUserRow() userRow {
	return userRow{
		ID:              s.ID,
		Phone:           s.Phone,
		Role:            s.Role,
		Name:            s.Name,
		Surname:         s.Surname,
		Patronymic:      s.Patronymic,
		Email:           s.Email,
		EmailVerifiedAt: s.EmailVerifiedAt,
		Timezone:        s.Timezone,
		PhoneEncrypted:  s.PhoneEncrypted,
	}
}

// userSourceFromSession adapts a GetSessionByTokenHash query row. The user id
// lives in the UserID field (the row's own ID is the session id).
type userSourceFromSession pgen.GetSessionByTokenHashRow

func (s userSourceFromSession) toUserRow() userRow {
	return userRow{
		ID:              s.UserID,
		Phone:           s.Phone,
		Role:            s.Role,
		Name:            s.Name,
		Surname:         s.Surname,
		Patronymic:      s.Patronymic,
		Email:           s.Email,
		EmailVerifiedAt: s.EmailVerifiedAt,
		Timezone:        s.Timezone,
		PhoneEncrypted:  s.PhoneEncrypted,
	}
}

// decryptPhoneField decrypts a stored phone (when encrypted) and parses it
// into a domain.Phone. Shared by mapUser and mapLoginCode so the
// decrypt+NewPhone sequence is not duplicated.
func decryptPhoneField(ctx context.Context, enc encryption.Encryptor, phone string, encrypted bool) (domain.Phone, error) {
	decrypted, err := decryptPhone(ctx, enc, phone, encrypted)
	if err != nil {
		return domain.Phone{}, err
	}
	parsed, err := domain.NewPhone(decrypted)
	if err != nil {
		return domain.Phone{}, fmt.Errorf("invalid phone from DB: %w", err)
	}
	return parsed, nil
}

// parseEmailField parses a nullable stored email into a domain.Email value and
// reports whether a non-empty email was present. A NULL or empty string yields
// a zero domain.Email and present=false. Shared by mapUser and mapLoginCode so
// the nil-safe EmailFrom sequence is not duplicated.
func parseEmailField(email pgtype.Text) (domain.Email, bool, error) {
	if !email.Valid || email.String == "" {
		return domain.Email{}, false, nil
	}
	e, err := domain.EmailFrom(email.String)
	if err != nil {
		return domain.Email{}, false, fmt.Errorf("invalid email from DB: %w", err)
	}
	return e, true, nil
}
