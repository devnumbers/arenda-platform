package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
	sharedpolicy "github.com/nambers/arenda-planform/apps/backend/internal/shared/policy"
)

// propertyStatusArchived mirrors the properties module archived status. The
// leases module must not depend on the properties module, so the status value
// is duplicated here.
const propertyStatusArchived = "archived"

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

func validateCategory(ctx context.Context, categories OperationCategoryRepository, scope uuid.UUID, opType domain.OperationType, categoryID uuid.UUID) error {
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

func getDefaultCategoryID(ctx context.Context, categories OperationCategoryRepository, scope uuid.UUID, code domain.OperationCategoryDefaultCode) (uuid.UUID, error) {
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
func resolveReadScope(ctx context.Context, policy sharedpolicy.Policy, properties PropertyRepository, actor, propertyID uuid.UUID) (uuid.UUID, error) {
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
// property-scoped data and returns the data owner (scope) for repository
// calls. RoleNone and RoleSuspended map to ErrNotFound (object privacy); a
// role that can view but not edit (viewer) maps to ErrForbidden.
func resolveWriteScope(ctx context.Context, policy sharedpolicy.Policy, properties PropertyRepository, actor, propertyID uuid.UUID) (uuid.UUID, error) {
	role, err := policy.RoleForProperty(ctx, actor, propertyID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("resolve role: %w", err)
	}
	if err := writeRoleGate(role); err != nil {
		return uuid.Nil, err
	}
	return propertyScope(ctx, properties, propertyID)
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
