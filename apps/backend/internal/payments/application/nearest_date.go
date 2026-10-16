package application

import (
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
)

// NearestDateOfPayment resolves the rule's «Следующая дата оплаты» (ticket
// #991; CONTEXT.md) — the shared seam of the object payments list and the
// global feed's «Ближайший»: the earliest stored planned operation on or
// after the owner's today (ADR 0048) wins; without one the pure projection
// projects from the domain's ProjectionCursor, and the nil result is the
// true null — an open pause cuts the occurrences, a settled rule has
// nothing past its cursor. The stored branch trusts the pause invariant the
// tick enforces in the same transaction: pausing drops the rule's future
// planned, so a paused rule always reaches the projection branch. The
// stored reads stay the caller's business — the feeds scope them
// differently (the object list by owner and property, the global feed by
// the actor's visibility).
func NearestDateOfPayment(rule domain.Payment, today time.Time, inputs NearestDateInputs) *time.Time {
	if inputs.NextPlannedDate != nil {
		return inputs.NextPlannedDate
	}
	next, ok := domain.NextOccurrenceAfter(rule, domain.ProjectionCursor(today, inputs.LastMaterialized))
	if !ok {
		return nil
	}
	return &next
}
