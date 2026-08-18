package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// Errors returned by the admin application service.
var (
	ErrNotFound      = errors.New("not found")
	ErrInvalidFilter = errors.New("invalid filter")
)

// AdminService orchestrates cross-user read-only admin operations.
type AdminService struct {
	users            UserRepository
	properties       PropertyRepository
	leases           LeaseRepository
	tenantContacts   TenantContactRepository
	operations       OperationRepository
	stats            StatsRepository
	subscriptions    SubscriptionProvider
	auditLogs        AuditLogRepository
	propertyContacts PropertyContactRepository
	clock            clock.Clock
}

// NewAdminService creates a new admin application service.
func NewAdminService(
	users UserRepository,
	properties PropertyRepository,
	leases LeaseRepository,
	tenantContacts TenantContactRepository,
	operations OperationRepository,
	stats StatsRepository,
	subscriptions SubscriptionProvider,
	auditLogs AuditLogRepository,
	propertyContacts PropertyContactRepository,
	clk clock.Clock,
) *AdminService {
	return &AdminService{
		users:            users,
		properties:       properties,
		leases:           leases,
		tenantContacts:   tenantContacts,
		operations:       operations,
		stats:            stats,
		subscriptions:    subscriptions,
		auditLogs:        auditLogs,
		propertyContacts: propertyContacts,
		clock:            clk,
	}
}

func normalizePagination(limit, offset int) (normLimit, normOffset int) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// Sort field whitelists for admin list endpoints, in API (camelCase) naming.
// The SQL layer maps these fixed values to columns via CASE expressions; user
// input is never interpolated into SQL. Extend the SQL CASE arms together with
// these lists when adding a new sortable column.
var (
	adminUserSortFields          = []string{"createdAt", "updatedAt"}
	adminPropertySortFields      = []string{"name", "createdAt", "updatedAt", "status"}
	adminLeaseSortFields         = []string{"startDate", "updatedAt", "status", "rentAmountKopecks"}
	adminTenantContactSortFields = []string{"name", "updatedAt"}
	adminOperationSortFields     = []string{"operationDate", "amountKopecks", "status"}
	adminAuditLogSortFields      = []string{"createdAt"}
)

// normalizeSort validates the requested sort field and order against the
// endpoint whitelist. An empty sort selects the endpoint's default ordering.
// An empty order defaults to desc, matching all existing default orderings.
func normalizeSort(sort, order string, allowed []string) (normSort, normOrder string, err error) {
	sort = strings.TrimSpace(sort)
	if sort == "" {
		return "", "", nil
	}
	if !slices.Contains(allowed, sort) {
		return "", "", fmt.Errorf("%w: unsupported sort field %q (allowed: %s)", ErrInvalidFilter, sort, strings.Join(allowed, ", "))
	}
	order = strings.TrimSpace(order)
	if order == "" {
		order = "desc"
	}
	if order != "asc" && order != "desc" {
		return "", "", fmt.Errorf("%w: unsupported sort order %q (allowed: asc, desc)", ErrInvalidFilter, order)
	}
	return sort, order, nil
}

// ListUsers returns a paginated list of users with a total count.
func (s *AdminService) ListUsers(ctx context.Context, filters AdminUserFilters) ([]AdminUserView, int64, error) {
	filters.Limit, filters.Offset = normalizePagination(filters.Limit, filters.Offset)
	var err error
	if filters.Sort, filters.Order, err = normalizeSort(filters.Sort, filters.Order, adminUserSortFields); err != nil {
		return nil, 0, err
	}
	return s.users.ListUsers(ctx, filters)
}

