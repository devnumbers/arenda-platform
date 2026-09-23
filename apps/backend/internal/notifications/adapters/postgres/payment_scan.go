package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// The adapter satisfies the consumer-declared ports (CODING_STANDARDS).
var (
	_ application.ScanZoneDirectory = (*PaymentScanStore)(nil)
	_ application.PaymentScanSource = (*PaymentScanStore)(nil)
)

// PaymentScanStore answers the payments scan's questions (#749, #776) over
// the owning tables directly: the sweep targets' zones, the zone's due-day
// operations, the zone's overdue ones, the upcoming boundaries' booking
// lists, the boundary jobs' reloads and the properties' active members.
// Read-only — the scan publishes through the pipeline, it writes nothing
// here.
type PaymentScanStore struct {
	db postgres.DBTX
	scanPopulationStore
}

// NewPaymentScanStore creates the payments scan source adapter over the pool.
func NewPaymentScanStore(db postgres.DBTX) *PaymentScanStore {
	return &PaymentScanStore{db: db, scanPopulationStore: scanPopulationStore{db: db}}
}

// ListScanZones lists the distinct owner timezones having planned
// payment-rule operations on non-archived properties — the sweep targets
// (ADR 0048 p.3).
func (s *PaymentScanStore) ListScanZones(ctx context.Context) ([]application.ScanZone, error) {
	return scanZones(ctx, "payments", postgres.New(s.db).ListPaymentScanZones)
}

// ListDueTargets lists the zone's planned operations dated exactly the
// zone's today (решение #737, тип №2), auto-pay rules excluded.
func (s *PaymentScanStore) ListDueTargets(
	ctx context.Context, zone string, today time.Time,
) ([]application.PaymentScanTarget, error) {
	rows, err := postgres.New(s.db).ListPaymentDueTargets(ctx, postgres.ListPaymentDueTargetsParams{
		Timezone: zone,
		Column2:  pgconv.DateToPgtype(today),
	})
	if err != nil {
		return nil, fmt.Errorf("list due payments of zone %s: %w", zone, err)
	}
	targets := make([]application.PaymentScanTarget, 0, len(rows))
	for _, row := range rows {
		targets = append(targets, paymentScanTarget(
			row.PaymentID, row.Date, row.Title, row.AmountKopecks,
			row.PropertyID, row.PropertyName, row.PropertyAddress, row.OwnerID))
	}
	return targets, nil
}

// ListOverdueTargets lists the zone's planned operations dated strictly
// before the zone's today (решение #737, тип №3), auto-pay rules included.
func (s *PaymentScanStore) ListOverdueTargets(
	ctx context.Context, zone string, today time.Time,
) ([]application.PaymentScanTarget, error) {
	rows, err := postgres.New(s.db).ListPaymentOverdueTargets(ctx, postgres.ListPaymentOverdueTargetsParams{
		Timezone: zone,
		Column2:  pgconv.DateToPgtype(today),
	})
	if err != nil {
		return nil, fmt.Errorf("list overdue payments of zone %s: %w", zone, err)
	}
	targets := make([]application.PaymentScanTarget, 0, len(rows))
	for _, row := range rows {
		targets = append(targets, paymentScanTarget(
			row.PaymentID, row.Date, row.Title, row.AmountKopecks,
			row.PropertyID, row.PropertyName, row.PropertyAddress, row.OwnerID))
	}
	return targets, nil
}

// ListScheduledDueTargets lists the operations whose due boundary — 00:00
// of the operation date in the owner's timezone — falls in the window
// (from, until]; the due leg's booking list (issue #776), auto-pay rules
// excluded.
func (s *PaymentScanStore) ListScheduledDueTargets(
	ctx context.Context, from, until time.Time,
) ([]application.PaymentScheduleTarget, error) {
	rows, err := postgres.New(s.db).ListPaymentScheduledDueTargets(ctx, postgres.ListPaymentScheduledDueTargetsParams{
		Column1: pgconv.TimePtrToPgtype(&from),
		Column2: pgconv.TimePtrToPgtype(&until),
	})
	if err != nil {
		return nil, fmt.Errorf("list scheduled due payments: %w", err)
	}
	targets := make([]application.PaymentScheduleTarget, 0, len(rows))
	for _, row := range rows {
		targets = append(targets, application.PaymentScheduleTarget{
			PaymentID: pgconv.UUIDFromPgtype(row.PaymentID),
			DueDate:   pgconv.DateFromPgtype(row.Date),
			FireAt:    pgconv.TimestamptzToTime(row.FireAt),
		})
	}
	return targets, nil
}

