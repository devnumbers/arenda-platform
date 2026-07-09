package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
)

// PaymentMethodService manages the user's saved payment methods.
type PaymentMethodService struct {
	deps flowDeps
}

// NewPaymentMethodService creates a PaymentMethodService.
func NewPaymentMethodService(deps flowDeps) *PaymentMethodService {
	return &PaymentMethodService{deps: deps}
}

// AddPaymentMethod stores a new inactive payment method for the user.
// For the fake provider the method is created synchronously from the raw token.
// For T-Kassa a bank-form flow is initiated and the confirmation URL is returned.
func (s *PaymentMethodService) AddPaymentMethod(ctx context.Context, userID uuid.UUID, req AddPaymentMethodRequest) (AddPaymentMethodResponse, error) {
	if s.deps.provider.Name() == domain.ProviderFake {
		pm, err := domain.NewPaymentMethod(
			userID,
			s.deps.provider.Name(),
			req.ProviderToken,
			maskToken(req.ProviderToken),
			s.deps.clock.Now().UTC(),
		)
		if err != nil {
			return AddPaymentMethodResponse{}, fmt.Errorf("create payment method: %w", err)
		}

		tx, err := s.deps.beginner.Begin(ctx)
		if err != nil {
			return AddPaymentMethodResponse{}, fmt.Errorf("begin transaction: %w", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		pm, err = s.deps.paymentMethods.WithTx(tx).Create(ctx, pm)
		if err != nil {
			return AddPaymentMethodResponse{}, fmt.Errorf("save payment method: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return AddPaymentMethodResponse{}, fmt.Errorf("commit add payment method transaction: %w", err)
		}

		return AddPaymentMethodResponse{PaymentMethod: &pm}, nil
	}

	result, err := s.deps.provider.InitAddCard(ctx, InitAddCardRequest{
		UserID:      userID,
		CustomerKey: userID.String(),
		CheckType:   "3DSHOLD",
	})
	if err != nil {
		return AddPaymentMethodResponse{}, sanitize.Wrap(err, "init add card")
	}

	return AddPaymentMethodResponse{ConfirmURL: result.PaymentURL}, nil
}

// SetActivePaymentMethod activates the given payment method for the user and
// makes it the active method for subscription renewals.
func (s *PaymentMethodService) SetActivePaymentMethod(ctx context.Context, userID, methodID uuid.UUID) error {
	tx, err := s.deps.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := s.deps.paymentMethods.WithTx(tx).SetActive(ctx, userID, methodID); err != nil {
		return fmt.Errorf("set active payment method: %w", err)
	}

	sub, err := s.deps.subscriptions.WithTx(tx).GetByUserIDForUpdate(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrSubscriptionNotFound
		}
		return fmt.Errorf("get subscription: %w", err)
	}
	sub.ActivePaymentMethodID = &methodID
	if err := s.deps.subscriptions.WithTx(tx).Update(ctx, sub); err != nil {
		return fmt.Errorf("update subscription active payment method: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit set active payment method transaction: %w", err)
	}
	return nil
}

// DeletePaymentMethod removes a payment method belonging to the user.
// The active/in-use check and the local deletion run atomically in a single
// transaction; for T-Kassa the provider card is detached best-effort afterwards,
// so a provider failure cannot leave the local row in place.
func (s *PaymentMethodService) DeletePaymentMethod(ctx context.Context, userID, methodID uuid.UUID) error {
	// Validate existence, ownership and active status and delete atomically inside
	// a single transaction so the active check cannot race with concurrent updates.
	tx, err := s.deps.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	pm, err := s.deps.paymentMethods.WithTx(tx).GetByID(ctx, methodID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrPaymentMethodNotFound
		}
		return fmt.Errorf("get payment method: %w", err)
	}
	if pm.UserID != userID {
		return ErrPaymentMethodNotFound
	}
	if pm.IsActive {
		return ErrPaymentMethodInUse
	}

	if err := s.deps.paymentMethods.WithTx(tx).Delete(ctx, userID, methodID); err != nil {
		return fmt.Errorf("delete payment method: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete payment method transaction: %w", err)
	}

	// The local row is gone; detach the card at the provider best-effort. A
	// provider failure must not fail the operation since the method is already deleted.
	if s.deps.provider.Name() == domain.ProviderTkassa && pm.ProviderCardID != "" {
		if err := s.deps.provider.RemoveCard(ctx, userID.String(), pm.ProviderCardID); err != nil {
			if errors.Is(err, ErrProviderCardNotFound) {
				s.deps.log.WarnContext(ctx, "provider card already removed; continuing local deletion",
					slog.String("payment_method_id", methodID.String()),
					slog.String("provider_card_id", maskCardID(pm.ProviderCardID)))
			} else {
				s.deps.log.ErrorContext(ctx, "failed to remove provider card after payment method deletion",
					slog.String("payment_method_id", methodID.String()),
					slog.String("provider_card_id", maskCardID(pm.ProviderCardID)),
					slog.String("error", sanitize.Error(err)))
			}
		}
	}

	return nil
}

// ListPaymentMethods returns all payment methods for the user.
func (s *PaymentMethodService) ListPaymentMethods(ctx context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error) {
	list, err := s.deps.paymentMethods.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list payment methods: %w", err)
	}
	return list, nil
}
