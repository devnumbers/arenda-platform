// Package postgres holds the postgres adapters of the Rentals context (ADR
// 0053): the rental store over the sqlc-generated queries.
package postgres

// RentalStore is the postgres adapter of the rentals persistence port (ADR
// 0053 §1, ticket #529). Reads and writes are scoped by the data owner and
// the nested rental→property path lives in the queries; the tenant fields
// arrive resolved by the store's LEFT JOIN and travel inside domain.Rental.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/rentals/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/rentals/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// Compile-time conformance of the adapter to the consumer-declared port.
var _ application.RentalStore = (*RentalStore)(nil)

// RentalStore is the postgres adapter of the rentals port.
type RentalStore struct {
	db postgres.DBTX
}

// NewRentalStore creates a rental store over the given connection or pool.
func NewRentalStore(db postgres.DBTX) *RentalStore {
	return &RentalStore{db: db}
}

func (s *RentalStore) q() *postgres.Queries {
	return postgres.New(s.db)
}

// WithTx returns a store bound to the provided transaction.
func (s *RentalStore) WithTx(tx transaction.Tx) (application.RentalStore, error) {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return nil, fmt.Errorf("rentals.RentalStore.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewRentalStore(dbtx), nil
}

// Get loads one rental with its tenant resolved; pgx.ErrNoRows — an unknown
// id, another owner's rental or another property's rental — becomes the
// application ErrNotFound.
func (s *RentalStore) Get(ctx context.Context, id, scope, propertyID uuid.UUID) (domain.Rental, error) {
	row, err := s.q().GetRentalByID(ctx, postgres.GetRentalByIDParams{
		ID:         pgconv.UUIDToPgtype(id),
		OwnerID:    pgconv.UUIDToPgtype(scope),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Rental{}, application.ErrNotFound
		}
		return domain.Rental{}, fmt.Errorf("get rental %s: %w", id, err)
	}
	rental, err := mapRentalRow(rentalFieldsFromGet(row))
	if err != nil {
		return domain.Rental{}, fmt.Errorf("map rental %s: %w", id, err)
	}
	return rental, nil
}

// ListByProperty returns the property's rentals: unfinished first (newest
// start on top), then the completed ones by completion date, fresh on top.
func (s *RentalStore) ListByProperty(
	ctx context.Context, scope, propertyID uuid.UUID,
) ([]domain.Rental, error) {
	rows, err := s.q().ListRentalsByProperty(ctx, postgres.ListRentalsByPropertyParams{
		OwnerID:    pgconv.UUIDToPgtype(scope),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
	if err != nil {
		return nil, fmt.Errorf("list rentals of property %s: %w", propertyID, err)
	}
	rentals := make([]domain.Rental, 0, len(rows))
	for _, row := range rows {
		rental, err := mapRentalRow(rentalFieldsFromList(row))
		if err != nil {
			return nil, fmt.Errorf("map rental of property %s: %w", propertyID, err)
		}
		rentals = append(rentals, rental)
	}
	return rentals, nil
}

// Create inserts the rental; the id, owner and payment link are app-side.
func (s *RentalStore) Create(ctx context.Context, r domain.Rental) error {
	if err := s.q().InsertRental(ctx, postgres.InsertRentalParams{
		ID:                pgconv.UUIDToPgtype(r.ID),
		OwnerID:           pgconv.UUIDToPgtype(r.OwnerID),
		PropertyID:        pgconv.UUIDToPgtype(r.PropertyID),
		PaymentID:         pgconv.UUIDToPgtypePtr(r.PaymentID),
		ContactID:         pgconv.UUIDToPgtypePtr(r.ContactID),
		StartDate:         pgconv.DateToPgtype(r.StartDate),
		PlannedEndDate:    pgconv.DatePtrToPgtype(r.PlannedEndDate),
		Utilities:         string(r.Utilities),
		DepositKopecks:    pgconv.Int8PtrToPgtype(r.DepositKopecks),
		CommissionKopecks: pgconv.Int8PtrToPgtype(r.CommissionKopecks),
		Comment:           pgconv.StringPtrToPgtype(stringPtr(r.Comment)),
	}); err != nil {
		return fmt.Errorf("insert rental %s: %w", r.ID, err)
	}
	return nil
}

// Update writes the editable fields of the rental (the start date is not
// among them; completion belongs to Complete). The change step has already
// proven existence inside the same transaction and lock, so rows affected is
// not checked.
func (s *RentalStore) Update(ctx context.Context, r domain.Rental) error {
	if err := s.q().UpdateRental(ctx, postgres.UpdateRentalParams{
		ID:                pgconv.UUIDToPgtype(r.ID),
		OwnerID:           pgconv.UUIDToPgtype(r.OwnerID),
		ContactID:         pgconv.UUIDToPgtypePtr(r.ContactID),
		PlannedEndDate:    pgconv.DatePtrToPgtype(r.PlannedEndDate),
		Utilities:         string(r.Utilities),
		DepositKopecks:    pgconv.Int8PtrToPgtype(r.DepositKopecks),
		CommissionKopecks: pgconv.Int8PtrToPgtype(r.CommissionKopecks),
		Comment:           pgconv.StringPtrToPgtype(stringPtr(r.Comment)),
	}); err != nil {
		return fmt.Errorf("update rental %s: %w", r.ID, err)
	}
	return nil
}

// Complete writes the completion fact, the optional deposit return and the
// terms archive in one UPDATE and releases the payment link (the payment
// dies in the same transaction, ревизия #1161); rows affected is not checked
// for the same reason as Update.
func (s *RentalStore) Complete(
	ctx context.Context, id, scope uuid.UUID, completedDate time.Time,
	depositReturn *application.DepositReturn, archived domain.TermsArchive,
) error {
	var amount *int64
	var comment *string
	if depositReturn != nil {
		amount = &depositReturn.AmountKopecks
		comment = depositReturn.Comment
	}
	day := paymentDayCode(archived.PaymentDay)
	if _, err := s.q().CompleteRental(ctx, postgres.CompleteRentalParams{
		ID:                   pgconv.UUIDToPgtype(id),
		OwnerID:              pgconv.UUIDToPgtype(scope),
		CompletedDate:        pgconv.DateToPgtype(completedDate),
		DepositReturnKopecks: pgconv.Int8PtrToPgtype(amount),
		DepositReturnComment: pgconv.StringPtrToPgtype(comment),
		RentAmountKopecks:    pgconv.Int8PtrToPgtype(&archived.AmountKopecks),
		RentPaymentDay:       pgconv.Int4PtrToPgtype(&day),
		RentAutoPay:          pgtype.Bool{Bool: archived.AutoPay, Valid: true},
	}); err != nil {
		return fmt.Errorf("complete rental %s: %w", id, err)
	}
	return nil
}

// paymentDayCode encodes the domain payment day onto the archive column's
// 1..31 spelling: the last-day marker stores as 31 — the two spellings are
// one behaviour (решение №5).
func paymentDayCode(d domain.PaymentDay) int {
	if d.IsLast() {
		return 31
	}
	return d.Day()
}

// termsArchivePresent reports the row carries the completion's archive: the
// schema CHECK keeps the three columns present exactly together on a
// completed rental, and a missing part (corrupt data the CHECK should have
// caught) reads as no archive — the view layer fails loudly on it.
func termsArchivePresent(amount pgtype.Int8, day pgtype.Int4, autoPay pgtype.Bool) bool {
	return amount.Valid && day.Valid && autoPay.Valid
}

// termsArchiveFromRow decodes the archive columns onto the domain snapshot.
// An out-of-range payment day is the durable invariant and the data
// disagreeing — the read fails loudly, with the cause.
func termsArchiveFromRow(amount pgtype.Int8, day pgtype.Int4, autoPay pgtype.Bool) (*domain.TermsArchive, error) {
	paymentDay, err := domain.NewPaymentDay(int(day.Int32))
	if err != nil {
		return nil, fmt.Errorf("decode terms archive payment day %d: %w", day.Int32, err)
	}
	return &domain.TermsArchive{
		AmountKopecks: amount.Int64,
		PaymentDay:    paymentDay,
		AutoPay:       autoPay.Bool,
	}, nil
}

// Delete removes the rental row; it must run before the managed payment's own
// delete (the RESTRICT FK releases only afterwards, ADR 0053 §3).
func (s *RentalStore) Delete(ctx context.Context, id, scope uuid.UUID) error {
	if _, err := s.q().DeleteRental(ctx, postgres.DeleteRentalParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(scope),
	}); err != nil {
		return fmt.Errorf("delete rental %s: %w", id, err)
	}
	return nil
}

// HasUnfinished reports the property's unfinished rental (invariant №12) —
// the create-time app check under the property lock.
func (s *RentalStore) HasUnfinished(ctx context.Context, scope, propertyID uuid.UUID) (bool, error) {
	exists, err := s.q().ExistsUnfinishedRental(ctx, postgres.ExistsUnfinishedRentalParams{
		OwnerID:    pgconv.UUIDToPgtype(scope),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
	if err != nil {
		return false, fmt.Errorf("check unfinished rental of property %s: %w", propertyID, err)
	}
	return exists, nil
}

// stringPtr lifts the rental comment onto the nullable pgconv shape: an empty
// comment is the stored NULL.
func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// mapRentalRow projects a query row onto the domain rental, the tenant view
// included. The error is the terms archive's payment-day decode: the column
// is CHECK-constrained to 1..31, so a failure here means the durable
// invariant and the data disagree — the read fails loudly with the cause.
func mapRentalRow(row rentalRowFields) (domain.Rental, error) {
	var archive *domain.TermsArchive
	if termsArchivePresent(row.RentAmountKopecks, row.RentPaymentDay, row.RentAutoPay) {
		var err error
		archive, err = termsArchiveFromRow(row.RentAmountKopecks, row.RentPaymentDay, row.RentAutoPay)
		if err != nil {
			return domain.Rental{}, err
		}
	}
	return domain.Rental{
		ID:                   pgconv.UUIDFromPgtype(row.ID),
		OwnerID:              pgconv.UUIDFromPgtype(row.OwnerID),
		PropertyID:           pgconv.UUIDFromPgtype(row.PropertyID),
		PaymentID:            pgconv.UUIDFromPgtypePtr(row.PaymentID),
		ContactID:            pgconv.UUIDFromPgtypePtr(row.ContactID),
		StartDate:            pgconv.DateFromPgtype(row.StartDate),
		PlannedEndDate:       pgconv.DatePtrFromPgtype(row.PlannedEndDate),
		CompletedDate:        pgconv.DatePtrFromPgtype(row.CompletedDate),
		Utilities:            domain.Utilities(row.Utilities),
		DepositKopecks:       pgconv.Int8ToPtr(row.DepositKopecks),
		CommissionKopecks:    pgconv.Int8ToPtr(row.CommissionKopecks),
		DepositReturnKopecks: pgconv.Int8ToPtr(row.DepositReturnKopecks),
		DepositReturnComment: pgconv.TextToPtrString(row.DepositReturnComment),
		TermsArchive:         archive,
		Comment:              pgconv.TextToString(row.Comment),
		Tenant:               tenantFromRow(row),
		CreatedAt:            pgconv.TimestamptzToTime(row.CreatedAt),
		UpdatedAt:            pgconv.TimestamptzToTime(row.UpdatedAt),
	}, nil
}

// tenantFromRow assembles the embedded tenant from the joined contact
// fields; any NULL part means the link is gone — «Контакта нет».
func tenantFromRow(row rentalRowFields) *domain.TenantContact {
	contactID := pgconv.UUIDFromPgtypePtr(row.ContactID)
	if contactID == nil {
		return nil
	}
	return &domain.TenantContact{
		ContactID: *contactID,
		FirstName: pgconv.TextToString(row.TenantFirstName),
		LastName:  pgconv.TextToString(row.TenantLastName),
		Phone:     pgconv.TextToString(row.TenantPhone),
	}
}
