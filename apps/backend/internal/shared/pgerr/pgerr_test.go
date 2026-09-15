package pgerr

import (
	"errors"
	"testing"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

func uniqueViolation(constraint string) error {
	return &pgconn.PgError{Code: pgerrcode.UniqueViolation, ConstraintName: constraint}
}

func TestIsUniqueViolation(t *testing.T) {
	t.Parallel()
	if !IsUniqueViolation(uniqueViolation("any_index")) {
		t.Error("IsUniqueViolation(unique violation) = false, want true")
	}
	if IsUniqueViolation(&pgconn.PgError{Code: pgerrcode.ForeignKeyViolation, ConstraintName: "any_fk"}) {
		t.Error("IsUniqueViolation(foreign-key violation) = true, want false")
	}
	wrapped := errors.Join(errors.New("query failed"), uniqueViolation("any_index"))
	if !IsUniqueViolation(wrapped) {
		t.Error("IsUniqueViolation(wrapped unique violation) = false, want true")
	}
}

func TestIsUniqueViolationOnConstraint(t *testing.T) {
	t.Parallel()
	err := uniqueViolation("idx_one_pending_form")
	if !IsUniqueViolationOnConstraint(err, "idx_one_pending_form") {
		t.Error("matching constraint = false, want true")
	}
	if IsUniqueViolationOnConstraint(err, "idx_other_index") {
		t.Error("other constraint = true, want false")
	}
	if IsUniqueViolationOnConstraint(
		&pgconn.PgError{Code: pgerrcode.ForeignKeyViolation, ConstraintName: "idx_one_pending_form"}, "idx_one_pending_form",
	) {
		t.Error("foreign-key violation on the named constraint = true, want false")
	}
}
