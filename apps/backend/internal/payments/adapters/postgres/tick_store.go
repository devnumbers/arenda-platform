// Package postgres holds the payments persistence adapters: the tick store
// (materialization queries of ADR 0049 §3) and the owner-timezone resolver
// (ADR 0048). Payment CRUD repositories arrive with their tickets (#457).
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// Compile-time conformance of the adapter to the consumer-declared ports.
var _ application.TickStore = (*TickStore)(nil)

// TickStore is the postgres adapter of the materialization tick port.
type TickStore struct {
	db postgres.DBTX
}

// NewTickStore creates a tick store over the given connection or pool.
func NewTickStore(db postgres.DBTX) *TickStore {
	return &TickStore{db: db}
}

func (s *TickStore) q() *postgres.Queries {
	return postgres.New(s.db)
}

// WithTx returns a store bound to the provided transaction.
func (s *TickStore) WithTx(tx transaction.Tx) (application.TickStore, error) {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return nil, fmt.Errorf("payments.TickStore.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewTickStore(dbtx), nil
}

// LockOwnerProperties takes the tick's serialization point: FOR UPDATE on
// the owner's active/maintenance property rows (ADR 0049 §3).
func (s *TickStore) LockOwnerProperties(ctx context.Context, ownerID uuid.UUID) error {
	if _, err := s.q().LockOwnerTickProperties(ctx, pgconv.UUIDToPgtype(ownerID)); err != nil {
		return fmt.Errorf("lock owner tick properties: %w", err)
	}
	return nil
}

// LoadOwnerPayments returns the owner's payment rules on non-archived
// properties with pause intervals attached.
func (s *TickStore) LoadOwnerPayments(ctx context.Context, ownerID uuid.UUID) ([]domain.Payment, error) {
	rows, err := s.q().ListTickPaymentsByOwner(ctx, pgconv.UUIDToPgtype(ownerID))
	if err != nil {
		return nil, fmt.Errorf("list tick payments: %w", err)
	}
	payments := make([]domain.Payment, 0, len(rows))
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		payment, err := mapTickPayment(row)
		if err != nil {
			return nil, err
		}
		payments = append(payments, payment)
		ids = append(ids, payment.ID)
	}
	if len(ids) == 0 {
		return payments, nil
	}
	pauses, err := s.listPauses(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range payments {
		payments[i].Pauses = pauses[payments[i].ID]
	}
	return payments, nil
}

// listPauses loads the pause intervals of the listed payments grouped by
// payment id.
func (s *TickStore) listPauses(ctx context.Context, paymentIDs []uuid.UUID) (map[uuid.UUID][]domain.PauseInterval, error) {
	rows, err := s.q().ListTickPausesByPaymentIDs(ctx, pgconv.UUIDSliceToPgtype(paymentIDs))
	if err != nil {
		return nil, fmt.Errorf("list tick pauses: %w", err)
	}
	out := make(map[uuid.UUID][]domain.PauseInterval, len(rows))
	for _, row := range rows {
		interval := domain.PauseInterval{
			From: pgconv.DateFromPgtype(row.FromDate),
			To:   pgconv.DatePtrFromPgtype(row.ToDate),
		}
		id := pgconv.UUIDFromPgtype(row.PaymentID)
		out[id] = append(out[id], interval)
	}
	return out, nil
}

// ListOperationStatuses returns the listed payments' operations keyed by
// payment and date.
func (s *TickStore) ListOperationStatuses(
	ctx context.Context, paymentIDs []uuid.UUID,
) (map[uuid.UUID]map[time.Time]domain.OperationStatus, error) {
	rows, err := s.q().ListTickOperationStatuses(ctx, pgconv.UUIDSliceToPgtype(paymentIDs))
	if err != nil {
		return nil, fmt.Errorf("list tick operation statuses: %w", err)
	}
	out := make(map[uuid.UUID]map[time.Time]domain.OperationStatus, len(rows))
	for _, row := range rows {
		id := pgconv.UUIDFromPgtype(row.PaymentID)
		if out[id] == nil {
			out[id] = make(map[time.Time]domain.OperationStatus)
		}
		out[id][pgconv.DateFromPgtype(row.Date)] = domain.OperationStatus(row.Status)
	}
	return out, nil
}

// InsertOperation inserts a materialized occurrence; the partial unique
// (payment_id, date) with ON CONFLICT DO NOTHING keeps it idempotent.
func (s *TickStore) InsertOperation(ctx context.Context, op domain.Operation) error {
	if op.PaymentID == nil {
		return errors.New("insert materialized operation: payment id is required")
	}
	if op.PaymentForm == nil {
		return errors.New("insert materialized operation: payment form snapshot is required")
	}
	err := s.q().InsertMaterializedOperation(ctx, postgres.InsertMaterializedOperationParams{
		ID:            pgconv.UUIDToPgtype(op.ID),
		OwnerID:       pgconv.UUIDToPgtype(op.OwnerID),
		PropertyID:    pgconv.UUIDToPgtype(op.PropertyID),
		PaymentID:     pgconv.UUIDToPgtype(*op.PaymentID),
		Date:          pgconv.DateToPgtype(op.Date),
		Type:          string(op.Type),
		Title:         op.Title,
		AmountKopecks: op.AmountKopecks,
		PaymentForm:   pgtype.Text{String: string(*op.PaymentForm), Valid: true},
		CategoryLabel: op.CategoryLabel,
		CategorySlug:  pgconv.StringPtrToPgtype(op.CategorySlug),
	})
	if err != nil {
		return fmt.Errorf("insert materialized operation: %w", err)
	}
	return nil
}

// PayDueToday closes the rule's planned occurrence dated today (the auto-pay
// day payment, ADR 0049 §2).
func (s *TickStore) PayDueToday(ctx context.Context, paymentID uuid.UUID, today time.Time) error {
	if _, err := s.q().PayOperationDueToday(ctx, postgres.PayOperationDueTodayParams{
		PaymentID: pgconv.UUIDToPgtype(paymentID),
		PaidDate:  pgconv.DateToPgtype(today),
	}); err != nil {
		return fmt.Errorf("pay operation due today: %w", err)
	}
	return nil
}

// DeleteFuturePlannedExcept removes the rule's future planned operations
// except the single allowed date.
func (s *TickStore) DeleteFuturePlannedExcept(
	ctx context.Context, paymentID uuid.UUID, today, keep time.Time,
) error {
	if _, err := s.q().DeleteFuturePlannedExcept(ctx, postgres.DeleteFuturePlannedExceptParams{
		PaymentID: pgconv.UUIDToPgtype(paymentID),
		Date:      pgconv.DateToPgtype(today),
		Date_2:    pgconv.DateToPgtype(keep),
	}); err != nil {
		return fmt.Errorf("delete future planned except: %w", err)
	}
	return nil
}

// DeleteFuturePlannedAll removes every future planned operation of the rule.
func (s *TickStore) DeleteFuturePlannedAll(ctx context.Context, paymentID uuid.UUID, today time.Time) error {
	if _, err := s.q().DeleteFuturePlannedAll(ctx, postgres.DeleteFuturePlannedAllParams{
		PaymentID: pgconv.UUIDToPgtype(paymentID),
		Date:      pgconv.DateToPgtype(today),
	}); err != nil {
		return fmt.Errorf("delete future planned all: %w", err)
	}
	return nil
}

// mapTickPayment maps a tick listing row to the domain rule. The recurrence
// jsonb goes through the domain constructors, so a stored row can never
// resurrect an invalid variant.
func mapTickPayment(row postgres.ListTickPaymentsByOwnerRow) (domain.Payment, error) {
	var recurrence domain.Recurrence
	if err := json.Unmarshal(row.Recurrence, &recurrence); err != nil {
		return domain.Payment{}, fmt.Errorf("parse recurrence of payment %s: %w",
			pgconv.UUIDFromPgtype(row.ID), err)
	}
	return domain.Payment{
		ID:            pgconv.UUIDFromPgtype(row.ID),
		OwnerID:       pgconv.UUIDFromPgtype(row.OwnerID),
		PropertyID:    pgconv.UUIDFromPgtype(row.PropertyID),
		Type:          domain.PaymentType(row.Type),
		Title:         row.Title,
		AmountKopecks: row.AmountKopecks,
		Recurrence:    recurrence,
		Since:         pgconv.DateFromPgtype(row.Since),
		EndDate:       pgconv.DatePtrFromPgtype(row.EndDate),
		AutoPay:       row.AutoPay,
		PaymentForm:   domain.PaymentForm(row.PaymentForm),
		Category: domain.CategoryRef{
			Slug:             pgconv.TextToPtrString(row.CategorySlug),
			UserCategoryID:   pgconv.UUIDFromPgtypePtr(row.UserCategoryID),
			UserCategoryName: pgconv.TextToPtrString(row.UserCategoryName),
		},
	}, nil
}

// OwnerTimezoneResolver is the postgres adapter of the owner-timezone port
// (ADR 0048): users.timezone, NOT NULL with the Europe/Moscow default,
// IANA-validated on write. A row that fails to load as a location is a data
// integrity error, not a fallback case.
type OwnerTimezoneResolver struct {
	db postgres.DBTX
}

// Compile-time conformance to the consumer-declared port.
var _ application.OwnerTimezoneResolver = (*OwnerTimezoneResolver)(nil)

// NewOwnerTimezoneResolver creates a resolver over the given connection or
// pool.
func NewOwnerTimezoneResolver(db postgres.DBTX) *OwnerTimezoneResolver {
	return &OwnerTimezoneResolver{db: db}
}

func (r *OwnerTimezoneResolver) q() *postgres.Queries {
	return postgres.New(r.db)
}

// OwnerTimezone loads the owner's IANA timezone name and resolves it to a
// location. The date itself is computed by the application in Go (ADR 0048
// p.2) — no AT TIME ZONE in SQL.
func (r *OwnerTimezoneResolver) OwnerTimezone(ctx context.Context, ownerID uuid.UUID) (*time.Location, error) {
	name, err := r.q().GetOwnerTimezone(ctx, pgconv.UUIDToPgtype(ownerID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("load owner timezone: owner %s has no users row: %w", ownerID, err)
		}
		return nil, fmt.Errorf("load owner timezone: %w", err)
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("load owner timezone %q as location: %w", name, err)
	}
	return loc, nil
}
