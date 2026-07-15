package postgres

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
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
		OwnerID:              pgconv.UUIDToPgtype(ownerID),
		PropertyID:           pgconv.UUIDToPgtype(lease.PropertyID),
		TenantContactID:      pgconv.UUIDToPgtypePtr(lease.TenantContactID),
		Status:               string(lease.Status),
		StartDate:            pgconv.DateToPgtype(lease.StartDate),
		EndDate:              pgconv.DatePtrToPgtype(lease.EndDate),
		RentAmountKopecks:    lease.RentAmountKopecks,
		DepositAmountKopecks: lease.DepositAmountKopecks,
		//nolint:gosec // PaymentDay is validated to be 1-31 in domain.
		PaymentDay: int32(lease.PaymentDay),
		Comment:    pgtype.Text{String: lease.Comment, Valid: true},
	})
	if err != nil {
		if isOpenLeaseUniqueViolation(err) {
			return domain.Lease{}, application.ErrOpenLeaseExists
		}
		return domain.Lease{}, err
	}
	return leaseFromRow(row)
}

func (r *LeaseRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Lease, error) {
	row, err := r.q().GetLeaseByID(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Lease{}, application.ErrNotFound
		}
		return domain.Lease{}, err
	}
	return leaseFromRow(row)
}

func (r *LeaseRepository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.Lease, error) {
	row, err := r.q().GetLeaseByIDForUpdate(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Lease{}, application.ErrNotFound
		}
		return domain.Lease{}, err
	}
	return leaseFromRow(row)
}

func (r *LeaseRepository) GetByIDAndOwner(ctx context.Context, id, ownerID uuid.UUID) (domain.Lease, error) {
	row, err := r.q().GetLeaseByIDAndOwner(ctx, postgres.GetLeaseByIDAndOwnerParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(ownerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Lease{}, application.ErrNotFound
		}
		return domain.Lease{}, err
	}
	return leaseFromRow(row)
}

func (r *LeaseRepository) GetByIDAndOwnerForUpdate(ctx context.Context, id, ownerID uuid.UUID) (domain.Lease, error) {
	row, err := r.q().GetLeaseByIDAndOwnerForUpdate(ctx, postgres.GetLeaseByIDAndOwnerForUpdateParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(ownerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Lease{}, application.ErrNotFound
		}
		return domain.Lease{}, err
	}
	return leaseFromRow(row)
}

func (r *LeaseRepository) ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]domain.Lease, error) {
	rows, err := r.q().ListLeasesByOwner(ctx, pgconv.UUIDToPgtype(ownerID))
	if err != nil {
		return nil, err
	}
	leases := make([]domain.Lease, 0, len(rows))
	for _, row := range rows {
		lease, err := leaseFromRow(row)
		if err != nil {
			return nil, err
		}
		leases = append(leases, lease)
	}
	return leases, nil
}

