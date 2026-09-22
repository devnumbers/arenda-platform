// Package postgres implements action journal persistence over PostgreSQL
// (ADR 0061): the entry insert and the actor display-name/email snapshot
// resolution. Two stores over the same pool — the recorder service binds
// them to a transaction independently, and one Go type cannot carry both
// WithTx signatures (EntryWriter's and ActorSnapshotSource's).
package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	accessapp "github.com/nambers/arenda-planform/apps/backend/internal/access/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/history/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// EntryStore persists journal entries.
type EntryStore struct {
	db postgres.DBTX
	// A non-nil err means WithTx was handed a transaction that is not a
	// DBTX; every operation returns it, so the failure propagates through
	// Record instead of panicking.
	err error
}

var _ application.EntryWriter = (*EntryStore)(nil)

// NewEntryStore creates a journal entry store.
func NewEntryStore(db postgres.DBTX) *EntryStore {
	return &EntryStore{db: db}
}

// WithTx returns a store bound to the provided transaction. A transaction
// that is not a postgres.DBTX yields a store whose Insert always fails, so
// the error propagates through Record instead of panicking.
func (s *EntryStore) WithTx(tx transaction.Tx) application.EntryWriter {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return &EntryStore{err: fmt.Errorf("history.EntryStore.WithTx: %T is not a postgres.DBTX", tx)}
	}
	return NewEntryStore(dbtx)
}

// Insert writes one journal entry. The searchable column and the actor
// snapshots arrive already materialized by the application service.
func (s *EntryStore) Insert(ctx context.Context, entry domain.Entry) error {
	if s.err != nil {
		return s.err
	}
	segmentsJSON, err := json.Marshal(entry.Segments)
	if err != nil {
		return fmt.Errorf("marshal history segments: %w", err)
	}
	contextJSON, err := json.Marshal(entry.Context)
	if err != nil {
		return fmt.Errorf("marshal history context: %w", err)
	}
	_, err = postgres.New(s.db).InsertActionJournal(ctx, postgres.InsertActionJournalParams{
		ID:         pgconv.UUIDToPgtype(entry.ID),
		PropertyID: pgconv.UUIDToPgtype(entry.PropertyID),
		ActorID:    pgconv.UUIDToPgtypePtr(entry.ActorID),
		ActorRole:  string(entry.ActorRole),
		ActorName:  entry.ActorName,
		ActorEmail: entry.ActorEmail,
		Kind:       string(entry.Kind),
		Action:     string(entry.Action),
		BaseAction: string(entry.BaseAction),
		Segments:   segmentsJSON,
		Searchable: entry.Searchable,
		Context:    contextJSON,
		CreatedAt:  pgtype.Timestamptz{Time: entry.CreatedAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("insert action journal: %w", err)
	}
	return nil
}

// ActorStore resolves actor display-name/email snapshots for the journal.
type ActorStore struct {
	db  postgres.DBTX
	err error
}

var _ application.ActorSnapshotSource = (*ActorStore)(nil)

// NewActorStore creates an actor snapshot store.
func NewActorStore(db postgres.DBTX) *ActorStore {
	return &ActorStore{db: db}
}

// WithTx binds the snapshot source to the transaction, so the actor read
// sees the same snapshot as the action. A transaction that is not a
// postgres.DBTX yields a source whose Snapshot always fails.
func (s *ActorStore) WithTx(tx transaction.Tx) application.ActorSnapshotSource {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return &ActorStore{err: fmt.Errorf("history.ActorStore.WithTx: %T is not a postgres.DBTX", tx)}
	}
	return NewActorStore(dbtx)
}

// Snapshot resolves the actor's display name and email at action time. The
// display name follows the access context's canon in one place
// (access.DisplayNameOf): "Name Surname" when present, otherwise a masked
// phone — never a raw phone.
func (s *ActorStore) Snapshot(ctx context.Context, userID uuid.UUID) (domain.ActorSnapshot, error) {
	if s.err != nil {
		return domain.ActorSnapshot{}, s.err
	}
	user, err := postgres.New(s.db).GetUserByID(ctx, pgconv.UUIDToPgtype(userID))
	if err != nil {
		return domain.ActorSnapshot{}, fmt.Errorf("lookup history actor: %w", err)
	}
	name := accessapp.DisplayNameOf(accessapp.MemberUser{
		ID:      userID,
		Name:    pgconv.TextToPtrString(user.Name),
		Surname: pgconv.TextToPtrString(user.Surname),
		Phone:   user.Phone,
	})
	return domain.ActorSnapshot{Name: name, Email: pgconv.TextToString(user.Email)}, nil
}
