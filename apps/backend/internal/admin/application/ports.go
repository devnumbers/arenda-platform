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
}

// PropertyRepository provides cross-user property reads for the admin context.
type PropertyRepository interface {
	ListProperties(ctx context.Context, filters AdminPropertyFilters) ([]AdminPropertyView, int64, error)
	GetProperty(ctx context.Context, id uuid.UUID) (AdminPropertyView, error)
}

// ContactRepository provides cross-user contact reads for the admin context
// (ADR 0054: the book lives in the contacts context; admin only reads).
type ContactRepository interface {
	ListContacts(ctx context.Context, filters AdminContactFilters) ([]AdminContactView, int64, error)
}

// StatsRepository provides platform-wide aggregates for the admin dashboard.
type StatsRepository interface {
	GetStats(ctx context.Context) (AdminStatsView, error)
}

// AuditLogRepository provides cross-user audit log reads for the admin context.
type AuditLogRepository interface {
	ListAuditLogs(ctx context.Context, filters AdminAuditLogFilters) ([]AdminAuditLogView, int64, error)
	GetAuditLog(ctx context.Context, id uuid.UUID) (AdminAuditLogView, error)
}

// SubscriptionProvider loads a user's subscription for the admin detail view.
type SubscriptionProvider interface {
	GetSubscription(ctx context.Context, userID uuid.UUID) (billingapp.SubscriptionView, error)
}