func (r *LeaseRepository) ListByProperty(ctx context.Context, ownerID, propertyID uuid.UUID) ([]domain.Lease, error) {
	rows, err := r.q().ListLeasesByProperty(ctx, postgres.ListLeasesByPropertyParams{
		OwnerID:    pgconv.UUIDToPgtype(ownerID),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
	if err != nil {
		return nil, err
	}
	leases := make([]domain.Lease, 0, len(rows))
	for _, row := range rows {
		lease, err := leaseFromRow(row)
		if err != nil {
			return nil, err
		}
		leases = append(leases, lease)
	}
	return leases, nil
}

func (r *LeaseRepository) Update(ctx context.Context, ownerID uuid.UUID, lease domain.Lease) (domain.Lease, error) {
	row, err := r.q().UpdateLease(ctx, postgres.UpdateLeaseParams{
		ID:                   pgconv.UUIDToPgtype(lease.ID),
		OwnerID:              pgconv.UUIDToPgtype(ownerID),
		PropertyID:           pgconv.UUIDToPgtype(lease.PropertyID),
		TenantContactID:      pgconv.UUIDToPgtypePtr(lease.TenantContactID),
		Status:               string(lease.Status),
		StartDate:            pgconv.DateToPgtype(lease.StartDate),
		EndDate:              pgconv.DatePtrToPgtype(lease.EndDate),
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
	return leaseFromRow(row)
}

func (r *LeaseRepository) Complete(ctx context.Context, id, ownerID uuid.UUID) (domain.Lease, error) {
	row, err := r.q().CompleteLease(ctx, postgres.CompleteLeaseParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(ownerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Lease{}, application.ErrNotFound
		}
		return domain.Lease{}, err
	}
	return leaseFromRow(row)
}

func (r *LeaseRepository) CountOpenLeasesByProperty(ctx context.Context, propertyID uuid.UUID) (int, error) {
	count, err := r.q().CountOpenLeasesByProperty(ctx, pgconv.UUIDToPgtype(propertyID))
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (r *LeaseRepository) GetOpenLeaseByProperty(ctx context.Context, ownerID, propertyID uuid.UUID) (domain.Lease, error) {
	row, err := r.q().GetOpenLeaseByProperty(ctx, postgres.GetOpenLeaseByPropertyParams{
		OwnerID:    pgconv.UUIDToPgtype(ownerID),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Lease{}, application.ErrNotFound
		}
		return domain.Lease{}, err
	}
	return leaseFromRow(row)
}

func (r *LeaseRepository) ListOpenLeasesWithPastEndDate(ctx context.Context, asOf time.Time, limit int) ([]domain.Lease, error) {
	rows, err := r.q().ListOpenLeasesWithPastEndDate(ctx, postgres.ListOpenLeasesWithPastEndDateParams{
		AsOf: pgconv.DateToPgtype(asOf),
		//nolint:gosec // Reconciliation batch size is configured and bounded.
		Limit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	leases := make([]domain.Lease, 0, len(rows))
	for _, row := range rows {
		lease, err := leaseFromRow(row)
		if err != nil {
			return nil, err
		}
		leases = append(leases, lease)
	}
	return leases, nil
}

func leaseFromRow(row postgres.Lease) (domain.Lease, error) {
	status, err := domain.ParseLeaseStatus(row.Status)
	if err != nil {
		return domain.Lease{}, fmt.Errorf("invalid lease status in database: %w", err)
	}
	return domain.Lease{
		ID:                   pgconv.UUIDFromPgtype(row.ID),
		OwnerID:              pgconv.UUIDFromPgtype(row.OwnerID),
		PropertyID:           pgconv.UUIDFromPgtype(row.PropertyID),
		TenantContactID:      pgconv.UUIDFromPgtypePtr(row.TenantContactID),
		Status:               status,
		StartDate:            row.StartDate.Time,
		EndDate:              pgconv.DatePtrFromPgtype(row.EndDate),
		RentAmountKopecks:    row.RentAmountKopecks,
		DepositAmountKopecks: row.DepositAmountKopecks,
		PaymentDay:           int(row.PaymentDay),
		Comment:              pgconv.TextToString(row.Comment),
		CreatedAt:            row.CreatedAt.Time,
		UpdatedAt:            row.UpdatedAt.Time,
	}, nil
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
		OwnerID:    pgconv.UUIDToPgtype(ownerID),
		Name:       contact.Name,
		Surname:    pgconv.StringPtrToPgtype(contact.Surname),
		Patronymic: pgconv.StringPtrToPgtype(contact.Patronymic),
		Phone:      pgconv.StringPtrToPgtype(contact.Phone),
		Email:      pgconv.StringPtrToPgtype(contact.Email),
		Comment:    pgconv.StringPtrToPgtype(contact.Comment),
	})
	if err != nil {
		if isDuplicatePhoneError(err) {
			return domain.TenantContact{}, application.ErrDuplicatePhone
		}
		return domain.TenantContact{}, err
	}
	return tenantContactFromRow(row), nil
}

func (r *TenantContactRepository) GetByIDAndOwner(ctx context.Context, id, ownerID uuid.UUID) (domain.TenantContact, error) {
	row, err := r.q().GetTenantContactByIDAndOwner(ctx, postgres.GetTenantContactByIDAndOwnerParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(ownerID),
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
	rows, err := r.q().ListTenantContactsByOwner(ctx, pgconv.UUIDToPgtype(ownerID))
	if err != nil {
		return nil, err
	}
	contacts := make([]domain.TenantContact, 0, len(rows))
	for _, row := range rows {
		contacts = append(contacts, tenantContactFromRow(row))
	}
	return contacts, nil
}

func (r *TenantContactRepository) ListByIDs(ctx context.Context, ownerID uuid.UUID, ids []uuid.UUID) ([]domain.TenantContact, error) {
	pgIDs := make([]pgtype.UUID, 0, len(ids))
	for _, id := range ids {
		pgIDs = append(pgIDs, pgconv.UUIDToPgtype(id))
	}
	rows, err := r.q().ListTenantContactsByIDs(ctx, postgres.ListTenantContactsByIDsParams{
		OwnerID: pgconv.UUIDToPgtype(ownerID),
		Ids:     pgIDs,
	})
	if err != nil {
		return nil, err
	}
	contacts := make([]domain.TenantContact, 0, len(rows))
	for _, row := range rows {
		contacts = append(contacts, tenantContactFromRow(row))
	}
	return contacts, nil
}

func (r *TenantContactRepository) ListWithLeaseStatus(ctx context.Context, ownerID uuid.UUID) ([]domain.TenantContactWithLeases, error) {
	rows, err := r.q().ListTenantContactsWithLeaseStatus(ctx, pgconv.UUIDToPgtype(ownerID))
	if err != nil {
		return nil, err
	}

	byID := make(map[uuid.UUID]*domain.TenantContactWithLeases)
	order := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		id := pgconv.UUIDFromPgtype(row.ID)
		entry, ok := byID[id]
		if !ok {
			entry = &domain.TenantContactWithLeases{
				TenantContact: domain.TenantContact{
					ID:         id,
					OwnerID:    pgconv.UUIDFromPgtype(row.OwnerID),
					Name:       row.Name,
					Surname:    pgconv.TextToPtrString(row.Surname),
					Patronymic: pgconv.TextToPtrString(row.Patronymic),
					Phone:      pgconv.TextToPtrString(row.Phone),
					Email:      pgconv.TextToPtrString(row.Email),
					Comment:    pgconv.TextToPtrString(row.Comment),
					CreatedAt:  row.CreatedAt.Time,
					UpdatedAt:  row.UpdatedAt.Time,
				},
			}
			byID[id] = entry
			order = append(order, id)
		}

		if !row.LeaseID.Valid {
			continue
		}

		lease, err := leaseFromStatusRow(row)
		if err != nil {
			return nil, err
		}
		if entry.ActiveLease == nil && lease.Status.IsOpen() {
			active := lease
			entry.ActiveLease = &active
		}
		if entry.LastLease == nil && !lease.Status.IsOpen() {
			last := lease
			entry.LastLease = &last
		}
	}

	result := make([]domain.TenantContactWithLeases, 0, len(order))
	for _, id := range order {
		result = append(result, *byID[id])
	}
	return result, nil
}

func (r *TenantContactRepository) Update(ctx context.Context, ownerID uuid.UUID, contact domain.TenantContact) (domain.TenantContact, error) {
	row, err := r.q().UpdateTenantContact(ctx, postgres.UpdateTenantContactParams{
		ID:         pgconv.UUIDToPgtype(contact.ID),
		Name:       contact.Name,
		Surname:    pgconv.StringPtrToPgtype(contact.Surname),
		Patronymic: pgconv.StringPtrToPgtype(contact.Patronymic),
		Phone:      pgconv.StringPtrToPgtype(contact.Phone),
		Email:      pgconv.StringPtrToPgtype(contact.Email),
		Comment:    pgconv.StringPtrToPgtype(contact.Comment),
		OwnerID:    pgconv.UUIDToPgtype(ownerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.TenantContact{}, application.ErrNotFound
		}
		if isDuplicatePhoneError(err) {
			return domain.TenantContact{}, application.ErrDuplicatePhone
		}
		return domain.TenantContact{}, err
	}
	return tenantContactFromRow(row), nil
}

func isDuplicatePhoneError(err error) bool {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		return pgErr.Code == pgerrcode.UniqueViolation &&
			strings.Contains(pgErr.ConstraintName, "tenant_contacts_owner_phone")
	}
	return false
}

func isOpenLeaseUniqueViolation(err error) bool {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		return pgErr.Code == pgerrcode.UniqueViolation &&
			strings.Contains(pgErr.ConstraintName, "idx_leases_one_open_per_property")
	}
	return false
}

func isDuplicateCategoryNameError(err error) bool {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		return pgErr.Code == pgerrcode.UniqueViolation &&
			strings.Contains(pgErr.ConstraintName, "idx_operation_categories_owner_type_lower_name")
	}
	return false
}

func tenantContactFromRow(row postgres.TenantContact) domain.TenantContact {
	return domain.TenantContact{
		ID:         pgconv.UUIDFromPgtype(row.ID),
		OwnerID:    pgconv.UUIDFromPgtype(row.OwnerID),
		Name:       row.Name,
		Surname:    pgconv.TextToPtrString(row.Surname),
		Patronymic: pgconv.TextToPtrString(row.Patronymic),
		Phone:      pgconv.TextToPtrString(row.Phone),
		Email:      pgconv.TextToPtrString(row.Email),
		Comment:    pgconv.TextToPtrString(row.Comment),
		CreatedAt:  row.CreatedAt.Time,
		UpdatedAt:  row.UpdatedAt.Time,
	}
}

func leaseFromStatusRow(row postgres.ListTenantContactsWithLeaseStatusRow) (domain.Lease, error) {
	status, err := domain.ParseLeaseStatus(pgconv.TextToString(row.LeaseStatus))
	if err != nil {
		return domain.Lease{}, fmt.Errorf("invalid lease status in database: %w", err)
	}
	return domain.Lease{
		ID:                   pgconv.UUIDFromPgtype(row.LeaseID),
		OwnerID:              pgconv.UUIDFromPgtype(row.LeaseOwnerID),
		PropertyID:           pgconv.UUIDFromPgtype(row.LeasePropertyID),
		TenantContactID:      pgconv.UUIDFromPgtypePtr(row.ID),
		Status:               status,
		StartDate:            pgconv.DateFromPgtype(row.LeaseStartDate),
		EndDate:              pgconv.DatePtrFromPgtype(row.LeaseEndDate),
		RentAmountKopecks:    row.LeaseRentAmountKopecks.Int64,
		DepositAmountKopecks: row.LeaseDepositAmountKopecks.Int64,
		PaymentDay:           int(row.LeasePaymentDay.Int32),
		Comment:              pgconv.TextToString(row.LeaseComment),
		CreatedAt:            row.LeaseCreatedAt.Time,
		UpdatedAt:            row.LeaseUpdatedAt.Time,
	}, nil
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
	params := postgres.CreateRecurringOperationParams{
		OwnerID:       pgconv.UUIDToPgtype(op.OwnerID),
		PropertyID:    pgconv.UUIDToPgtype(op.PropertyID),
		LeaseID:       pgconv.UUIDToPgtype(op.LeaseID),
		Type:          string(op.Type),
		CategoryID:    pgconv.UUIDToPgtype(op.CategoryID),
		Name:          op.Name,
		AmountKopecks: op.AmountKopecks,
		StartDate:     pgconv.DateToPgtype(op.StartDate),
		//nolint:gosec // PaymentDay is validated to be 1-31 in domain.
		PaymentDay:  int32(op.PaymentDay),
		EndDate:     pgconv.DatePtrToPgtype(op.EndDate),
		Periodicity: string(op.Periodicity),
		Status:      string(op.Status),
		Comment:     pgtype.Text{String: op.Comment, Valid: true},
	}
	if op.ReminderOffsetDays != nil {
		//nolint:gosec // Reminder offset is bounded by application validation.
		params.ReminderOffsetDays = pgtype.Int4{Int32: int32(*op.ReminderOffsetDays), Valid: true}
	}
	row, err := r.q().CreateRecurringOperation(ctx, params)
	if err != nil {
		return domain.RecurringOperation{}, err
	}
	return recurringOperationFromRow(row), nil
}

func (r *RecurringOperationRepository) GetByLeaseID(ctx context.Context, ownerID, leaseID uuid.UUID) (domain.RecurringOperation, error) {
	row, err := r.q().GetRecurringOperationByLeaseIDAndOwner(ctx, postgres.GetRecurringOperationByLeaseIDAndOwnerParams{
		LeaseID: pgconv.UUIDToPgtype(leaseID),
		OwnerID: pgconv.UUIDToPgtype(ownerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RecurringOperation{}, application.ErrNotFound
		}
		return domain.RecurringOperation{}, err
	}
	return recurringOperationFromRow(row), nil
}

func (r *RecurringOperationRepository) GetByIDAndOwner(ctx context.Context, id, ownerID uuid.UUID) (domain.RecurringOperation, error) {
	row, err := r.q().GetRecurringOperationByIDAndOwner(ctx, postgres.GetRecurringOperationByIDAndOwnerParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(ownerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RecurringOperation{}, application.ErrNotFound
		}
		return domain.RecurringOperation{}, err
	}
	return recurringOperationFromRow(row), nil
}

func (r *RecurringOperationRepository) GetByIDAndOwnerForUpdate(ctx context.Context, id, ownerID uuid.UUID) (domain.RecurringOperation, error) {
	row, err := r.q().GetRecurringOperationByIDAndOwnerForUpdate(ctx, postgres.GetRecurringOperationByIDAndOwnerForUpdateParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(ownerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RecurringOperation{}, application.ErrNotFound
		}
		return domain.RecurringOperation{}, err
	}
	return recurringOperationFromRow(row), nil
}

func (r *RecurringOperationRepository) ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]domain.RecurringOperation, error) {
	rows, err := r.q().ListRecurringOperationsByOwner(ctx, pgconv.UUIDToPgtype(ownerID))
	if err != nil {
		return nil, err
	}
	ops := make([]domain.RecurringOperation, 0, len(rows))
	for _, row := range rows {
		ops = append(ops, recurringOperationFromRow(row))
	}
	return ops, nil
}

func (r *RecurringOperationRepository) ListByProperty(ctx context.Context, ownerID, propertyID uuid.UUID) ([]domain.RecurringOperation, error) {
	rows, err := r.q().ListRecurringOperationsByProperty(ctx, postgres.ListRecurringOperationsByPropertyParams{
		OwnerID:    pgconv.UUIDToPgtype(ownerID),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
	if err != nil {
		return nil, err
	}
	ops := make([]domain.RecurringOperation, 0, len(rows))
	for _, row := range rows {
		ops = append(ops, recurringOperationFromRow(row))
	}
	return ops, nil
}

func (r *RecurringOperationRepository) Update(ctx context.Context, op domain.RecurringOperation) (domain.RecurringOperation, error) {
	params := postgres.UpdateRecurringOperationParams{
		ID:            pgconv.UUIDToPgtype(op.ID),
		Type:          string(op.Type),
		CategoryID:    pgconv.UUIDToPgtype(op.CategoryID),
		Name:          op.Name,
		AmountKopecks: op.AmountKopecks,
		StartDate:     pgconv.DateToPgtype(op.StartDate),
		//nolint:gosec // PaymentDay is validated to be 1-31 in domain.
		PaymentDay:  int32(op.PaymentDay),
		EndDate:     pgconv.DatePtrToPgtype(op.EndDate),
		Periodicity: string(op.Periodicity),
		Comment:     pgtype.Text{String: op.Comment, Valid: true},
		OwnerID:     pgconv.UUIDToPgtype(op.OwnerID),
	}
	if op.ReminderOffsetDays != nil {
		//nolint:gosec // Reminder offset is bounded by application validation.
		params.ReminderOffsetDays = pgtype.Int4{Int32: int32(*op.ReminderOffsetDays), Valid: true}
	}
	row, err := r.q().UpdateRecurringOperation(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RecurringOperation{}, application.ErrNotFound
		}
		return domain.RecurringOperation{}, err
	}
	return recurringOperationFromRow(row), nil
}

func (r *RecurringOperationRepository) UpdateStatus(ctx context.Context, id, ownerID uuid.UUID, status string) (domain.RecurringOperation, error) {
	row, err := r.q().UpdateRecurringOperationStatus(ctx, postgres.UpdateRecurringOperationStatusParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(ownerID),
		Status:  status,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RecurringOperation{}, application.ErrNotFound
		}
		return domain.RecurringOperation{}, err
	}
	return recurringOperationFromRow(row), nil
}

func (r *RecurringOperationRepository) UpdateStatusByID(ctx context.Context, id uuid.UUID, status string) (domain.RecurringOperation, error) {
	row, err := r.q().UpdateRecurringOperationStatusByID(ctx, postgres.UpdateRecurringOperationStatusByIDParams{
		ID:     pgconv.UUIDToPgtype(id),
		Status: status,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.RecurringOperation{}, application.ErrNotFound
		}
		return domain.RecurringOperation{}, err
	}
	return recurringOperationFromRow(row), nil
}

// UpdateStatusByLeaseID updates the status of all recurring operations
// associated with the given lease and owner.
func (r *RecurringOperationRepository) UpdateStatusByLeaseID(ctx context.Context, leaseID, ownerID uuid.UUID, status string) error {
	_, err := r.q().UpdateRecurringOperationStatusByLeaseID(ctx, postgres.UpdateRecurringOperationStatusByLeaseIDParams{
		Status:  status,
		LeaseID: pgconv.UUIDToPgtype(leaseID),
		OwnerID: pgconv.UUIDToPgtype(ownerID),
	})
	if err != nil {
		return fmt.Errorf("update recurring operation status by lease: %w", err)
	}
	return nil
}

// ListByPropertyID returns all recurring operations for the given property.
func (r *RecurringOperationRepository) ListByPropertyID(ctx context.Context, propertyID uuid.UUID) ([]domain.RecurringOperation, error) {
	rows, err := r.q().ListRecurringOperationsByPropertyID(ctx, pgconv.UUIDToPgtype(propertyID))
	if err != nil {
		return nil, err
	}
	ops := make([]domain.RecurringOperation, 0, len(rows))
	for _, row := range rows {
		ops = append(ops, recurringOperationFromRow(row))
	}
	return ops, nil
}

// SetReminderOffset stores or clears the reminder offset for a recurring operation.
func (r *RecurringOperationRepository) SetReminderOffset(ctx context.Context, ownerID, recID uuid.UUID, offsetDays *int) error {
	var reminderOffsetDays pgtype.Int4
	if offsetDays != nil {
		//nolint:gosec // Reminder offset is bounded by application validation.
		reminderOffsetDays = pgtype.Int4{Int32: int32(*offsetDays), Valid: true}
	}
	_, err := r.q().UpdateRecurringOperationReminderOffset(ctx, postgres.UpdateRecurringOperationReminderOffsetParams{
		ReminderOffsetDays: reminderOffsetDays,
		ID:                 pgconv.UUIDToPgtype(recID),
		OwnerID:            pgconv.UUIDToPgtype(ownerID),
	})
	if err != nil {
		return fmt.Errorf("set reminder offset: %w", err)
	}
	return nil
}

// UpdateStatusByPropertyID updates the status of all recurring operations for
// the given property.
func (r *RecurringOperationRepository) UpdateStatusByPropertyID(ctx context.Context, propertyID uuid.UUID, status string) error {
	return r.q().UpdateRecurringOperationStatusByPropertyID(ctx, postgres.UpdateRecurringOperationStatusByPropertyIDParams{
		Status:     status,
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
}

func (r *RecurringOperationRepository) DeleteByLease(ctx context.Context, leaseID uuid.UUID) error {
	return r.q().DeleteRecurringOperationByLease(ctx, pgconv.UUIDToPgtype(leaseID))
}

func (r *RecurringOperationRepository) SoftDelete(ctx context.Context, id, ownerID uuid.UUID) error {
	_, err := r.q().SoftDeleteRecurringOperation(ctx, postgres.SoftDeleteRecurringOperationParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(ownerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrNotFound
		}
		return err
	}
	return nil
}

func recurringOperationFromRow(row postgres.RecurringOperation) domain.RecurringOperation {
	rec := domain.RecurringOperation{
		ID:            pgconv.UUIDFromPgtype(row.ID),
		OwnerID:       pgconv.UUIDFromPgtype(row.OwnerID),
		PropertyID:    pgconv.UUIDFromPgtype(row.PropertyID),
		LeaseID:       pgconv.UUIDFromPgtype(row.LeaseID),
		Type:          domain.OperationType(row.Type),
		CategoryID:    pgconv.UUIDFromPgtype(row.CategoryID),
		Name:          row.Name,
		AmountKopecks: row.AmountKopecks,
		StartDate:     row.StartDate.Time,
		PaymentDay:    int(row.PaymentDay),
		EndDate:       pgconv.DatePtrFromPgtype(row.EndDate),
		Periodicity:   domain.RecurringOperationPeriodicity(row.Periodicity),
		Status:        domain.RecurringOperationStatus(row.Status),
		Comment:       pgconv.TextToString(row.Comment),
		CreatedAt:     row.CreatedAt.Time,
		UpdatedAt:     row.UpdatedAt.Time,
	}
	if row.ReminderOffsetDays.Valid {
		rec.ReminderOffsetDays = new(int(row.ReminderOffsetDays.Int32))
	}
	if row.DeletedAt.Valid {
		rec.DeletedAt = pgconv.TimestamptzToPtrTime(row.DeletedAt)
	}
	return rec
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

func (r *OperationRepository) Create(ctx context.Context, op domain.Operation) (domain.Operation, error) {
	params := postgres.CreateOperationParams{
		OwnerID:              pgconv.UUIDToPgtype(op.OwnerID),
		PropertyID:           pgconv.UUIDToPgtype(op.PropertyID),
		LeaseID:              pgconv.UUIDToPgtype(op.LeaseID),
		RecurringOperationID: pgconv.UUIDToPgtype(op.RecurringOperationID),
		Type:                 string(op.Type),
		CategoryID:           pgconv.UUIDToPgtype(op.CategoryID),
		Name:                 op.Name,
		AmountKopecks:        op.AmountKopecks,
		OperationDate:        pgconv.DateToPgtype(op.OperationDate),
		SourceOperationDate:  pgconv.DatePtrToPgtype(op.SourceOperationDate),
		Comment:              pgtype.Text{String: op.Comment, Valid: true},
		IsException:          op.IsException,
		Status:               string(op.Status),
	}
	if op.ReminderOffsetDays != nil {
		//nolint:gosec // Reminder offset is bounded by application validation.
		params.ReminderOffsetDays = pgtype.Int4{Int32: int32(*op.ReminderOffsetDays), Valid: true}
	}
	row, err := r.q().CreateOperation(ctx, params)
	if err != nil {
		return domain.Operation{}, err
	}
	return operationFromRow(row)
}

func (r *OperationRepository) BulkCreate(ctx context.Context, ops []domain.Operation) error {
	if len(ops) == 0 {
		return nil
	}

	rows := make([][]any, len(ops))
	for i, op := range ops {
		var reminderOffsetDays pgtype.Int4
		if op.ReminderOffsetDays != nil {
			//nolint:gosec // Reminder offset is bounded by application validation.
			reminderOffsetDays = pgtype.Int4{Int32: int32(*op.ReminderOffsetDays), Valid: true}
		}
		rows[i] = []any{
			pgconv.UUIDToPgtype(op.OwnerID),
			pgconv.UUIDToPgtype(op.PropertyID),
			pgconv.UUIDToPgtype(op.LeaseID),
			pgconv.UUIDToPgtype(op.RecurringOperationID),
			string(op.Type),
			pgconv.UUIDToPgtype(op.CategoryID),
			op.Name,
			op.AmountKopecks,
			pgconv.DateToPgtype(op.OperationDate),
			pgconv.DatePtrToPgtype(op.SourceOperationDate),
			pgtype.Text{String: op.Comment, Valid: true},
			op.IsException,
			string(op.Status),
			reminderOffsetDays,
		}
	}

	copier, ok := r.db.(copyFromer)
	if !ok {
		return fmt.Errorf("bulk create operations: CopyFrom not supported by %T", r.db)
	}

	_, err := copier.CopyFrom(ctx, pgx.Identifier{"operations"}, []string{
		"owner_id", "property_id", "lease_id", "recurring_operation_id",
		"type", "category_id", "name", "amount_kopecks", "operation_date", "source_operation_date", "comment", "is_exception", "status",
		"reminder_offset_days",
	}, pgx.CopyFromRows(rows))
	if err != nil {
		return fmt.Errorf("copy from failed: %w", err)
	}
	return nil
}

type copyFromer interface {
	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
}

func (r *OperationRepository) ListByOwner(ctx context.Context, ownerID uuid.UUID, filter application.OperationFilter) ([]domain.Operation, error) {
	types := make([]string, 0, len(filter.Types))
	for _, t := range filter.Types {
		types = append(types, string(t))
	}
	statuses := make([]string, 0, len(filter.Statuses))
	for _, s := range filter.Statuses {
		statuses = append(statuses, string(s))
	}
	categoryIDs := make([]pgtype.UUID, 0, len(filter.CategoryIDs))
	for _, c := range filter.CategoryIDs {
		categoryIDs = append(categoryIDs, pgconv.UUIDToPgtype(c))
	}

	fromDate := pgconv.DatePtrToPgtype(filter.FromDate)
	toDate := pgconv.DatePtrToPgtype(filter.ToDate)

	//nolint:gosec // Pagination limit is bounded by the API layer.
	limit := int32(filter.Limit)
	if limit <= 0 {
		limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	sort := application.NormalizeOperationSort(filter.Sort)
	var rows []any
	if sort == application.OperationSortOperationDateAsc {
		ascRows, err := r.q().ListOperationsByOwnerAsc(ctx, postgres.ListOperationsByOwnerAscParams{
			OwnerID:                   pgconv.UUIDToPgtype(ownerID),
			Types:                     types,
			Statuses:                  statuses,
			CategoryIds:               categoryIDs,
			PropertyID:                pgconv.UUIDToPgtype(filter.PropertyID),
			LeaseID:                   pgconv.UUIDToPgtype(filter.LeaseID),
			FromDate:                  fromDate,
			ToDate:                    toDate,
			RecurringOperationID:      pgconv.UUIDToPgtype(filter.RecurringOperationID),
			ExcludeArchivedProperties: filter.ExcludeArchivedProperties,
			Limit:                     limit,
			//nolint:gosec // Pagination offset is bounded by the API layer.
			Offset: int32(filter.Offset),
		})
		if err != nil {
			return nil, err
		}
		rows = toAnySlice(ascRows)
	} else {
		descRows, err := r.q().ListOperationsByOwner(ctx, postgres.ListOperationsByOwnerParams{
			OwnerID:                   pgconv.UUIDToPgtype(ownerID),
			Types:                     types,
			Statuses:                  statuses,
			CategoryIds:               categoryIDs,
			PropertyID:                pgconv.UUIDToPgtype(filter.PropertyID),
			LeaseID:                   pgconv.UUIDToPgtype(filter.LeaseID),
			FromDate:                  fromDate,
			ToDate:                    toDate,
			RecurringOperationID:      pgconv.UUIDToPgtype(filter.RecurringOperationID),
			ExcludeArchivedProperties: filter.ExcludeArchivedProperties,
			Limit:                     limit,
			//nolint:gosec // Pagination offset is bounded by the API layer.
			Offset: int32(filter.Offset),
		})
		if err != nil {
			return nil, err
		}
		rows = toAnySlice(descRows)
	}

	ops := make([]domain.Operation, 0, len(rows))
	for _, row := range rows {
		op, err := operationFromAnyRow(row)
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	return ops, nil
}

func (r *OperationRepository) ListByLease(ctx context.Context, leaseID uuid.UUID) ([]domain.Operation, error) {
	rows, err := r.q().ListOperationsByLease(ctx, pgconv.UUIDToPgtype(leaseID))
	if err != nil {
		return nil, err
	}
	ops := make([]domain.Operation, 0, len(rows))
	for _, row := range rows {
		op, err := operationFromAnyRow(row)
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	return ops, nil
}

func (r *OperationRepository) ListByRecurringOperation(ctx context.Context, recurringOperationID uuid.UUID) ([]domain.Operation, error) {
	rows, err := r.q().ListOperationsByRecurringOperation(ctx, pgconv.UUIDToPgtype(recurringOperationID))
	if err != nil {
		return nil, err
	}
	ops := make([]domain.Operation, 0, len(rows))
	for _, row := range rows {
		op, err := operationFromAnyRow(row)
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	return ops, nil
}

func (r *OperationRepository) ListOperationDatesByLease(ctx context.Context, leaseID uuid.UUID) ([]time.Time, error) {
	rows, err := r.q().ListOperationDatesByLease(ctx, pgconv.UUIDToPgtype(leaseID))
	if err != nil {
		return nil, err
	}
	dates := make([]time.Time, 0, len(rows))
	for _, row := range rows {
		dates = append(dates, row.Time)
	}
	return dates, nil
}

func (r *OperationRepository) ListOperationDatesByRecurringOperation(ctx context.Context, recurringOperationID uuid.UUID) ([]time.Time, error) {
	rows, err := r.q().ListOperationDatesByRecurringOperation(ctx, pgconv.UUIDToPgtype(recurringOperationID))
	if err != nil {
		return nil, err
	}
	dates := make([]time.Time, 0, len(rows))
	for _, row := range rows {
		dates = append(dates, row.Time)
	}
	return dates, nil
}

func (r *OperationRepository) UpdateFutureGeneratedOperationReminderOffsets(ctx context.Context, ownerID, recurringOperationID uuid.UUID, offsetDays *int, from time.Time) error {
	var reminderOffsetDays pgtype.Int4
	if offsetDays != nil {
		//nolint:gosec // Reminder offset is bounded by application validation.
		reminderOffsetDays = pgtype.Int4{Int32: int32(*offsetDays), Valid: true}
	}
	_, err := r.q().UpdateFutureGeneratedOperationReminderOffsets(ctx, postgres.UpdateFutureGeneratedOperationReminderOffsetsParams{
		ReminderOffsetDays:   reminderOffsetDays,
		RecurringOperationID: pgconv.UUIDToPgtype(recurringOperationID),
		OwnerID:              pgconv.UUIDToPgtype(ownerID),
		OperationDate:        pgconv.DateToPgtype(from),
	})
	if err != nil {
		return fmt.Errorf("sync generated operation reminder offsets: %w", err)
	}
	return nil
}

func (r *OperationRepository) DeleteUneditedFutureOperationsByRecurringOperation(ctx context.Context, recurringOperationID uuid.UUID, after time.Time) error {
	return r.q().DeleteUneditedFutureOperationsByRecurringOperation(ctx, postgres.DeleteUneditedFutureOperationsByRecurringOperationParams{
		RecurringOperationID: pgconv.UUIDToPgtype(recurringOperationID),
		OperationDate:        pgconv.DateToPgtype(after),
	})
}

func (r *OperationRepository) DeleteFutureGeneratedOperations(ctx context.Context, recurringOperationID, ownerID uuid.UUID) error {
	return r.q().DeleteFutureGeneratedOperations(ctx, postgres.DeleteFutureGeneratedOperationsParams{
		RecurringOperationID: pgconv.UUIDToPgtype(recurringOperationID),
		OwnerID:              pgconv.UUIDToPgtype(ownerID),
	})
}

func (r *OperationRepository) DeleteUneditedOperationsByRecurringOperation(ctx context.Context, recurringOperationID uuid.UUID, from time.Time) error {
	return r.q().DeleteUneditedOperationsByRecurringOperation(ctx, postgres.DeleteUneditedOperationsByRecurringOperationParams{
		RecurringOperationID: pgconv.UUIDToPgtype(recurringOperationID),
		OperationDate:        pgconv.DateToPgtype(from),
	})
}

func (r *OperationRepository) DeleteUneditedFutureOperationsByLease(ctx context.Context, leaseID uuid.UUID, after time.Time) error {
	return r.q().DeleteUneditedFutureOperationsByLease(ctx, postgres.DeleteUneditedFutureOperationsByLeaseParams{
		LeaseID:       pgconv.UUIDToPgtype(leaseID),
		OperationDate: pgconv.DateToPgtype(after),
	})
}

func (r *OperationRepository) DeleteFutureUneditedOperationsByProperty(ctx context.Context, propertyID uuid.UUID, after time.Time) error {
	return r.q().DeleteFutureUneditedOperationsByProperty(ctx, postgres.DeleteFutureUneditedOperationsByPropertyParams{
		PropertyID:    pgconv.UUIDToPgtype(propertyID),
		OperationDate: pgconv.DateToPgtype(after),
	})
}

func (r *OperationRepository) DeleteOperationsOutsideLeaseRange(ctx context.Context, leaseID uuid.UUID, start time.Time, end *time.Time) error {
	var endDate pgtype.Date
	if end != nil {
		endDate = pgconv.DateToPgtype(*end)
	}
	return r.q().DeleteOperationsOutsideLeaseRange(ctx, postgres.DeleteOperationsOutsideLeaseRangeParams{
		LeaseID:       pgconv.UUIDToPgtype(leaseID),
		OperationDate: pgconv.DateToPgtype(start),
		Column3:       endDate,
	})
}

func (r *OperationRepository) DeleteUneditedOperationsByLease(ctx context.Context, leaseID uuid.UUID, from time.Time) error {
	return r.q().DeleteUneditedOperationsByLease(ctx, postgres.DeleteUneditedOperationsByLeaseParams{
		LeaseID:       pgconv.UUIDToPgtype(leaseID),
		OperationDate: pgconv.DateToPgtype(from),
	})
}

func (r *OperationRepository) ListByProperty(ctx context.Context, ownerID, propertyID uuid.UUID) ([]domain.Operation, error) {
	rows, err := r.q().ListOperationsByProperty(ctx, postgres.ListOperationsByPropertyParams{
		OwnerID:    pgconv.UUIDToPgtype(ownerID),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
	})
	if err != nil {
		return nil, err
	}
	ops := make([]domain.Operation, 0, len(rows))
	for _, row := range rows {
		op, err := operationFromAnyRow(row)
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	return ops, nil
}

func (r *OperationRepository) GetPropertyOperationsSummary(ctx context.Context, ownerID, propertyID uuid.UUID, asOf time.Time) (application.OperationsSummary, error) {
	row, err := r.q().GetPropertyOperationsSummary(ctx, postgres.GetPropertyOperationsSummaryParams{
		OwnerID:    pgconv.UUIDToPgtype(ownerID),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
		AsOf:       pgconv.DateToPgtype(asOf),
	})
	if err != nil {
		return application.OperationsSummary{}, err
	}
	return application.OperationsSummary{
		MonthlyProfitKopecks: row.MonthlyProfitKopecks,
		AllTimeProfitKopecks: row.AllTimeProfitKopecks,
		OverdueRentCount:     int(row.OverdueRentCount),
		OverdueTotalCount:    int(row.OverdueTotalCount),
		NextPaymentDate:      pgconv.DatePtrFromPgtype(row.NextPaymentDate),
	}, nil
}

func (r *OperationRepository) ListOverdueRentOperations(ctx context.Context, ownerID uuid.UUID) ([]application.OverdueRentOperation, error) {
	rows, err := r.q().ListOverdueRentOperationsByOwner(ctx, pgconv.UUIDToPgtype(ownerID))
	if err != nil {
		return nil, err
	}
	result := make([]application.OverdueRentOperation, 0, len(rows))
	for _, row := range rows {
		result = append(result, application.OverdueRentOperation{
			LeaseID:       pgconv.UUIDFromPgtype(row.LeaseID),
			OperationDate: row.OperationDate.Time,
		})
	}
	return result, nil
}

func (r *OperationRepository) ListNextRentPayments(ctx context.Context, ownerID uuid.UUID, asOf time.Time) ([]application.NextRentPayment, error) {
	rows, err := r.q().ListNextRentPaymentsByOwner(ctx, postgres.ListNextRentPaymentsByOwnerParams{
		OwnerID: pgconv.UUIDToPgtype(ownerID),
		AsOf:    pgconv.DateToPgtype(asOf),
	})
	if err != nil {
		return nil, err
	}
	result := make([]application.NextRentPayment, 0, len(rows))
	for _, row := range rows {
		if !row.NextPaymentDate.Valid {
			continue
		}
		result = append(result, application.NextRentPayment{
			LeaseID:         pgconv.UUIDFromPgtype(row.LeaseID),
			NextPaymentDate: row.NextPaymentDate.Time,
		})
	}
	return result, nil
}

func (r *OperationRepository) GetByIDAndOwner(ctx context.Context, id, ownerID uuid.UUID) (domain.Operation, error) {
	row, err := r.q().GetOperationByIDAndOwner(ctx, postgres.GetOperationByIDAndOwnerParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(ownerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Operation{}, application.ErrNotFound
		}
		return domain.Operation{}, err
	}
	return operationFromAnyRow(row)
}

func (r *OperationRepository) GetByIDAndOwnerForUpdate(ctx context.Context, id, ownerID uuid.UUID) (domain.Operation, error) {
	row, err := r.q().GetOperationByIDAndOwnerForUpdate(ctx, postgres.GetOperationByIDAndOwnerForUpdateParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(ownerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Operation{}, application.ErrNotFound
		}
		return domain.Operation{}, err
	}
	return operationFromAnyRow(row)
}

func (r *OperationRepository) Update(ctx context.Context, op domain.Operation) (domain.Operation, error) {
	params := postgres.UpdateOperationParams{
		ID:            pgconv.UUIDToPgtype(op.ID),
		OwnerID:       pgconv.UUIDToPgtype(op.OwnerID),
		Type:          string(op.Type),
		CategoryID:    pgconv.UUIDToPgtype(op.CategoryID),
		Name:          op.Name,
		AmountKopecks: op.AmountKopecks,
		OperationDate: pgconv.DateToPgtype(op.OperationDate),
		Comment:       pgtype.Text{String: op.Comment, Valid: true},
		LeaseID:       pgconv.UUIDToPgtype(op.LeaseID),
		Status:        string(op.Status),
	}
	if op.ReminderOffsetDays != nil {
		//nolint:gosec // Reminder offset is bounded by application validation.
		params.ReminderOffsetDays = pgtype.Int4{Int32: int32(*op.ReminderOffsetDays), Valid: true}
	}
	row, err := r.q().UpdateOperation(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Operation{}, application.ErrNotFound
		}
		return domain.Operation{}, err
	}
	return operationFromRow(row)
}

func (r *OperationRepository) ListByPropertyWithStatuses(ctx context.Context, ownerID, propertyID uuid.UUID, statuses []domain.OperationStatus) ([]domain.Operation, error) {
	if len(statuses) == 0 {
		return r.ListByProperty(ctx, ownerID, propertyID)
	}

	statusStrs := make([]string, len(statuses))
	for i, s := range statuses {
		statusStrs[i] = string(s)
	}

	rows, err := r.q().ListOperationsByPropertyWithStatuses(ctx, postgres.ListOperationsByPropertyWithStatusesParams{
		OwnerID:    pgconv.UUIDToPgtype(ownerID),
		PropertyID: pgconv.UUIDToPgtype(propertyID),
		Statuses:   statusStrs,
	})
	if err != nil {
		return nil, err
	}

	ops := make([]domain.Operation, 0, len(rows))
	for _, row := range rows {
		op, err := operationFromAnyRow(row)
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	return ops, nil
}

func (r *OperationRepository) ListPendingOperationsWithPastDate(ctx context.Context, ownerID uuid.UUID, asOf time.Time, limit int) ([]domain.Operation, error) {
	rows, err := r.q().ListPendingOperationsWithPastDate(ctx, postgres.ListPendingOperationsWithPastDateParams{
		OwnerID: pgconv.UUIDToPgtype(ownerID),
		AsOf:    pgconv.DateToPgtype(asOf),
		//nolint:gosec // Batch size is configured and bounded by caller.
		Limit: int32(limit),
	})
	if err != nil {
		return nil, err
	}

	ops := make([]domain.Operation, 0, len(rows))
	for _, row := range rows {
		op, err := operationFromRow(row)
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	return ops, nil
}

func (r *OperationRepository) ListAllPendingOperationsWithPastDate(ctx context.Context, asOf time.Time, limit int) ([]domain.Operation, error) {
	rows, err := r.q().ListAllPendingOperationsWithPastDate(ctx, postgres.ListAllPendingOperationsWithPastDateParams{
		AsOf: pgconv.DateToPgtype(asOf),
		//nolint:gosec // Batch size is configured and bounded by caller.
		Limit: int32(limit),
	})
	if err != nil {
		return nil, err
	}

	ops := make([]domain.Operation, 0, len(rows))
	for _, row := range rows {
		op, err := operationFromRow(row)
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	return ops, nil
}

// MarkOverdue transitions a pending operation to overdue. The returned bool is
// true when the row was actually updated from pending to overdue, and false
// when the operation was already in a non-pending state, its date is no longer
// in the past, or it was not found.
func (r *OperationRepository) MarkOverdue(ctx context.Context, ownerID, id uuid.UUID, asOf time.Time) (domain.Operation, bool, error) {
	row, err := r.q().MarkOperationOverdue(ctx, postgres.MarkOperationOverdueParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(ownerID),
		AsOf:    pgconv.DateToPgtype(asOf),
	})
	if err == nil {
		op, err := operationFromRow(row)
		return op, true, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.Operation{}, false, err
	}

	// The row is either missing or not pending. Fetch to distinguish and
	// return the existing operation when it is already in a non-pending state.
	existing, err := r.q().GetOperationByIDAndOwner(ctx, postgres.GetOperationByIDAndOwnerParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(ownerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Operation{}, false, application.ErrNotFound
		}
		return domain.Operation{}, false, err
	}
	op, err := operationFromAnyRow(existing)
	return op, false, err
}

func (r *OperationRepository) SoftDeleteOperation(ctx context.Context, id, ownerID uuid.UUID) error {
	_, err := r.q().SoftDeleteOperation(ctx, postgres.SoftDeleteOperationParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(ownerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrNotFound
		}
		return err
	}
	return nil
}

func (r *OperationRepository) GetFinanceReportTotals(ctx context.Context, ownerID uuid.UUID, from, to *time.Time) (application.FinanceReportTotals, error) {
	row, err := r.q().GetFinanceReportTotals(ctx, postgres.GetFinanceReportTotalsParams{
		OwnerID:  pgconv.UUIDToPgtype(ownerID),
		FromDate: pgconv.DatePtrToPgtype(from),
		ToDate:   pgconv.DatePtrToPgtype(to),
	})
	if err != nil {
		return application.FinanceReportTotals{}, err
	}
	return application.FinanceReportTotals{
		IncomeKopecks:  row.IncomeKopecks,
		ExpenseKopecks: row.ExpenseKopecks,
	}, nil
}

func (r *OperationRepository) GetFinanceReportByProperty(ctx context.Context, ownerID uuid.UUID, from, to *time.Time) ([]application.FinanceReportPropertyRow, error) {
	rows, err := r.q().GetFinanceReportByProperty(ctx, postgres.GetFinanceReportByPropertyParams{
		OwnerID:  pgconv.UUIDToPgtype(ownerID),
		FromDate: pgconv.DatePtrToPgtype(from),
		ToDate:   pgconv.DatePtrToPgtype(to),
	})
	if err != nil {
		return nil, err
	}

	result := make([]application.FinanceReportPropertyRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, application.FinanceReportPropertyRow{
			PropertyID:     pgconv.UUIDFromPgtype(row.PropertyID),
			IncomeKopecks:  row.IncomeKopecks,
			ExpenseKopecks: row.ExpenseKopecks,
		})
	}
	return result, nil
}

func (r *OperationRepository) GetFinanceReportByCategory(ctx context.Context, ownerID uuid.UUID, from, to *time.Time) ([]application.FinanceReportCategoryRow, error) {
	rows, err := r.q().GetFinanceReportByCategory(ctx, postgres.GetFinanceReportByCategoryParams{
		OwnerID:  pgconv.UUIDToPgtype(ownerID),
		FromDate: pgconv.DatePtrToPgtype(from),
		ToDate:   pgconv.DatePtrToPgtype(to),
	})
	if err != nil {
		return nil, err
	}

	result := make([]application.FinanceReportCategoryRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, application.FinanceReportCategoryRow{
			Type:         domain.OperationType(row.Type),
			CategoryID:   pgconv.UUIDFromPgtype(row.CategoryID),
			CategoryName: row.CategoryName,
			TotalKopecks: row.TotalKopecks,
		})
	}
	return result, nil
}

func (r *OperationRepository) GetFinanceReportByMonth(ctx context.Context, ownerID uuid.UUID, from, to *time.Time) ([]application.FinanceReportMonthRow, error) {
	rows, err := r.q().GetFinanceReportByMonth(ctx, postgres.GetFinanceReportByMonthParams{
		OwnerID:  pgconv.UUIDToPgtype(ownerID),
		FromDate: pgconv.DatePtrToPgtype(from),
		ToDate:   pgconv.DatePtrToPgtype(to),
	})
	if err != nil {
		return nil, err
	}

	result := make([]application.FinanceReportMonthRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, application.FinanceReportMonthRow{
			Month:          row.Month.Time,
			IncomeKopecks:  row.IncomeKopecks,
			ExpenseKopecks: row.ExpenseKopecks,
		})
	}
	return result, nil
}

func operationFromRow(row postgres.Operation) (domain.Operation, error) {
	status, err := domain.ParseOperationStatus(row.Status)
	if err != nil {
		return domain.Operation{}, fmt.Errorf("invalid operation status in database: %w", err)
	}
	op := domain.Operation{
		ID:                   pgconv.UUIDFromPgtype(row.ID),
		OwnerID:              pgconv.UUIDFromPgtype(row.OwnerID),
		PropertyID:           pgconv.UUIDFromPgtype(row.PropertyID),
		LeaseID:              pgconv.UUIDFromPgtype(row.LeaseID),
		RecurringOperationID: pgconv.UUIDFromPgtype(row.RecurringOperationID),
		Type:                 domain.OperationType(row.Type),
		CategoryID:           pgconv.UUIDFromPgtype(row.CategoryID),
		Status:               status,
		Name:                 row.Name,
		AmountKopecks:        row.AmountKopecks,
		OperationDate:        row.OperationDate.Time,
		SourceOperationDate:  pgconv.DatePtrFromPgtype(row.SourceOperationDate),
		Comment:              pgconv.TextToString(row.Comment),
		IsException:          row.IsException,
		CreatedAt:            row.CreatedAt.Time,
		UpdatedAt:            row.UpdatedAt.Time,
	}
	if row.ReminderOffsetDays.Valid {
		offset := int(row.ReminderOffsetDays.Int32)
		op.ReminderOffsetDays = &offset
	}
	if row.DeletedAt.Valid {
		op.DeletedAt = pgconv.TimestamptzToPtrTime(row.DeletedAt)
	}
	return op, nil
}

// toOperation copies any generated operation row struct to postgres.Operation
// by copying fields with matching names and types. The row type must have the
// same fields as postgres.Operation.
func toOperation(row any) postgres.Operation {
	if op, ok := row.(postgres.Operation); ok {
		return op
	}
	var op postgres.Operation
	rv := reflect.ValueOf(row)
	ov := reflect.ValueOf(&op).Elem()
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		fieldName := rt.Field(i).Name
		src := rv.Field(i)
		dst := ov.FieldByName(fieldName)
		if dst.IsValid() && dst.Type() == src.Type() {
			dst.Set(src)
		}
	}
	return op
}

// operationFromAnyRow maps any generated operation row struct to a domain.Operation.
func operationFromAnyRow(row any) (domain.Operation, error) {
	return operationFromRow(toOperation(row))
}

// toAnySlice converts a typed slice to a slice of any values.
func toAnySlice[T any](s []T) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}

func operationCategoryFromRow(row postgres.OperationCategory) domain.OperationCategory {
	var code *string
	if row.Code.Valid {
		code = &row.Code.String
	}
	return domain.OperationCategory{
		ID:        pgconv.UUIDFromPgtype(row.ID),
		OwnerID:   pgconv.UUIDFromPgtype(row.OwnerID),
		Type:      domain.OperationType(row.Type),
		Name:      row.Name,
		Code:      code,
		CreatedAt: pgconv.TimestamptzToTime(row.CreatedAt),
		UpdatedAt: pgconv.TimestamptzToTime(row.UpdatedAt),
	}
}

// OperationCategoryRepository persists operation categories.
type OperationCategoryRepository struct {
	db postgres.DBTX
}

// NewOperationCategoryRepository creates a new operation category repository.
func NewOperationCategoryRepository(db postgres.DBTX) *OperationCategoryRepository {
	return &OperationCategoryRepository{db: db}
}

func (r *OperationCategoryRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// WithTx returns a repository instance bound to the provided transaction.
func (r *OperationCategoryRepository) WithTx(tx transaction.Tx) application.OperationCategoryRepository {
	return NewOperationCategoryRepository(tx.(postgres.DBTX))
}

func (r *OperationCategoryRepository) Create(ctx context.Context, ownerID uuid.UUID, categoryType domain.OperationType, name string) (domain.OperationCategory, error) {
	row, err := r.q().CreateOperationCategory(ctx, postgres.CreateOperationCategoryParams{
		OwnerID: pgconv.UUIDToPgtype(ownerID),
		Type:    string(categoryType),
		Name:    name,
	})
	if err != nil {
		if isDuplicateCategoryNameError(err) {
			return domain.OperationCategory{}, application.ErrDuplicateCategoryName
		}
		return domain.OperationCategory{}, err
	}
	return operationCategoryFromRow(row), nil
}

func (r *OperationCategoryRepository) ListByOwner(ctx context.Context, ownerID uuid.UUID, categoryType *domain.OperationType) ([]domain.OperationCategory, error) {
	var t string
	if categoryType != nil {
		t = string(*categoryType)
	}
	rows, err := r.q().ListOperationCategoriesByOwner(ctx, postgres.ListOperationCategoriesByOwnerParams{
		OwnerID: pgconv.UUIDToPgtype(ownerID),
		Type:    t,
	})
	if err != nil {
		return nil, err
	}
	categories := make([]domain.OperationCategory, 0, len(rows))
	for _, row := range rows {
		categories = append(categories, operationCategoryFromRow(row))
	}
	return categories, nil
}

func (r *OperationCategoryRepository) GetByIDAndOwner(ctx context.Context, id, ownerID uuid.UUID) (domain.OperationCategory, error) {
	row, err := r.q().GetOperationCategoryByIDAndOwner(ctx, postgres.GetOperationCategoryByIDAndOwnerParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(ownerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.OperationCategory{}, application.ErrNotFound
		}
		return domain.OperationCategory{}, err
	}
	return operationCategoryFromRow(row), nil
}

func (r *OperationCategoryRepository) GetByOwnerAndCode(ctx context.Context, ownerID uuid.UUID, code domain.OperationCategoryDefaultCode) (domain.OperationCategory, error) {
	row, err := r.q().GetOperationCategoryByOwnerAndCode(ctx, postgres.GetOperationCategoryByOwnerAndCodeParams{
		OwnerID: pgconv.UUIDToPgtype(ownerID),
		Code:    pgtype.Text{String: string(code), Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.OperationCategory{}, application.ErrNotFound
		}
		return domain.OperationCategory{}, err
	}
	return operationCategoryFromRow(row), nil
}

func (r *OperationCategoryRepository) CreateDefaultCategories(ctx context.Context, ownerID uuid.UUID) error {
	ownerPgID := pgconv.UUIDToPgtype(ownerID)
	defaults := []struct {
		code          string
		operationType string
		name          string
	}{
		{code: string(domain.OperationCategoryCodeRent), operationType: string(domain.OperationTypeIncome), name: "Аренда"},
		{code: string(domain.OperationCategoryCodeUtilities), operationType: string(domain.OperationTypeExpense), name: "Коммунальные услуги"},
		{code: string(domain.OperationCategoryCodeRepair), operationType: string(domain.OperationTypeExpense), name: "Ремонт"},
		{code: string(domain.OperationCategoryCodeTax), operationType: string(domain.OperationTypeExpense), name: "Налог"},
	}
	for _, cat := range defaults {
		err := r.q().CreateOperationCategoryIgnoreConflict(ctx, postgres.CreateOperationCategoryIgnoreConflictParams{
			OwnerID: ownerPgID,
			Type:    cat.operationType,
			Name:    cat.name,
			Code:    pgtype.Text{String: cat.code, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("create default operation category %q: %w", cat.code, err)
		}
	}
	return nil
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

// WithTx returns a repository instance bound to the provided transaction.
func (r *PropertyRepository) WithTx(tx transaction.Tx) application.PropertyRepository {
	return NewPropertyRepository(tx.(postgres.DBTX))
}

// ExistsActiveByOwner reports whether an active property exists for the owner.
func (r *PropertyRepository) ExistsActiveByOwner(ctx context.Context, id, ownerID uuid.UUID) (bool, error) {
	row, err := r.q().GetPropertyByIDAndOwner(ctx, postgres.GetPropertyByIDAndOwnerParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(ownerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return row.Status == "active", nil
}

// ExistsByOwner reports whether a property exists for the owner regardless of status.
func (r *PropertyRepository) ExistsByOwner(ctx context.Context, id, ownerID uuid.UUID) (bool, error) {
	_, err := r.q().GetPropertyByIDAndOwner(ctx, postgres.GetPropertyByIDAndOwnerParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(ownerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetStatusByOwner returns the property status for the owner, or an empty
// string when the property does not exist.
func (r *PropertyRepository) GetStatusByOwner(ctx context.Context, id, ownerID uuid.UUID) (string, error) {
	status, err := r.q().GetPropertyStatusByOwner(ctx, postgres.GetPropertyStatusByOwnerParams{
		ID:      pgconv.UUIDToPgtype(id),
		OwnerID: pgconv.UUIDToPgtype(ownerID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return status, nil
}

// HasOpenLease reports whether the property currently has an open lease.
func (r *PropertyRepository) HasOpenLease(ctx context.Context, id uuid.UUID) (bool, error) {
	count, err := r.q().CountOpenLeasesByProperty(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
