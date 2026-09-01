package postgres

// OperationStore is the postgres adapter of the operations port (ticket #461,
// ADR 0049 §4). Reads and writes are scoped by the data owner and the nested
// property→operation path lives in the queries; the mutating methods run
// inside the transaction holding the property serialization lock. The two
// listing scopes share one SQL query behind the port — a NULL payment widens
// it from one rule to the whole property.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// Compile-time conformance of the adapter to the consumer-declared port.
var _ application.OperationStore = (*OperationStore)(nil)

// OperationStore persists and lists operations.
type OperationStore struct {
	db postgres.DBTX
}

// NewOperationStore creates an operation store over the given connection or
// pool.
func NewOperationStore(db postgres.DBTX) *OperationStore {
	return &OperationStore{db: db}
}

func (s *OperationStore) q() *postgres.Queries {
	return postgres.New(s.db)
}

// WithTx returns a store bound to the provided transaction.
func (s *OperationStore) WithTx(tx transaction.Tx) (application.OperationStore, error) {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return nil, fmt.Errorf("payments.OperationStore.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewOperationStore(dbtx), nil
}

// Get loads one operation; pgx.ErrNoRows — an unknown id, another owner's row
// or another property's row — becomes the application ErrNotFound.
func (s *OperationStore) Get(
	ctx context.Context, id, scope, propertyID uuid.UUID,
) (domain.Operation, error) {
	row, err := s.q().GetOperationByID(ctx, postgres.GetOperationByIDParams{
		ID:         pgconv.UUIDToPgtype(id),
		OwnerID:    pgconv.UUIDToPgtype(scope),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Operation{}, application.ErrNotFound
		}
		return domain.Operation{}, fmt.Errorf("get operation %s: %w", id, err)
	}
	return mapOperationRow(operationRowFieldsFromGet(row)), nil
}

// MarkPaid flips the still-planned operation to paid with the given date;
// rows affected = 0 — a repeated pay under the same serialization lock —
// surfaces as ErrAlreadyPaid.
func (s *OperationStore) MarkPaid(ctx context.Context, id, scope uuid.UUID, paidDate time.Time) error {
	affected, err := s.q().PayOperationByID(ctx, postgres.PayOperationByIDParams{
		ID:       pgconv.UUIDToPgtype(id),
		OwnerID:  pgconv.UUIDToPgtype(scope),
		PaidDate: pgconv.DateToPgtype(paidDate),
	})
	if err != nil {
		return fmt.Errorf("pay operation %s: %w", id, err)
	}
	if affected == 0 {
		return application.ErrAlreadyPaid
	}
	return nil
}

// Cancel flips a planned or paid operation to the cancelled tombstone and
// clears paid_date with the payment fact; the row stays and keeps its
// (payment_id, date) key so the tick never re-materializes the occurrence.
// Rows affected = 0 — an unknown id, a foreign row or an already-cancelled
// one — surfaces as ErrNotFound: cancelled operations are gone for every
// read (lists exclude the status, the single GET reports not-found).
func (s *OperationStore) Cancel(ctx context.Context, id, scope, propertyID uuid.UUID) error {
	affected, err := s.q().CancelOperationByID(ctx, postgres.CancelOperationByIDParams{
		ID:         pgconv.UUIDToPgtype(id),
		OwnerID:    pgconv.UUIDToPgtype(scope),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
	if err != nil {
		return fmt.Errorf("cancel operation %s: %w", id, err)
	}
	if affected == 0 {
		return application.ErrNotFound
	}
	return nil
}

// ListByPayment returns one rule's operations in the query's order.
func (s *OperationStore) ListByPayment(
	ctx context.Context, scope, propertyID, paymentID uuid.UUID, q application.OperationsListQuery,
) ([]domain.Operation, error) {
	rows, err := s.q().ListOperations(ctx, listOperationsParams(postgres.ListOperationsParams{
		Owner:    pgconv.UUIDToPgtype(scope),
		Property: pgconv.UUIDToPgtype(propertyID),
		Payment:  pgconv.UUIDToPgtype(paymentID),
	}, q))
	if err != nil {
		return nil, fmt.Errorf("list operations of payment %s: %w", paymentID, err)
	}
	return mapOperationRows(rows), nil
}

// ListByProperty returns the property's operations across its rules; the
// payment filter stays NULL so every rule's rows come through.
func (s *OperationStore) ListByProperty(
	ctx context.Context, scope, propertyID uuid.UUID, q application.OperationsListQuery,
) ([]domain.Operation, error) {
	rows, err := s.q().ListOperations(ctx, listOperationsParams(postgres.ListOperationsParams{
		Owner:    pgconv.UUIDToPgtype(scope),
		Property: pgconv.UUIDToPgtype(propertyID),
	}, q))
	if err != nil {
		return nil, fmt.Errorf("list operations of property %s: %w", propertyID, err)
	}
	return mapOperationRows(rows), nil
}

// listOperationsParams folds the normalized query into the merged SQL
// parameters; the pagination width clamp and the direction/status encodings
// are this adapter's business.
func listOperationsParams(
	params postgres.ListOperationsParams, q application.OperationsListQuery,
) postgres.ListOperationsParams {
	params.Status = operationsStatusFilter(q.Status)
	params.Today = pgconv.DateToPgtype(q.Today)
	params.DateFrom = pgconv.DatePtrToPgtype(q.DateFrom)
	params.DateTo = pgconv.DatePtrToPgtype(q.DateTo)
	params.Search = escapeLikePattern(q.Search)
	params.Order = operationsOrder(q.Asc)
	params.Offset = paginationToInt32(q.Offset)
	params.Limit = paginationToInt32(q.Limit)
	return params
}

// mapOperationRows projects listed rows onto the domain shape.
func mapOperationRows(rows []postgres.ListOperationsRow) []domain.Operation {
	out := make([]domain.Operation, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapOperationRow(operationRowFields{
			ID: row.ID, OwnerID: row.OwnerID, PropertyID: row.PropertyID,
			PaymentID: row.PaymentID, Origin: row.Origin, Date: row.Date,
			PaidDate: row.PaidDate, Status: row.Status, Type: row.Type,
			Title: row.Title, AmountKopecks: row.AmountKopecks,
			PaymentForm: row.PaymentForm, CategoryLabel: row.CategoryLabel,
			CategorySlug: row.CategorySlug,
		}))
	}
	return out
}

// operationsStatusFilter encodes the view status filter for the SQL: nil is
// any status; planned/paid/overdue split against the query's today inside the
// WHERE clause (ticket #461).
func operationsStatusFilter(status *domain.OperationViewStatus) string {
	if status == nil {
		return ""
	}
	return string(*status)
}

// operationsOrder encodes the sort direction ('asc' | 'desc'); false encodes
// the contract default — desc, newest first.
func operationsOrder(asc bool) string {
	if asc {
		return "asc"
	}
	return "desc"
}

// paginationToInt32 narrows the validated pagination values onto SQL's LIMIT/
// OFFSET width. PrepareOperationsQuery has already bounded both at the
// application boundary; the clamp keeps this adapter total regardless.
func paginationToInt32(v int) int32 {
	const maxInt32 = int64(^uint32(0) >> 1)
	switch {
	case v < 0:
		return 0
	case int64(v) > maxInt32:
		return int32(maxInt32)
	default:
		return int32(v)
	}
}
