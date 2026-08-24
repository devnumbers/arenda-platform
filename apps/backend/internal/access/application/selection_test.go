package application

import (
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
)

// mustUUID parses a fixed UUID string, failing the test on a bad literal. Fixed
// ids make the table rows readable; production callers use uuid.NewV7().
func mustUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatalf("mustUUID(%q): %v", s, err)
	}
	return id
}

// idsOf returns the PropertyID of each candidate in order, for terse assertions.
func idsOf(cands []SlotCandidate) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(cands))
	for _, c := range cands {
		out = append(out, c.PropertyID)
	}
	return out
}

// own builds an own-property candidate (IsShared=false).
func own(propertyID uuid.UUID, updated time.Time) SlotCandidate {
	return SlotCandidate{PropertyID: propertyID, IsShared: false, UpdatedAt: updated}
}

// shared builds a shared-membership candidate (IsShared=true) for a recipient.
func shared(propertyID, memberID, recipientID uuid.UUID, updated time.Time) SlotCandidate {
	return SlotCandidate{
		PropertyID:  propertyID,
		IsShared:    true,
		MemberID:    memberID,
		RecipientID: recipientID,
		UpdatedAt:   updated,
	}
}

func TestSelectForEviction(t *testing.T) {
	t.Parallel()

	// Fixed instants so table rows read chronologically; t100 is oldest.
	t100 := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	t200 := t100.Add(100 * time.Hour)
	t300 := t100.Add(200 * time.Hour)
	t400 := t100.Add(300 * time.Hour)

	// Fixed object ids so expected slices read like a checklist.
	p1 := mustUUID(t, "00000000-0000-0000-0000-000000000001")
	p2 := mustUUID(t, "00000000-0000-0000-0000-000000000002")
	p3 := mustUUID(t, "00000000-0000-0000-0000-000000000003")
	p4 := mustUUID(t, "00000000-0000-0000-0000-000000000004")

	tests := []struct {
		name       string
		candidates []SlotCandidate
		limit      int
		want       []uuid.UUID // Evicted PropertyIDs, order-independent.
	}{
		{
			name:       "unlimited tariff (limit<0) evicts nobody",
			candidates: []SlotCandidate{own(p1, t100), own(p2, t200)},
			limit:      -1,
			want:       nil,
		},
		{
			name:       "limit 0 evicts everyone",
			candidates: []SlotCandidate{own(p1, t100), own(p2, t200)},
			limit:      0,
			want:       []uuid.UUID{p1, p2},
		},
		{
			name:       "empty candidates evicts nothing",
			candidates: nil,
			limit:      2,
			want:       nil,
		},
		{
			name:       "candidates fit exactly into limit evicts nothing",
			candidates: []SlotCandidate{own(p1, t100), own(p2, t200)},
			limit:      2,
			want:       nil,
		},
		{
			// 4 candidates: the 2 least-recently-updated are evicted. Order of
			// UpdatedAt: t400>t300>t200>t100, so the tail is {t200, t100} =
			// {p2, p1}.
			name:       "least recently updated evicted",
			candidates: []SlotCandidate{own(p4, t400), own(p3, t300), own(p2, t200), own(p1, t100)},
			limit:      2,
			want:       []uuid.UUID{p1, p2},
		},
		{
			// Mix of own and shared: selection does not distinguish them; IsShared
			// is preserved on the returned candidates. Limit 2 keeps the 2 most
			// recently updated (p4, p3); evicts p2 and p1. The shared flag
			// survival is asserted in a dedicated check below.
			name: "own and shared treated equally by selection",
			candidates: []SlotCandidate{
				own(p4, t400),
				shared(p3, mustUUID(t, "00000000-0000-0000-0000-0000000000aa"), mustUUID(t, "00000000-0000-0000-0000-0000000000bb"), t300),
				own(p2, t200),
				shared(p1, mustUUID(t, "00000000-0000-0000-0000-0000000000cc"), mustUUID(t, "00000000-0000-0000-0000-0000000000dd"), t100),
			},
			limit: 2,
			want:  []uuid.UUID{p1, p2},
		},
		{
			// Full tie: same UpdatedAt. The comparator must be deterministic and
			// never panic. Stable sort preserves input order, so with limit 1 the
			// first input element stays and the rest are evicted.
			name: "full tie is deterministic",
			candidates: []SlotCandidate{
				own(p1, t100),
				own(p2, t100),
				own(p3, t100),
			},
			limit: 1,
			want:  []uuid.UUID{p2, p3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Snapshot the input to prove SelectForEviction does not mutate it.
			inputBefore := slices.Clone(tt.candidates)

			got := SelectForEviction(tt.candidates, tt.limit)

			if !slices.Equal(tt.candidates, inputBefore) {
				t.Errorf("SelectForEviction mutated its input")
			}

			gotIDs := idsOf(got)
			wantSet := make(map[uuid.UUID]struct{}, len(tt.want))
			for _, id := range tt.want {
				wantSet[id] = struct{}{}
			}
			if len(gotIDs) != len(tt.want) {
				t.Fatalf("evicted %d, want %d (got %v, want %v)", len(gotIDs), len(tt.want), gotIDs, tt.want)
			}
			for _, id := range gotIDs {
				if _, ok := wantSet[id]; !ok {
					t.Errorf("unexpected evicted id %v; want set %v", id, tt.want)
				}
			}
		})
	}
}