// GetUser returns a user together with their subscription and resource stats.
func (s *AdminService) GetUser(ctx context.Context, id uuid.UUID) (AdminUserDetailView, error) {
	user, err := s.users.GetUser(ctx, id)
	if err != nil {
		return AdminUserDetailView{}, err
	}

	stats, err := s.userStats(ctx, id)
	if err != nil {
		return AdminUserDetailView{}, err
	}

	subView, err := s.subscriptions.GetSubscription(ctx, id)
	var subPtr *billingapp.SubscriptionView
	switch {
	case err == nil:
		subPtr = &subView
	case errors.Is(err, billingapp.ErrNotFound), errors.Is(err, billingapp.ErrSubscriptionNotFound):
		// Subscription may be missing for incomplete accounts; leave it nil.
	default:
		return AdminUserDetailView{}, fmt.Errorf("load subscription: %w", err)
	}

	return AdminUserDetailView{
		User:         user,
		Stats:        stats,
		Subscription: subPtr,
	}, nil
}

func (s *AdminService) userStats(ctx context.Context, id uuid.UUID) (AdminUserStats, error) {
	var stats AdminUserStats
	var err error

	stats.ActivePropertiesCount, err = s.users.CountActivePropertiesByOwner(ctx, id)
	if err != nil {
		return stats, fmt.Errorf("count active properties: %w", err)
	}
	stats.ArchivedPropertiesCount, err = s.users.CountArchivedPropertiesByOwner(ctx, id)
	if err != nil {
		return stats, fmt.Errorf("count archived properties: %w", err)
	}
	stats.LeasesCount, err = s.users.CountLeasesByOwner(ctx, id)
	if err != nil {
		return stats, fmt.Errorf("count leases: %w", err)
	}
	stats.OperationsCount, err = s.users.CountOperationsByOwner(ctx, id)
	if err != nil {
		return stats, fmt.Errorf("count operations: %w", err)
	}
	stats.TenantContactsCount, err = s.users.CountTenantContactsByOwner(ctx, id)
	if err != nil {
		return stats, fmt.Errorf("count tenant contacts: %w", err)
	}

	return stats, nil
}

// ListUserProperties returns a paginated list of a user's properties.
func (s *AdminService) ListUserProperties(ctx context.Context, userID uuid.UUID, filters AdminPropertyFilters) ([]AdminPropertyView, int64, error) {
	filters.OwnerID = userID
	return s.ListProperties(ctx, filters)
}

// ListProperties returns a paginated cross-user list of properties.
func (s *AdminService) ListProperties(ctx context.Context, filters AdminPropertyFilters) ([]AdminPropertyView, int64, error) {
	filters.Limit, filters.Offset = normalizePagination(filters.Limit, filters.Offset)
	if filters.Status == "all" {
		filters.Status = ""
	}
	var err error
	if filters.Sort, filters.Order, err = normalizeSort(filters.Sort, filters.Order, adminPropertySortFields); err != nil {
		return nil, 0, err
	}
	return s.properties.ListProperties(ctx, filters)
}

// GetProperty returns a single property by ID.
func (s *AdminService) GetProperty(ctx context.Context, id uuid.UUID) (AdminPropertyView, error) {
	return s.properties.GetProperty(ctx, id)
}

// ListUserLeases returns a paginated list of a user's leases.
func (s *AdminService) ListUserLeases(ctx context.Context, userID uuid.UUID, filters AdminLeaseFilters) ([]AdminLeaseView, int64, error) {
	filters.OwnerID = userID
	return s.ListLeases(ctx, filters)
}

// ListLeases returns a paginated cross-user list of leases.
func (s *AdminService) ListLeases(ctx context.Context, filters AdminLeaseFilters) ([]AdminLeaseView, int64, error) {
	filters.Limit, filters.Offset = normalizePagination(filters.Limit, filters.Offset)
	var err error
	if filters.Sort, filters.Order, err = normalizeSort(filters.Sort, filters.Order, adminLeaseSortFields); err != nil {
		return nil, 0, err
	}
	return s.leases.ListLeases(ctx, filters)
}

// GetLease returns a single lease by ID.
func (s *AdminService) GetLease(ctx context.Context, id uuid.UUID) (AdminLeaseView, error) {
	return s.leases.GetLease(ctx, id)
}

