package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// LeaseRepository persists leases.
type LeaseRepository struct {
	db postgres.DBTX
}

// NewLeaseRepository creates a new lease repository.
func NewLeaseRepository(db postgres.DBTX) *LeaseRepository {
	return &LeaseRepository{db: db}
}

func (r *LeaseRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *LeaseRepository) WithTx(tx transaction.Tx) application.LeaseRepository {
	return NewLeaseRepository(tx.(postgres.DBTX))
}

func (r *LeaseRepository) Create(ctx context.Context, ownerID uuid.UUID, lease domain.Lease) (domain.Lease, error) {
	row, err := r.q().CreateLease(ctx, postgres.CreateLeaseParams{
		OwnerID:              uuidToPgtype(ownerID),
		PropertyID:           uuidToPgtype(lease.PropertyID),
		TenantContactID:      uuidPtrToPgtype(lease.TenantContactID),
		Status:               string(lease.Status),
		StartDate:            dateToPgtype(lease.StartDate),
		EndDate:              datePtrToPgtype(lease.EndDate),
		RentAmountKopecks:    lease.RentAmountKopecks,
		DepositAmountKopecks: lease.DepositAmountKopecks,
		//nolint:gosec // PaymentDay is validated to be 1-31 in domain.
		PaymentDay: int32(lease.PaymentDay),
		Comment:    pgtype.Text{String: lease.Comment, Valid: true},
	})
	if err != nil {
		return domain.Lease{}, err
	}
	return leaseFromRow(row), nil
}

func (r *LeaseRepository) GetByIDAndOwner(ctx context.Context, id, ownerID uuid.UUID) (domain.Lease, error) {
	row, err := r.q().GetLeaseByIDAndOwner(ctx, postgres.GetLeaseByIDAndOwnerParams{
		ID:      uuidToPgtype(id),
		OwnerID: uuidToPgtype(ownerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Lease{}, application.ErrNotFound
		}
		return domain.Lease{}, err
	}
	return leaseFromRow(row), nil
}

func (r *LeaseRepository) ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]domain.Lease, error) {
	rows, err := r.q().ListLeasesByOwner(ctx, uuidToPgtype(ownerID))
	if err != nil {
		return nil, err
	}
	leases := make([]domain.Lease, 0, len(rows))
	for _, row := range rows {
		leases = append(leases, leaseFromRow(row))
	}
	return leases, nil
}

