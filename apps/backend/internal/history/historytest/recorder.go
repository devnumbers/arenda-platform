// Package historytest provides the capturing Recorder double for the action
// journal (ADR 0061) — the shared replacement for the per-module fakeHistory
// copies in the in-memory use case suites.
package historytest

import (
	"context"

	historyapp "github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// CapturingRecorder is a historyapp.Recorder that captures journaled entries
// instead of persisting them. The zero value captures silently; Err turns
// every Record into the configured failure for the fail-safe variant (the
// mutation and its journal row are born and die together, ADR 0061 §3).
type CapturingRecorder struct {
	// Entries holds the recorded entries in call order. Tests reset it by
	// assigning nil between a setup step and the asserted action.
	Entries []historydomain.Entry

	// Err, when set, is returned from every Record — assert it with errors.Is
	// against the test's own sentinel (RecordScoped wraps it with %w).
	Err error

	// OnRecord, when set, fires before the Err gate — the side-channel for a
	// test's cross-double call-order journal (the rentals conveyor). It sees
	// the entry even when the recorder is about to fail: a real store records
	// the attempt before its insert verdict.
	OnRecord func(historydomain.Entry)
}

var _ historyapp.Recorder = (*CapturingRecorder)(nil)

// Record captures the entry, or fails with Err.
func (r *CapturingRecorder) Record(_ context.Context, e historydomain.Entry) error {
	if r.OnRecord != nil {
		r.OnRecord(e)
	}
	if r.Err != nil {
		return r.Err
	}
	r.Entries = append(r.Entries, e)
	return nil
}

// WithTx returns the same recorder: the captures ignore transaction
// boundaries — the in-memory suites assert entries, not tx identity.
func (r *CapturingRecorder) WithTx(transaction.Tx) historyapp.Recorder { return r }
