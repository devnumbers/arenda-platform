package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// Property statuses mirror the properties module vocabulary. The leases module
// must not depend on the properties module, so the status values are
// duplicated here.
const (
	propertyStatusActive      = "active"
	propertyStatusMaintenance = "maintenance"
	propertyStatusArchived    = "archived"
)

// Audit context keys shared by the leases application services (same naming as
// the access module, wave #360).
const (
	auditKeyFields     = "fields"
	auditKeyPropertyID = "property_id"
)

// validatePropertyNotArchived verifies that the property exists for the owner
// and is not archived. Mutations linked to an archived property are rejected;
// a zero propertyID means the entity is not linked to a property and the
// check is skipped.
func validatePropertyNotArchived(ctx context.Context, properties PropertyRepository, scope, propertyID uuid.UUID) error {
	if propertyID == uuid.Nil {
		return nil
	}
	status, err := properties.GetStatusByOwner(ctx, propertyID, scope)
	if err != nil {
		return fmt.Errorf("check property: %w", err)
	}
	if status == "" {
		return ErrNotFound
	}
	if status == propertyStatusArchived {
		return ErrArchivedProperty
	}
	return nil
}

func parseTypeAndCategory(cmdType string, categoryID uuid.UUID) (domain.OperationType, uuid.UUID, error) {
	opType, err := domain.ParseOperationType(cmdType)
	if err != nil {
		return "", uuid.Nil, newInvalidInputError(err.Error())
	}
	if categoryID == uuid.Nil {
		return "", uuid.Nil, newInvalidInputError("category_id is required")
	}
	return opType, categoryID, nil
}

func validateCategory(
	ctx context.Context,
	categories OperationCategoryRepository,
	scope uuid.UUID,
	opType domain.OperationType,
	categoryID uuid.UUID,
) error {
	cat, err := categories.GetByIDAndOwner(ctx, categoryID, scope)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return newInvalidInputError("category not found")
		}
		return fmt.Errorf("get category: %w", err)
	}
	if cat.Type != opType {
		return newInvalidInputError("category type mismatch")
	}
	return nil
}

func getDefaultCategoryID(
	ctx context.Context,
	categories OperationCategoryRepository,
	scope uuid.UUID,
	code domain.OperationCategoryDefaultCode,
) (uuid.UUID, error) {
	cat, err := categories.GetByOwnerAndCode(ctx, scope, code)
	if err != nil {
		return uuid.Nil, fmt.Errorf("get default category %q: %w", code, err)
	}
	return cat.ID, nil
}

// resolveReadScope applies the T3 shared-access read gate for property-scoped
// data and returns the data owner (scope) for repository calls. Any role
// without view capability — including a suspended membership — maps to
// ErrNotFound so the existence of an object is never revealed (issue #166).
func resolveReadScope(
	ctx context.Context,
	policy sharedpolicy.Policy,
	properties PropertyRepository,
	actor, propertyID uuid.UUID,
) (uuid.UUID, error) {
	role, err := policy.RoleForProperty(ctx, actor, propertyID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("resolve role: %w", err)
	}
	if !sharedpolicy.CanView(role) {
		return uuid.Nil, ErrNotFound
	}
	return propertyScope(ctx, properties, propertyID)
}

// resolveWriteScope applies the T3 shared-access write gate for
// property-scoped data and returns the resolved role alongside the data owner
// (scope) for repository calls. RoleNone and RoleSuspended map to ErrNotFound
// (object privacy); a role that can view but not edit (viewer) maps to
// ErrForbidden. The role is returned so callers can attribute audit entries
// to the actor's real role (issue #166 follow-up).
func resolveWriteScope(
	ctx context.Context,
	policy sharedpolicy.Policy,
	properties PropertyRepository,
	actor, propertyID uuid.UUID,
) (sharedpolicy.Role, uuid.UUID, error) {
	role, err := policy.RoleForProperty(ctx, actor, propertyID)
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("resolve role: %w", err)
	}
	if err := writeRoleGate(role); err != nil {
		return "", uuid.Nil, err
	}
	scope, err := propertyScope(ctx, properties, propertyID)
	if err != nil {
		return "", uuid.Nil, err
	}
	return role, scope, nil
}

