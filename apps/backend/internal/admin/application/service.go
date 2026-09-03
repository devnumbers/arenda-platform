// Package application holds the admin use cases and ports: cross-user, cross-context read-only oversight operations.
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
	users         UserRepository
	properties    PropertyRepository
	stats         StatsRepository
	subscriptions SubscriptionProvider
	auditLogs     AuditLogRepository
	contacts      ContactRepository
	clock         clock.Clock
}

// NewAdminService creates a new admin application service.
func NewAdminService(
	users UserRepository,
	properties PropertyRepository,
	stats StatsRepository,
	subscriptions SubscriptionProvider,
	auditLogs AuditLogRepository,
	contacts ContactRepository,
	clk clock.Clock,
) *AdminService {
	return &AdminService{
		users:         users,
		properties:    properties,
		stats:         stats,
		subscriptions: subscriptions,
		auditLogs:     auditLogs,
		contacts:      contacts,
		clock:         clk,
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

// Sort field names shared across the whitelists below.
const (
	sortFieldCreatedAt = "createdAt"
	sortFieldUpdatedAt = "updatedAt"
	sortFieldStatus    = "status"
)

// Sort field whitelists for admin list endpoints, in API (camelCase) naming.
// The SQL layer maps these fixed values to columns via CASE expressions; user
// input is never interpolated into SQL. Extend the SQL CASE arms together with
// these lists when adding a new sortable column.
var (
	adminUserSortFields     = []string{sortFieldCreatedAt, sortFieldUpdatedAt}
	adminPropertySortFields = []string{"name", sortFieldCreatedAt, sortFieldUpdatedAt, sortFieldStatus}
	adminAuditLogSortFields = []string{sortFieldCreatedAt}
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

	return stats, nil
}

// ListUserProperties returns a paginated list of a user's properties.
func (s *AdminService) ListUserProperties(
	ctx context.Context, userID uuid.UUID, filters AdminPropertyFilters,
) ([]AdminPropertyView, int64, error) {
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

// ListContacts returns a paginated list of the contact cards bound to a
// property (ADR 0051).
// Sort is fixed to created_at ASC; there is no client-controlled sort.
func (s *AdminService) ListContacts(
	ctx context.Context, filters AdminContactFilters,
) ([]AdminContactView, int64, error) {
	filters.Limit, filters.Offset = normalizePagination(filters.Limit, filters.Offset)
	return s.contacts.ListContacts(ctx, filters)
}

// GetStats returns platform-wide counters and recent activity for the admin dashboard.
func (s *AdminService) GetStats(ctx context.Context) (AdminStatsView, error) {
	return s.stats.GetStats(ctx)
}

// ListUserAuditLogs returns a paginated list of a user's audit log entries.
func (s *AdminService) ListUserAuditLogs(
	ctx context.Context, userID uuid.UUID, filters AdminAuditLogFilters,
) ([]AdminAuditLogView, int64, error) {
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
