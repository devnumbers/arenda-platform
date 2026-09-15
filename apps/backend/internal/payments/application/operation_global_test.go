package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// The global feed's page contract (tickets #540, #597): the page-size
// default and bounds of the property listing plus the keyset continuation —
// the cursor decodes into the AfterDate/AfterID the SQL resumes strictly
// after, and anything malformed is ErrInvalidInput. Table-driven over the
// outcomes; the passing cases double as the default-applied check.
func TestPrepareGlobalOperationsQuery(t *testing.T) {
	t.Parallel()

	cursorID := uuid.Must(uuid.NewV7())
	cursorDate := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	validCursor := encodeOperationCursor(cursorDate, cursorID)

	cases := []struct {
		name      string
		q         GlobalOperationsListQuery
		wantLimit int
		wantErr   error
	}{
		{"zero limit takes the default page", GlobalOperationsListQuery{}, DefaultOperationsPageSize, nil},
		{"an explicit limit passes untouched", GlobalOperationsListQuery{Limit: 10}, 10, nil},
		{"limit over the ceiling", GlobalOperationsListQuery{Limit: MaxOperationsPageSize + 1}, 0, ErrInvalidInput},
		{"negative limit", GlobalOperationsListQuery{Limit: -1}, 0, ErrInvalidInput},
		{"a malformed cursor", GlobalOperationsListQuery{Limit: 10, Cursor: "!!!"}, 0, ErrInvalidInput},
		{"the empty cursor payload", GlobalOperationsListQuery{Limit: 10, Cursor: ""}, 10, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := PrepareGlobalOperationsQuery(&tc.q)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				return
			}
			if tc.q.Limit != tc.wantLimit {
				t.Errorf("limit = %d, want %d", tc.q.Limit, tc.wantLimit)
			}
		})
	}

	// The valid cursor decodes into the keyset key the page resumes after.
	q := GlobalOperationsListQuery{Limit: 10, Cursor: validCursor}
	if err := PrepareGlobalOperationsQuery(&q); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if q.AfterDate == nil || !q.AfterDate.Equal(cursorDate) {
		t.Errorf("after date = %v, want %v", q.AfterDate, cursorDate)
	}
	if q.AfterID == nil || *q.AfterID != cursorID {
		t.Errorf("after id = %v, want %s", q.AfterID, cursorID)
	}

	// The empty cursor is the feed's beginning: no keyset key at all.
	q = GlobalOperationsListQuery{Limit: 10}
	if err := PrepareGlobalOperationsQuery(&q); err != nil {
		t.Fatalf("prepare without cursor: %v", err)
	}
	if q.AfterDate != nil || q.AfterID != nil {
		t.Errorf("after key = %v/%v, want nil/nil", q.AfterDate, q.AfterID)
	}
}

func (noopOperationStore) CountGlobal(context.Context, uuid.UUID, GlobalOperationsListQuery) (int64, error) {
	panic("unused")
}
