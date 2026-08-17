package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/identity/domain"
)

func TestNotFound(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"pgx.ErrNoRows", pgx.ErrNoRows, true},
		{"wrapped pgx.ErrNoRows", errors.Join(pgx.ErrNoRows, errors.New("context")), true},
		{"unrelated error", errors.New("connection refused"), false},
		{"nil error", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := notFound(tc.err); got != tc.want {
				t.Fatalf("notFound(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestDeleteBatched_LoopsUntilZeroRowsAndReturnsTotal(t *testing.T) {
	t.Parallel()

	var calls int
	// First two calls return 1000 rows; third returns 0 (completeness).
	rowsSequence := []int64{1000, 1000, 0}
	total, err := deleteBatched(t.Context(), time.Now(), func(_ context.Context, _ time.Time, _ int32) (int64, error) {
		rows := rowsSequence[calls]
		calls++
		return rows, nil
	})
	if err != nil {
		t.Fatalf("deleteBatched error = %v", err)
	}
	if calls != 3 {
		t.Fatalf("deleteBatch calls = %d, want 3 (loop until 0 rows)", calls)
	}
	if total != 2000 {
		t.Fatalf("total = %d, want 2000 (sum across batches)", total)
	}
}

func TestDeleteBatched_ReturnsErrorImmediately(t *testing.T) {
	t.Parallel()

	dbErr := errors.New("batch delete failed")
	calls := 0
	total, err := deleteBatched(t.Context(), time.Now(), func(_ context.Context, _ time.Time, _ int32) (int64, error) {
		calls++
		return 0, dbErr
	})
	if !errors.Is(err, dbErr) {
		t.Fatalf("deleteBatched error = %v, want wrap of dbErr", err)
	}
	if total != 0 {
		t.Fatalf("total = %d, want 0 on error", total)
	}
	if calls != 1 {
		t.Fatalf("deleteBatch calls = %d, want 1 (stops on first error)", calls)
	}
}

func TestParseEmailField(t *testing.T) {
	t.Parallel()

	t.Run("valid email returns parsed value and true", func(t *testing.T) {
		t.Parallel()
		got, present, err := parseEmailField(pgtype.Text{String: "owner@example.com", Valid: true})
		if err != nil {
			t.Fatalf("parseEmailField error = %v", err)
		}
		if !present {
			t.Fatal("present = false, want true")
		}
		if got.String() != "owner@example.com" {
			t.Fatalf("email = %s, want owner@example.com", got.String())
		}
	})

	t.Run("NULL pgtype.Text returns false", func(t *testing.T) {
		t.Parallel()
		_, present, err := parseEmailField(pgtype.Text{Valid: false})
		if err != nil {
			t.Fatalf("parseEmailField error = %v", err)
		}
		if present {
			t.Fatal("present = true, want false for NULL")
		}
	})

	t.Run("empty string returns false", func(t *testing.T) {
		t.Parallel()
		_, present, err := parseEmailField(pgtype.Text{String: "", Valid: true})
		if err != nil {
			t.Fatalf("parseEmailField error = %v", err)
		}
		if present {
			t.Fatal("present = true, want false for empty string")
		}
	})

	t.Run("invalid email returns error", func(t *testing.T) {
		t.Parallel()
		_, _, err := parseEmailField(pgtype.Text{String: "not-an-email", Valid: true})
		if err == nil {
			t.Fatal("parseEmailField(invalid) error = nil, want error")
		}
		if !errors.Is(err, domain.ErrInvalidEmail) {
			t.Fatalf("parseEmailField error = %v, want wrap of ErrInvalidEmail", err)
		}
	})
}
