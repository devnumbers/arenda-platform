package application

import (
	"errors"
	"testing"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
)

// The list command folding is pure: the pagination bounds and the filter
// vocabulary are enforced here so neither transport nor store can drift
// (ticket #461).
func TestNormalizeOperationsCommand(t *testing.T) {
	t.Parallel()
	planned := domain.ViewStatusPlanned
	from := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)

	t.Run("applies page defaults", func(t *testing.T) {
		t.Parallel()
		q, err := NormalizeOperationsCommand(ListOperationsCommand{})
		if err != nil {
			t.Fatalf("normalize: %v", err)
		}
		if q.Limit != DefaultOperationsPageSize {
			t.Errorf("limit = %d, want default %d", q.Limit, DefaultOperationsPageSize)
		}
		if q.Offset != 0 || q.Status != "" {
			t.Errorf("offset/status = %d/%q, want zeros", q.Offset, q.Status)
		}
	})

	t.Run("keeps an explicit direction and filter", func(t *testing.T) {
		t.Parallel()
		q, err := NormalizeOperationsCommand(ListOperationsCommand{
			Status: &planned, DateFrom: &from, DateTo: &to, Limit: 10, Offset: 5, Desc: true,
		})
		if err != nil {
			t.Fatalf("normalize: %v", err)
		}
		if q.Limit != 10 || q.Offset != 5 || !q.Desc || q.Status != planned {
			t.Errorf("query = %+v, want the command's own values", q)
		}
	})

	cases := []struct {
		name string
		cmd  ListOperationsCommand
	}{
		{"limit over the ceiling", ListOperationsCommand{Limit: MaxOperationsPageSize + 1}},
		{"zero limit below one", ListOperationsCommand{Limit: -1}},
		{"negative offset", ListOperationsCommand{Offset: -1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NormalizeOperationsCommand(tc.cmd); !errors.Is(err, ErrInvalidInput) {
				t.Errorf("err = %v, want ErrInvalidInput", err)
			}
		})
	}

	t.Run("unknown view status is invalid input", func(t *testing.T) {
		t.Parallel()
		bogus := domain.OperationViewStatus("cancelled")
		if _, err := NormalizeOperationsCommand(ListOperationsCommand{Status: &bogus}); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("err = %v, want ErrInvalidInput", err)
		}
	})
}
