package application

import (
	"context"
	"errors"
	"fmt"

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
	users          UserRepository
	properties     PropertyRepository
	leases         LeaseRepository
	tenantContacts TenantContactRepository
	operations     OperationRepository
	subscriptions  SubscriptionProvider
	clock          clock.Clock
}

// NewAdminService creates a new admin application service.
func NewAdminService(
	users UserRepository,
	properties PropertyRepository,
	leases LeaseRepository,
	tenantContacts TenantContactRepository,
	operations OperationRepository,
	subscriptions SubscriptionProvider,
	clock clock.Clock,
) *AdminService {
	return &AdminService{
		users:          users,
		properties:     properties,
		leases:         leases,
		tenantContacts: tenantContacts,
		operations:     operations,
		subscriptions:  subscriptions,
		clock:          clock,
	}
}

func normalizePagination(limit, offset int) (int, int) {
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

// ListUsers returns a paginated list of users with a total count.
func (s *AdminService) ListUsers(ctx context.Context, filters AdminUserFilters) ([]AdminUserView, int64, error) {
	filters.Limit, filters.Offset = normalizePagination(filters.Limit, filters.Offset)
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
	filters.Limit, filters.Offset = normalizePagination(filters.Limit, filters.Offset)
	if filters.Status == "all" {
		filters.Status = ""
	}
	return s.properties.ListUserProperties(ctx, userID, filters)
}

// GetProperty returns a single property by ID.
func (s *AdminService) GetProperty(ctx context.Context, id uuid.UUID) (AdminPropertyView, error) {
	return s.properties.GetProperty(ctx, id)
}

// ListUserLeases returns a paginated list of a user's leases.
func (s *AdminService) ListUserLeases(ctx context.Context, userID uuid.UUID, filters AdminListFilters) ([]AdminLeaseView, int64, error) {
	filters.Limit, filters.Offset = normalizePagination(filters.Limit, filters.Offset)
	return s.leases.ListUserLeases(ctx, userID, filters)
}

// GetLease returns a single lease by ID.
func (s *AdminService) GetLease(ctx context.Context, id uuid.UUID) (AdminLeaseView, error) {
	return s.leases.GetLease(ctx, id)
}

// ListUserTenantContacts returns a paginated list of a user's tenant contacts.
func (s *AdminService) ListUserTenantContacts(ctx context.Context, userID uuid.UUID, filters AdminListFilters) ([]AdminTenantContactView, int64, error) {
	filters.Limit, filters.Offset = normalizePagination(filters.Limit, filters.Offset)
	return s.tenantContacts.ListUserTenantContacts(ctx, userID, filters)
}

// GetTenantContact returns a single tenant contact by ID.
func (s *AdminService) GetTenantContact(ctx context.Context, id uuid.UUID) (AdminTenantContactView, error) {
	return s.tenantContacts.GetTenantContact(ctx, id)
}

// ListUserOperations returns a paginated list of a user's operations.
func (s *AdminService) ListUserOperations(ctx context.Context, userID uuid.UUID, filters AdminOperationFilters) ([]AdminOperationView, int64, error) {
	filters.Limit, filters.Offset = normalizePagination(filters.Limit, filters.Offset)
	return s.operations.ListUserOperations(ctx, userID, filters)
}

// GetOperation returns a single operation by ID.
func (s *AdminService) GetOperation(ctx context.Context, id uuid.UUID) (AdminOperationView, error) {
	return s.operations.GetOperation(ctx, id)
}
