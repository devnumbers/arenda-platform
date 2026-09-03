package application

import (
	"errors"
	"testing"
)

func TestPrepareTasksQuery_AppliesDefaultsAndBounds(t *testing.T) {
	t.Parallel()

	t.Run("zero limit becomes the default page", func(t *testing.T) {
		t.Parallel()
		q := TasksListQuery{}
		if err := PrepareTasksQuery(&q); err != nil {
			t.Fatalf("PrepareTasksQuery: %v", err)
		}
		if q.Limit != DefaultTasksPageSize {
			t.Fatalf("Limit = %d, want %d", q.Limit, DefaultTasksPageSize)
		}
	})

	t.Run("explicit values travel through", func(t *testing.T) {
		t.Parallel()
		q := TasksListQuery{Completed: true, Limit: 7, Offset: 14}
		if err := PrepareTasksQuery(&q); err != nil {
			t.Fatalf("PrepareTasksQuery: %v", err)
		}
		if q.Limit != 7 || q.Offset != 14 || !q.Completed {
			t.Fatalf("query mutated: %+v", q)
		}
	})

	t.Run("out of bounds is invalid input", func(t *testing.T) {
		t.Parallel()
		for name, q := range map[string]TasksListQuery{
			"negative limit":  {Limit: -1},
			"over ceiling":    {Limit: MaxTasksPageSize + 1},
			"negative offset": {Limit: 10, Offset: -1},
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				if err := PrepareTasksQuery(&q); !errors.Is(err, ErrInvalidInput) {
					t.Fatalf("PrepareTasksQuery(%+v) = %v, want ErrInvalidInput", q, err)
				}
			})
		}
	})
}
