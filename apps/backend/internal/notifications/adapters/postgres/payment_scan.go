package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
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

// PaymentScanStore answers the payments scan's questions (#749) over the
// owning tables directly: the sweep targets' zones, the zone's due-day
// operations, the zone's overdue ones and the properties' active members.
// Read-only — the scan publishes through the pipeline, it writes nothing
// here.
type PaymentScanStore struct {
	db postgres.DBTX
}

// NewPaymentScanStore creates the payments scan source adapter over the pool.
func NewPaymentScanStore(db postgres.DBTX) *PaymentScanStore {
	return &PaymentScanStore{db: db}
}

// ListScanZones lists the distinct owner timezones having planned
// payment-rule operations on non-archived properties — the sweep targets
// (ADR 0048 p.3).
func (s *PaymentScanStore) ListScanZones(ctx context.Context) ([]application.ScanZone, error) {
	zones, err := postgres.New(s.db).ListPaymentScanZones(ctx)
	if err != nil {
		return nil, fmt.Errorf("list payments scan zones: %w", err)
	}
	result := make([]application.ScanZone, 0, len(zones))
	for _, timezone := range zones {
		result = append(result, application.ScanZone{Timezone: timezone})
	}
	return result, nil
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

// ListActiveRecipients lists the property's active members' user ids — the
// event's recipients besides the owner (решение #737: «Просмотр» включён,
// suspended is not an active participant).
func (s *PaymentScanStore) ListActiveRecipients(ctx context.Context, propertyID uuid.UUID) ([]uuid.UUID, error) {
	ids, err := postgres.New(s.db).ListPropertyActiveRecipients(ctx, pgconv.UUIDToPgtype(propertyID))
	if err != nil {
		return nil, fmt.Errorf("list property recipients %s: %w", propertyID, err)
	}
	return pgconv.UUIDSliceFromPgtype(ids), nil
}