// ListUserTenantContacts returns a paginated list of a user's tenant contacts.
func (s *AdminService) ListUserTenantContacts(ctx context.Context, userID uuid.UUID, filters AdminTenantContactFilters) ([]AdminTenantContactView, int64, error) {
	filters.OwnerID = userID
	return s.ListTenantContacts(ctx, filters)
}

// ListTenantContacts returns a paginated cross-user list of tenant contacts.
func (s *AdminService) ListTenantContacts(ctx context.Context, filters AdminTenantContactFilters) ([]AdminTenantContactView, int64, error) {
	filters.Limit, filters.Offset = normalizePagination(filters.Limit, filters.Offset)
	var err error
	if filters.Sort, filters.Order, err = normalizeSort(filters.Sort, filters.Order, adminTenantContactSortFields); err != nil {
		return nil, 0, err
	}
	return s.tenantContacts.ListTenantContacts(ctx, filters)
}

// GetTenantContact returns a single tenant contact by ID.
func (s *AdminService) GetTenantContact(ctx context.Context, id uuid.UUID) (AdminTenantContactView, error) {
	return s.tenantContacts.GetTenantContact(ctx, id)
}

// ListPropertyContacts returns a paginated list of property contacts for a property.
// Sort is fixed to created_at ASC; there is no client-controlled sort.
func (s *AdminService) ListPropertyContacts(ctx context.Context, filters AdminPropertyContactFilters) ([]AdminPropertyContactView, int64, error) {
	filters.Limit, filters.Offset = normalizePagination(filters.Limit, filters.Offset)
	return s.propertyContacts.ListPropertyContacts(ctx, filters)
}

// ListUserOperations returns a paginated list of a user's operations.
func (s *AdminService) ListUserOperations(ctx context.Context, userID uuid.UUID, filters AdminOperationFilters) ([]AdminOperationView, int64, error) {
	filters.OwnerID = userID
	return s.ListOperations(ctx, filters)
}

// ListOperations returns a paginated cross-user list of operations.
func (s *AdminService) ListOperations(ctx context.Context, filters AdminOperationFilters) ([]AdminOperationView, int64, error) {
	filters.Limit, filters.Offset = normalizePagination(filters.Limit, filters.Offset)
	var err error
	if filters.Sort, filters.Order, err = normalizeSort(filters.Sort, filters.Order, adminOperationSortFields); err != nil {
		return nil, 0, err
	}
	return s.operations.ListOperations(ctx, filters)
}

// GetOperation returns a single operation by ID.
func (s *AdminService) GetOperation(ctx context.Context, id uuid.UUID) (AdminOperationView, error) {
	return s.operations.GetOperation(ctx, id)
}

// GetStats returns platform-wide counters and recent activity for the admin dashboard.
func (s *AdminService) GetStats(ctx context.Context) (AdminStatsView, error) {
	return s.stats.GetStats(ctx)
}

// ListUserAuditLogs returns a paginated list of a user's audit log entries.
func (s *AdminService) ListUserAuditLogs(ctx context.Context, userID uuid.UUID, filters AdminAuditLogFilters) ([]AdminAuditLogView, int64, error) {
	filters.ActorID = userID
	return s.ListAuditLogs(ctx, filters)
}

// ListAuditLogs returns a paginated cross-user list of audit log entries.
func (s *AdminService) ListAuditLogs(ctx context.Context, filters AdminAuditLogFilters) ([]AdminAuditLogView, int64, error) {
	filters.Limit, filters.Offset = normalizePagination(filters.Limit, filters.Offset)
	var err error
	if filters.Sort, filters.Order, err = normalizeSort(filters.Sort, filters.Order, adminAuditLogSortFields); err != nil {
		return nil, 0, err
	}
	return s.auditLogs.ListAuditLogs(ctx, filters)
}

// GetAuditLog returns a single audit log entry by ID.
func (s *AdminService) GetAuditLog(ctx context.Context, id uuid.UUID) (AdminAuditLogView, error) {
	return s.auditLogs.GetAuditLog(ctx, id)
}
