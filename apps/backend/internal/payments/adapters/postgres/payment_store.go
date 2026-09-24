package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
var _ application.PaymentStore = (*PaymentStore)(nil)

// PaymentStore is the postgres adapter of the payment rules port (ADR 0049
// §4). Reads and writes are scoped by the data owner and the nested
// payment→property path lives in the queries.
type PaymentStore struct {
	db postgres.DBTX
}

// NewPaymentStore creates a payment store over the given connection or pool.
func NewPaymentStore(db postgres.DBTX) *PaymentStore {
	return &PaymentStore{db: db}
}

func (s *PaymentStore) q() *postgres.Queries {
	return postgres.New(s.db)
}

// WithTx returns a store bound to the provided transaction.
func (s *PaymentStore) WithTx(tx transaction.Tx) (application.PaymentStore, error) {
	dbtx, ok := tx.(postgres.DBTX)
	if !ok {
		return nil, fmt.Errorf("payments.PaymentStore.WithTx: %T is not a postgres.DBTX", tx)
	}
	return NewPaymentStore(dbtx), nil
}

// Get loads one rule with its pause intervals; pgx.ErrNoRows — an unknown id,
// another owner's rule or another property's rule — becomes the application
// ErrNotFound.
func (s *PaymentStore) Get(
	ctx context.Context, id, scope, propertyID uuid.UUID,
) (domain.Payment, error) {
	row, err := s.q().GetPaymentByID(ctx, postgres.GetPaymentByIDParams{
		ID:         pgconv.UUIDToPgtype(id),
		OwnerID:    pgconv.UUIDToPgtype(scope),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Payment{}, application.ErrNotFound
		}
		return domain.Payment{}, fmt.Errorf("get payment %s: %w", id, err)
	}
	payment, err := mapPaymentRow(paymentFieldsFromGetRow(row))
	if err != nil {
		return domain.Payment{}, err
	}
	payments, err := attachPausesAsSlice(ctx, s, payment)
	if err != nil {
		return domain.Payment{}, err
	}
	return payments[0], nil
}

// ListByProperty returns the property's rules in creation order, each with
// its pause intervals; search (” = no filter) is a case-insensitive
// substring filter on the title.
func (s *PaymentStore) ListByProperty(
	ctx context.Context, scope, propertyID uuid.UUID, search string,
) ([]domain.Payment, error) {
	rows, err := s.q().ListPaymentsByProperty(ctx, postgres.ListPaymentsByPropertyParams{
		OwnerID:    pgconv.UUIDToPgtype(scope),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
		Search:     escapeLikePattern(search),
	})
	if err != nil {
		return nil, fmt.Errorf("list payments of property %s: %w", propertyID, err)
	}
	payments := make([]domain.Payment, 0, len(rows))
	for _, row := range rows {
		payment, err := mapPaymentRow(paymentFieldsFromListRow(row))
		if err != nil {
			return nil, err
		}
		payments = append(payments, payment)
	}
	return attachPausesAsSlice(ctx, s, payments...)
}

// Create inserts a new rule; the recurrence jsonb goes through the domain's
// canonical marshaling.
func (s *PaymentStore) Create(ctx context.Context, p domain.Payment) error {
	recurrence, err := marshalRecurrence(p)
	if err != nil {
		return err
	}
	if err := s.q().InsertPayment(ctx, postgres.InsertPaymentParams{
		ID:                 pgconv.UUIDToPgtype(p.ID),
		OwnerID:            pgconv.UUIDToPgtype(p.OwnerID),
		PropertyID:         pgconv.UUIDToPgtype(p.PropertyID),
		Type:               string(p.Type),
		Title:              p.Title,
		AmountKopecks:      p.AmountKopecks,
		Recurrence:         recurrence,
		Since:              pgconv.DateToPgtype(p.Since),
		EndDate:            pgconv.DatePtrToPgtype(p.EndDate),
		AutoPay:            p.AutoPay,
		ReminderOffsetDays: pgconv.Int4PtrToPgtype(p.ReminderOffsetDays),
		PaymentForm:        string(p.PaymentForm),
		CategorySlug:       pgconv.StringPtrToPgtype(p.Category.Slug),
	}); err != nil {
		return fmt.Errorf("insert payment %s: %w", p.ID, err)
	}
	return nil
}

// Update writes the editable fields of the rule; since is not among them.
func (s *PaymentStore) Update(ctx context.Context, p domain.Payment) error {
	recurrence, err := marshalRecurrence(p)
	if err != nil {
		return err
	}
	if err := s.q().UpdatePayment(ctx, postgres.UpdatePaymentParams{
		ID:                 pgconv.UUIDToPgtype(p.ID),
		OwnerID:            pgconv.UUIDToPgtype(p.OwnerID),
		Type:               string(p.Type),
		Title:              p.Title,
		AmountKopecks:      p.AmountKopecks,
		Recurrence:         recurrence,
		EndDate:            pgconv.DatePtrToPgtype(p.EndDate),
		AutoPay:            p.AutoPay,
		ReminderOffsetDays: pgconv.Int4PtrToPgtype(p.ReminderOffsetDays),
		PaymentForm:        string(p.PaymentForm),
		CategorySlug:       pgconv.StringPtrToPgtype(p.Category.Slug),
	}); err != nil {
		return fmt.Errorf("update payment %s: %w", p.ID, err)
	}
	return nil
}

