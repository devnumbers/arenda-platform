package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/leases/domain"
)

func validateProperty(ctx context.Context, properties PropertyRepository, ownerID, propertyID uuid.UUID) error {
	exists, err := properties.ExistsByOwner(ctx, propertyID, ownerID)
	if err != nil {
		return fmt.Errorf("check property: %w", err)
	}
	if !exists {
		return ErrNotFound
	}
	return nil
}

func parseTypeAndCategory(typeStr, categoryStr string) (domain.OperationType, domain.OperationCategory, error) {
	opType, err := domain.ParseOperationType(typeStr)
	if err != nil {
		return "", "", newInvalidInputError(err.Error())
	}

	category, err := domain.ParseOperationCategory(categoryStr)
	if err != nil {
		return "", "", newInvalidInputError(err.Error())
	}

	if !domain.IsValidCategoryForType(category, opType) {
		return "", "", newInvalidInputError(fmt.Sprintf("category %q is not valid for type %q", category, opType))
	}

	return opType, category, nil
}
