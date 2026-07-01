package application

import (
	"context"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
)

// UserRepository provides cross-user user reads for the admin context.
type UserRepository interface {
	ListUsers(ctx context.Context, filters AdminUserFilters) ([]AdminUserView, int64, error)
	GetUser(ctx context.Context, id uuid.UUID) (AdminUserView, error)
	CountActivePropertiesByOwner(ctx context.Context, ownerID uuid.UUID) (int64, error)
	CountArchivedPropertiesByOwner(ctx context.Context, ownerID uuid.UUID) (int64, error)
	CountLeasesByOwner(ctx context.Context, ownerID uuid.UUID) (int64, error)
	CountOperationsByOwner(ctx context.Context, ownerID uuid.UUID) (int64, error)
	CountTenantContactsByOwner(ctx context.Context, ownerID uuid.UUID) (int64, error)
}

// PropertyRepository provides cross-user property reads for the admin context.
type PropertyRepository interface {
	ListUserProperties(ctx context.Context, userID uuid.UUID, filters AdminPropertyFilters) ([]AdminPropertyView, int64, error)
	GetProperty(ctx context.Context, id uuid.UUID) (AdminPropertyView, error)
}

// LeaseRepository provides cross-user lease reads for the admin context.
type LeaseRepository interface {
	ListUserLeases(ctx context.Context, userID uuid.UUID, filters AdminListFilters) ([]AdminLeaseView, int64, error)
	GetLease(ctx context.Context, id uuid.UUID) (AdminLeaseView, error)
}

// TenantContactRepository provides cross-user tenant contact reads for the admin context.
type TenantContactRepository interface {
	ListUserTenantContacts(ctx context.Context, userID uuid.UUID, filters AdminListFilters) ([]AdminTenantContactView, int64, error)
	GetTenantContact(ctx context.Context, id uuid.UUID) (AdminTenantContactView, error)
}

// OperationRepository provides cross-user operation reads for the admin context.
type OperationRepository interface {
	ListUserOperations(ctx context.Context, userID uuid.UUID, filters AdminOperationFilters) ([]AdminOperationView, int64, error)
	GetOperation(ctx context.Context, id uuid.UUID) (AdminOperationView, error)
}

// SubscriptionProvider loads a user's subscription for the admin detail view.
type SubscriptionProvider interface {
	GetSubscription(ctx context.Context, userID uuid.UUID) (billingapp.SubscriptionView, error)
}
