package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
)

func validateProperty(ctx context.Context, properties PropertyRepository, scope, propertyID uuid.UUID) error {
	exists, err := properties.ExistsByOwner(ctx, propertyID, scope)
	if err != nil {
		return fmt.Errorf("check property: %w", err)
	}
	if !exists {
		return ErrNotFound
	}
	return nil
}

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
