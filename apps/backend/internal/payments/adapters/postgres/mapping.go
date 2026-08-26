package postgres

// The single mapping of a payments table row to the domain rule. Both query
// families of the adapter — the tick's owner listing and the CRUD reads —
// select the same columns (the tick's rows additionally lack the timestamps),
// so sqlc generates several row types over one shape; each query converts its
// row into paymentRowFields below and the domain mapping happens exactly
// once.

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// paymentRowFields is the column shape shared by the tick listing and the
// CRUD reads (sqlc generates one struct per query; the converters below fold
// them into this one).
type paymentRowFields struct {
	ID               pgtype.UUID
	OwnerID          pgtype.UUID
	PropertyID       pgtype.UUID
	Type             string
	Title            string
	AmountKopecks    int64
	Recurrence       []byte
	Since            pgtype.Date
	EndDate          pgtype.Date
	AutoPay          bool
	PaymentForm      string
	CategorySlug     pgtype.Text
	UserCategoryID   pgtype.UUID
	UserCategoryName pgtype.Text
	CreatedAt        pgtype.Timestamptz
	UpdatedAt        pgtype.Timestamptz
}

// paymentFieldsFromTickRow converts a tick listing row; the tick's rows carry
// no timestamps, so they stay zero (the tick never reads them).
func paymentFieldsFromTickRow(row postgres.ListTickPaymentsByOwnerRow) paymentRowFields {
	return paymentRowFields{
		ID: row.ID, OwnerID: row.OwnerID, PropertyID: row.PropertyID, Type: row.Type,
		Title: row.Title, AmountKopecks: row.AmountKopecks, Recurrence: row.Recurrence,
		Since: row.Since, EndDate: row.EndDate, AutoPay: row.AutoPay,
		PaymentForm: row.PaymentForm, CategorySlug: row.CategorySlug,
		UserCategoryID:   row.UserCategoryID,
		UserCategoryName: row.UserCategoryName,
	}
}

// paymentFieldsFromGetRow converts a single-rule read row (with timestamps).
func paymentFieldsFromGetRow(row postgres.GetPaymentByIDRow) paymentRowFields {
	return paymentRowFields{
		ID: row.ID, OwnerID: row.OwnerID, PropertyID: row.PropertyID, Type: row.Type,
		Title: row.Title, AmountKopecks: row.AmountKopecks, Recurrence: row.Recurrence,
		Since: row.Since, EndDate: row.EndDate, AutoPay: row.AutoPay,
		PaymentForm: row.PaymentForm, CategorySlug: row.CategorySlug,
		UserCategoryID:   row.UserCategoryID,
		UserCategoryName: row.UserCategoryName,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
}

// paymentFieldsFromListRow converts a property listing row (with timestamps).
func paymentFieldsFromListRow(row postgres.ListPaymentsByPropertyRow) paymentRowFields {
	return paymentRowFields{
		ID: row.ID, OwnerID: row.OwnerID, PropertyID: row.PropertyID, Type: row.Type,
		Title: row.Title, AmountKopecks: row.AmountKopecks, Recurrence: row.Recurrence,
		Since: row.Since, EndDate: row.EndDate, AutoPay: row.AutoPay,
		PaymentForm: row.PaymentForm, CategorySlug: row.CategorySlug,
		UserCategoryID:   row.UserCategoryID,
		UserCategoryName: row.UserCategoryName,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
}

// mapPaymentRow maps the shared row shape to the domain rule: the recurrence
// jsonb goes through the domain constructors, so a stored row can never
// resurrect an invalid variant.
func mapPaymentRow(row paymentRowFields) (domain.Payment, error) {
	var recurrence domain.Recurrence
	if err := json.Unmarshal(row.Recurrence, &recurrence); err != nil {
		return domain.Payment{}, fmt.Errorf("parse recurrence of payment %s: %w", pgconv.UUIDFromPgtype(row.ID), err)
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
		CreatedAt: pgconv.TimestamptzToTime(row.CreatedAt),
		UpdatedAt: pgconv.TimestamptzToTime(row.UpdatedAt),
	}, nil
}

// attachPauses loads the pause intervals of the listed rules over the caller's
// connection (or transaction) and attaches them, keyed by rule id.
func attachPauses(ctx context.Context, q *postgres.Queries, payments []domain.Payment) error {
	if len(payments) == 0 {
		return nil
	}
	ids := paymentIDs(payments)
	rows, err := q.ListTickPausesByPaymentIDs(ctx, pgconv.UUIDSliceToPgtype(ids))
	if err != nil {
		return fmt.Errorf("list pauses: %w", err)
	}
	pauses := make(map[uuid.UUID][]domain.PauseInterval, len(rows))
	for _, row := range rows {
		id := pgconv.UUIDFromPgtype(row.PaymentID)
		pauses[id] = append(pauses[id], domain.PauseInterval{
			From: pgconv.DateFromPgtype(row.FromDate),
			To:   pgconv.DatePtrFromPgtype(row.ToDate),
		})
	}
	for i := range payments {
		payments[i].Pauses = pauses[payments[i].ID]
	}
	return nil
}

// paymentIDs collects the rules' identifiers for the batch listings.
func paymentIDs(payments []domain.Payment) []uuid.UUID {
	ids := make([]uuid.UUID, len(payments))
	for i, p := range payments {
		ids[i] = p.ID
	}
	return ids
}
