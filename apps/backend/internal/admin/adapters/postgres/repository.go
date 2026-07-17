package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
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
	_ adminapp.StatsRepository         = (*AdminRepository)(nil)
	_ adminapp.AuditLogRepository      = (*AdminRepository)(nil)
)

// ListUsers implements UserRepository.ListUsers.
func (r *AdminRepository) ListUsers(ctx context.Context, filters adminapp.AdminUserFilters) ([]adminapp.AdminUserView, int64, error) {
	// Trim so that whitespace-only input disables the filter instead of
	// encrypting a value that can never match a stored phone.
	phone := strings.TrimSpace(filters.Phone)
	phoneEnc := ""
	if phone != "" {
		encrypted, err := r.enc.DeterministicEncrypt(ctx, phone)
		if err != nil {
			return nil, 0, fmt.Errorf("encrypt phone filter: %w", err)
		}
		phoneEnc = encrypted
	}

	total, err := r.q().CountUsersAdmin(ctx, postgres.CountUsersAdminParams{
		Phone:              phone,
		PhoneEnc:           phoneEnc,
		Email:              filters.Email,
		Role:               filters.Role,
		SubscriptionStatus: filters.SubscriptionStatus,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	rows, err := r.q().ListUsersAdmin(ctx, postgres.ListUsersAdminParams{
		Phone:              phone,
		PhoneEnc:           phoneEnc,
		Email:              filters.Email,
		Role:               filters.Role,
		SubscriptionStatus: filters.SubscriptionStatus,
		Sort:               filters.Sort,
		Order:              filters.Order,
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
	return r.q().CountLeasesAdmin(ctx, postgres.CountLeasesAdminParams{
		OwnerID:    pgconv.UUIDToPgtype(ownerID),
		PropertyID: pgtype.UUID{},
		Status:     "",
	})
}

// CountOperationsByOwner implements UserRepository.CountOperationsByOwner.
func (r *AdminRepository) CountOperationsByOwner(ctx context.Context, ownerID uuid.UUID) (int64, error) {
	return r.q().CountOperationsAdmin(ctx, postgres.CountOperationsAdminParams{
		OwnerID:    pgconv.UUIDToPgtype(ownerID),
		Status:     "",
		Type:       "",
		PropertyID: pgtype.UUID{},
		LeaseID:    pgtype.UUID{},
		Q:          "",
	})
}

// CountTenantContactsByOwner implements UserRepository.CountTenantContactsByOwner.
func (r *AdminRepository) CountTenantContactsByOwner(ctx context.Context, ownerID uuid.UUID) (int64, error) {
	return r.q().CountTenantContactsAdmin(ctx, postgres.CountTenantContactsAdminParams{
		OwnerID: pgconv.UUIDToPgtype(ownerID),
		Q:       "",
	})
}

// ListProperties implements PropertyRepository.ListProperties.
func (r *AdminRepository) ListProperties(ctx context.Context, filters adminapp.AdminPropertyFilters) ([]adminapp.AdminPropertyView, int64, error) {
	q := escapeLikePattern(filters.Q)
	total, err := r.q().CountPropertiesAdmin(ctx, postgres.CountPropertiesAdminParams{
		OwnerID: pgconv.UUIDToPgtype(filters.OwnerID),
		Status:  filters.Status,
		Q:       q,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count properties: %w", err)
	}

	rows, err := r.q().ListPropertiesAdmin(ctx, postgres.ListPropertiesAdminParams{
		OwnerID: pgconv.UUIDToPgtype(filters.OwnerID),
		Status:  filters.Status,
		Q:       q,
		Sort:    filters.Sort,
		Order:   filters.Order,
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
	return r.propertyViewFromRow(ctx, adminGetPropertyRowToListRow(row))
}

// adminGetPropertyRowToListRow converts the generated get-row type to the
// list-row type. Both structs carry the same fields because both admin
// property queries select properties columns plus owner phone columns.
func adminGetPropertyRowToListRow(row postgres.GetPropertyByIDAdminRow) postgres.ListPropertiesAdminRow {
	return postgres.ListPropertiesAdminRow(row)
}

func (r *AdminRepository) propertyViewFromRow(ctx context.Context, row postgres.ListPropertiesAdminRow) (adminapp.AdminPropertyView, error) {
	propType, err := propertiesdomain.ParsePropertyType(row.Type)
	if err != nil {
		return adminapp.AdminPropertyView{}, fmt.Errorf("invalid property type: %w", err)
	}

	status, err := propertiesdomain.ParsePropertyStatus(row.Status)
	if err != nil {
		return adminapp.AdminPropertyView{}, fmt.Errorf("invalid property status: %w", err)
	}

	ownerPhone, err := r.decryptPhone(ctx, row.OwnerPhone, row.OwnerPhoneEncrypted)
	if err != nil {
		return adminapp.AdminPropertyView{}, err
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
		OwnerPhone:  ownerPhone,
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

// ListLeases implements LeaseRepository.ListLeases.
func (r *AdminRepository) ListLeases(ctx context.Context, filters adminapp.AdminLeaseFilters) ([]adminapp.AdminLeaseView, int64, error) {
	total, err := r.q().CountLeasesAdmin(ctx, postgres.CountLeasesAdminParams{
		OwnerID:    pgconv.UUIDToPgtype(filters.OwnerID),
		PropertyID: pgconv.UUIDToPgtype(filters.PropertyID),
		Status:     filters.Status,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count leases: %w", err)
	}

	rows, err := r.q().ListLeasesAdmin(ctx, postgres.ListLeasesAdminParams{
		OwnerID:    pgconv.UUIDToPgtype(filters.OwnerID),
		PropertyID: pgconv.UUIDToPgtype(filters.PropertyID),
		Status:     filters.Status,
		Sort:       filters.Sort,
		Order:      filters.Order,
		Offset:     toInt32(filters.Offset),
		Limit:      toInt32(filters.Limit),
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
	row, err := r.q().GetLeaseByIDAdmin(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return adminapp.AdminLeaseView{}, adminapp.ErrNotFound
		}
		return adminapp.AdminLeaseView{}, fmt.Errorf("get lease: %w", err)
	}

	return r.leaseViewFromRow(adminGetLeaseRowToListRow(row), r.clock.Now())
}

// adminGetLeaseRowToListRow converts the generated get-row type to the
// list-row type. Both structs carry the same fields because both admin
// lease queries select leases columns plus property_name and tenant contact
// columns.
func adminGetLeaseRowToListRow(row postgres.GetLeaseByIDAdminRow) postgres.ListLeasesAdminRow {
	return postgres.ListLeasesAdminRow(row)
}

func (r *AdminRepository) leaseViewFromRow(row postgres.ListLeasesAdminRow, now time.Time) (adminapp.AdminLeaseView, error) {
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

	view := adminapp.AdminLeaseView{
		ID:                   lease.ID,
		OwnerID:              lease.OwnerID,
		PropertyID:           lease.PropertyID,
		PropertyName:         row.PropertyName,
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
	}

	if row.TcID.Valid {
		contact := leasesdomain.TenantContact{
			ID:         pgconv.UUIDFromPgtype(row.TcID),
			OwnerID:    pgconv.UUIDFromPgtype(row.TcOwnerID),
			Name:       pgconv.TextToString(row.TcName),
			Surname:    pgconv.TextToPtrString(row.TcSurname),
			Patronymic: pgconv.TextToPtrString(row.TcPatronymic),
			Phone:      pgconv.TextToPtrString(row.TcPhone),
			Email:      pgconv.TextToPtrString(row.TcEmail),
			Comment:    pgconv.TextToPtrString(row.TcComment),
			CreatedAt:  row.TcCreatedAt.Time,
			UpdatedAt:  row.TcUpdatedAt.Time,
		}
		view.TenantContact = &contact
	}

	return view, nil
}

// ListTenantContacts implements TenantContactRepository.ListTenantContacts.
func (r *AdminRepository) ListTenantContacts(ctx context.Context, filters adminapp.AdminTenantContactFilters) ([]adminapp.AdminTenantContactView, int64, error) {
	q := escapeLikePattern(filters.Q)
	total, err := r.q().CountTenantContactsAdmin(ctx, postgres.CountTenantContactsAdminParams{
		OwnerID: pgconv.UUIDToPgtype(filters.OwnerID),
		Q:       q,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count tenant contacts: %w", err)
	}

	rows, err := r.q().ListTenantContactsAdmin(ctx, postgres.ListTenantContactsAdminParams{
		OwnerID: pgconv.UUIDToPgtype(filters.OwnerID),
		Q:       q,
		Sort:    filters.Sort,
		Order:   filters.Order,
		Offset:  toInt32(filters.Offset),
		Limit:   toInt32(filters.Limit),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list tenant contacts: %w", err)
	}

	views := make([]adminapp.AdminTenantContactView, 0, len(rows))
	for _, row := range rows {
		view, err := r.tenantContactViewFromRow(ctx, row)
		if err != nil {
			return nil, 0, err
		}
		views = append(views, view)
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
	return r.tenantContactViewFromRow(ctx, adminGetTenantContactRowToListRow(row))
}

// adminGetTenantContactRowToListRow converts the generated get-row type to the
// list-row type. Both structs carry the same fields because both admin tenant
// contact queries select tenant_contacts columns plus owner phone columns.
func adminGetTenantContactRowToListRow(row postgres.GetTenantContactByIDAdminRow) postgres.ListTenantContactsAdminRow {
	return postgres.ListTenantContactsAdminRow(row)
}

func (r *AdminRepository) tenantContactViewFromRow(ctx context.Context, row postgres.ListTenantContactsAdminRow) (adminapp.AdminTenantContactView, error) {
	ownerPhone, err := r.decryptPhone(ctx, row.OwnerPhone, row.OwnerPhoneEncrypted)
	if err != nil {
		return adminapp.AdminTenantContactView{}, err
	}

	return adminapp.AdminTenantContactView{
		TenantContact: leasesdomain.TenantContact{
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
		},
		OwnerPhone: ownerPhone,
	}, nil
}

// ListOperations implements OperationRepository.ListOperations.
func (r *AdminRepository) ListOperations(ctx context.Context, filters adminapp.AdminOperationFilters) ([]adminapp.AdminOperationView, int64, error) {
	q := escapeLikePattern(filters.Q)
	total, err := r.q().CountOperationsAdmin(ctx, postgres.CountOperationsAdminParams{
		OwnerID:    pgconv.UUIDToPgtype(filters.OwnerID),
		Status:     filters.Status,
		Type:       filters.Type,
		PropertyID: pgconv.UUIDToPgtype(filters.PropertyID),
		LeaseID:    pgconv.UUIDToPgtype(filters.LeaseID),
		Q:          q,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count operations: %w", err)
	}

	rows, err := r.q().ListOperationsAdmin(ctx, postgres.ListOperationsAdminParams{
		OwnerID:    pgconv.UUIDToPgtype(filters.OwnerID),
		Status:     filters.Status,
		Type:       filters.Type,
		PropertyID: pgconv.UUIDToPgtype(filters.PropertyID),
		LeaseID:    pgconv.UUIDToPgtype(filters.LeaseID),
		Q:          q,
		Sort:       filters.Sort,
		Order:      filters.Order,
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
	return r.operationViewFromRow(adminGetOperationRowToListRow(row))
}

func (r *AdminRepository) operationViewFromRow(row postgres.ListOperationsAdminRow) (adminapp.AdminOperationView, error) {
	status, err := leasesdomain.ParseOperationStatus(row.Status)
	if err != nil {
		return adminapp.AdminOperationView{}, fmt.Errorf("invalid operation status: %w", err)
	}

	view := adminapp.AdminOperationView{
		ID:            pgconv.UUIDFromPgtype(row.ID),
		OwnerID:       pgconv.UUIDFromPgtype(row.OwnerID),
		PropertyID:    pgconv.UUIDFromPgtype(row.PropertyID),
		PropertyName:  row.PropertyName,
		Type:          leasesdomain.OperationType(row.Type),
		CategoryID:    pgconv.UUIDFromPgtype(row.CategoryID),
		CategoryName:  row.CategoryName,
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

// adminGetOperationRowToListRow converts the generated get-row type to the
// list-row type. Both structs carry the same fields because both admin
// operation queries select operations columns plus category_name.
func adminGetOperationRowToListRow(row postgres.GetOperationByIDAdminRow) postgres.ListOperationsAdminRow {
	return postgres.ListOperationsAdminRow(row)
}

// ListAuditLogs implements AuditLogRepository.ListAuditLogs.
func (r *AdminRepository) ListAuditLogs(ctx context.Context, filters adminapp.AdminAuditLogFilters) ([]adminapp.AdminAuditLogView, int64, error) {
	params := postgres.ListAuditLogsAdminParams{
		ActorID:    pgconv.UUIDToPgtype(filters.ActorID),
		Action:     filters.Action,
		EntityType: filters.EntityType,
		DateFrom:   timestamptzFromTime(filters.DateFrom),
		DateTo:     timestamptzFromTime(filters.DateTo),
		Sort:       filters.Sort,
		Order:      filters.Order,
		Offset:     toInt32(filters.Offset),
		Limit:      toInt32(filters.Limit),
	}

	total, err := r.q().CountAuditLogsAdmin(ctx, postgres.CountAuditLogsAdminParams{
		ActorID:    params.ActorID,
		Action:     params.Action,
		EntityType: params.EntityType,
		DateFrom:   params.DateFrom,
		DateTo:     params.DateTo,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	rows, err := r.q().ListAuditLogsAdmin(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit logs: %w", err)
	}

	views := make([]adminapp.AdminAuditLogView, 0, len(rows))
	for _, row := range rows {
		view, err := auditLogViewFromRow(row)
		if err != nil {
			return nil, 0, err
		}
		views = append(views, view)
	}

	return views, total, nil
}

// GetAuditLog implements AuditLogRepository.GetAuditLog.
func (r *AdminRepository) GetAuditLog(ctx context.Context, id uuid.UUID) (adminapp.AdminAuditLogView, error) {
	row, err := r.q().GetAuditLogByIDAdmin(ctx, pgconv.UUIDToPgtype(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return adminapp.AdminAuditLogView{}, adminapp.ErrNotFound
		}
		return adminapp.AdminAuditLogView{}, fmt.Errorf("get audit log: %w", err)
	}
	return auditLogViewFromRow(row)
}

func auditLogViewFromRow(row postgres.AuditLog) (adminapp.AdminAuditLogView, error) {
	ctxMap := map[string]any{}
	if len(row.Context) > 0 {
		if err := json.Unmarshal(row.Context, &ctxMap); err != nil {
			return adminapp.AdminAuditLogView{}, fmt.Errorf("decode audit log context: %w", err)
		}
	}
	// A jsonb 'null' unmarshals to a nil map; the API contract requires an object.
	if ctxMap == nil {
		ctxMap = map[string]any{}
	}

	view := adminapp.AdminAuditLogView{
		ID:         pgconv.UUIDFromPgtype(row.ID),
		CreatedAt:  row.CreatedAt.Time,
		ActorID:    pgconv.UUIDFromPgtypePtr(row.ActorID),
		ActorRole:  row.ActorRole,
		Action:     row.Action,
		EntityType: pgconv.TextToPtrString(row.EntityType),
		EntityID:   pgconv.UUIDFromPgtypePtr(row.EntityID),
		Context:    ctxMap,
		RequestID:  pgconv.TextToPtrString(row.RequestID),
	}
	if row.Ip != nil {
		ip := row.Ip.String()
		view.IP = &ip
	}
	return view, nil
}

// timestamptzFromTime converts a filter time to its pgtype form; a zero time
// becomes an invalid (NULL) value, which disables the filter on the SQL side.
func timestamptzFromTime(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// GetStats implements StatsRepository.GetStats. It runs a fixed set of
// aggregate queries; volumes are admin-scale, so no dedicated indexes or
// parallel fan-out are needed.
func (r *AdminRepository) GetStats(ctx context.Context) (adminapp.AdminStatsView, error) {
	var stats adminapp.AdminStatsView
	var err error

	if stats.UsersTotal, err = r.q().CountUsersTotalAdmin(ctx); err != nil {
		return stats, fmt.Errorf("count users: %w", err)
	}
	if stats.UsersNewLast30d, err = r.q().CountNewUsersLast30dAdmin(ctx); err != nil {
		return stats, fmt.Errorf("count new users: %w", err)
	}
	if stats.SubscriptionsActive, err = r.q().CountActiveSubscriptionsAdmin(ctx); err != nil {
		return stats, fmt.Errorf("count active subscriptions: %w", err)
	}

	propertyStats, err := r.q().GetPropertiesStatsAdmin(ctx)
	if err != nil {
		return stats, fmt.Errorf("count properties: %w", err)
	}
	stats.PropertiesActive = propertyStats.ActiveCount
	stats.PropertiesArchived = propertyStats.ArchivedCount

	if stats.LeasesTotal, err = r.q().CountLeasesTotalAdmin(ctx); err != nil {
		return stats, fmt.Errorf("count leases: %w", err)
	}
	if stats.OperationsTotal, err = r.q().CountOperationsTotalAdmin(ctx); err != nil {
		return stats, fmt.Errorf("count operations: %w", err)
	}

	paymentStats, err := r.q().GetSubscriptionPaymentsStatsLast30dAdmin(ctx)
	if err != nil {
		return stats, fmt.Errorf("aggregate subscription payments: %w", err)
	}
	stats.PaymentsSucceededTotalKopecksLast30d = paymentStats.SucceededTotalKopecks
	stats.PaymentsFailedCountLast30d = paymentStats.FailedCount
	stats.PaymentsRefundedCountLast30d = paymentStats.RefundedCount

	if stats.RecentUsers, err = r.recentUsers(ctx); err != nil {
		return stats, err
	}
	if stats.RecentPayments, err = r.recentPayments(ctx); err != nil {
		return stats, err
	}

	return stats, nil
}

func (r *AdminRepository) recentUsers(ctx context.Context) ([]adminapp.AdminRecentUserView, error) {
	rows, err := r.q().ListRecentUsersAdmin(ctx)
	if err != nil {
		return nil, fmt.Errorf("list recent users: %w", err)
	}

	views := make([]adminapp.AdminRecentUserView, 0, len(rows))
	for _, row := range rows {
		phone, err := r.decryptPhone(ctx, row.Phone, row.PhoneEncrypted)
		if err != nil {
			return nil, err
		}
		views = append(views, adminapp.AdminRecentUserView{
			ID:        pgconv.UUIDFromPgtype(row.ID),
			Phone:     phone,
			Name:      pgconv.TextToPtrString(row.Name),
			Surname:   pgconv.TextToPtrString(row.Surname),
			CreatedAt: row.CreatedAt.Time,
		})
	}
	return views, nil
}

func (r *AdminRepository) recentPayments(ctx context.Context) ([]adminapp.AdminRecentPaymentView, error) {
	rows, err := r.q().ListRecentSubscriptionPaymentsAdmin(ctx)
	if err != nil {
		return nil, fmt.Errorf("list recent subscription payments: %w", err)
	}

	views := make([]adminapp.AdminRecentPaymentView, 0, len(rows))
	for _, row := range rows {
		phone, err := r.decryptPhone(ctx, row.UserPhone, row.UserPhoneEncrypted)
		if err != nil {
			return nil, err
		}
		views = append(views, adminapp.AdminRecentPaymentView{
			ID:            pgconv.UUIDFromPgtype(row.ID),
			UserID:        pgconv.UUIDFromPgtype(row.UserID),
			UserPhone:     phone,
			AmountKopecks: row.AmountKopecks,
			Status:        row.Status,
			CreatedAt:     row.CreatedAt.Time,
		})
	}
	return views, nil
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

// likePatternEscaper escapes the ILIKE metacharacters in user-supplied search
// text. The matching SQL patterns use ESCAPE '\'.
var likePatternEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// escapeLikePattern trims and escapes a user-supplied substring so it can be
// safely embedded in an ILIKE '%...%' pattern. An empty result disables the
// filter on the SQL side.
func escapeLikePattern(q string) string {
	return likePatternEscaper.Replace(strings.TrimSpace(q))
}
