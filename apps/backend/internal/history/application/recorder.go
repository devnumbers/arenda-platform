// Package application provides the action journal recording port used by
// other modules and its default service implementation (ADR 0061).
package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// EntryWriter persists journal entries. Implemented by persistence adapters.
type EntryWriter interface {
	Insert(ctx context.Context, entry domain.Entry) error
	WithTx(tx transaction.Tx) EntryWriter
}

// ActorSnapshotSource resolves the actor's display name and email at action
// time (ADR 0061 §3: the name/email snapshots never re-resolve afterwards).
// The display name follows the access context's canon: "Name Surname" when
// present, otherwise the full phone (карта #1105) — never an email.
type ActorSnapshotSource interface {
	Snapshot(ctx context.Context, userID uuid.UUID) (domain.ActorSnapshot, error)
	WithTx(tx transaction.Tx) ActorSnapshotSource
}

// Recorder is the port other modules use to write journal entries. Recording
// is fail-safe like the audit (ADR 0020): an insert error returns to the
// caller so a transaction-bound caller rolls the action back — a mutation and
// its journal row are born and die together.
type Recorder interface {
	Record(ctx context.Context, entry domain.Entry) error
	WithTx(tx transaction.Tx) Recorder
}

// Service is the default Recorder implementation.
type Service struct {
	writer EntryWriter
	actors ActorSnapshotSource
	clock  clock.Clock
}

var _ Recorder = (*Service)(nil)

// NewService creates a Service.
func NewService(writer EntryWriter, actors ActorSnapshotSource, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.Real{}
	}
	return &Service{writer: writer, actors: actors, clock: clk}
}

// WithTx returns a Recorder bound to the provided transaction — both the
// writer and the actor snapshot source read and write the same snapshot as
// the action itself.
func (s *Service) WithTx(tx transaction.Tx) Recorder {
	return &Service{writer: s.writer.WithTx(tx), actors: s.actors.WithTx(tx), clock: s.clock}
}

// Record assigns the entry ID and timestamp, resolves the actor's name/email
// snapshot when the caller did not supply one, materializes the searchable
// column and persists the entry. Callers set everything else — the scope, the
// actor, the row text from the domain catalog.
func (s *Service) Record(ctx context.Context, entry domain.Entry) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate history id: %w", err)
	}
	entry.ID = id
	entry.CreatedAt = s.clock.Now()
	if entry.Context == nil {
		entry.Context = map[string]any{}
	}
	// Manual actions always carry an actor; the snapshot resolution is the
	// recorder's job so no calling module has to fetch the user itself.
	if entry.ActorID != nil && entry.ActorName == "" {
		snap, err := s.actors.Snapshot(ctx, *entry.ActorID)
		if err != nil {
			return fmt.Errorf("resolve history actor: %w", err)
		}
		entry.ActorName = snap.Name
		entry.ActorEmail = snap.Email
	}
	entry.Searchable = searchableOf(entry)
	if err := s.writer.Insert(ctx, entry); err != nil {
		return fmt.Errorf("insert history entry: %w", err)
	}
	return nil
}

// searchableOf materializes the search text (research #705): the row's plain
// text plus the actor name and email, so a member is findable by what they
// did and by who they are.
func searchableOf(entry domain.Entry) string {
	parts := make([]string, 0, 3)
	if text := entry.Segments.PlainText(); text != "" {
		parts = append(parts, text)
	}
	if entry.ActorName != "" {
		parts = append(parts, entry.ActorName)
	}
	if entry.ActorEmail != "" {
		parts = append(parts, entry.ActorEmail)
	}
	return strings.Join(parts, " ")
}

// Noop is a Recorder that discards entries.
type Noop struct{}

var _ Recorder = Noop{}

// Record discards the entry.
func (Noop) Record(_ context.Context, _ domain.Entry) error { return nil }

// WithTx returns the same Noop recorder.
func (n Noop) WithTx(_ transaction.Tx) Recorder { return n }

// RecordScoped fills the entry's property anchor and actor fields and
// records it — the shared tail of the calling modules' journal writes
// (ADR 0061 §3). The role arrives already mapped onto the journal's
// vocabulary: each owning module maps the policy role it resolved before
// its transaction opened, as the audit does. Fail-safe like the audit
// (ADR 0020): the wrapped error returns to a transaction-bound caller, so
// the mutation and its journal row are born and die together.
func RecordScoped(
	ctx context.Context, rec Recorder, propertyID, actor uuid.UUID,
	actorRole domain.ActorRole, entry domain.Entry,
) error {
	entry.PropertyID = propertyID
	entry.ActorID = &actor
	entry.ActorRole = actorRole
	if err := rec.Record(ctx, entry); err != nil {
		return fmt.Errorf("record history: %w", err)
	}
	return nil
}
