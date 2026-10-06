package postgres

// The rentalRowFields struct is the common projection of the read queries:
// both read queries project the same columns (the tenant join included), so
// the shared field struct carries one mapping (the payments adapter's
// paymentFieldsFrom* pattern).

import (
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
)

// rentalRowFields is the common projection of the rentals read queries.
type rentalRowFields struct {
	ID                   pgtype.UUID
	OwnerID              pgtype.UUID
	PropertyID           pgtype.UUID
	PaymentID            pgtype.UUID
	ContactID            pgtype.UUID
	StartDate            pgtype.Date
	PlannedEndDate       pgtype.Date
	CompletedDate        pgtype.Date
	Utilities            string
	DepositKopecks       pgtype.Int8
	CommissionKopecks    pgtype.Int8
	DepositReturnKopecks pgtype.Int8
	DepositReturnComment pgtype.Text
	RentAmountKopecks    pgtype.Int8
	RentPaymentDay       pgtype.Int4
	RentAutoPay          pgtype.Bool
	Comment              pgtype.Text
	CreatedAt            pgtype.Timestamptz
	UpdatedAt            pgtype.Timestamptz
	TenantFirstName      pgtype.Text
	TenantLastName       pgtype.Text
	TenantPhone          pgtype.Text
}

// rentalFieldsFromGet lifts the single-rental query row.
func rentalFieldsFromGet(row postgres.GetRentalByIDRow) rentalRowFields {
	return rentalRowFields{
		ID: row.ID, OwnerID: row.OwnerID, PropertyID: row.PropertyID,
		PaymentID: row.PaymentID, ContactID: row.ContactID,
		StartDate: row.StartDate, PlannedEndDate: row.PlannedEndDate,
		CompletedDate: row.CompletedDate, Utilities: row.Utilities,
		DepositKopecks: row.DepositKopecks, CommissionKopecks: row.CommissionKopecks,
		DepositReturnKopecks: row.DepositReturnKopecks,
		DepositReturnComment: row.DepositReturnComment,
		RentAmountKopecks:    row.RentAmountKopecks, RentPaymentDay: row.RentPaymentDay,
		RentAutoPay: row.RentAutoPay,
		Comment:     row.Comment, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		TenantFirstName: row.TenantFirstName, TenantLastName: row.TenantLastName,
		TenantPhone: row.TenantPhone,
	}
}

// rentalFieldsFromList lifts one list query row.
func rentalFieldsFromList(row postgres.ListRentalsByPropertyRow) rentalRowFields {
	return rentalRowFields{
		ID: row.ID, OwnerID: row.OwnerID, PropertyID: row.PropertyID,
		PaymentID: row.PaymentID, ContactID: row.ContactID,
		StartDate: row.StartDate, PlannedEndDate: row.PlannedEndDate,
		CompletedDate: row.CompletedDate, Utilities: row.Utilities,
		DepositKopecks: row.DepositKopecks, CommissionKopecks: row.CommissionKopecks,
		DepositReturnKopecks: row.DepositReturnKopecks,
		DepositReturnComment: row.DepositReturnComment,
		RentAmountKopecks:    row.RentAmountKopecks, RentPaymentDay: row.RentPaymentDay,
		RentAutoPay: row.RentAutoPay,
		Comment:     row.Comment, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		TenantFirstName: row.TenantFirstName, TenantLastName: row.TenantLastName,
		TenantPhone: row.TenantPhone,
	}
}
