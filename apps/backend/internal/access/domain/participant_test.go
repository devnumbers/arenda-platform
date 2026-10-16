package domain

import (
	"testing"

	"github.com/google/uuid"
)

// TestComputeParticipantAggregate covers the owner's-participant aggregate
// status rule (issue #693): any suspended entry wins («превышен лимит
// объектов»), full coverage of the scope's active properties reads as «доступ
// ко всем объектам», everything else is «доступно N объектов» with N = the
// number of active entries. Pending entries are not access: they count toward
// neither the accessible number nor the full-coverage check.
func TestComputeParticipantAggregate(t *testing.T) {
	t.Parallel()

	prop := func() uuid.UUID {
		id, err := uuid.NewV7()
		if err != nil {
			t.Fatalf("new uuid: %v", err)
		}
		return id
	}

	entry := func(status ParticipantEntryStatus) ParticipantEntry {
		return ParticipantEntry{PropertyID: prop(), Role: RoleFullAccess, Status: status}
	}

	tests := []struct {
		name       string
		entries    []ParticipantEntry
		scopeCount int
		wantStatus ParticipantAggregateStatus
		wantActive int
	}{
		{
			name:       "active on the single scoped property",
			entries:    []ParticipantEntry{entry(ParticipantEntryActive)},
			scopeCount: 1,
			wantStatus: ParticipantStatusAllProperties,
			wantActive: 1,
		},
		{
			name:       "active on one of three scoped properties",
			entries:    []ParticipantEntry{entry(ParticipantEntryActive)},
			scopeCount: 3,
			wantStatus: ParticipantStatusPartial,
			wantActive: 1,
		},
		{
			name:       "active on every scoped property",
			entries:    []ParticipantEntry{entry(ParticipantEntryActive), entry(ParticipantEntryActive)},
			scopeCount: 2,
			wantStatus: ParticipantStatusAllProperties,
			wantActive: 2,
		},
		{
			name:       "one suspended wins over actives",
			entries:    []ParticipantEntry{entry(ParticipantEntryActive), entry(ParticipantEntrySuspended)},
			scopeCount: 3,
			wantStatus: ParticipantStatusLimitExceeded,
			wantActive: 1,
		},
		{
			name:       "all suspended",
			entries:    []ParticipantEntry{entry(ParticipantEntrySuspended), entry(ParticipantEntrySuspended)},
			scopeCount: 2,
			wantStatus: ParticipantStatusLimitExceeded,
			wantActive: 0,
		},
		{
			name:       "pending only",
			entries:    []ParticipantEntry{entry(ParticipantEntryPending)},
			scopeCount: 2,
			wantStatus: ParticipantStatusPartial,
			wantActive: 0,
		},
		{
			name:       "no entries at all",
			entries:    nil,
			scopeCount: 2,
			wantStatus: ParticipantStatusPartial,
			wantActive: 0,
		},
		{
			name:       "empty scope",
			entries:    []ParticipantEntry{entry(ParticipantEntryActive)},
			scopeCount: 0,
			wantStatus: ParticipantStatusPartial,
			wantActive: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotStatus, gotActive := ComputeParticipantAggregate(tt.entries, tt.scopeCount)
			if gotStatus != tt.wantStatus {
				t.Errorf("status = %q, want %q", gotStatus, tt.wantStatus)
			}
			if gotActive != tt.wantActive {
				t.Errorf("accessible count = %d, want %d", gotActive, tt.wantActive)
			}
		})
	}
}
