package application

import (
	"context"
	"slices"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/realtime/domain"
)

// Dispatch is the mutation pipelines' shared post-commit tail (карта #714,
// #716; ADR 0062 §3) — the grace-events canon in one place: it hands the
// transaction's captured change pairs to the carrier strictly after the
// commit. A nil carrier keeps the pre-#716 silence (tests, wiring not yet
// bound); a rolled-back transaction dispatches nothing (the caller's conveyor
// discipline); the carrier itself is best-effort and never fails the
// committed mutation. Every journaled anchor piggybacks its history pair — a
// written journal row is a history change for the object's feed (the journal
// anchors every row to a property, so the owner book has no history anchor).
func Dispatch(ctx context.Context, p Publisher, actor uuid.UUID, changed []domain.Change, journaledAnchors ...uuid.UUID) {
	if p == nil {
		return
	}
	if len(journaledAnchors) > 0 {
		// Clone before appending: the caller's slice is often a shared
		// outcome field, and append past its capacity would write through.
		changed = slices.Clone(changed)
		for _, anchor := range journaledAnchors {
			changed = append(changed, domain.HistoryOn(anchor))
		}
	}
	if len(changed) > 0 {
		p.EntityChanged(ctx, actor, changed...)
	}
}