// ListScheduledOverdueTargets lists the operations whose overdue boundary —
// 00:00 of the day after the operation date in the owner's timezone — falls
// in the window (from, until]; the overdue leg's booking list (issue #776),
// auto-pay rules included.
func (s *PaymentScanStore) ListScheduledOverdueTargets(
	ctx context.Context, from, until time.Time,
) ([]application.PaymentScheduleTarget, error) {
	rows, err := postgres.New(s.db).ListPaymentScheduledOverdueTargets(ctx, postgres.ListPaymentScheduledOverdueTargetsParams{
		Column1: pgconv.TimePtrToPgtype(&from),
		Column2: pgconv.TimePtrToPgtype(&until),
	})
	if err != nil {
		return nil, fmt.Errorf("list scheduled overdue payments: %w", err)
	}
	targets := make([]application.PaymentScheduleTarget, 0, len(rows))
	for _, row := range rows {
		targets = append(targets, application.PaymentScheduleTarget{
			PaymentID: pgconv.UUIDFromPgtype(row.PaymentID),
			DueDate:   pgconv.DateFromPgtype(row.Date),
			FireAt:    pgconv.TimestamptzToTime(row.FireAt),
		})
	}
	return targets, nil
}

// GetScheduledDuePayment reloads one operation at its due boundary — the
// due job's delivery-time resolution (issue #776). A paid, cancelled,
// auto-pay, moved or orphaned operation, an archived property and a job
// awake after the day rolled over are pgx.ErrNoRows here and answer
// live=false: the job finishes without publishing.
func (s *PaymentScanStore) GetScheduledDuePayment(
	ctx context.Context, paymentID uuid.UUID, date, now time.Time,
) (application.PaymentScanTarget, bool, error) {
	row, err := postgres.New(s.db).GetScheduledDuePayment(ctx, postgres.GetScheduledDuePaymentParams{
		Column1: pgconv.UUIDToPgtype(paymentID),
		Column2: pgconv.DateToPgtype(date),
		Column3: pgconv.TimePtrToPgtype(&now),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.PaymentScanTarget{}, false, nil
		}
		return application.PaymentScanTarget{}, false, fmt.Errorf("load scheduled due payment %s: %w", paymentID, err)
	}
	return paymentScanTarget(
		row.PaymentID, row.Date, row.Title, row.AmountKopecks,
		row.PropertyID, row.PropertyName, row.PropertyAddress, row.OwnerID), true, nil
}

// GetScheduledOverduePayment reloads one operation at its overdue boundary
// — the overdue job's delivery-time resolution (issue #776). A paid,
// cancelled, moved or orphaned operation, an archived property and a job
// awake before the day's end are pgx.ErrNoRows here and answer live=false.
func (s *PaymentScanStore) GetScheduledOverduePayment(
	ctx context.Context, paymentID uuid.UUID, date, now time.Time,
) (application.PaymentScanTarget, bool, error) {
	row, err := postgres.New(s.db).GetScheduledOverduePayment(ctx, postgres.GetScheduledOverduePaymentParams{
		Column1: pgconv.UUIDToPgtype(paymentID),
		Column2: pgconv.DateToPgtype(date),
		Column3: pgconv.TimePtrToPgtype(&now),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.PaymentScanTarget{}, false, nil
		}
		return application.PaymentScanTarget{}, false, fmt.Errorf("load scheduled overdue payment %s: %w", paymentID, err)
	}
	return paymentScanTarget(
		row.PaymentID, row.Date, row.Title, row.AmountKopecks,
		row.PropertyID, row.PropertyName, row.PropertyAddress, row.OwnerID), true, nil
}

