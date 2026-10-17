package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// ensurePaidSections consults the tariff gate for the actor — the shared tail
// of the use-case-level gate (ADR 0064). A nil gate disables the check, the
// package convention for optional dependencies (mirrors slots, events,
// statuses); the composition root always wires the billing adapter. A gate
// storage error propagates for the HTTP layer's 500 — the gate never fails
// open.
func ensurePaidSections(ctx context.Context, gate PaidSectionsGate, actor uuid.UUID) error {
	if gate == nil {
		return nil
	}
	allowed, err := gate.CanUsePaidFeatures(ctx, actor)
	if err != nil {
		return fmt.Errorf("paid sections gate: %w", err)
	}
	if !allowed {
		return ErrTariffRequired
	}
	return nil
}
