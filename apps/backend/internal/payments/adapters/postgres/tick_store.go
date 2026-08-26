// Package postgres holds the payments persistence adapters: the tick store
// (materialization queries of ADR 0049 §3), the owner calendar (ADR 0048),
// the payment rule CRUD store with its favorite flag (#457, #461), and the
// operation store of the second contracts slice (#461).
package postgres

import (
	"context"
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
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// Compile-time conformance of the adapters to the consumer-declared ports.
var (
	_ application.TickStore         = (*TickStore)(nil)
	_ application.OwnerCalendar     = (*OwnerCalendar)(nil)
	_ application.TickZoneDirectory = (*TickZoneDirectory)(nil)
)

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

// LoadOwnerSnapshot returns the owner's payment rules on non-archived
// properties with pause intervals and existing operation statuses — the
// tick's read side in one call.
func (s *TickStore) LoadOwnerSnapshot(ctx context.Context, ownerID uuid.UUID) (application.OwnerSnapshot, error) {
	rows, err := s.q().ListTickPaymentsByOwner(ctx, pgconv.UUIDToPgtype(ownerID))
	if err != nil {
		return application.OwnerSnapshot{}, fmt.Errorf("list tick payments: %w", err)
	}
	payments := make([]domain.Payment, 0, len(rows))
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		payment, err := mapPaymentRow(paymentFieldsFromTickRow(row))
		if err != nil {
			return application.OwnerSnapshot{}, err
		}
		payments = append(payments, payment)
		ids = append(ids, payment.ID)
	}
	snapshot := application.OwnerSnapshot{
		Payments: payments,
		Statuses: make(map[uuid.UUID]map[time.Time]domain.OperationStatus, len(rows)),
	}
	if len(ids) == 0 {
		return snapshot, nil
	}
	if err := attachPauses(ctx, s.q(), payments); err != nil {
		return application.OwnerSnapshot{}, err
	}
	if err := s.attachStatuses(ctx, ids, snapshot.Statuses); err != nil {
		return application.OwnerSnapshot{}, err
	}
	return snapshot, nil
}

// attachStatuses loads the listed payments' operation statuses into the
// snapshot map, keyed by payment and date.
func (s *TickStore) attachStatuses(
	ctx context.Context, ids []uuid.UUID, into map[uuid.UUID]map[time.Time]domain.OperationStatus,
) error {
	rows, err := s.q().ListTickOperationStatuses(ctx, pgconv.UUIDSliceToPgtype(ids))
	if err != nil {
		return fmt.Errorf("list tick operation statuses: %w", err)
	}
	for _, row := range rows {
		id := pgconv.UUIDFromPgtype(row.PaymentID)
		if into[id] == nil {
			into[id] = make(map[time.Time]domain.OperationStatus)
		}
		into[id][pgconv.DateFromPgtype(row.Date)] = domain.OperationStatus(row.Status)
	}
	return nil
}

// ApplyTickPlan applies one rule's tick plan inside the caller's
// transaction: the idempotent occurrence inserts (due dates and the missing
// future planned), the auto-pay day payment — strictly today (ADR 0049 §2) —
// and the future-planned rebuild in one keep-or-none statement. The call
// order and the keep-or-none duality are this implementation's business.
func (s *TickStore) ApplyTickPlan(
	ctx context.Context, p domain.Payment, today time.Time, plan domain.PaymentTickPlan,
) error {
	for _, date := range plan.Materialize {
		if err := s.insertOccurrence(ctx, p, date); err != nil {
			return err
		}
	}
	if plan.InsertFuture != nil {
		if err := s.insertOccurrence(ctx, p, *plan.InsertFuture); err != nil {
			return err
		}
	}
	if plan.AutoPayToday {
		if _, err := s.q().PayOperationDueToday(ctx, postgres.PayOperationDueTodayParams{
			PaymentID: pgconv.UUIDToPgtype(p.ID),
			PaidDate:  pgconv.DateToPgtype(today),
		}); err != nil {
			return fmt.Errorf("auto-pay occurrence of payment %s due today: %w", p.ID, err)
		}
	}
	var keep pgtype.Date
	if plan.KeepFuture != nil {
		keep = pgconv.DateToPgtype(*plan.KeepFuture)
	}
	if _, err := s.q().DeleteFuturePlannedExcept(ctx, postgres.DeleteFuturePlannedExceptParams{
		PaymentID: pgconv.UUIDToPgtype(p.ID),
		Date:      pgconv.DateToPgtype(today),
		Column3:   keep,
	}); err != nil {
		return fmt.Errorf("rebuild future planned of payment %s: %w", p.ID, err)
	}
	return nil
}

