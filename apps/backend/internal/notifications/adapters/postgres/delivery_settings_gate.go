package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
)

// The adapter satisfies the consumer-declared port (CODING_STANDARDS).
var _ application.DeliverySettings = (*DeliverySettingsGate)(nil)

// DeliverySettingsGate answers the category × channel matrix at delivery
// time: the email leg asks for the recipient's account-level matrix, the
// missing row reads as the all-on default. The push side carries its matrix
// on the subscription itself and needs no gate.
type DeliverySettingsGate struct {
	email *EmailPreferencesRepository
}

// NewDeliverySettingsGate creates the delivery-time settings gate.
func NewDeliverySettingsGate(email *EmailPreferencesRepository) *DeliverySettingsGate {
	return &DeliverySettingsGate{email: email}
}

// EmailAllowed reports whether the account's email setting lets the
// category's email leg through.
func (g *DeliverySettingsGate) EmailAllowed(ctx context.Context, userID uuid.UUID, category domain.Category) (bool, error) {
	prefs, err := g.email.Get(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("delivery settings gate: %w", err)
	}
	return prefs.Allows(category), nil
}
