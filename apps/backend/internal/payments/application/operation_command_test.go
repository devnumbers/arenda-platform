package application

import (
	"errors"
	"testing"
)

// The pagination bounds live in PrepareOperationsQuery so neither transport
// nor store can drift (ticket #461); the filter vocabulary is the transport
// binder's enum Valid() check and is not repeated here.
func TestPrepareOperationsQuery(t *testing.T) {
	t.Parallel()

	t.Run("applies page defaults", func(t *testing.T) {
		t.Parallel()
		q := OperationsListQuery{}
		if err := PrepareOperationsQuery(&q); err != nil {
			t.Fatalf("prepare: %v", err)
		}
		if q.Limit != DefaultOperationsPageSize {
			t.Errorf("limit = %d, want default %d", q.Limit, DefaultOperationsPageSize)
		}
		if q.Offset != 0 {
			t.Errorf("offset = %d, want 0", q.Offset)
		}
		if q.Desc {
			t.Error("Desc set by prepare, want untouched — the direction is the transport's default")
		}
	})

	t.Run("keeps an explicit window", func(t *testing.T) {
		t.Parallel()
		q := OperationsListQuery{Limit: 10, Offset: 5}
		if err := PrepareOperationsQuery(&q); err != nil {
			t.Fatalf("prepare: %v", err)
		}
		if q.Limit != 10 || q.Offset != 5 {
			t.Errorf("limit/offset = %d/%d, want 10/5", q.Limit, q.Offset)
		}
	})

	cases := []struct {
		name string
		q    OperationsListQuery
	}{
		{"limit over the ceiling", OperationsListQuery{Limit: MaxOperationsPageSize + 1}},
		{"negative limit", OperationsListQuery{Limit: -1}},
		{"negative offset", OperationsListQuery{Offset: -1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := PrepareOperationsQuery(&tc.q); !errors.Is(err, ErrInvalidInput) {
				t.Errorf("err = %v, want ErrInvalidInput", err)
			}
		})
	}
}