// TestSelectForEviction_PreservesSharedFlag checks that IsShared (and MemberID)
// survive on the evicted candidates, so the caller knows whether to archive the
// object or suspend the membership.
func TestSelectForEviction_PreservesSharedFlag(t *testing.T) {
	t.Parallel()

	p1 := mustUUID(t, "00000000-0000-0000-0000-000000000001")
	p2 := mustUUID(t, "00000000-0000-0000-0000-000000000002")
	member := mustUUID(t, "00000000-0000-0000-0000-0000000000aa")
	recipient := mustUUID(t, "00000000-0000-0000-0000-0000000000bb")
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	candidates := []SlotCandidate{
		own(p2, now),                       // Stays (first input on a full tie).
		shared(p1, member, recipient, now), // Full tie: the tail is evicted.
	}

	got := SelectForEviction(candidates, 1)
	if len(got) != 1 {
		t.Fatalf("evicted %d, want 1", len(got))
	}
	c := got[0]
	if c.PropertyID != p1 {
		t.Errorf("PropertyID = %v, want %v", c.PropertyID, p1)
	}
	if !c.IsShared {
		t.Errorf("IsShared = false, want true (shared flag must survive)")
	}
	if c.MemberID != member {
		t.Errorf("MemberID = %v, want %v", c.MemberID, member)
	}
	if c.RecipientID != recipient {
		t.Errorf("RecipientID = %v, want %v", c.RecipientID, recipient)
	}
}