// resolveOwnerWideWriteScope resolves the data owner (scope) for owner-wide
// entities (operation categories, tenant contacts) written in the context of
// a property (issue #157 follow-up, card #145 decision): a member with the
// edit capability creates the entity in the account of the property's data
// owner. A nil or zero property context keeps the historical owner behaviour
// (scope = actor, no policy call). Otherwise the actor's owner-wide role is
// resolved via policy.Role — the strongest role across the owner's
// properties, consistent with the T2a list paths — and gated by
// writeRoleGate: none/suspended map to ErrNotFound (object privacy), viewer
// to ErrForbidden. A property context without the policy or the property
// repository wired maps to ErrNotFound as well: the degraded pre-T2a service
// can only write the actor's own scope, which is the branch above.
func resolveOwnerWideWriteScope(
	ctx context.Context,
	policy sharedpolicy.Policy,
	properties PropertyRepository,
	actor uuid.UUID,
	propertyID *uuid.UUID,
) (sharedpolicy.Role, uuid.UUID, error) {
	if propertyID == nil || *propertyID == uuid.Nil {
		return sharedpolicy.RoleOwner, actor, nil
	}
	if policy == nil || properties == nil {
		return "", uuid.Nil, ErrNotFound
	}
	scope, err := propertyScope(ctx, properties, *propertyID)
	if err != nil {
		return "", uuid.Nil, err
	}
	role, err := policy.Role(ctx, actor, scope)
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("resolve role: %w", err)
	}
	if err := writeRoleGate(role); err != nil {
		return "", uuid.Nil, err
	}
	return role, scope, nil
}

// writeRoleGate maps a resolved role to the write-gate outcome shared by all
// leases write paths (issue #166).
func writeRoleGate(role sharedpolicy.Role) error {
	if role == sharedpolicy.RoleNone || role == sharedpolicy.RoleSuspended {
		return ErrNotFound
	}
	if !sharedpolicy.CanEdit(role) {
		return ErrForbidden
	}
	return nil
}

// propertyScope resolves the property's data owner for scoped repository
// calls. A missing property maps to ErrNotFound.
func propertyScope(ctx context.Context, properties PropertyRepository, propertyID uuid.UUID) (uuid.UUID, error) {
	scope, err := properties.GetOwnerByID(ctx, propertyID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return uuid.Nil, ErrNotFound
		}
		return uuid.Nil, fmt.Errorf("resolve property owner: %w", err)
	}
	return scope, nil
}

// roleForStandalone resolves the actor's role for an entity addressed by its
// own id (not by a property path). Entities detached from their property
// (property_id IS NULL) have no membership surface and are owner-only.
func roleForStandalone(ctx context.Context, policy sharedpolicy.Policy, actor, propertyID, ownerID uuid.UUID) (sharedpolicy.Role, error) {
	if propertyID == uuid.Nil {
		if actor == ownerID {
			return sharedpolicy.RoleOwner, nil
		}
		return sharedpolicy.RoleNone, nil
	}
	role, err := policy.RoleForProperty(ctx, actor, propertyID)
	if err != nil {
		return "", fmt.Errorf("resolve role: %w", err)
	}
	return role, nil
}

// actorRoleFromPolicyRole maps a policy role to the audit actor role so
// actions of shared-access members are attributed to their real role instead
// of being masked as the owner's own (issue #166 follow-up). Roles that never
// reach a Record call through the write gates (suspended, none) and any
// unknown role fall back to the historical owner attribution.
func actorRoleFromPolicyRole(role sharedpolicy.Role) auditdomain.ActorRole {
	switch role {
	case sharedpolicy.RoleFullAccess:
		return auditdomain.ActorRoleFullAccess
	case sharedpolicy.RoleViewer:
		return auditdomain.ActorRoleViewer
	default:
		return auditdomain.ActorRoleOwner
	}
}