// insertOccurrence materializes one planned occurrence of the rule; the
// insert is idempotent through the (payment_id, date) partial unique index.
func (s *TickStore) insertOccurrence(ctx context.Context, p domain.Payment, date time.Time) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("mint operation id: %w", err)
	}
	op := domain.NewMaterializedOperation(p, date)
	op.ID = id
	if op.PaymentID == nil {
		return fmt.Errorf("materialize occurrence %s of payment %s: payment id is required",
			date.Format(time.DateOnly), p.ID)
	}
	if op.PaymentForm == nil {
		return fmt.Errorf("materialize occurrence %s of payment %s: payment form snapshot is required",
			date.Format(time.DateOnly), p.ID)
	}
	err = s.q().InsertMaterializedOperation(ctx, postgres.InsertMaterializedOperationParams{
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
		return fmt.Errorf("materialize occurrence %s of payment %s: %w",
			date.Format(time.DateOnly), p.ID, err)
	}
	return nil
}

// OwnerCalendar is the postgres adapter of the owner calendar port
// (ADR 0048): users.timezone, NOT NULL with the Europe/Moscow default,
// IANA-validated on write, plus the injected clock. A row that fails to load
// as a location is a data integrity error, not a fallback case.
type OwnerCalendar struct {
	db    postgres.DBTX
	clock clock.Clock
}

// NewOwnerCalendar creates a calendar over the given connection or pool.
func NewOwnerCalendar(db postgres.DBTX, clk clock.Clock) *OwnerCalendar {
	return &OwnerCalendar{db: db, clock: clk}
}

func (c *OwnerCalendar) q() *postgres.Queries {
	return postgres.New(c.db)
}

// Today returns the owner's calendar date at the clock's now: the date in
// the owner's timezone, rebuilt at UTC midnight so it compares correctly
// against the UTC-midnight dates stored in DATE columns (ADR 0048 p.2 —
// computed in Go, no AT TIME ZONE in SQL).
func (c *OwnerCalendar) Today(ctx context.Context, ownerID uuid.UUID) (time.Time, error) {
	name, err := c.q().GetOwnerTimezone(ctx, pgconv.UUIDToPgtype(ownerID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, fmt.Errorf("load owner timezone: owner %s has no users row: %w", ownerID, err)
		}
		return time.Time{}, fmt.Errorf("load owner timezone: %w", err)
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.Time{}, fmt.Errorf("load owner timezone %q as location: %w", name, err)
	}
	return application.DateAtUTCMidnight(c.clock.Now(), loc), nil
}

// TickZoneDirectory is the postgres adapter of the hourly sweep port
// (ADR 0048 p.3): the distinct owner timezones with payment rules on
// active/maintenance properties, each with its owners. The grouping of the
// flat DISTINCT rows into zones is this adapter's business.
type TickZoneDirectory struct {
	db postgres.DBTX
}

// NewTickZoneDirectory creates a zone directory over the given connection or
// pool.
func NewTickZoneDirectory(db postgres.DBTX) *TickZoneDirectory {
	return &TickZoneDirectory{db: db}
}

// ListTickZones returns the sweep targets grouped by timezone, in the
// query's timezone order; owners within a zone keep the query's owner order.
func (d *TickZoneDirectory) ListTickZones(ctx context.Context) ([]application.TickZone, error) {
	rows, err := postgres.New(d.db).ListTickZones(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tick zones: %w", err)
	}
	zones := make([]application.TickZone, 0, len(rows))
	for _, row := range rows {
		if len(zones) == 0 || zones[len(zones)-1].Timezone != row.Timezone {
			zones = append(zones, application.TickZone{Timezone: row.Timezone})
		}
		last := &zones[len(zones)-1]
		last.Owners = append(last.Owners, pgconv.UUIDFromPgtype(row.OwnerID))
	}
	return zones, nil
}