func TestSelectForRecovery(t *testing.T) {
	t.Parallel()

	// Fixed instants; s1 is earliest (FIFO front).
	s1 := time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC)
	s2 := s1.Add(48 * time.Hour)
	s3 := s1.Add(96 * time.Hour)

	// Update instants for tie-break inside a downgrade batch (same SuspendedAt).
	u100 := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	u200 := u100.Add(100 * time.Hour)
	u300 := u100.Add(200 * time.Hour)

	p1 := mustUUID(t, "00000000-0000-0000-0000-000000000001")
	p2 := mustUUID(t, "00000000-0000-0000-0000-000000000002")
	p3 := mustUUID(t, "00000000-0000-0000-0000-000000000003")
	p4 := mustUUID(t, "00000000-0000-0000-0000-000000000004")

	// SuspendedCand marks SuspendedAt on a SlotCandidate for recovery tests.
	suspendedCand := func(id uuid.UUID, suspendedAt, updatedAt time.Time) SlotCandidate {
		c := own(id, updatedAt)
		c.SuspendedAt = suspendedAt
		return c
	}

	tests := []struct {
		name      string
		suspended []SlotCandidate
		freeSlots int
		want      []uuid.UUID // Recovered PropertyIDs in recovery order.
	}{
		{
			name:      "no free slots recovers nobody",
			suspended: []SlotCandidate{suspendedCand(p1, s1, u100)},
			freeSlots: 0,
			want:      nil,
		},
		{
			name:      "empty queue recovers nobody",
			suspended: nil,
			freeSlots: 5,
			want:      nil,
		},
		{
			// FreeSlots >= len(queue): all returned in FIFO order (SuspendedAt ASC).
			// Input is deliberately shuffled to prove the sort, not the input order.
			name: "all recovered in FIFO order",
			suspended: []SlotCandidate{
				suspendedCand(p3, s3, u100),
				suspendedCand(p1, s1, u100),
				suspendedCand(p2, s2, u100),
			},
			freeSlots: 5,
			want:      []uuid.UUID{p1, p2, p3},
		},
		{
			// FreeSlots < len(queue): only the earliest are recovered.
			name: "partial recovery keeps the earliest",
			suspended: []SlotCandidate{
				suspendedCand(p3, s3, u100),
				suspendedCand(p1, s1, u100),
				suspendedCand(p2, s2, u100),
			},
			freeSlots: 2,
			want:      []uuid.UUID{p1, p2},
		},
		{
			// Downgrade batch: same SuspendedAt (s1) for p1, p2, p3. Tie-break is
			// most-recently-updated first: p3 (u300) > p2 (u200) > p1 (u100).
			// P4 has a later SuspendedAt (s2) so it goes after the whole batch.
			name: "downgrade batch tie-break: recency",
			suspended: []SlotCandidate{
				suspendedCand(p4, s2, u100), // Later batch.
				suspendedCand(p1, s1, u100),
				suspendedCand(p3, s1, u300),
				suspendedCand(p2, s1, u200),
			},
			freeSlots: 4,
			want:      []uuid.UUID{p3, p2, p1, p4},
		},
		{
			// Batch cut in half by freeSlots: only the front of the tie-broken
			// order is recovered, never the later batch.
			name: "batch partially recovered",
			suspended: []SlotCandidate{
				suspendedCand(p4, s2, u100),
				suspendedCand(p1, s1, u100),
				suspendedCand(p3, s1, u300),
				suspendedCand(p2, s1, u200),
			},
			freeSlots: 2,
			want:      []uuid.UUID{p3, p2},
		},
		{
			// FreeSlots greater than queue length returns everything (still ordered).
			name: "freeSlots exceeds queue length",
			suspended: []SlotCandidate{
				suspendedCand(p2, s2, u100),
				suspendedCand(p1, s1, u100),
			},
			freeSlots: 10,
			want:      []uuid.UUID{p1, p2},
		},
		{
			// Full tie (same SuspendedAt, same UpdatedAt): the comparator is
			// deterministic and never panics; stable sort keeps input order, so
			// the first inputs are recovered first.
			name: "full tie is deterministic",
			suspended: []SlotCandidate{
				suspendedCand(p1, s1, u100),
				suspendedCand(p2, s1, u100),
				suspendedCand(p3, s1, u100),
			},
			freeSlots: 2,
			want:      []uuid.UUID{p1, p2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inputBefore := slices.Clone(tt.suspended)

			got := SelectForRecovery(tt.suspended, tt.freeSlots)

			if !slices.Equal(tt.suspended, inputBefore) {
				t.Errorf("SelectForRecovery mutated its input")
			}

			gotIDs := idsOf(got)
			if !slices.Equal(gotIDs, tt.want) {
				t.Fatalf("recovery order = %v, want %v", gotIDs, tt.want)
			}
		})
	}
}

// TestSelectForRecovery_ReturnsSharedCandidates confirms shared candidates flow
// through recovery with their membership labels intact (universal candidate).
func TestSelectForRecovery_ReturnsSharedCandidates(t *testing.T) {
	t.Parallel()

	p1 := mustUUID(t, "00000000-0000-0000-0000-000000000001")
	member := mustUUID(t, "00000000-0000-0000-0000-0000000000aa")
	recipient := mustUUID(t, "00000000-0000-0000-0000-0000000000bb")
	suspendedAt := time.Date(2025, 2, 1, 10, 0, 0, 0, time.UTC)

	c := shared(p1, member, recipient, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	c.SuspendedAt = suspendedAt

	got := SelectForRecovery([]SlotCandidate{c}, 1)
	if len(got) != 1 {
		t.Fatalf("recovered %d, want 1", len(got))
	}
	out := got[0]
	if out.PropertyID != p1 || !out.IsShared || out.MemberID != member || out.RecipientID != recipient {
		t.Errorf("shared candidate labels lost: %+v", out)
	}
}
