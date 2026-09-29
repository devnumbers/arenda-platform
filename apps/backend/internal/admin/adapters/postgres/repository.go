// Package postgres implements the admin application ports with cross-context SQLC queries.
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
	contactsdomain "github.com/nambers/arenda-planform/apps/backend/internal/contacts/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/pgconv"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/encryption"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/generated/postgres"
	propertiesdomain "github.com/nambers/arenda-planform/apps/backend/internal/properties/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// AdminRepository implements the admin application ports using generated SQLC queries.
type AdminRepository struct {
	db    postgres.DBTX
	enc   encryption.Encryptor
	clock clock.Clock
}

// NewAdminRepository creates a new admin repository.
func NewAdminRepository(
	db postgres.DBTX, enc encryption.Encryptor, clk clock.Clock,
) *AdminRepository {
	return &AdminRepository{db: db, enc: enc, clock: clk}
}

func (r *AdminRepository) q() *postgres.Queries {
	return postgres.New(r.db)
}

// Compile-time interface checks.
var (
	_ adminapp.UserRepository     = (*AdminRepository)(nil)
	_ adminapp.PropertyRepository = (*AdminRepository)(nil)
	_ adminapp.ContactRepository  = (*AdminRepository)(nil)
	_ adminapp.StatsRepository    = (*AdminRepository)(nil)
	_ adminapp.AuditLogRepository = (*AdminRepository)(nil)
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

// ListProperties implements PropertyRepository.ListProperties.
func (r *AdminRepository) ListProperties(
	ctx context.Context, filters adminapp.AdminPropertyFilters,
) ([]adminapp.AdminPropertyView, int64, error) {
	q := pgconv.EscapeLikePattern(filters.Q)
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

func (r *AdminRepository) propertyViewFromRow(
	ctx context.Context, row postgres.ListPropertiesAdminRow,
) (adminapp.AdminPropertyView, error) {
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

	propertyID := pgconv.UUIDFromPgtype(row.ID)
	ownerID := pgconv.UUIDFromPgtype(row.OwnerID)

	var description *string
	if row.Description.Valid {
		description = &row.Description.String
	}

	attrs := propertiesdomain.Attributes{}
	if len(row.Attributes) > 0 {
		if err := json.Unmarshal(row.Attributes, &attrs); err != nil {
			return adminapp.AdminPropertyView{}, fmt.Errorf("decode property attributes: %w", err)
		}
		if attrs == nil {
			attrs = propertiesdomain.Attributes{}
		}
	}

	return adminapp.AdminPropertyView{
		ID:          propertyID,
		OwnerID:     ownerID,
		OwnerPhone:  ownerPhone,
		Name:        row.Name,
		Type:        propType,
		Address:     row.Address,
		Description: description,
		Attributes:  attrs,
		Status:      status,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}, nil
}

// ListContacts implements ContactRepository.ListContacts: the property's
// bound cards of the contacts context (ADR 0054), the display name composed
// from the name fields; the phone column is plaintext, so no decryption is
// needed.
func (r *AdminRepository) ListContacts(
	ctx context.Context, filters adminapp.AdminContactFilters,
) ([]adminapp.AdminContactView, int64, error) {
	total, err := r.q().CountContactsAdmin(ctx, pgconv.UUIDToPgtype(filters.PropertyID))
	if err != nil {
		return nil, 0, fmt.Errorf("count contacts: %w", err)
	}

	rows, err := r.q().ListContactsAdmin(ctx, postgres.ListContactsAdminParams{
		PropertyID: pgconv.UUIDToPgtype(filters.PropertyID),
		Limit:      toInt32(filters.Limit),
		Offset:     toInt32(filters.Offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list contacts: %w", err)
	}

	views := make([]adminapp.AdminContactView, 0, len(rows))
	for _, row := range rows {
		name := contactsdomain.Contact{
			FirstName:  row.FirstName,
			LastName:   pgconv.TextToString(row.LastName),
			Patronymic: pgconv.TextToString(row.Patronymic),
		}.FullName()
		views = append(views, adminapp.AdminContactView{
			ID:         pgconv.UUIDFromPgtype(row.ID),
			PropertyID: pgconv.UUIDFromPgtype(row.PropertyID),
			OwnerID:    pgconv.UUIDFromPgtype(row.OwnerID),
			Name:       name,
			Phone:      pgconv.TextToString(row.Phone),
			CreatedAt:  row.CreatedAt.Time,
			UpdatedAt:  row.UpdatedAt.Time,
		})
	}

	return views, total, nil
}

// ListAuditLogs implements AuditLogRepository.ListAuditLogs.
func (r *AdminRepository) ListAuditLogs(
	ctx context.Context, filters adminapp.AdminAuditLogFilters,
) ([]adminapp.AdminAuditLogView, int64, error) {
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
