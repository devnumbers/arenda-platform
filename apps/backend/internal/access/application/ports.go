// Package application holds the access bounded context use cases and ports
// (issue #156, T3): managing property memberships and the membership-aware
// authorization policy.
package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/access/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// MembershipRepository persists property membership records. Membership rows
// are not owner-scoped at the SQL level: access to management operations is
// regulated by the policy port in the application layer.
type MembershipRepository interface {
	Create(ctx context.Context, membership domain.Membership) (domain.Membership, error)
	GetByID(ctx context.Context, id, propertyID uuid.UUID) (domain.Membership, error)
	GetByPropertyAndUser(ctx context.Context, propertyID, userID uuid.UUID) (domain.Membership, error)
	GetRole(ctx context.Context, propertyID, userID uuid.UUID) (domain.Role, error)
	ListByProperty(ctx context.Context, propertyID uuid.UUID) ([]domain.Membership, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Membership, error)
	UpdateRole(ctx context.Context, id, propertyID uuid.UUID, role domain.Role) (domain.Membership, error)
	Delete(ctx context.Context, id, propertyID uuid.UUID) error
	WithTx(tx transaction.Tx) MembershipRepository
}

// PropertyOwnerResolver resolves the data owner of a property. Used by the
// policy and the access service to turn a property id into the owner id
// (scope) without leaking the property's existence to unauthorized actors.
type PropertyOwnerResolver interface {
	GetOwnerID(ctx context.Context, propertyID uuid.UUID) (uuid.UUID, error)
}

// MemberUser describes a user for member display purposes. Only non-PII
// display fields are exposed; emails and phones are never placed in audit
// context (ADR 0020).
type MemberUser struct {
	ID       uuid.UUID
	Name     *string
	Surname  *string
	Phone    string
	HasEmail bool
}

// UserLookup resolves registered users by id for member display.
type UserLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (MemberUser, error)
	GetByEmail(ctx context.Context, email string) (MemberUser, error)
}

// txBeginner begins a database transaction. Mirrors the local interface used
// by every other bounded context's service.
type txBeginner interface {
	Begin(ctx context.Context) (transaction.Tx, error)
}
