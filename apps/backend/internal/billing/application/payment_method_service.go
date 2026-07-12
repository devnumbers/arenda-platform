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
	deps     paymentMethodServiceDeps
	checker  PaymentMethodInUseChecker
	provider CardProvider
}

// NewPaymentMethodService creates a PaymentMethodService.
func NewPaymentMethodService(deps paymentMethodServiceDeps, checker PaymentMethodInUseChecker, provider CardProvider) *PaymentMethodService {
	return &PaymentMethodService{deps: deps, checker: checker, provider: provider}
}

// AddPaymentMethod stores a new inactive payment method for the user.
// For the fake provider the method is created synchronously from the raw token.
// For T-Kassa a bank-form flow is initiated and the confirmation URL is returned.
func (s *PaymentMethodService) AddPaymentMethod(ctx context.Context, userID uuid.UUID, req AddPaymentMethodRequest) (AddPaymentMethodResponse, error) {
	if s.provider.Name() == domain.ProviderFake {
		pm, err := domain.NewPaymentMethod(
			userID,
			s.provider.Name(),
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

		txPaymentMethods, err := s.deps.paymentMethods.WithTx(tx)
		if err != nil {
			return AddPaymentMethodResponse{}, fmt.Errorf("bind payment methods transaction: %w", err)
		}

		pm, err = txPaymentMethods.Create(ctx, pm)
		if err != nil {
			return AddPaymentMethodResponse{}, fmt.Errorf("save payment method: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return AddPaymentMethodResponse{}, fmt.Errorf("commit add payment method transaction: %w", err)
		}

		return AddPaymentMethodResponse{PaymentMethod: &pm}, nil
	}

	result, err := s.provider.InitAddCard(ctx, InitAddCardRequest{
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

	txPaymentMethods, err := s.deps.paymentMethods.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind payment methods transaction: %w", err)
	}
	txSubscriptions, err := s.deps.subscriptions.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind subscriptions transaction: %w", err)
	}

	if err := txPaymentMethods.SetActive(ctx, userID, methodID); err != nil {
		return fmt.Errorf("set active payment method: %w", err)
	}

	sub, err := txSubscriptions.GetByUserIDForUpdate(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrSubscriptionNotFound
		}
		return fmt.Errorf("get subscription: %w", err)
	}
	sub.SetActivePaymentMethod(methodID)
	if err := txSubscriptions.Update(ctx, sub); err != nil {
		return fmt.Errorf("update subscription active payment method: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit set active payment method transaction: %w", err)
	}
	return nil
}

// DeletePaymentMethod removes a payment method belonging to the user.
// Existence, ownership, the active-method guard and the in-use check all run
// inside a single transaction so they cannot race with concurrent updates.
// For T-Kassa the provider card is detached best-effort afterwards, so a
// provider failure cannot leave the local row in place.
func (s *PaymentMethodService) DeletePaymentMethod(ctx context.Context, userID, methodID uuid.UUID) error {
	tx, err := s.deps.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txPaymentMethods, err := s.deps.paymentMethods.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind payment methods transaction: %w", err)
	}

	txChecker, err := s.checker.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind in-use checker transaction: %w", err)
	}

	pm, err := txPaymentMethods.GetByID(ctx, methodID)
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

	inUse, err := txChecker.IsInUse(ctx, methodID)
	if err != nil {
		return fmt.Errorf("check payment method in use: %w", err)
	}
	if inUse {
		return ErrPaymentMethodInUse
	}

	if err := txPaymentMethods.Delete(ctx, userID, methodID); err != nil {
		return fmt.Errorf("delete payment method: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete payment method transaction: %w", err)
	}

	// The local row is gone; detach the card at the provider best-effort. A
	// provider failure must not fail the operation since the method is already deleted.
	if s.provider.Name() == domain.ProviderTkassa && pm.ProviderCardID != "" {
		if err := s.provider.RemoveCard(ctx, userID.String(), pm.ProviderCardID); err != nil {
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

// SyncPaymentMethods imports the cards bound at the provider into local payment
// methods and returns the user's up-to-date list. It is the self-healing path
// for card binding when the AddCard webhook is not delivered (for example on
// demo terminals without a configured notification URL). Only cards the
// provider reports as active with a recurrent token are imported, and the
// upsert is keyed by token hash, so repeated syncs and webhook deliveries
// converge on the same row. When the user has no active method after the
// import, the freshest imported card is activated and linked to the
// subscription, mirroring the AddCard webhook flow. A customer that does not
// exist at the provider yet (never paid or bound a card) is treated as "no
// cards": the sync returns the local list unchanged. The method is idempotent.
func (s *PaymentMethodService) SyncPaymentMethods(ctx context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error) {
	cards, err := s.provider.GetCardList(ctx, userID.String())
	if err != nil {
		if errors.Is(err, ErrProviderCustomerNotFound) {
			return s.ListPaymentMethods(ctx, userID)
		}
		return nil, sanitize.Wrap(err, "get card list")
	}

	var importable []ProviderCard
	for _, card := range cards {
		if card.Status != ProviderCardStatusActive || card.RebillID == "" {
			continue
		}
		importable = append(importable, card)
	}

	if len(importable) == 0 {
		return s.ListPaymentMethods(ctx, userID)
	}

	tx, err := s.deps.beginner.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txPaymentMethods, err := s.deps.paymentMethods.WithTx(tx)
	if err != nil {
		return nil, fmt.Errorf("bind payment methods transaction: %w", err)
	}
	txSubscriptions, err := s.deps.subscriptions.WithTx(tx)
	if err != nil {
		return nil, fmt.Errorf("bind subscriptions transaction: %w", err)
	}

	now := s.deps.clock.Now().UTC()
	var freshest *domain.PaymentMethod
	for _, card := range importable {
		pm, err := domain.NewPaymentMethod(userID, s.provider.Name(), card.RebillID, card.Pan, now)
		if err != nil {
			return nil, fmt.Errorf("create payment method from provider card: %w", err)
		}
		pm.ProviderCardID = card.CardID
		pm.ExpDate = card.ExpDate
		pm, err = txPaymentMethods.UpsertByTokenHash(ctx, pm)
		if err != nil {
			return nil, fmt.Errorf("upsert synced payment method: %w", err)
		}
		if freshest == nil || !pm.CreatedAt.Before(freshest.CreatedAt) {
			freshest = &pm
		}
	}

	methods, err := txPaymentMethods.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list payment methods after sync: %w", err)
	}

	hasActive := false
	for _, m := range methods {
		if m.IsActive {
			hasActive = true
			break
		}
	}

	// Only bootstrap activation when nothing is active yet: an explicit user
	// choice (or a webhook-activated method) must not be overridden by a sync.
	if !hasActive && freshest != nil {
		if err := txPaymentMethods.SetActive(ctx, userID, freshest.ID); err != nil {
			return nil, fmt.Errorf("activate synced payment method: %w", err)
		}

		// Link the activated card to the subscription so renewals charge the
		// right method. A missing subscription is not fatal, same as in the
		// AddCard webhook flow.
		sub, err := txSubscriptions.GetByUserIDForUpdate(ctx, userID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				s.deps.log.WarnContext(ctx, "sync payment methods: subscription not found; skipping active method link",
					slog.String("user_id", userID.String()),
					slog.String("payment_method_id", freshest.ID.String()))
			} else {
				return nil, fmt.Errorf("get subscription for synced payment method: %w", err)
			}
		} else {
			sub.SetActivePaymentMethod(freshest.ID)
			if err := txSubscriptions.Update(ctx, sub); err != nil {
				return nil, fmt.Errorf("update subscription active payment method: %w", err)
			}
		}

		methods, err = txPaymentMethods.ListByUserID(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("list payment methods after activation: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit sync payment methods transaction: %w", err)
	}

	return methods, nil
}
