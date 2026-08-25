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
	payment, err := mapPaymentRow(paymentRowFields{
		ID: row.ID, OwnerID: row.OwnerID, PropertyID: row.PropertyID, Type: row.Type,
		Title: row.Title, AmountKopecks: row.AmountKopecks, Recurrence: row.Recurrence,
		Since: row.Since, EndDate: row.EndDate, AutoPay: row.AutoPay,
		PaymentForm: row.PaymentForm, CategorySlug: row.CategorySlug,
		UserCategoryID:   row.UserCategoryID,
		UserCategoryName: row.UserCategoryName,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	})
	if err != nil {
		return domain.Payment{}, err
	}
	return s.withPauses(ctx, payment)
}

// ListByProperty returns the property's rules in creation order, each with
// its pause intervals.
func (s *PaymentStore) ListByProperty(
	ctx context.Context, scope, propertyID uuid.UUID,
) ([]domain.Payment, error) {
	rows, err := s.q().ListPaymentsByProperty(ctx, postgres.ListPaymentsByPropertyParams{
		OwnerID:    pgconv.UUIDToPgtype(scope),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
	if err != nil {
		return nil, fmt.Errorf("list payments of property %s: %w", propertyID, err)
	}
	payments := make([]domain.Payment, 0, len(rows))
	for _, row := range rows {
		payment, err := mapPaymentRow(paymentRowFields{
			ID: row.ID, OwnerID: row.OwnerID, PropertyID: row.PropertyID, Type: row.Type,
			Title: row.Title, AmountKopecks: row.AmountKopecks, Recurrence: row.Recurrence,
			Since: row.Since, EndDate: row.EndDate, AutoPay: row.AutoPay,
			PaymentForm: row.PaymentForm, CategorySlug: row.CategorySlug,
			UserCategoryID:   row.UserCategoryID,
			UserCategoryName: row.UserCategoryName,
			CreatedAt:        row.CreatedAt,
			UpdatedAt:        row.UpdatedAt,
		})
		if err != nil {
			return nil, err
		}
		payments = append(payments, payment)
	}
	return s.attachPauses(ctx, payments)
}

// Create inserts a new rule; the recurrence jsonb goes through the domain's
// canonical marshaling.
func (s *PaymentStore) Create(ctx context.Context, p domain.Payment) error {
	recurrence, err := marshalRecurrence(p)
	if err != nil {
		return err
	}
	if err := s.q().InsertPayment(ctx, postgres.InsertPaymentParams{
		ID:            pgconv.UUIDToPgtype(p.ID),
		OwnerID:       pgconv.UUIDToPgtype(p.OwnerID),
		PropertyID:    pgconv.UUIDToPgtype(p.PropertyID),
		Type:          string(p.Type),
		Title:         p.Title,
		AmountKopecks: p.AmountKopecks,
		Recurrence:    recurrence,
		Since:         pgconv.DateToPgtype(p.Since),
		EndDate:       pgconv.DatePtrToPgtype(p.EndDate),
		AutoPay:       p.AutoPay,
		PaymentForm:   string(p.PaymentForm),
		CategorySlug:  pgconv.StringPtrToPgtype(p.Category.Slug),
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
		ID:            pgconv.UUIDToPgtype(p.ID),
		OwnerID:       pgconv.UUIDToPgtype(p.OwnerID),
		Type:          string(p.Type),
		Title:         p.Title,
		AmountKopecks: p.AmountKopecks,
		Recurrence:    recurrence,
		EndDate:       pgconv.DatePtrToPgtype(p.EndDate),
		AutoPay:       p.AutoPay,
		PaymentForm:   string(p.PaymentForm),
		CategorySlug:  pgconv.StringPtrToPgtype(p.Category.Slug),
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

// marshalRecurrence encodes the rule's recurrence through the json.Marshaler
// interface. The domain MarshalJSON has a pointer receiver — marshaling the
// bare value would fall back to the empty struct (the fields are
// unexported) — and routing through the interface keeps the error check
// honest: an unknown kind is a real, if unreachable, marshal failure.
func marshalRecurrence(p domain.Payment) ([]byte, error) {
	var marshaler json.Marshaler = &p.Recurrence
	return json.Marshal(marshaler)
}

// withPauses loads the single rule's pause intervals.
func (s *PaymentStore) withPauses(ctx context.Context, payment domain.Payment) (domain.Payment, error) {
	payments, err := s.attachPauses(ctx, []domain.Payment{payment})
	if err != nil {
		return domain.Payment{}, err
	}
	return payments[0], nil
}

// attachPauses loads the pause intervals of the listed rules and attaches
// them, keyed by rule id.
func (s *PaymentStore) attachPauses(ctx context.Context, payments []domain.Payment) ([]domain.Payment, error) {
	if len(payments) == 0 {
		return payments, nil
	}
	ids := paymentIDs(payments)
	rows, err := s.q().ListTickPausesByPaymentIDs(ctx, pgconv.UUIDSliceToPgtype(ids))
	if err != nil {
		return nil, fmt.Errorf("list pauses: %w", err)
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
	return payments, nil
}

// paymentRowFields decouples the two CRUD row shapes (Get and List select the
// same columns, sqlc generates one struct per query) from the shared mapper.
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

// mapPaymentRow maps a CRUD row to the domain rule: the recurrence jsonb goes
// through the domain constructors, so a stored row can never resurrect an
// invalid variant (same guarantee as the tick mapping).
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
