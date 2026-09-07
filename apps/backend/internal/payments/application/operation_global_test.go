package application

import (
	"errors"
	"testing"
)

// The global feed's pagination shares the property listing's contract (ticket
// #540): PrepareGlobalOperationsQuery applies the same page default and the
// same bounds — one pagination vocabulary across both screens. Table-driven
// over the outcomes; the passing cases double as the default-applied check.
func TestPrepareGlobalOperationsQuery(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		q         GlobalOperationsListQuery
		wantLimit int
		wantErr   error
	}{
		{"zero limit takes the default page", GlobalOperationsListQuery{}, DefaultOperationsPageSize, nil},
		{"an explicit window passes untouched", GlobalOperationsListQuery{Limit: 10, Offset: 5}, 10, nil},
		{"limit over the ceiling", GlobalOperationsListQuery{Limit: MaxOperationsPageSize + 1}, 0, ErrInvalidInput},
		{"negative limit", GlobalOperationsListQuery{Limit: -1}, 0, ErrInvalidInput},
		{"negative offset", GlobalOperationsListQuery{Offset: -1}, 0, ErrInvalidInput},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := PrepareGlobalOperationsQuery(&tc.q)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr == nil && tc.q.Limit != tc.wantLimit {
				t.Errorf("limit = %d, want %d", tc.q.Limit, tc.wantLimit)
			}
		})
	}
}
