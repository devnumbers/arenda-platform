// Package pgerr classifies PostgreSQL driver errors (unique, foreign-key violations) for sentinel mapping.
package pgerr

import (
	"errors"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

// IsUniqueViolation reports whether err is a PostgreSQL unique violation.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation
}

// IsUniqueViolationOnConstraint reports whether err is a PostgreSQL unique
// violation raised by the named constraint or index — the narrowing a table
// with several partial unique indexes needs to tell their races apart.
func IsUniqueViolationOnConstraint(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation && pgErr.ConstraintName == constraint
}

// IsForeignKeyViolation reports whether err is a PostgreSQL foreign-key
// violation.
func IsForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.ForeignKeyViolation
}