// Delete removes the rule; rows affected is not checked — the use case has
// already proven existence inside the same transaction and lock.
func (s *PaymentStore) Delete(ctx context.Context, id, scope uuid.UUID) error {
	if _, err := s.q().DeletePayment(ctx, postgres.DeletePaymentParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(scope),
	}); err != nil {
		return fmt.Errorf("delete payment %s: %w", id, err)
	}
	return nil
}

// InsertPause opens the open-ended pause; the pause id is minted here.
func (s *PaymentStore) InsertPause(ctx context.Context, paymentID uuid.UUID, from time.Time) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("mint pause id: %w", err)
	}
	if err := s.q().InsertPaymentPause(ctx, postgres.InsertPaymentPauseParams{
		ID:        pgconv.UUIDToPgtype(id),
		PaymentID: pgconv.UUIDToPgtype(paymentID),
		FromDate:  pgconv.DateToPgtype(from),
	}); err != nil {
		return fmt.Errorf("insert pause of payment %s: %w", paymentID, err)
	}
	return nil
}

// CloseActivePause closes the open pause with the resume day.
func (s *PaymentStore) CloseActivePause(ctx context.Context, paymentID uuid.UUID, resumeDay time.Time) error {
	if _, err := s.q().CloseActivePaymentPause(ctx, postgres.CloseActivePaymentPauseParams{
		PaymentID: pgconv.UUIDToPgtype(paymentID),
		ToDate:    pgconv.DateToPgtype(resumeDay),
	}); err != nil {
		return fmt.Errorf("close active pause of payment %s: %w", paymentID, err)
	}
	return nil
}

// DeletePlannedFrom removes the rule's planned operations from a date on.
func (s *PaymentStore) DeletePlannedFrom(ctx context.Context, paymentID uuid.UUID, from time.Time) error {
	if _, err := s.q().DeletePaymentPlannedFrom(ctx, postgres.DeletePaymentPlannedFromParams{
		PaymentID: pgconv.UUIDToPgtype(paymentID),
		Date:      pgconv.DateToPgtype(from),
	}); err != nil {
		return fmt.Errorf("delete planned from %s of payment %s: %w",
			from.Format(time.DateOnly), paymentID, err)
	}
	return nil
}

// DeletePlannedBefore removes the rule's planned operations before a date.
func (s *PaymentStore) DeletePlannedBefore(ctx context.Context, paymentID uuid.UUID, before time.Time) error {
	if _, err := s.q().DeletePaymentPlannedBefore(ctx, postgres.DeletePaymentPlannedBeforeParams{
		PaymentID: pgconv.UUIDToPgtype(paymentID),
		Date:      pgconv.DateToPgtype(before),
	}); err != nil {
		return fmt.Errorf("delete planned before %s of payment %s: %w",
			before.Format(time.DateOnly), paymentID, err)
	}
	return nil
}

// DeleteFuturePlanned removes the rule's strictly future planned operations.
func (s *PaymentStore) DeleteFuturePlanned(ctx context.Context, paymentID uuid.UUID, today time.Time) error {
	if _, err := s.q().DeletePaymentFuturePlanned(ctx, postgres.DeletePaymentFuturePlannedParams{
		PaymentID: pgconv.UUIDToPgtype(paymentID),
		Date:      pgconv.DateToPgtype(today),
	}); err != nil {
		return fmt.Errorf("delete future planned of payment %s: %w", paymentID, err)
	}
	return nil
}

// SetFavorite writes the favorite star in one atomic UPDATE (PUT favorite,
// ticket #461); rows affected is not checked — the conveyor has already
// proven the rule's existence inside the same transaction and lock.
func (s *PaymentStore) SetFavorite(ctx context.Context, id, scope uuid.UUID, favorite bool) error {
	if _, err := s.q().SetPaymentFavorite(ctx, postgres.SetPaymentFavoriteParams{
		ID:         pgconv.UUIDToPgtype(id),
		OwnerID:    pgconv.UUIDToPgtype(scope),
		IsFavorite: favorite,
	}); err != nil {
		return fmt.Errorf("set favorite of payment %s: %w", id, err)
	}
	return nil
}

// attachPausesAsSlice loads the pause intervals of the given rules over the
// store's connection and returns them back (the shared attachPauses mutates
// in place; this adapts it to the value-returning store methods).
func attachPausesAsSlice(ctx context.Context, s *PaymentStore, payments ...domain.Payment) ([]domain.Payment, error) {
	if err := attachPauses(ctx, s.q(), payments); err != nil {
		return nil, err
	}
	return payments, nil
}

// marshalRecurrence encodes the rule's recurrence through the json.Marshaler
// interface. The domain MarshalJSON has a pointer receiver — marshaling the
// bare value would fall back to the empty struct (the fields are
// unexported) — and routing through the interface keeps the error check
// honest: an unknown kind is a real, if unreachable, marshal failure.
func marshalRecurrence(p domain.Payment) ([]byte, error) {
	var marshaler json.Marshaler = &p.Recurrence
	return json.Marshal(marshaler)
}

// likePatternEscaper escapes the ILIKE metacharacters in user-supplied search
// text. The matching SQL patterns use ESCAPE '\'.
var likePatternEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// escapeLikePattern trims and escapes a user-supplied substring so it can be
// safely embedded in an ILIKE '%...%' pattern. An empty result disables the
// filter on the SQL side.
func escapeLikePattern(q string) string {
	return likePatternEscaper.Replace(strings.TrimSpace(q))
}