// ListReminderTargets lists the zone's planned operations of rules with a
// reminder set whose reminder day — the operation date minus the rule's lead
// time — is exactly the zone's today (карта #822, #824), auto-pay rules
// included (решение #823: напоминание независимо от auto_pay).
func (s *PaymentScanStore) ListReminderTargets(
	ctx context.Context, zone string, today time.Time,
) ([]application.PaymentScanTarget, error) {
	rows, err := postgres.New(s.db).ListPaymentReminderTargets(ctx, postgres.ListPaymentReminderTargetsParams{
		Timezone: zone,
		Column2:  pgconv.DateToPgtype(today),
	})
	if err != nil {
		return nil, fmt.Errorf("list reminder payments of zone %s: %w", zone, err)
	}
	targets := make([]application.PaymentScanTarget, 0, len(rows))
	for _, row := range rows {
		targets = append(targets, paymentScanTarget(
			row.PaymentID, row.Date, row.Title, row.AmountKopecks,
			row.PropertyID, row.PropertyName, row.PropertyAddress, row.OwnerID))
	}
	return targets, nil
}

// ListScheduledReminderTargets lists the operations whose reminder boundary
// — 00:00 of (operation date − lead time) in the owner's timezone — falls in
// the window (from, until]; the reminder leg's booking list (карта #822),
// auto-pay rules included.
func (s *PaymentScanStore) ListScheduledReminderTargets(
	ctx context.Context, from, until time.Time,
) ([]application.PaymentScheduleTarget, error) {
	rows, err := postgres.New(s.db).ListPaymentScheduledReminderTargets(ctx, postgres.ListPaymentScheduledReminderTargetsParams{
		Column1: pgconv.TimePtrToPgtype(&from),
		Column2: pgconv.TimePtrToPgtype(&until),
	})
	if err != nil {
		return nil, fmt.Errorf("list scheduled reminder payments: %w", err)
	}
	targets := make([]application.PaymentScheduleTarget, 0, len(rows))
	for _, row := range rows {
		targets = append(targets, application.PaymentScheduleTarget{
			PaymentID: pgconv.UUIDFromPgtype(row.PaymentID),
			DueDate:   pgconv.DateFromPgtype(row.Date),
			FireAt:    pgconv.TimestamptzToTime(row.FireAt),
		})
	}
	return targets, nil
}

// GetScheduledReminderPayment reloads one operation at its reminder boundary
// (карта #822) — the reminder job's delivery-time resolution. A paid,
// cancelled, moved or orphaned operation, an archived property, a lead time
// changed after the booking (the rule's current reminder day is no longer
// today) and a job awake after the day rolled over are pgx.ErrNoRows here
// and answer live=false.
func (s *PaymentScanStore) GetScheduledReminderPayment(
	ctx context.Context, paymentID uuid.UUID, date, now time.Time,
) (application.PaymentScanTarget, bool, error) {
	row, err := postgres.New(s.db).GetScheduledReminderPayment(ctx, postgres.GetScheduledReminderPaymentParams{
		Column1: pgconv.UUIDToPgtype(paymentID),
		Column2: pgconv.DateToPgtype(date),
		Column3: pgconv.TimePtrToPgtype(&now),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.PaymentScanTarget{}, false, nil
		}
		return application.PaymentScanTarget{}, false, fmt.Errorf("load scheduled reminder payment %s: %w", paymentID, err)
	}
	return paymentScanTarget(
		row.PaymentID, row.Date, row.Title, row.AmountKopecks,
		row.PropertyID, row.PropertyName, row.PropertyAddress, row.OwnerID), true, nil
}

// paymentScanTarget maps one scan row's fields to the application target.
// The two sqlc legs (due, overdue) share the select shape but not the
// generated row type, so the mapping travels by value.
func paymentScanTarget(
	paymentID pgtype.UUID, date pgtype.Date, title string, amountKopecks int64,
	propertyID pgtype.UUID, propertyName, propertyAddress string, ownerID pgtype.UUID,
) application.PaymentScanTarget {
	return application.PaymentScanTarget{
		PaymentID:       pgconv.UUIDFromPgtype(paymentID),
		DueDate:         pgconv.DateFromPgtype(date),
		Title:           title,
		AmountKopecks:   amountKopecks,
		PropertyID:      pgconv.UUIDFromPgtype(propertyID),
		PropertyName:    propertyName,
		PropertyAddress: propertyAddress,
		OwnerID:         pgconv.UUIDFromPgtype(ownerID),
	}
}
