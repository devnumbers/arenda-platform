package application

import (
	"slices"
	"time"

	"github.com/google/uuid"
)

// SlotCandidate describes a single object in a recipient's tariff pool: either a
// recipient's own active property (IsShared=false) or an active shared property
// the recipient holds a membership on (IsShared=true). It is used uniformly by
// both the eviction selection (when a downgrade leaves more objects than the new
// limit allows) and the FIFO recovery selection (when freed slots let suspended
// objects come back).
//
// The selection rule is defined in docs/entities/tarif.md ("Автоархив при
// понижении" and the recovery counterpart) and PRD #153, issue #158 (T4):
//
//   - Eviction keeps objects in this order: first those with an open lease, then
//     the most recently updated. If open leases exceed the limit, the excess
//     objects with open leases are evicted too.
//   - Recovery is FIFO by the suspension moment; the tie-break inside a single
//     downgrade batch (one trigger that stamped the same SuspendedAt) is again
//     "open lease first, then most recently updated".
//
// IsShared is only a label for the caller: own objects are auto-archived on
// eviction, shared memberships are suspended. The selection itself treats own and
// shared candidates identically.
type SlotCandidate struct {
	PropertyID   uuid.UUID
	IsShared     bool // Own property when false; recipient's membership when true.
	MemberID     uuid.UUID
	RecipientID  uuid.UUID // IsShared=true only.
	HasOpenLease bool      // Whether an open lease exists on the object.
	UpdatedAt    time.Time // own: property.UpdatedAt; shared: membership.UpdatedAt
	SuspendedAt  time.Time // Recovery only: the moment of suspension.
}

// SelectForEviction returns the candidates that do NOT fit into limit — i.e. the
// ones the caller must evict (auto-archive for own objects, suspend for shared
// memberships). It is the tail of the "best stay" ordering described in
// docs/entities/tarif.md and PRD #153.
//
// The "best stays" ordering (top of the sorted pool) is:
//  1. HasOpenLease true ranks above false.
//  2. Within equal HasOpenLease, more recent UpdatedAt ranks above older.
//
// Edge cases: limit < 0 means an unlimited tariff and nobody is evicted; limit
// == 0 evicts everyone; len(candidates) <= limit evicts nobody; an empty input
// yields an empty result. The input slice is never mutated.
func SelectForEviction(candidates []SlotCandidate, limit int) []SlotCandidate {
	if limit < 0 || len(candidates) == 0 || len(candidates) <= limit {
		return []SlotCandidate{}
	}

	pool := slices.Clone(candidates)
	slices.SortStableFunc(pool, func(a, b SlotCandidate) int {
		switch {
		case evictionBetter(a, b):
			return -1
		case evictionBetter(b, a):
			return 1
		default:
			return 0
		}
	})

	// The first `limit` entries stay; the rest are evicted.
	tail := pool[limit:]
	return slices.Clone(tail)
}

// SelectForRecovery returns the candidates that fit into freeSlots, in the order
// they must be recovered (FIFO by suspension moment, with a deterministic
// tie-break inside a downgrade batch). See docs/entities/tarif.md and PRD #153,
// issue #158 (T4).
//
// The recovery ordering is:
//  1. Earlier SuspendedAt ranks above later (FIFO).
//  2. Within the same SuspendedAt (a downgrade batch): HasOpenLease true first,
//     then more recent UpdatedAt first.
//
// Edge cases: freeSlots <= 0 recovers nobody; an empty input yields an empty
// result. The input slice is never mutated.
func SelectForRecovery(suspended []SlotCandidate, freeSlots int) []SlotCandidate {
	if freeSlots <= 0 || len(suspended) == 0 {
		return []SlotCandidate{}
	}

	pool := slices.Clone(suspended)
	slices.SortStableFunc(pool, func(a, b SlotCandidate) int {
		switch {
		case recoveryBefore(a, b):
			return -1
		case recoveryBefore(b, a):
			return 1
		default:
			return 0
		}
	})

	take := min(freeSlots, len(pool))
	return slices.Clone(pool[:take])
}

// evictionBetter reports whether a "stays better" than b and therefore must sort
// before b in the eviction pool (i.e. a is closer to the protected top-N). It
// implements the rule "open lease first, then most recently updated" from
// docs/entities/tarif.md.
func evictionBetter(a, b SlotCandidate) bool {
	if a.HasOpenLease != b.HasOpenLease {
		return a.HasOpenLease
	}
	return a.UpdatedAt.After(b.UpdatedAt)
}

// recoveryBefore reports whether a must be recovered before b. It implements
// FIFO by SuspendedAt, with the downgrade-batch tie-break "open lease first,
// then most recently updated" from docs/entities/tarif.md and PRD #153.
func recoveryBefore(a, b SlotCandidate) bool {
	if !a.SuspendedAt.Equal(b.SuspendedAt) {
		return a.SuspendedAt.Before(b.SuspendedAt)
	}
	if a.HasOpenLease != b.HasOpenLease {
		return a.HasOpenLease
	}
	return a.UpdatedAt.After(b.UpdatedAt)
}
