package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
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

func (r *LeaseRepository) GetOpenLeaseByProperty(ctx context.Context, propertyID uuid.UUID) (domain.Lease, error) {
	row, err := r.q().GetOpenLeaseByProperty(ctx, pgconv.UUIDToPgtype(propertyID))
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
	row, err := r.q().CreateRecurringOperation(ctx, postgres.CreateRecurringOperationParams{
		OwnerID:       pgconv.UUIDToPgtype(op.OwnerID),
		PropertyID:    pgconv.UUIDToPgtype(op.PropertyID),
		LeaseID:       pgconv.UUIDToPgtype(op.LeaseID),
		Type:          string(op.Type),
		Category:      string(op.Category),
		Name:          op.Name,
		AmountKopecks: op.AmountKopecks,
		StartDate:     pgconv.DateToPgtype(op.StartDate),
		//nolint:gosec // PaymentDay is validated to be 1-31 in domain.
		PaymentDay:  int32(op.PaymentDay),
		EndDate:     pgconv.DatePtrToPgtype(op.EndDate),
		Periodicity: string(op.Periodicity),
		Status:      string(op.Status),
		Comment:     pgtype.Text{String: op.Comment, Valid: true},
	})
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
	row, err := r.q().UpdateRecurringOperation(ctx, postgres.UpdateRecurringOperationParams{
		ID:            pgconv.UUIDToPgtype(op.ID),
		Type:          string(op.Type),
		Category:      string(op.Category),
		Name:          op.Name,
		AmountKopecks: op.AmountKopecks,
		StartDate:     pgconv.DateToPgtype(op.StartDate),
		//nolint:gosec // PaymentDay is validated to be 1-31 in domain.
		PaymentDay:  int32(op.PaymentDay),
		EndDate:     pgconv.DatePtrToPgtype(op.EndDate),
		Periodicity: string(op.Periodicity),
		Comment:     pgtype.Text{String: op.Comment, Valid: true},
		OwnerID:     pgconv.UUIDToPgtype(op.OwnerID),
	})
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

// SetReminderOffset stores the reminder offset for a recurring operation.
func (r *RecurringOperationRepository) SetReminderOffset(ctx context.Context, ownerID, recID uuid.UUID, offsetDays int) error {
	//nolint:gosec // Reminder offset is bounded by application validation.
	_, err := r.q().UpdateRecurringOperationReminderOffset(ctx, postgres.UpdateRecurringOperationReminderOffsetParams{
		ReminderOffsetDays: pgtype.Int4{Int32: int32(offsetDays), Valid: true},
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

func recurringOperationFromRow(row postgres.RecurringOperation) domain.RecurringOperation {
	rec := domain.RecurringOperation{
		ID:            pgconv.UUIDFromPgtype(row.ID),
		OwnerID:       pgconv.UUIDFromPgtype(row.OwnerID),
		PropertyID:    pgconv.UUIDFromPgtype(row.PropertyID),
		LeaseID:       pgconv.UUIDFromPgtype(row.LeaseID),
		Type:          domain.OperationType(row.Type),
		Category:      domain.OperationCategory(row.Category),
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
	row, err := r.q().CreateOperation(ctx, postgres.CreateOperationParams{
		OwnerID:              pgconv.UUIDToPgtype(op.OwnerID),
		PropertyID:           pgconv.UUIDToPgtype(op.PropertyID),
		LeaseID:              pgconv.UUIDToPgtype(op.LeaseID),
		RecurringOperationID: pgconv.UUIDToPgtype(op.RecurringOperationID),
		Type:                 string(op.Type),
		Category:             string(op.Category),
		Name:                 op.Name,
		AmountKopecks:        op.AmountKopecks,
		OperationDate:        pgconv.DateToPgtype(op.OperationDate),
		Comment:              pgtype.Text{String: op.Comment, Valid: true},
		IsException:          op.IsException,
		Status:               string(op.Status),
	})
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
		rows[i] = []any{
			pgconv.UUIDToPgtype(op.OwnerID),
			pgconv.UUIDToPgtype(op.PropertyID),
			pgconv.UUIDToPgtype(op.LeaseID),
			pgconv.UUIDToPgtype(op.RecurringOperationID),
			string(op.Type),
			string(op.Category),
			op.Name,
			op.AmountKopecks,
			pgconv.DateToPgtype(op.OperationDate),
			pgtype.Text{String: op.Comment, Valid: true},
			op.IsException,
			string(op.Status),
		}
	}

	copier, ok := r.db.(copyFromer)
	if !ok {
		return fmt.Errorf("bulk create operations: CopyFrom not supported by %T", r.db)
	}

	_, err := copier.CopyFrom(ctx, pgx.Identifier{"operations"}, []string{
		"owner_id", "property_id", "lease_id", "recurring_operation_id",
		"type", "category", "name", "amount_kopecks", "operation_date", "comment", "is_exception", "status",
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
	categories := make([]string, 0, len(filter.Categories))
	for _, c := range filter.Categories {
		categories = append(categories, string(c))
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

	rows, err := r.q().ListOperationsByOwner(ctx, postgres.ListOperationsByOwnerParams{
		OwnerID:              pgconv.UUIDToPgtype(ownerID),
		Types:                types,
		Statuses:             statuses,
		Categories:           categories,
		PropertyID:           pgconv.UUIDToPgtype(filter.PropertyID),
		FromDate:             fromDate,
		ToDate:               toDate,
		RecurringOperationID: pgconv.UUIDToPgtype(filter.RecurringOperationID),
		Limit:                limit,
		//nolint:gosec // Pagination offset is bounded by the API layer.
		Offset: int32(filter.Offset),
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

func (r *OperationRepository) ListByLease(ctx context.Context, leaseID uuid.UUID) ([]domain.Operation, error) {
	rows, err := r.q().ListOperationsByLease(ctx, pgconv.UUIDToPgtype(leaseID))
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

func (r *OperationRepository) ListByRecurringOperation(ctx context.Context, recurringOperationID uuid.UUID) ([]domain.Operation, error) {
	rows, err := r.q().ListOperationsByRecurringOperation(ctx, pgconv.UUIDToPgtype(recurringOperationID))
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

func (r *OperationRepository) DeleteUneditedFutureOperationsByRecurringOperation(ctx context.Context, recurringOperationID uuid.UUID, after time.Time) error {
	return r.q().DeleteUneditedFutureOperationsByRecurringOperation(ctx, postgres.DeleteUneditedFutureOperationsByRecurringOperationParams{
		RecurringOperationID: pgconv.UUIDToPgtype(recurringOperationID),
		OperationDate:        pgconv.DateToPgtype(after),
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
		op, err := operationFromRow(row)
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	return ops, nil
}

func (r *OperationRepository) HasDepositReturnForLease(ctx context.Context, leaseID uuid.UUID) (bool, error) {
	found, err := r.q().HasDepositReturnForLease(ctx, pgconv.UUIDToPgtype(leaseID))
	if err != nil {
		return false, err
	}
	return found, nil
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
	return operationFromRow(row)
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
	return operationFromRow(row)
}

func (r *OperationRepository) Update(ctx context.Context, op domain.Operation) (domain.Operation, error) {
	row, err := r.q().UpdateOperation(ctx, postgres.UpdateOperationParams{
		ID:            pgconv.UUIDToPgtype(op.ID),
		OwnerID:       pgconv.UUIDToPgtype(op.OwnerID),
		Type:          string(op.Type),
		Category:      string(op.Category),
		Name:          op.Name,
		AmountKopecks: op.AmountKopecks,
		OperationDate: pgconv.DateToPgtype(op.OperationDate),
		Comment:       pgtype.Text{String: op.Comment, Valid: true},
		LeaseID:       pgconv.UUIDToPgtype(op.LeaseID),
		Status:        string(op.Status),
	})
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
		op, err := operationFromRow(row)
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
	op, err := operationFromRow(existing)
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

func (r *OperationRepository) GetFinanceReportTotals(ctx context.Context, ownerID uuid.UUID, from, to time.Time) (application.FinanceReportTotals, error) {
	row, err := r.q().GetFinanceReportTotals(ctx, postgres.GetFinanceReportTotalsParams{
		OwnerID:         pgconv.UUIDToPgtype(ownerID),
		OperationDate:   pgconv.DateToPgtype(from),
		OperationDate_2: pgconv.DateToPgtype(to),
	})
	if err != nil {
		return application.FinanceReportTotals{}, err
	}
	return application.FinanceReportTotals{
		IncomeKopecks:  row.IncomeKopecks,
		ExpenseKopecks: row.ExpenseKopecks,
	}, nil
}

func (r *OperationRepository) GetFinanceReportByProperty(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]application.FinanceReportPropertyRow, error) {
	rows, err := r.q().GetFinanceReportByProperty(ctx, postgres.GetFinanceReportByPropertyParams{
		OwnerID:         pgconv.UUIDToPgtype(ownerID),
		OperationDate:   pgconv.DateToPgtype(from),
		OperationDate_2: pgconv.DateToPgtype(to),
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

func (r *OperationRepository) GetFinanceReportByCategory(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]application.FinanceReportCategoryRow, error) {
	rows, err := r.q().GetFinanceReportByCategory(ctx, postgres.GetFinanceReportByCategoryParams{
		OwnerID:         pgconv.UUIDToPgtype(ownerID),
		OperationDate:   pgconv.DateToPgtype(from),
		OperationDate_2: pgconv.DateToPgtype(to),
	})
	if err != nil {
		return nil, err
	}

	result := make([]application.FinanceReportCategoryRow, 0, len(rows))
	for _, row := range rows {
		opType, err := domain.ParseOperationType(row.Type)
		if err != nil {
			return nil, fmt.Errorf("invalid operation type in database: %w", err)
		}
		category, err := domain.ParseOperationCategory(row.Category)
		if err != nil {
			return nil, fmt.Errorf("invalid operation category in database: %w", err)
		}
		result = append(result, application.FinanceReportCategoryRow{
			Type:         opType,
			Category:     category,
			TotalKopecks: row.TotalKopecks,
		})
	}
	return result, nil
}

func (r *OperationRepository) GetFinanceReportByMonth(ctx context.Context, ownerID uuid.UUID, from, to time.Time) ([]application.FinanceReportMonthRow, error) {
	rows, err := r.q().GetFinanceReportByMonth(ctx, postgres.GetFinanceReportByMonthParams{
		OwnerID:         pgconv.UUIDToPgtype(ownerID),
		OperationDate:   pgconv.DateToPgtype(from),
		OperationDate_2: pgconv.DateToPgtype(to),
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
		Category:             domain.OperationCategory(row.Category),
		Status:               status,
		Name:                 row.Name,
		AmountKopecks:        row.AmountKopecks,
		OperationDate:        row.OperationDate.Time,
		Comment:              pgconv.TextToString(row.Comment),
		IsException:          row.IsException,
		CreatedAt:            row.CreatedAt.Time,
		UpdatedAt:            row.UpdatedAt.Time,
	}
	if row.DeletedAt.Valid {
		op.DeletedAt = pgconv.TimestamptzToPtrTime(row.DeletedAt)
	}
	return op, nil
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

// HasOpenLease reports whether the property currently has an open lease.
func (r *PropertyRepository) HasOpenLease(ctx context.Context, id uuid.UUID) (bool, error) {
	count, err := r.q().CountOpenLeasesByProperty(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
