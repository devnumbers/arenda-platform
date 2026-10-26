package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
	pgen "github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// defaultDeleteBatchSize is the row cap of each SQL DELETE ... USING ctid IN
// (subselect LIMIT) round. The deleteExpiredBefore/deleteStaleBefore helpers
// loop the batch query until it reports zero rows affected, so the batch size
// only bounds the work done per statement, not the total rows removed.
const defaultDeleteBatchSize = 1000

// deleteBatched repeatedly invokes deleteBatch — a sqlc :execrows query that
// deletes up to defaultDeleteBatchSize rows matching before — until the query
// reports zero rows affected (completeness), and returns the total number of
// rows removed. It hides the ctid-batching SQL dialect from the application
// port: the port says "delete everything older than before", the adapter
// decides how to chunk the work. The cancellation policy (swallowing or
// logging ctx.Canceled) is left to the caller, so any error — including
// context.Canceled — is returned as-is.
func deleteBatched(
	ctx context.Context,
	before time.Time,
	deleteBatch func(ctx context.Context, before time.Time, limit int32) (int64, error),
) (int64, error) {
	var total int64
	for {
		n, err := deleteBatch(ctx, before, defaultDeleteBatchSize)
		if err != nil {
			return total, err
		}
		if n == 0 {
			return total, nil
		}
		total += n
	}
}

// notFound reports whether err is a pgx.ErrNoRows. Every repository maps a
// not-found row to application.ErrNotFound, so the check is shared here instead
// of repeating errors.Is(err, pgx.ErrNoRows) at each call site.
func notFound(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// repoBase is embedded by every identity repository. It owns the database
// handle, exposes a sqlc Queries accessor (q), and centralizes the
// transaction type-assertion (withTxDB) so each repository's WithTx stays a
// thin wrapper that only selects its own concrete application.*Repository type.
type repoBase struct {
	db pgen.DBTX
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
	ID               pgtype.UUID
	Role             string
	Name             pgtype.Text
	Surname          pgtype.Text
	Patronymic       pgtype.Text
	Email            pgtype.Text
	EmailVerifiedAt  pgtype.Timestamptz
	Timezone         string
	PhotoKey         pgtype.Text
	PhotoContentType pgtype.Text
	PhotoBlur        pgtype.Text
}

// userSourceFromUser adapts a pgen.User model row.
type userSourceFromUser pgen.User

func (s userSourceFromUser) toUserRow() userRow {
	return userRow{
		ID:               s.ID,
		Role:             s.Role,
		Name:             s.Name,
		Surname:          s.Surname,
		Patronymic:       s.Patronymic,
		Email:            pgtype.Text{String: s.Email, Valid: true},
		EmailVerifiedAt:  s.EmailVerifiedAt,
		Timezone:         s.Timezone,
		PhotoKey:         s.PhotoKey,
		PhotoContentType: s.PhotoContentType,
		PhotoBlur:        s.PhotoBlur,
	}
}

// userSourceFromCreateUser adapts a CreateUser query row.
type userSourceFromCreateUser pgen.CreateUserRow

func (s userSourceFromCreateUser) toUserRow() userRow {
	return userRow{
		ID:              s.ID,
		Role:            s.Role,
		Name:            s.Name,
		Surname:         s.Surname,
		Patronymic:      s.Patronymic,
		Email:           pgtype.Text{String: s.Email, Valid: true},
		EmailVerifiedAt: s.EmailVerifiedAt,
		Timezone:        s.Timezone,
	}
}

// userSourceFromSession adapts a GetSessionByTokenHash query row. The user id
// lives in the UserID field (the row's own ID is the session id).
type userSourceFromSession pgen.GetSessionByTokenHashRow

func (s userSourceFromSession) toUserRow() userRow {
	return userRow{
		ID:              s.UserID,
		Role:            s.Role,
		Name:            s.Name,
		Surname:         s.Surname,
		Patronymic:      s.Patronymic,
		Email:           pgtype.Text{String: s.Email, Valid: true},
		EmailVerifiedAt: s.EmailVerifiedAt,
		Timezone:        s.Timezone,
	}
}

// emailToText dereferences the domain email for a NOT NULL column. A nil is a
// domain invariant violation the database rejects loudly (NOT NULL), so the
// empty string is the correct poison value here.
func emailToText(e *domain.Email) string {
	if e == nil {
		return ""
	}
	return e.String()
}

// parseEmailField parses a stored email into a domain.Email value and reports
// whether a non-empty email was present. An empty string yields a zero
// domain.Email and present=false. Shared by mapUser and mapLoginCode so the
// nil-safe EmailFrom sequence is not duplicated.
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
