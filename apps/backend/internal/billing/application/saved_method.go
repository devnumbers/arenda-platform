package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// ensureSavedMethod stores the saved payment method a payment notification
// delivers (issue #1512). The parent CIT payment's RebillId arrives in its
// AUTHORIZED notification (ADR 0017), and the terminal notification or a
// GetState answer is the fallback carrier when that delivery was lost —
// wherever the token shows up, it must become the user's chargeable method,
// or the renewal worker finds nothing to charge with after the first payment
// and the subscription drops into grace despite the bank having kept the
// card. The storage runs through the shared completion seam of the
// card-binding flow (issue #251): upsert by token hash, activation as the
// user's single method, the subscription charge-target link. A token the
// user already stores is a silent no-op — a redelivered notification of an
// old payment must never re-activate a card a newer payment has replaced. A
// genuinely different card's notification does activate that card: the
// provider's per-payment retries converge within hours, and either way both
// cards are the user's own chargeable credentials — a dead token surfaces at
// the next charge and enters the ordinary grace path. An empty charge token
// is a provider defect worth a warning, not a failed delivery: the payment
// outcome does not depend on it. The method lives on the subscription only:
// the CIT payment row carries no method id, so the issue #426 grace guard
// keeps treating customer-initiated failures as subscription-neutral.
func (s *PaymentService) ensureSavedMethod(
	ctx context.Context, stores *txStores, userID uuid.UUID, method SavedMethod, now time.Time,
) error {
	if method.ChargeToken == "" {
		s.log.WarnContext(ctx, "payment notification carries a saved method without a charge token; ignoring",
			slog.String("user_id", userID.String()))
		return nil
	}
	stored, err := stores.methods.ListByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("list payment methods: %w", err)
	}
	for _, existing := range stored {
		if existing.ProviderToken == method.ChargeToken {
			return nil
		}
	}
	bound, err := paymentMethodFromSaved(userID, s.provider.Name(), method, now)
	if err != nil {
		return err
	}
	_, err = applyCompletedCardBinding(ctx, stores, s.log, bound, nil, now)
	return err
}

// paymentMethodFromSaved builds the domain method from a provider-neutral
// saved-method delivery: the charge token plus the display fields, shared by
// every delivery channel that produces one (the binding webhook, the binding
// sync, the payment notification of issue #1512).
func paymentMethodFromSaved(
	userID uuid.UUID, provider domain.PaymentProvider, method SavedMethod, now time.Time,
) (domain.PaymentMethod, error) {
	bound, err := domain.NewPaymentMethod(userID, provider, method.ChargeToken, now)
	if err != nil {
		return domain.PaymentMethod{}, err
	}
	bound.ProviderCardID = method.ProviderMethodID
	bound.DisplayMask = method.MaskedPan
	bound.ExpDate = method.ExpDate
	return bound, nil
}
