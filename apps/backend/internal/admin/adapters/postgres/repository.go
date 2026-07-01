package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	adminapp "github.com/nambers/arenda-planform/apps/backend/internal/admin/application"
	leasesdomain "github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/encryption"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
	propertiesdomain "github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// AdminRepository implements the admin application ports using generated SQLC queries.
type AdminRepository struct {
	db        postgres.DBTX
	enc       encryption.Encryptor
	clock     clock.Clock
	occupancy propertiesapp.OccupancyProvider
}

// NewAdminRepository creates a new admin repository.
func NewAdminRepository(db postgres.DBTX, enc encryption.Encryptor, clock clock.Clock, occupancy propertiesapp.OccupancyProvider) *AdminRepository {
	return &AdminRepository{db: db, enc: enc, clock: clock, occupancy: occupancy}
}

func (r *AdminRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// Compile-time interface checks.
var (
	_ adminapp.UserRepository          = (*AdminRepository)(nil)
	_ adminapp.PropertyRepository      = (*AdminRepository)(nil)
	_ adminapp.LeaseRepository         = (*AdminRepository)(nil)
	_ adminapp.TenantContactRepository = (*AdminRepository)(nil)
	_ adminapp.OperationRepository     = (*AdminRepository)(nil)
)

// ListUsers implements UserRepository.ListUsers.
func (r *AdminRepository) ListUsers(ctx context.Context, filters adminapp.AdminUserFilters) ([]adminapp.AdminUserView, int64, error) {
	phone := filters.Phone
	if phone != "" {
		encrypted, err := r.enc.DeterministicEncrypt(ctx, phone)
		if err != nil {
			return nil, 0, fmt.Errorf("encrypt phone filter: %w", err)
		}
		phone = encrypted
	}

	total, err := r.q().CountUsersAdmin(ctx, postgres.CountUsersAdminParams{
		Phone:              phone,
		Email:              filters.Email,
		Role:               filters.Role,
		SubscriptionStatus: filters.SubscriptionStatus,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	rows, err := r.q().ListUsersAdmin(ctx, postgres.ListUsersAdminParams{
		Phone:              phone,
		Email:              filters.Email,
		Role:               filters.Role,
		SubscriptionStatus: filters.SubscriptionStatus,
		Offset:             toInt32(filters.Offset),
		Limit:              toInt32(filters.Limit),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}

	views := make([]adminapp.AdminUserView, 0, len(rows))
	for _, row := range rows {
		phone, err := r.decryptPhone(ctx, row.Phone, row.PhoneEncrypted)
		if err != nil {
			return nil, 0, err
		}
		views = append(views, adminapp.AdminUserView{
			ID:                 pgconv.UUIDFromPgtype(row.ID),
			Phone:              phone,
			Role:               row.Role,
			Name:               pgconv.TextToPtrString(row.Name),
			Surname:            pgconv.TextToPtrString(row.Surname),
			Patronymic:         pgconv.TextToPtrString(row.Patronymic),
			Email:              pgconv.TextToPtrString(row.Email),
			CreatedAt:          row.CreatedAt.Time,
			UpdatedAt:          row.UpdatedAt.Time,
			SubscriptionStatus: pgconv.TextToString(row.SubscriptionStatus),
		})
	}

	return views, total, nil
}

// GetUser implements UserRepository.GetUser.
func (r *AdminRepository) GetUser(ctx context.Context, id uuid.UUID) (adminapp.AdminUserView, error) {
	row, err := r.q().GetUserByID(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return adminapp.AdminUserView{}, adminapp.ErrNotFound
		}
		return adminapp.AdminUserView{}, fmt.Errorf("get user: %w", err)
	}

	phone, err := r.decryptPhone(ctx, row.Phone, row.PhoneEncrypted)
	if err != nil {
		return adminapp.AdminUserView{}, err
	}

	return adminapp.AdminUserView{
		ID:         pgconv.UUIDFromPgtype(row.ID),
		Phone:      phone,
		Role:       row.Role,
		Name:       pgconv.TextToPtrString(row.Name),
		Surname:    pgconv.TextToPtrString(row.Surname),
		Patronymic: pgconv.TextToPtrString(row.Patronymic),
		Email:      pgconv.TextToPtrString(row.Email),
		CreatedAt:  row.CreatedAt.Time,
		UpdatedAt:  row.UpdatedAt.Time,
	}, nil
}

// CountActivePropertiesByOwner implements UserRepository.CountActivePropertiesByOwner.
func (r *AdminRepository) CountActivePropertiesByOwner(ctx context.Context, ownerID uuid.UUID) (int64, error) {
	return r.q().CountActivePropertiesByOwnerAdmin(ctx, pgconv.UUIDToPgtype(ownerID))
}

// CountArchivedPropertiesByOwner implements UserRepository.CountArchivedPropertiesByOwner.
func (r *AdminRepository) CountArchivedPropertiesByOwner(ctx context.Context, ownerID uuid.UUID) (int64, error) {
	return r.q().CountArchivedPropertiesByOwnerAdmin(ctx, pgconv.UUIDToPgtype(ownerID))
}

// CountLeasesByOwner implements UserRepository.CountLeasesByOwner.
func (r *AdminRepository) CountLeasesByOwner(ctx context.Context, ownerID uuid.UUID) (int64, error) {
	return r.q().CountLeasesByOwnerAdmin(ctx, pgconv.UUIDToPgtype(ownerID))
}

// CountOperationsByOwner implements UserRepository.CountOperationsByOwner.
func (r *AdminRepository) CountOperationsByOwner(ctx context.Context, ownerID uuid.UUID) (int64, error) {
	return r.q().CountOperationsByOwnerAdmin(ctx, postgres.CountOperationsByOwnerAdminParams{
		OwnerID:    pgconv.UUIDToPgtype(ownerID),
		Status:     "",
		Type:       "",
		PropertyID: pgtype.UUID{},
		LeaseID:    pgtype.UUID{},
	})
}

// CountTenantContactsByOwner implements UserRepository.CountTenantContactsByOwner.
func (r *AdminRepository) CountTenantContactsByOwner(ctx context.Context, ownerID uuid.UUID) (int64, error) {
	return r.q().CountTenantContactsByOwnerAdmin(ctx, pgconv.UUIDToPgtype(ownerID))
}

// ListUserProperties implements PropertyRepository.ListUserProperties.
func (r *AdminRepository) ListUserProperties(ctx context.Context, userID uuid.UUID, filters adminapp.AdminPropertyFilters) ([]adminapp.AdminPropertyView, int64, error) {
	total, err := r.q().CountPropertiesByOwnerAdmin(ctx, postgres.CountPropertiesByOwnerAdminParams{
		OwnerID: pgconv.UUIDToPgtype(userID),
		Status:  filters.Status,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count properties: %w", err)
	}

	rows, err := r.q().ListPropertiesByOwnerAdmin(ctx, postgres.ListPropertiesByOwnerAdminParams{
		OwnerID: pgconv.UUIDToPgtype(userID),
		Status:  filters.Status,
		Offset:  toInt32(filters.Offset),
		Limit:   toInt32(filters.Limit),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list properties: %w", err)
	}

	views := make([]adminapp.AdminPropertyView, 0, len(rows))
	for _, row := range rows {
		view, err := r.propertyViewFromRow(ctx, row)
		if err != nil {
			return nil, 0, err
		}
		views = append(views, view)
	}

	return views, total, nil
}

// GetProperty implements PropertyRepository.GetProperty.
func (r *AdminRepository) GetProperty(ctx context.Context, id uuid.UUID) (adminapp.AdminPropertyView, error) {
	row, err := r.q().GetPropertyByIDAdmin(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return adminapp.AdminPropertyView{}, adminapp.ErrNotFound
		}
		return adminapp.AdminPropertyView{}, fmt.Errorf("get property: %w", err)
	}
	return r.propertyViewFromRow(ctx, row)
}

func (r *AdminRepository) propertyViewFromRow(ctx context.Context, row postgres.Property) (adminapp.AdminPropertyView, error) {
	propType, err := propertiesdomain.ParsePropertyType(row.Type)
	if err != nil {
		return adminapp.AdminPropertyView{}, fmt.Errorf("invalid property type: %w", err)
	}

	status, err := propertiesdomain.ParsePropertyStatus(row.Status)
	if err != nil {
		return adminapp.AdminPropertyView{}, fmt.Errorf("invalid property status: %w", err)
	}

	ownerID := pgconv.UUIDFromPgtype(row.OwnerID)
	propertyID := pgconv.UUIDFromPgtype(row.ID)
	occupied, err := r.occupancy.IsOccupied(ctx, ownerID, propertyID)
	if err != nil {
		return adminapp.AdminPropertyView{}, fmt.Errorf("check occupancy: %w", err)
	}

	occupancy := propertiesdomain.OccupancyFree
	if occupied {
		occupancy = propertiesdomain.OccupancyOccupied
	}

	var description *string
	if row.Description.Valid {
		description = &row.Description.String
	}

	return adminapp.AdminPropertyView{
		ID:          propertyID,
		OwnerID:     ownerID,
		Name:        row.Name,
		Type:        propType,
		Address:     row.Address,
		Description: description,
		Status:      status,
		Occupancy:   occupancy,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}, nil
}

// ListUserLeases implements LeaseRepository.ListUserLeases.
func (r *AdminRepository) ListUserLeases(ctx context.Context, userID uuid.UUID, filters adminapp.AdminListFilters) ([]adminapp.AdminLeaseView, int64, error) {
	total, err := r.q().CountLeasesByOwnerAdmin(ctx, pgconv.UUIDToPgtype(userID))
	if err != nil {
		return nil, 0, fmt.Errorf("count leases: %w", err)
	}

	rows, err := r.q().ListLeasesByOwnerAdmin(ctx, postgres.ListLeasesByOwnerAdminParams{
		OwnerID: pgconv.UUIDToPgtype(userID),
		Offset:  toInt32(filters.Offset),
		Limit:   toInt32(filters.Limit),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list leases: %w", err)
	}

	now := r.clock.Now()
	views := make([]adminapp.AdminLeaseView, 0, len(rows))
	for _, row := range rows {
		view, err := r.leaseViewFromRow(row, now)
		if err != nil {
			return nil, 0, err
		}
		views = append(views, view)
	}

	return views, total, nil
}

// GetLease implements LeaseRepository.GetLease.
func (r *AdminRepository) GetLease(ctx context.Context, id uuid.UUID) (adminapp.AdminLeaseView, error) {
	row, err := r.q().GetLeaseByID(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return adminapp.AdminLeaseView{}, adminapp.ErrNotFound
		}
		return adminapp.AdminLeaseView{}, fmt.Errorf("get lease: %w", err)
	}

	view, err := r.leaseViewFromRow(row, r.clock.Now())
	if err != nil {
		return adminapp.AdminLeaseView{}, err
	}

	if view.TenantContactID != nil {
		contact, err := r.q().GetTenantContactByIDAdmin(ctx, pgconv.UUIDToPgtype(*view.TenantContactID))
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return adminapp.AdminLeaseView{}, fmt.Errorf("load tenant contact: %w", err)
		}
		if err == nil {
			c := r.tenantContactFromRow(contact)
			view.TenantContact = &c
		}
	}

	return view, nil
}

func (r *AdminRepository) leaseViewFromRow(row postgres.Lease, now time.Time) (adminapp.AdminLeaseView, error) {
	status, err := leasesdomain.ParseLeaseStatus(row.Status)
	if err != nil {
		return adminapp.AdminLeaseView{}, fmt.Errorf("invalid lease status: %w", err)
	}

	lease := leasesdomain.Lease{
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
	}
	lease.Status = lease.EffectiveStatus(now)

	return adminapp.AdminLeaseView{
		ID:                   lease.ID,
		OwnerID:              lease.OwnerID,
		PropertyID:           lease.PropertyID,
		TenantContactID:      lease.TenantContactID,
		Status:               lease.Status,
		StartDate:            lease.StartDate,
		EndDate:              lease.EndDate,
		RentAmountKopecks:    lease.RentAmountKopecks,
		DepositAmountKopecks: lease.DepositAmountKopecks,
		PaymentDay:           lease.PaymentDay,
		Comment:              lease.Comment,
		CreatedAt:            lease.CreatedAt,
		UpdatedAt:            lease.UpdatedAt,
	}, nil
}

// ListUserTenantContacts implements TenantContactRepository.ListUserTenantContacts.
func (r *AdminRepository) ListUserTenantContacts(ctx context.Context, userID uuid.UUID, filters adminapp.AdminListFilters) ([]adminapp.AdminTenantContactView, int64, error) {
	total, err := r.q().CountTenantContactsByOwnerAdmin(ctx, pgconv.UUIDToPgtype(userID))
	if err != nil {
		return nil, 0, fmt.Errorf("count tenant contacts: %w", err)
	}

	rows, err := r.q().ListTenantContactsByOwnerAdmin(ctx, postgres.ListTenantContactsByOwnerAdminParams{
		OwnerID: pgconv.UUIDToPgtype(userID),
		Offset:  toInt32(filters.Offset),
		Limit:   toInt32(filters.Limit),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list tenant contacts: %w", err)
	}

	views := make([]adminapp.AdminTenantContactView, 0, len(rows))
	for _, row := range rows {
		views = append(views, adminapp.AdminTenantContactView(r.tenantContactFromRow(row)))
	}

	return views, total, nil
}

// GetTenantContact implements TenantContactRepository.GetTenantContact.
func (r *AdminRepository) GetTenantContact(ctx context.Context, id uuid.UUID) (adminapp.AdminTenantContactView, error) {
	row, err := r.q().GetTenantContactByIDAdmin(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return adminapp.AdminTenantContactView{}, adminapp.ErrNotFound
		}
		return adminapp.AdminTenantContactView{}, fmt.Errorf("get tenant contact: %w", err)
	}
	return adminapp.AdminTenantContactView(r.tenantContactFromRow(row)), nil
}

func (r *AdminRepository) tenantContactFromRow(row postgres.TenantContact) leasesdomain.TenantContact {
	return leasesdomain.TenantContact{
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

// ListUserOperations implements OperationRepository.ListUserOperations.
func (r *AdminRepository) ListUserOperations(ctx context.Context, userID uuid.UUID, filters adminapp.AdminOperationFilters) ([]adminapp.AdminOperationView, int64, error) {
	total, err := r.q().CountOperationsByOwnerAdmin(ctx, postgres.CountOperationsByOwnerAdminParams{
		OwnerID:    pgconv.UUIDToPgtype(userID),
		Status:     filters.Status,
		Type:       filters.Type,
		PropertyID: pgconv.UUIDToPgtype(filters.PropertyID),
		LeaseID:    pgconv.UUIDToPgtype(filters.LeaseID),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count operations: %w", err)
	}

	rows, err := r.q().ListOperationsByOwnerAdmin(ctx, postgres.ListOperationsByOwnerAdminParams{
		OwnerID:    pgconv.UUIDToPgtype(userID),
		Status:     filters.Status,
		Type:       filters.Type,
		PropertyID: pgconv.UUIDToPgtype(filters.PropertyID),
		LeaseID:    pgconv.UUIDToPgtype(filters.LeaseID),
		Offset:     toInt32(filters.Offset),
		Limit:      toInt32(filters.Limit),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list operations: %w", err)
	}

	views := make([]adminapp.AdminOperationView, 0, len(rows))
	for _, row := range rows {
		view, err := r.operationViewFromRow(row)
		if err != nil {
			return nil, 0, err
		}
		views = append(views, view)
	}

	return views, total, nil
}

// GetOperation implements OperationRepository.GetOperation.
func (r *AdminRepository) GetOperation(ctx context.Context, id uuid.UUID) (adminapp.AdminOperationView, error) {
	row, err := r.q().GetOperationByIDAdmin(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return adminapp.AdminOperationView{}, adminapp.ErrNotFound
		}
		return adminapp.AdminOperationView{}, fmt.Errorf("get operation: %w", err)
	}
	return r.operationViewFromRow(row)
}

func (r *AdminRepository) operationViewFromRow(row postgres.Operation) (adminapp.AdminOperationView, error) {
	status, err := leasesdomain.ParseOperationStatus(row.Status)
	if err != nil {
		return adminapp.AdminOperationView{}, fmt.Errorf("invalid operation status: %w", err)
	}

	view := adminapp.AdminOperationView{
		ID:            pgconv.UUIDFromPgtype(row.ID),
		OwnerID:       pgconv.UUIDFromPgtype(row.OwnerID),
		PropertyID:    pgconv.UUIDFromPgtype(row.PropertyID),
		Type:          leasesdomain.OperationType(row.Type),
		Category:      leasesdomain.OperationCategory(row.Category),
		Status:        status,
		Name:          row.Name,
		AmountKopecks: row.AmountKopecks,
		OperationDate: row.OperationDate.Time,
		IsException:   row.IsException,
		CreatedAt:     row.CreatedAt.Time,
		UpdatedAt:     row.UpdatedAt.Time,
	}

	if row.LeaseID.Valid {
		id := pgconv.UUIDFromPgtype(row.LeaseID)
		view.LeaseID = &id
	}
	if row.RecurringOperationID.Valid {
		id := pgconv.UUIDFromPgtype(row.RecurringOperationID)
		view.RecurringOperationID = &id
	}
	if row.Comment.Valid {
		view.Comment = &row.Comment.String
	}
	if row.ReminderOffsetDays.Valid {
		offset := int(row.ReminderOffsetDays.Int32)
		view.ReminderOffsetDays = &offset
	}

	return view, nil
}

func (r *AdminRepository) decryptPhone(ctx context.Context, phone string, encrypted bool) (string, error) {
	if !encrypted {
		return phone, nil
	}
	decrypted, err := r.enc.Decrypt(ctx, phone)
	if err != nil {
		return "", fmt.Errorf("decrypt phone: %w", err)
	}
	return decrypted, nil
}

func toInt32(v int) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v)
}