func (r *LeaseRepository) Update(ctx context.Context, ownerID uuid.UUID, lease domain.Lease) (domain.Lease, error) {
	row, err := r.q().UpdateLease(ctx, postgres.UpdateLeaseParams{
		ID:                   uuidToPgtype(lease.ID),
		OwnerID:              uuidToPgtype(ownerID),
		PropertyID:           uuidToPgtype(lease.PropertyID),
		TenantContactID:      uuidPtrToPgtype(lease.TenantContactID),
		Status:               string(lease.Status),
		StartDate:            dateToPgtype(lease.StartDate),
		EndDate:              datePtrToPgtype(lease.EndDate),
		RentAmountKopecks:    lease.RentAmountKopecks,
		DepositAmountKopecks: lease.DepositAmountKopecks,
		//nolint:gosec // PaymentDay is validated to be 1-31 in domain.
		PaymentDay: int32(lease.PaymentDay),
		Comment:    pgtype.Text{String: lease.Comment, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Lease{}, application.ErrNotFound
		}
		return domain.Lease{}, err
	}
	return leaseFromRow(row), nil
}

func (r *LeaseRepository) Complete(ctx context.Context, id, ownerID uuid.UUID) (domain.Lease, error) {
	row, err := r.q().CompleteLease(ctx, postgres.CompleteLeaseParams{
		ID:      uuidToPgtype(id),
		OwnerID: uuidToPgtype(ownerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Lease{}, application.ErrNotFound
		}
		return domain.Lease{}, err
	}
	return leaseFromRow(row), nil
}

func (r *LeaseRepository) CountOpenLeasesByProperty(ctx context.Context, propertyID uuid.UUID) (int, error) {
	count, err := r.q().CountOpenLeasesByProperty(ctx, uuidToPgtype(propertyID))
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (r *LeaseRepository) GetOpenLeaseByProperty(ctx context.Context, propertyID uuid.UUID) (domain.Lease, error) {
	row, err := r.q().GetOpenLeaseByProperty(ctx, uuidToPgtype(propertyID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Lease{}, application.ErrNotFound
		}
		return domain.Lease{}, err
	}
	return leaseFromRow(row), nil
}

func leaseFromRow(row postgres.Lease) domain.Lease {
	status, _ := domain.ParseLeaseStatus(row.Status)
	return domain.Lease{
		ID:                   uuidFromPgtype(row.ID),
		OwnerID:              uuidFromPgtype(row.OwnerID),
		PropertyID:           uuidFromPgtype(row.PropertyID),
		TenantContactID:      uuidPtrFromPgtype(row.TenantContactID),
		Status:               status,
		StartDate:            dateFromPgtype(row.StartDate),
		EndDate:              datePtrFromPgtype(row.EndDate),
		RentAmountKopecks:    row.RentAmountKopecks,
		DepositAmountKopecks: row.DepositAmountKopecks,
		PaymentDay:           int(row.PaymentDay),
		Comment:              textToString(row.Comment),
		CreatedAt:            row.CreatedAt.Time,
		UpdatedAt:            row.UpdatedAt.Time,
	}
}

// TenantContactRepository reads tenant contacts.
type TenantContactRepository struct {
	db postgres.DBTX
}

// NewTenantContactRepository creates a new tenant contact repository.
func NewTenantContactRepository(db postgres.DBTX) *TenantContactRepository {
	return &TenantContactRepository{db: db}
}

func (r *TenantContactRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *TenantContactRepository) WithTx(tx transaction.Tx) application.TenantContactRepository {
	return NewTenantContactRepository(tx.(postgres.DBTX))
}

func (r *TenantContactRepository) Create(ctx context.Context, ownerID uuid.UUID, contact domain.TenantContact) (domain.TenantContact, error) {
	row, err := r.q().CreateTenantContact(ctx, postgres.CreateTenantContactParams{
		OwnerID:    uuidToPgtype(ownerID),
		Name:       contact.Name,
		Surname:    stringPtrToPgtype(contact.Surname),
		Patronymic: stringPtrToPgtype(contact.Patronymic),
		Phone:      stringPtrToPgtype(contact.Phone),
		Email:      stringPtrToPgtype(contact.Email),
		Comment:    stringPtrToPgtype(contact.Comment),
	})
	if err != nil {
		return domain.TenantContact{}, err
	}
	return tenantContactFromRow(row), nil
}

func (r *TenantContactRepository) GetByIDAndOwner(ctx context.Context, id, ownerID uuid.UUID) (domain.TenantContact, error) {
	row, err := r.q().GetTenantContactByIDAndOwner(ctx, postgres.GetTenantContactByIDAndOwnerParams{
		ID:      uuidToPgtype(id),
		OwnerID: uuidToPgtype(ownerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.TenantContact{}, application.ErrNotFound
		}
		return domain.TenantContact{}, err
	}
	return tenantContactFromRow(row), nil
}

func (r *TenantContactRepository) ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]domain.TenantContact, error) {
	rows, err := r.q().ListTenantContactsByOwner(ctx, uuidToPgtype(ownerID))
	if err != nil {
		return nil, err
	}
	contacts := make([]domain.TenantContact, 0, len(rows))
	for _, row := range rows {
		contacts = append(contacts, tenantContactFromRow(row))
	}
	return contacts, nil
}

func tenantContactFromRow(row postgres.TenantContact) domain.TenantContact {
	return domain.TenantContact{
		ID:         uuidFromPgtype(row.ID),
		OwnerID:    uuidFromPgtype(row.OwnerID),
		Name:       row.Name,
		Surname:    stringPtrFromText(row.Surname),
		Patronymic: stringPtrFromText(row.Patronymic),
		Phone:      stringPtrFromText(row.Phone),
		Email:      stringPtrFromText(row.Email),
		Comment:    stringPtrFromText(row.Comment),
		CreatedAt:  row.CreatedAt.Time,
		UpdatedAt:  row.UpdatedAt.Time,
	}
}

// RecurringOperationRepository persists recurring operations.
type RecurringOperationRepository struct {
	db postgres.DBTX
}

// NewRecurringOperationRepository creates a new recurring operation repository.
func NewRecurringOperationRepository(db postgres.DBTX) *RecurringOperationRepository {
	return &RecurringOperationRepository{db: db}
}

func (r *RecurringOperationRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *RecurringOperationRepository) WithTx(tx transaction.Tx) application.RecurringOperationRepository {
	return NewRecurringOperationRepository(tx.(postgres.DBTX))
}

func (r *RecurringOperationRepository) Create(ctx context.Context, op domain.RecurringOperation) (domain.RecurringOperation, error) {
	row, err := r.q().CreateRecurringOperation(ctx, postgres.CreateRecurringOperationParams{
		OwnerID:       uuidToPgtype(op.OwnerID),
		PropertyID:    uuidToPgtype(op.PropertyID),
		LeaseID:       uuidToPgtype(op.LeaseID),
		Type:          op.Type,
		Category:      op.Category,
		AmountKopecks: op.AmountKopecks,
		StartDate:     dateToPgtype(op.StartDate),
		//nolint:gosec // PaymentDay is validated to be 1-31 in domain.
		PaymentDay: int32(op.PaymentDay),
		EndDate:    datePtrToPgtype(op.EndDate),
	})
	if err != nil {
		return domain.RecurringOperation{}, err
	}
	return recurringOperationFromRow(row), nil
}

func (r *RecurringOperationRepository) GetByLease(ctx context.Context, leaseID uuid.UUID) (domain.RecurringOperation, error) {
	rows, err := r.q().GetRecurringOperationByLease(ctx, uuidToPgtype(leaseID))
	if err != nil {
		return domain.RecurringOperation{}, err
	}
	if len(rows) == 0 {
		return domain.RecurringOperation{}, application.ErrNotFound
	}
	return recurringOperationFromRow(rows[0]), nil
}

func (r *RecurringOperationRepository) Update(ctx context.Context, op domain.RecurringOperation) (domain.RecurringOperation, error) {
	row, err := r.q().UpdateRecurringOperation(ctx, postgres.UpdateRecurringOperationParams{
		ID:            uuidToPgtype(op.ID),
		Type:          op.Type,
		Category:      op.Category,
		AmountKopecks: op.AmountKopecks,
		StartDate:     dateToPgtype(op.StartDate),
		//nolint:gosec // PaymentDay is validated to be 1-31 in domain.
		PaymentDay: int32(op.PaymentDay),
		EndDate:    datePtrToPgtype(op.EndDate),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RecurringOperation{}, application.ErrNotFound
		}
		return domain.RecurringOperation{}, err
	}
	return recurringOperationFromRow(row), nil
}

func (r *RecurringOperationRepository) DeleteByLease(ctx context.Context, leaseID uuid.UUID) error {
	return r.q().DeleteRecurringOperationByLease(ctx, uuidToPgtype(leaseID))
}

func recurringOperationFromRow(row postgres.RecurringOperation) domain.RecurringOperation {
	return domain.RecurringOperation{
		ID:            uuidFromPgtype(row.ID),
		OwnerID:       uuidFromPgtype(row.OwnerID),
		PropertyID:    uuidFromPgtype(row.PropertyID),
		LeaseID:       uuidFromPgtype(row.LeaseID),
		Type:          row.Type,
		Category:      row.Category,
		AmountKopecks: row.AmountKopecks,
		StartDate:     dateFromPgtype(row.StartDate),
		PaymentDay:    int(row.PaymentDay),
		EndDate:       datePtrFromPgtype(row.EndDate),
		CreatedAt:     row.CreatedAt.Time,
		UpdatedAt:     row.UpdatedAt.Time,
	}
}

// OperationRepository persists operations.
type OperationRepository struct {
	db postgres.DBTX
}

// NewOperationRepository creates a new operation repository.
func NewOperationRepository(db postgres.DBTX) *OperationRepository {
	return &OperationRepository{db: db}
}

func (r *OperationRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *OperationRepository) WithTx(tx transaction.Tx) application.OperationRepository {
	return NewOperationRepository(tx.(postgres.DBTX))
}

func (r *OperationRepository) BulkCreate(ctx context.Context, ops []domain.Operation) error {
	for _, op := range ops {
		_, err := r.q().CreateOperation(ctx, postgres.CreateOperationParams{
			OwnerID:              uuidToPgtype(op.OwnerID),
			PropertyID:           uuidToPgtype(op.PropertyID),
			LeaseID:              uuidToPgtype(op.LeaseID),
			RecurringOperationID: uuidToPgtype(op.RecurringOperationID),
			Type:                 op.Type,
			Category:             op.Category,
			AmountKopecks:        op.AmountKopecks,
			OperationDate:        dateToPgtype(op.OperationDate),
			Comment:              pgtype.Text{String: op.Comment, Valid: true},
			IsException:          op.IsException,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *OperationRepository) ListByLease(ctx context.Context, leaseID uuid.UUID) ([]domain.Operation, error) {
	rows, err := r.q().ListOperationsByLease(ctx, uuidToPgtype(leaseID))
	if err != nil {
		return nil, err
	}
	ops := make([]domain.Operation, 0, len(rows))
	for _, row := range rows {
		ops = append(ops, operationFromRow(row))
	}
	return ops, nil
}

func (r *OperationRepository) DeleteUneditedFutureOperationsByLease(ctx context.Context, leaseID uuid.UUID, after time.Time) error {
	return r.q().DeleteUneditedFutureOperationsByLease(ctx, postgres.DeleteUneditedFutureOperationsByLeaseParams{
		LeaseID:       uuidToPgtype(leaseID),
		OperationDate: dateToPgtype(after),
	})
}

func (r *OperationRepository) DeleteOperationsOutsideLeaseRange(ctx context.Context, leaseID uuid.UUID, start time.Time, end *time.Time) error {
	var endDate pgtype.Date
	if end != nil {
		endDate = dateToPgtype(*end)
	}
	return r.q().DeleteOperationsOutsideLeaseRange(ctx, postgres.DeleteOperationsOutsideLeaseRangeParams{
		LeaseID:       uuidToPgtype(leaseID),
		OperationDate: dateToPgtype(start),
		Column3:       endDate,
	})
}

func (r *OperationRepository) DeleteUneditedOperationsByLease(ctx context.Context, leaseID uuid.UUID) error {
	return r.q().DeleteUneditedOperationsByLease(ctx, uuidToPgtype(leaseID))
}

func operationFromRow(row postgres.Operation) domain.Operation {
	return domain.Operation{
		ID:                   uuidFromPgtype(row.ID),
		OwnerID:              uuidFromPgtype(row.OwnerID),
		PropertyID:           uuidFromPgtype(row.PropertyID),
		LeaseID:              uuidFromPgtype(row.LeaseID),
		RecurringOperationID: uuidFromPgtype(row.RecurringOperationID),
		Type:                 row.Type,
		Category:             row.Category,
		AmountKopecks:        row.AmountKopecks,
		OperationDate:        dateFromPgtype(row.OperationDate),
		Comment:              textToString(row.Comment),
		IsException:          row.IsException,
		CreatedAt:            row.CreatedAt.Time,
		UpdatedAt:            row.UpdatedAt.Time,
	}
}

// PropertyRepository provides property information needed by the lease bounded context.
type PropertyRepository struct {
	db postgres.DBTX
}

// NewPropertyRepository creates a new property repository for the lease context.
func NewPropertyRepository(db postgres.DBTX) *PropertyRepository {
	return &PropertyRepository{db: db}
}

func (r *PropertyRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// ExistsActiveByOwner reports whether an active property exists for the owner.
func (r *PropertyRepository) ExistsActiveByOwner(ctx context.Context, id, ownerID uuid.UUID) (bool, error) {
	row, err := r.q().GetPropertyByIDAndOwner(ctx, postgres.GetPropertyByIDAndOwnerParams{
		ID:      uuidToPgtype(id),
		OwnerID: uuidToPgtype(ownerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return row.Status == "active", nil
}

// HasOpenLease reports whether the property currently has an open lease.
func (r *PropertyRepository) HasOpenLease(ctx context.Context, id uuid.UUID) (bool, error) {
	count, err := r.q().CountOpenLeasesByProperty(ctx, uuidToPgtype(id))
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func uuidToPgtype(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: u != uuid.UUID{}}
}

func uuidPtrToPgtype(u *uuid.UUID) pgtype.UUID {
	if u == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *u, Valid: true}
}

func uuidFromPgtype(u pgtype.UUID) uuid.UUID {
	if !u.Valid {
		return uuid.UUID{}
	}
	return uuid.UUID(u.Bytes)
}

func uuidPtrFromPgtype(u pgtype.UUID) *uuid.UUID {
	if !u.Valid {
		return nil
	}
	v := uuid.UUID(u.Bytes)
	return &v
}

func dateToPgtype(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}

func datePtrToPgtype(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *t, Valid: true}
}

func dateFromPgtype(d pgtype.Date) time.Time {
	return d.Time
}

func datePtrFromPgtype(d pgtype.Date) *time.Time {
	if !d.Valid {
		return nil
	}
	t := d.Time
	return &t
}

func textToString(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

func stringPtrFromText(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

func stringPtrToPgtype(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}
