package application

import "errors"

// The application error vocabulary of the rental use cases: the transport
// maps them onto the wire contract (400/403/404/409).
var (
	// ErrNotFound covers a missing property, a missing or foreign rental and
	// an actor without the view capability — the privacy-preserving 404.
	ErrNotFound = errors.New("rentals: not found")
	// ErrInvalidInput marks a command that violates the create/update
	// contract (dates out of order, amounts out of bounds, an unknown
	// utilities mode or an overlong comment).
	ErrInvalidInput = errors.New("rentals: invalid input")
	// ErrForbidden marks an actor whose role grants the view capability but
	// not the one the use case needs (a viewer on mutations, a non-owner on
	// deletion — the ADR 0028 matrix).
	ErrForbidden = errors.New("rentals: forbidden")
	// ErrArchivedProperty marks a mutation on an archived property — the
	// financial read-only state.
	ErrArchivedProperty = errors.New("rentals: property is archived")
	// ErrPropertyMaintenance marks a mutation of an unfinished rental on a
	// property under maintenance (ticket #1050): the front hides the rental
	// CTAs, the conveyor is the backstop for the direct API call. The reads
	// and the completed history (its deletion included) stay open; «Завершить
	// ремонт» is the rescue hatch that makes every mutation reachable again.
	ErrPropertyMaintenance = errors.New("rentals: property is under maintenance")
	// ErrPropertyOccupied marks the creation of a second unfinished rental on
	// the property (durable invariant №12): the honest 409 under the
	// property lock; the partial unique index backstops the race.
	ErrPropertyOccupied = errors.New("rentals: property already has an unfinished rental")
	// ErrRentalCompleted marks a mutation of a completed rental: its terms
	// and its completion are final.
	ErrRentalCompleted = errors.New("rentals: rental is completed")
	// ErrRentalStarted marks the deletion of an unfinished rental that has
	// already started: it can only go through the completion (ADR 0053 §3).
	ErrRentalStarted = errors.New("rentals: started rental can only be completed")
)
