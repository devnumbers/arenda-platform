package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// Compile-time conformance of the adapter to the consumer-declared port.
var _ application.PaymentChangeLogStore = (*PaymentChangeLogStore)(nil)

// PaymentChangeLogStore is the postgres adapter of the payment change log
// port (ADR 0065): the append-only insert inside the caller's transaction and
// the bidirectional keyset read over the log's single index. Reads and writes
// are scoped by the data owner; the payment→property path lives in the
// queries.
type PaymentChangeLogStore struct {
	db postgres.DBTX
}

// NewPaymentChangeLogStore creates a change log store over the given
// connection or pool.
func NewPaymentChangeLogStore(db postgres.DBTX) *PaymentChangeLogStore {
	return &PaymentChangeLogStore{db: db}
}

func (s *PaymentChangeLogStore) q() *postgres.Queries {
	return postgres.New(s.db)
}

// WithTx returns a store bound to the provided transaction.
func (s *PaymentChangeLogStore) WithTx(tx transaction.Tx) (application.PaymentChangeLogStore, error) {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return nil, fmt.Errorf("payments.PaymentChangeLogStore.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewPaymentChangeLogStore(dbtx), nil
}

// Record appends one entry; the id is minted here (UUIDv7, ADR 0019),
// created_at is the database's. An empty diff (pause/resume) lands as the
// literal [] — a nil slice would marshal to null and break the column's
// «array, even empty» shape.
func (s *PaymentChangeLogStore) Record(
	ctx context.Context, scope, propertyID, actorID uuid.UUID, write application.ChangeLogWrite,
) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("mint change log id: %w", err)
	}
	changes := write.Changes
	if changes == nil {
		changes = []domain.FieldChange{}
	}
	blob, err := json.Marshal(changes)
	if err != nil {
		return fmt.Errorf("marshal change log diff: %w", err)
	}
	if err := s.q().InsertPaymentChangeLog(ctx, postgres.InsertPaymentChangeLogParams{
		ID:         pgconv.UUIDToPgtype(id),
		PaymentID:  pgconv.UUIDToPgtype(write.PaymentID),
		OwnerID:    pgconv.UUIDToPgtype(scope),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
		ActorID:    pgconv.UUIDToPgtype(actorID),
		Action:     string(write.Action),
		Changes:    blob,
	}); err != nil {
		return fmt.Errorf("insert payment change log %s: %w", write.PaymentID, err)
	}
	return nil
}

// ListByPayment returns one bidirectional keyset page of the payment's log,
// newest first: the Before-leg resumes strictly before its key over the
// feed's DESC order; the After-leg walks strictly after it in ASC — a burst
// wider than the page is carried from the anchor toward the fresh edge — and
// is reversed back into the feed's order; no cursor is the first page.
func (s *PaymentChangeLogStore) ListByPayment(
	ctx context.Context, scope, propertyID, paymentID uuid.UUID,
	before, after *application.ChangeLogKey, limit int,
) ([]domain.ChangeEntry, error) {
	// Сервис гарантирует 1..ChangesMaxLimit; стор проверяет границу явно —
	// честный int→int32 без переполнения (gosec G115).
	if limit <= 0 || limit > application.ChangesMaxLimit {
		return nil, fmt.Errorf("list payment changes: limit %d out of range", limit)
	}
	pageLimit := shared.ToInt32Clamped(limit)
	switch {
	case before != nil:
		rows, err := s.q().ListPaymentChangesBefore(ctx, postgres.ListPaymentChangesBeforeParams{
			PaymentID:       pgconv.UUIDToPgtype(paymentID),
			OwnerID:         pgconv.UUIDToPgtype(scope),
			PropertyID:      pgconv.UUIDToPgtype(propertyID),
			BeforeCreatedAt: pgtype.Timestamptz{Time: before.CreatedAt, Valid: true},
			BeforeID:        pgconv.UUIDToPgtype(before.ID),
			PageLimit:       pageLimit,
		})
		if err != nil {
			return nil, fmt.Errorf("list payment changes before: %w", err)
		}
		return changePage(rows)
	case after != nil:
		rows, err := s.q().ListPaymentChangesAfter(ctx, postgres.ListPaymentChangesAfterParams{
			PaymentID:      pgconv.UUIDToPgtype(paymentID),
			OwnerID:        pgconv.UUIDToPgtype(scope),
			PropertyID:     pgconv.UUIDToPgtype(propertyID),
			AfterCreatedAt: pgtype.Timestamptz{Time: after.CreatedAt, Valid: true},
			AfterID:        pgconv.UUIDToPgtype(after.ID),
			PageLimit:      pageLimit,
		})
		if err != nil {
			return nil, fmt.Errorf("list payment changes after: %w", err)
		}
		slices.Reverse(rows)
		return changePage(rows)
	default:
		rows, err := s.q().ListPaymentChanges(ctx, postgres.ListPaymentChangesParams{
			PaymentID:  pgconv.UUIDToPgtype(paymentID),
			OwnerID:    pgconv.UUIDToPgtype(scope),
			PropertyID: pgconv.UUIDToPgtype(propertyID),
			PageLimit:  pageLimit,
		})
		if err != nil {
			return nil, fmt.Errorf("list payment changes: %w", err)
		}
		return changePage(rows)
	}
}

// changeEntryOf unmarshals one row into the domain entry. All three queries
// select the same columns, so their row types convert losslessly (канон
// read_store.feedEntryOf). A malformed changes blob is a data-integrity
// failure, not a wire error — it surfaces as a 500 through the wrapped error.
func changeEntryOf(row postgres.ListPaymentChangesRow) (domain.ChangeEntry, error) {
	var changes []domain.FieldChange
	if err := json.Unmarshal(row.Changes, &changes); err != nil {
		return domain.ChangeEntry{}, fmt.Errorf(
			"unmarshal change log diff %s: %w", pgconv.UUIDFromPgtype(row.ID), err)
	}
	return domain.ChangeEntry{
		ID:        pgconv.UUIDFromPgtype(row.ID),
		PaymentID: pgconv.UUIDFromPgtype(row.PaymentID),
		ActorID:   pgconv.UUIDFromPgtype(row.ActorID),
		Action:    domain.ChangeAction(row.Action),
		Changes:   changes,
		CreatedAt: row.CreatedAt.Time,
	}, nil
}

// changeLogRow is the union of the log's three sqlc row shapes — the first
// page and the two keyset legs select the same columns.
type changeLogRow interface {
	postgres.ListPaymentChangesRow |
		postgres.ListPaymentChangesBeforeRow |
		postgres.ListPaymentChangesAfterRow
}

// changePage converts a page of before/after rows into domain entries. All
// three queries select the same columns, so the row shapes convert losslessly
// (канон read_store.feedEntryOf); the first conversion error fails the page —
// a half-read page never travels out.
func changePage[T changeLogRow](rows []T) ([]domain.ChangeEntry, error) {
	out := make([]domain.ChangeEntry, len(rows))
	for i, row := range rows {
		entry, err := changeEntryOf(postgres.ListPaymentChangesRow(row))
		if err != nil {
			return nil, fmt.Errorf("map payment change page: %w", err)
		}
		out[i] = entry
	}
	return out, nil
}
