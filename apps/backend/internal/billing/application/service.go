package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// BillingService orchestrates subscription, tariff and payment operations.
type BillingService struct {
	tariffs              TariffRepository
	subscriptions        SubscriptionRepository
	paymentMethods       PaymentMethodRepository
	subscriptionPayments SubscriptionPaymentRepository
	propertyArchiver     PropertyArchiver
	provider             Provider
	beginner             transaction.Beginner
	clock                clock.Clock
	log                  *slog.Logger
}

// NewBillingService creates a new billing application service.
func NewBillingService(
	tariffs TariffRepository,
	subscriptions SubscriptionRepository,
	paymentMethods PaymentMethodRepository,
	subscriptionPayments SubscriptionPaymentRepository,
	provider Provider,
	beginner transaction.Beginner,
	clock clock.Clock,
	log *slog.Logger,
	propertyArchiver PropertyArchiver,
) *BillingService {
	if log == nil {
		log = slog.Default()
	}
	return &BillingService{
		tariffs:              tariffs,
		subscriptions:        subscriptions,
		paymentMethods:       paymentMethods,
		subscriptionPayments: subscriptionPayments,
		propertyArchiver:     propertyArchiver,
		provider:             provider,
		beginner:             beginner,
		clock:                clock,
		log:                  log,
	}
}

// ListTariffs returns all tariffs ordered by price.
func (s *BillingService) ListTariffs(ctx context.Context) ([]domain.Tariff, error) {
	list, err := s.tariffs.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tariffs: %w", err)
	}

	// Ensure a stable, ascending price order even if the repository does not.
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].MonthlyPriceKopecks != list[j].MonthlyPriceKopecks {
			return list[i].MonthlyPriceKopecks < list[j].MonthlyPriceKopecks
		}
		if list[i].YearlyPriceKopecks != list[j].YearlyPriceKopecks {
			return list[i].YearlyPriceKopecks < list[j].YearlyPriceKopecks
		}
		return list[i].ActivePropertyLimit < list[j].ActivePropertyLimit
	})

	return list, nil
}

// GetSubscription returns the current subscription with its tariff and active payment method.
func (s *BillingService) GetSubscription(ctx context.Context, userID uuid.UUID) (SubscriptionView, error) {
	sub, err := s.subscriptions.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return SubscriptionView{}, ErrSubscriptionNotFound
		}
		return SubscriptionView{}, fmt.Errorf("get subscription: %w", err)
	}

	tariff, err := s.tariffs.GetByID(ctx, sub.TariffID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return SubscriptionView{}, ErrTariffNotFound
		}
		return SubscriptionView{}, fmt.Errorf("get subscription tariff: %w", err)
	}

	view := SubscriptionView{Subscription: sub, Tariff: tariff}
	if sub.PendingTariffID != nil {
		pending, err := s.tariffs.GetByID(ctx, *sub.PendingTariffID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return SubscriptionView{}, ErrTariffNotFound
			}
			return SubscriptionView{}, fmt.Errorf("get pending tariff: %w", err)
		}
		view.PendingTariff = &pending
	}

	if sub.ActivePaymentMethodID != nil {
		pm, err := s.paymentMethods.GetByID(ctx, *sub.ActivePaymentMethodID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				s.log.WarnContext(ctx, "subscription references missing active payment method",
					"user_id", userID.String(),
					"payment_method_id", sub.ActivePaymentMethodID.String())
				return view, nil
			}
			return SubscriptionView{}, fmt.Errorf("get active payment method: %w", err)
		}
		view.ActivePaymentMethod = &pm
	}

	return view, nil
}

func (s *BillingService) existingUpgradeResponse(ctx context.Context, paymentID uuid.UUID) ChangeTariffResponse {
	res := ChangeTariffResponse{PaymentID: paymentID}
	if urlProvider, ok := s.provider.(PaymentURLProvider); ok {
		if url, err := urlProvider.PaymentURL(ctx, paymentID); err == nil {
			res.ConfirmURL = url
		}
	}
	return res
}

// ChangeTariff starts an upgrade payment or schedules a downgrade.
func (s *BillingService) ChangeTariff(ctx context.Context, userID uuid.UUID, req ChangeTariffRequest) (ChangeTariffResponse, error) {
	if req.Period != domain.PeriodMonth && req.Period != domain.PeriodYear {
		return ChangeTariffResponse{}, domain.ErrInvalidPeriod
	}

	newTariff, err := s.tariffs.GetByName(ctx, domain.TariffName(req.TariffName))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ChangeTariffResponse{}, ErrTariffNotFound
		}
		return ChangeTariffResponse{}, fmt.Errorf("get tariff: %w", err)
	}

	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		return ChangeTariffResponse{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	sub, err := s.subscriptions.WithTx(tx).GetByUserIDForUpdate(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ChangeTariffResponse{}, ErrSubscriptionNotFound
		}
		return ChangeTariffResponse{}, fmt.Errorf("get subscription: %w", err)
	}

	if sub.Source != domain.SubscriptionSourcePaid {
		return ChangeTariffResponse{}, domain.ErrInvalidSubscriptionState
	}

	if sub.TariffID == newTariff.ID {
		return ChangeTariffResponse{}, ErrAlreadyOnTariff
	}

	currentTariff, err := s.tariffs.WithTx(tx).GetByID(ctx, sub.TariffID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ChangeTariffResponse{}, ErrTariffNotFound
		}
		return ChangeTariffResponse{}, fmt.Errorf("get current tariff: %w", err)
	}

	changeType := domain.ClassifyTariffChange(currentTariff, newTariff)
	if changeType == domain.TariffChangeSame {
		return ChangeTariffResponse{}, ErrInvalidTariffChange
	}

	amount := newTariff.MonthlyPriceKopecks
	if req.Period == domain.PeriodYear {
		amount = newTariff.YearlyPriceKopecks
	}

	now := s.clock.Now().UTC()
	canInitiate := sub.CanInitiatePayment(now)
	isRecoveryUpgrade := changeType == domain.TariffChangeUpgrade &&
		(sub.Status == domain.SubscriptionStatusBlocked || sub.Status == domain.SubscriptionStatusCancelled)
	if !canInitiate && !isRecoveryUpgrade {
		return ChangeTariffResponse{}, domain.ErrInvalidSubscriptionState
	}

	if changeType == domain.TariffChangeUpgrade {
		// Return an existing pending upgrade payment for the same tariff and
		// period instead of creating a duplicate. The partial unique index on
		// pending payments is the durable backstop for races.
		pending, err := s.subscriptionPayments.WithTx(tx).ListPendingSubscriptionPaymentsByUserID(ctx, userID)
		if err != nil {
			return ChangeTariffResponse{}, fmt.Errorf("list pending subscription payments: %w", err)
		}
		for _, p := range pending {
			if p.TariffID == newTariff.ID && p.Period == req.Period {
				return s.existingUpgradeResponse(ctx, p.ID), nil
			}
		}
		return s.changeTariffUpgrade(ctx, tx, userID, sub, newTariff, req.Period, amount)
	}

	if sub.ValidUntil == nil {
		return ChangeTariffResponse{}, ErrInvalidTariffChange
	}
	if err := sub.ScheduleDowngrade(currentTariff, newTariff, req.Period, *sub.ValidUntil); err != nil {
		return ChangeTariffResponse{}, err
	}
	if err := s.subscriptions.WithTx(tx).Update(ctx, sub); err != nil {
		return ChangeTariffResponse{}, fmt.Errorf("schedule downgrade: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return ChangeTariffResponse{}, fmt.Errorf("commit schedule downgrade transaction: %w", err)
	}

	return ChangeTariffResponse{}, nil
}

func (s *BillingService) changeTariffUpgrade(
	ctx context.Context,
	tx transaction.Tx,
	userID uuid.UUID,
	sub domain.Subscription,
	newTariff domain.Tariff,
	period domain.SubscriptionPeriod,
	amount int64,
) (ChangeTariffResponse, error) {
	if amount <= 0 {
		return ChangeTariffResponse{}, domain.ErrInvalidAmount
	}

	now := s.clock.Now().UTC()
	payment, err := domain.NewSubscriptionPayment(
		userID,
		sub.ID,
		newTariff.ID,
		nil,
		period,
		amount,
		s.provider.Name(),
		now,
	)
	if err != nil {
		return ChangeTariffResponse{}, fmt.Errorf("create subscription payment: %w", err)
	}

	// Persist the pending payment before calling the external provider so the
	// record survives a crash and the provider has an internal payment id to
	// reference.
	payment, err = s.subscriptionPayments.WithTx(tx).Create(ctx, payment)
	if err != nil {
		return ChangeTariffResponse{}, fmt.Errorf("save subscription payment: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return ChangeTariffResponse{}, fmt.Errorf("commit transaction: %w", err)
	}

	initRes, err := s.provider.Init(ctx, InitRequest{
		PaymentID:     payment.ID,
		AmountKopecks: amount,
		Period:        period,
		UserID:        userID,
		Description:   fmt.Sprintf("Upgrade to %s (%s)", newTariff.Name, period),
	})
	if err != nil {
		markTx, beginErr := s.beginner.Begin(ctx)
		if beginErr == nil {
			if markErr := s.subscriptionPayments.WithTx(markTx).MarkFailed(ctx, payment.ID, nil, s.clock.Now().UTC()); markErr != nil {
				s.log.ErrorContext(ctx, "failed to mark payment failed after init error",
					"payment_id", payment.ID,
					"error", markErr)
			}
			if commitErr := markTx.Commit(ctx); commitErr != nil {
				s.log.ErrorContext(ctx, "failed to commit payment failed mark after init error",
					"payment_id", payment.ID,
					"error", commitErr)
			}
		} else {
			s.log.ErrorContext(ctx, "failed to begin transaction for marking payment failed after init error",
				"payment_id", payment.ID,
				"error", beginErr)
		}
		return ChangeTariffResponse{}, fmt.Errorf("init payment: %w", err)
	}

	// The provider token is only available after Init succeeds. Create an
	// inactive payment method and attach it to the pending payment; activation
	// is deferred until the payment is confirmed.
	if initRes.SavedToken == "" {
		providerTx, err := s.beginner.Begin(ctx)
		if err != nil {
			return ChangeTariffResponse{}, fmt.Errorf("begin transaction: %w", err)
		}
		defer func() { _ = providerTx.Rollback(ctx) }()

		if _, err := s.subscriptionPayments.WithTx(providerTx).UpdateProviderPaymentID(ctx, payment.ID, initRes.ProviderPaymentID); err != nil {
			return ChangeTariffResponse{}, fmt.Errorf("update provider payment id: %w", err)
		}
		if err := providerTx.Commit(ctx); err != nil {
			return ChangeTariffResponse{}, fmt.Errorf("commit provider payment id update: %w", err)
		}

		return ChangeTariffResponse{
			PaymentID:  payment.ID,
			ConfirmURL: initRes.PaymentURL,
		}, nil
	}

	pm, err := domain.NewPaymentMethod(
		userID,
		s.provider.Name(),
		initRes.SavedToken,
		maskToken(initRes.SavedToken),
		now,
	)
	if err != nil {
		return ChangeTariffResponse{}, fmt.Errorf("create payment method: %w", err)
	}

	pmTx, err := s.beginner.Begin(ctx)
	if err != nil {
		return ChangeTariffResponse{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = pmTx.Rollback(ctx) }()

	pm, err = s.paymentMethods.WithTx(pmTx).Create(ctx, pm)
	if err != nil {
		return ChangeTariffResponse{}, fmt.Errorf("save payment method: %w", err)
	}

	payment, err = s.subscriptionPayments.WithTx(pmTx).UpdatePaymentMethodAndProviderID(ctx, payment.ID, pm.ID, initRes.ProviderPaymentID)
	if err != nil {
		return ChangeTariffResponse{}, fmt.Errorf("update payment method and provider payment id: %w", err)
	}

	if err := pmTx.Commit(ctx); err != nil {
		return ChangeTariffResponse{}, fmt.Errorf("commit transaction: %w", err)
	}

	return ChangeTariffResponse{
		PaymentID:  payment.ID,
		ConfirmURL: initRes.PaymentURL,
	}, nil
}

// CancelSubscription terminates the paid subscription. The current tariff remains
// valid until valid_until, after which the worker downgrades the subscription to
// basic.
func (s *BillingService) CancelSubscription(ctx context.Context, userID uuid.UUID) error {
	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	sub, err := s.subscriptions.WithTx(tx).GetByUserIDForUpdate(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrSubscriptionNotFound
		}
		return fmt.Errorf("get subscription: %w", err)
	}

	if sub.Source != domain.SubscriptionSourcePaid {
		return domain.ErrInvalidSubscriptionState
	}

	if err := sub.Cancel(); err != nil {
		return err
	}

	if err := s.subscriptions.WithTx(tx).Update(ctx, sub); err != nil {
		return fmt.Errorf("update subscription: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit cancel subscription transaction: %w", err)
	}
	return nil
}

// ToggleAutoRenew enables or disables automatic subscription renewal.
func (s *BillingService) ToggleAutoRenew(ctx context.Context, userID uuid.UUID, enabled bool) error {
	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	sub, err := s.subscriptions.WithTx(tx).GetByUserIDForUpdate(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrSubscriptionNotFound
		}
		return fmt.Errorf("get subscription: %w", err)
	}

	if err := sub.SetAutoRenew(enabled); err != nil {
		return err
	}

	if err := s.subscriptions.WithTx(tx).Update(ctx, sub); err != nil {
		return fmt.Errorf("update subscription: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit toggle auto-renew transaction: %w", err)
	}
	return nil
}

// AddPaymentMethod stores a new inactive payment method for the user.
func (s *BillingService) AddPaymentMethod(ctx context.Context, userID uuid.UUID, req AddPaymentMethodRequest) (domain.PaymentMethod, error) {
	pm, err := domain.NewPaymentMethod(
		userID,
		s.provider.Name(),
		req.ProviderToken,
		maskToken(req.ProviderToken),
		s.clock.Now().UTC(),
	)
	if err != nil {
		return domain.PaymentMethod{}, fmt.Errorf("create payment method: %w", err)
	}

	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		return domain.PaymentMethod{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	pm, err = s.paymentMethods.WithTx(tx).Create(ctx, pm)
	if err != nil {
		return domain.PaymentMethod{}, fmt.Errorf("save payment method: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.PaymentMethod{}, fmt.Errorf("commit add payment method transaction: %w", err)
	}

	return pm, nil
}

// SetActivePaymentMethod activates the given payment method for the user and
// makes it the active method for subscription renewals.
func (s *BillingService) SetActivePaymentMethod(ctx context.Context, userID, methodID uuid.UUID) error {
	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := s.paymentMethods.WithTx(tx).SetActive(ctx, userID, methodID); err != nil {
		return fmt.Errorf("set active payment method: %w", err)
	}

	sub, err := s.subscriptions.WithTx(tx).GetByUserIDForUpdate(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrSubscriptionNotFound
		}
		return fmt.Errorf("get subscription: %w", err)
	}
	sub.ActivePaymentMethodID = &methodID
	if err := s.subscriptions.WithTx(tx).Update(ctx, sub); err != nil {
		return fmt.Errorf("update subscription active payment method: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit set active payment method transaction: %w", err)
	}
	return nil
}

// DeletePaymentMethod removes a payment method belonging to the user.
func (s *BillingService) DeletePaymentMethod(ctx context.Context, userID, methodID uuid.UUID) error {
	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := s.paymentMethods.WithTx(tx).Delete(ctx, userID, methodID); err != nil {
		return fmt.Errorf("delete payment method: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete payment method transaction: %w", err)
	}
	return nil
}

// ListPaymentMethods returns all payment methods for the user.
func (s *BillingService) ListPaymentMethods(ctx context.Context, userID uuid.UUID) ([]domain.PaymentMethod, error) {
	list, err := s.paymentMethods.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list payment methods: %w", err)
	}
	return list, nil
}

// ListPayments returns all subscription payments for the user with their tariffs.
func (s *BillingService) ListPayments(ctx context.Context, userID uuid.UUID) ([]SubscriptionPaymentView, error) {
	payments, err := s.subscriptionPayments.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list payments: %w", err)
	}

	tariffs, err := s.tariffs.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tariffs: %w", err)
	}
	tariffByID := make(map[uuid.UUID]domain.Tariff, len(tariffs))
	for _, t := range tariffs {
		tariffByID[t.ID] = t
	}

	views := make([]SubscriptionPaymentView, 0, len(payments))
	for _, p := range payments {
		t, ok := tariffByID[p.TariffID]
		if !ok {
			return nil, fmt.Errorf("payment %s references unknown tariff %s", p.ID, p.TariffID)
		}
		views = append(views, SubscriptionPaymentView{Payment: p, Tariff: t})
	}
	return views, nil
}

// ConfirmFakePayment confirms a previously initialized fake payment and applies its result.
func (s *BillingService) ConfirmFakePayment(ctx context.Context, paymentID uuid.UUID) error {
	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	payment, err := s.subscriptionPayments.WithTx(tx).GetByIDForUpdate(ctx, paymentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrPaymentNotFound
		}
		return fmt.Errorf("get payment: %w", err)
	}

	if payment.Status == domain.PaymentStatusSucceeded || payment.Status == domain.PaymentStatusFailed {
		return nil
	}

	cp, ok := s.provider.(ConfirmableProvider)
	if !ok {
		return ErrProviderNotConfirmable
	}

	payload, err := cp.ConfirmPayment(ctx, payment.ID.String())
	if err != nil {
		return fmt.Errorf("confirm fake payment: %w", err)
	}

	if err := s.applyPaymentResult(ctx, tx, payment, payload); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// HandleWebhook parses and applies a provider webhook payload.
func (s *BillingService) HandleWebhook(ctx context.Context, providerName string, payload []byte) error {
	if providerName != string(s.provider.Name()) {
		return fmt.Errorf("unexpected provider %q, expected %q", providerName, s.provider.Name())
	}

	result, err := s.provider.ParseWebhook(ctx, payload)
	if err != nil {
		return fmt.Errorf("parse webhook: %w", err)
	}

	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	payment, err := s.subscriptionPayments.WithTx(tx).GetByIDForUpdate(ctx, result.InternalPaymentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrPaymentNotFound
		}
		return fmt.Errorf("get payment: %w", err)
	}

	if payment.Status == domain.PaymentStatusSucceeded || payment.Status == domain.PaymentStatusFailed {
		return nil
	}

	if err := s.applyPaymentResult(ctx, tx, payment, result); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *BillingService) applyPaymentResult(
	ctx context.Context,
	tx transaction.Tx,
	payment domain.SubscriptionPayment,
	payload WebhookPayload,
) error {
	if payment.ProviderPaymentID != nil && *payment.ProviderPaymentID != payload.ProviderPaymentID {
		return fmt.Errorf("provider payment id mismatch: expected %q, got %q", *payment.ProviderPaymentID, payload.ProviderPaymentID)
	}

	now := s.clock.Now().UTC()
	switch payload.Status {
	case domain.PaymentStatusSucceeded:
		if err := s.subscriptionPayments.WithTx(tx).MarkSucceeded(ctx, payment.ID, now); err != nil {
			return fmt.Errorf("mark payment succeeded: %w", err)
		}

		sub, err := s.subscriptions.WithTx(tx).GetByIDForUpdate(ctx, payment.SubscriptionID)
		if err != nil {
			return fmt.Errorf("get subscription: %w", err)
		}

		if payment.PaymentMethodID != nil {
			if err := s.paymentMethods.WithTx(tx).SetActive(ctx, payment.UserID, *payment.PaymentMethodID); err != nil {
				return fmt.Errorf("activate payment method: %w", err)
			}
			sub.ActivePaymentMethodID = payment.PaymentMethodID
		}

		if sub.TariffID == payment.TariffID {
			// Renewal for the current tariff: extend validity from the current
			// period end (or now) by the paid period.
			if err := sub.ApplyRenewal(payment.Period, now); err != nil {
				return fmt.Errorf("apply renewal: %w", err)
			}
		} else {
			// Upgrade or scheduled change to a different tariff.
			currentTariff, err := s.tariffs.WithTx(tx).GetByID(ctx, sub.TariffID)
			if err != nil {
				return fmt.Errorf("get current tariff: %w", err)
			}
			newTariff, err := s.tariffs.WithTx(tx).GetByID(ctx, payment.TariffID)
			if err != nil {
				return fmt.Errorf("get payment tariff: %w", err)
			}
			if err := sub.ApplyTariffChange(currentTariff, newTariff, payment.Period, now); err != nil {
				return fmt.Errorf("apply tariff change: %w", err)
			}
		}

		if err := s.subscriptions.WithTx(tx).Update(ctx, sub); err != nil {
			return fmt.Errorf("update subscription: %w", err)
		}

	case domain.PaymentStatusFailed:
		if err := s.subscriptionPayments.WithTx(tx).MarkFailed(ctx, payment.ID, payload.ErrorCode, now); err != nil {
			return fmt.Errorf("mark payment failed: %w", err)
		}

	default:
		return fmt.Errorf("unsupported webhook status: %s", payload.Status)
	}

	return nil
}

func maskToken(token string) string {
	if len(token) <= 4 {
		return "****"
	}
	return "****" + token[len(token)-4:]
}

const (
	renewalBatchSize = 100
	graceBatchSize   = 100
	gracePeriod      = 7 * 24 * time.Hour
)

// ProcessRenewals processes all subscriptions whose validity period has ended.
// For subscriptions with auto-renew enabled it attempts to charge and renew them;
// failed charges move the subscription to a grace period. For subscriptions with
// auto-renew disabled it downgrades them to the free basic tariff. Returns the
// total number of subscriptions processed.
func (s *BillingService) ProcessRenewals(ctx context.Context, now time.Time) (int, error) {
	processed := 0

	for {
		subs, err := s.subscriptions.ListUpForRenewal(ctx, now.UTC(), renewalBatchSize)
		if err != nil {
			return processed, fmt.Errorf("list subscriptions up for renewal: %w", err)
		}
		if len(subs) == 0 {
			break
		}
		for _, sub := range subs {
			if err := s.renewSubscription(ctx, sub, now.UTC()); err != nil {
				s.log.ErrorContext(ctx, "renew subscription failed",
					slog.String("subscription_id", sub.ID.String()),
					slog.String("user_id", sub.UserID.String()),
					slog.String("error", err.Error()))
				continue
			}
			processed++
		}
		if len(subs) < renewalBatchSize {
			break
		}
	}

	basicTariff, err := s.tariffs.GetByName(ctx, domain.TariffBasic)
	if err != nil {
		return processed, fmt.Errorf("get basic tariff for non-renewing cleanup: %w", err)
	}

	for {
		subs, err := s.subscriptions.ListExpiredNonRenewing(ctx, now.UTC(), renewalBatchSize)
		if err != nil {
			return processed, fmt.Errorf("list expired non-renewing subscriptions: %w", err)
		}
		if len(subs) == 0 {
			break
		}
		for _, sub := range subs {
			if err := s.expireNonRenewingSubscription(ctx, sub, basicTariff, now.UTC()); err != nil {
				s.log.ErrorContext(ctx, "expire non-renewing subscription failed",
					slog.String("subscription_id", sub.ID.String()),
					slog.String("user_id", sub.UserID.String()),
					slog.String("error", err.Error()))
				continue
			}
			processed++
		}
		if len(subs) < renewalBatchSize {
			break
		}
	}

	for {
		subs, err := s.subscriptions.ListExpiredCancelled(ctx, now.UTC(), renewalBatchSize)
		if err != nil {
			return processed, fmt.Errorf("list expired cancelled subscriptions: %w", err)
		}
		if len(subs) == 0 {
			break
		}
		for _, sub := range subs {
			if err := s.expireNonRenewingSubscription(ctx, sub, basicTariff, now.UTC()); err != nil {
				s.log.ErrorContext(ctx, "expire cancelled subscription failed",
					slog.String("subscription_id", sub.ID.String()),
					slog.String("user_id", sub.UserID.String()),
					slog.String("error", err.Error()))
				continue
			}
			processed++
		}
		if len(subs) < renewalBatchSize {
			break
		}
	}

	return processed, nil
}

func (s *BillingService) renewSubscription(ctx context.Context, sub domain.Subscription, now time.Time) error {
	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	sub, err = s.subscriptions.WithTx(tx).GetByIDForUpdate(ctx, sub.ID)
	if err != nil {
		return fmt.Errorf("get subscription for update: %w", err)
	}

	if sub.Status != domain.SubscriptionStatusActive || !sub.AutoRenewEnabled || sub.ValidUntil == nil || sub.ValidUntil.After(now) {
		return nil
	}

	renewalTariff, period, amount, err := s.resolveRenewalTariffAndAmount(ctx, tx, sub)
	if err != nil {
		return err
	}

	// Free tariff changes (e.g. downgrade to basic) do not require a charge.
	if amount <= 0 {
		if err := s.applyFreeRenewalOrDowngrade(ctx, tx, &sub, renewalTariff, period, now); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	if sub.ActivePaymentMethodID == nil {
		s.transitionToGrace(&sub, now)
		if err := s.subscriptions.WithTx(tx).Update(ctx, sub); err != nil {
			return fmt.Errorf("transition to grace: %w", err)
		}
		return tx.Commit(ctx)
	}

	pm, err := s.paymentMethods.WithTx(tx).GetByID(ctx, *sub.ActivePaymentMethodID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			s.transitionToGrace(&sub, now)
			if err := s.subscriptions.WithTx(tx).Update(ctx, sub); err != nil {
				return fmt.Errorf("transition to grace: %w", err)
			}
			return tx.Commit(ctx)
		}
		return fmt.Errorf("get active payment method: %w", err)
	}

	payment, err := domain.NewSubscriptionPayment(
		sub.UserID,
		sub.ID,
		renewalTariff.ID,
		&pm.ID,
		period,
		amount,
		s.provider.Name(),
		now,
	)
	if err != nil {
		return fmt.Errorf("create renewal payment: %w", err)
	}

	payment, err = s.subscriptionPayments.WithTx(tx).Create(ctx, payment)
	if err != nil {
		return fmt.Errorf("save renewal payment: %w", err)
	}

	chargeResult, err := s.provider.Charge(ctx, ChargeRequest{
		PaymentID:     payment.ID,
		AmountKopecks: amount,
		Token:         pm.ProviderToken,
	})
	if err != nil {
		// Provider errors (network, timeout) are treated like a failed charge so
		// the subscription enters grace instead of staying active indefinitely.
		if markErr := s.subscriptionPayments.WithTx(tx).MarkFailed(ctx, payment.ID, nil, now); markErr != nil {
			return fmt.Errorf("mark renewal payment failed after provider error: %w", markErr)
		}
		s.transitionToGrace(&sub, now)
		if updateErr := s.subscriptions.WithTx(tx).Update(ctx, sub); updateErr != nil {
			return fmt.Errorf("transition to grace after provider error: %w", updateErr)
		}
		return tx.Commit(ctx)
	}

	if chargeResult.ProviderPaymentID != "" {
		if _, updateErr := s.subscriptionPayments.WithTx(tx).UpdateProviderPaymentID(ctx, payment.ID, chargeResult.ProviderPaymentID); updateErr != nil {
			return fmt.Errorf("update renewal provider payment id: %w", updateErr)
		}
	}

	switch chargeResult.Status {
	case domain.PaymentStatusSucceeded:
		if err := s.subscriptionPayments.WithTx(tx).MarkSucceeded(ctx, payment.ID, now); err != nil {
			return fmt.Errorf("mark renewal payment succeeded: %w", err)
		}
		if err := s.applySuccessfulRenewal(ctx, tx, sub, renewalTariff, period, now); err != nil {
			return err
		}
	case domain.PaymentStatusFailed:
		if err := s.subscriptionPayments.WithTx(tx).MarkFailed(ctx, payment.ID, nil, now); err != nil {
			return fmt.Errorf("mark renewal payment failed: %w", err)
		}
		s.transitionToGrace(&sub, now)
		if err := s.subscriptions.WithTx(tx).Update(ctx, sub); err != nil {
			return fmt.Errorf("transition to grace after failed renewal: %w", err)
		}
	case domain.PaymentStatusPending:
		// The provider will finalize the charge asynchronously via a webhook.
		// The pending payment is already persisted; leave the subscription active
		// and wait for the webhook.
		return tx.Commit(ctx)
	default:
		return fmt.Errorf("unexpected charge status: %s", chargeResult.Status)
	}

	return tx.Commit(ctx)
}

func (s *BillingService) resolveRenewalTariffAndAmount(ctx context.Context, tx transaction.Tx, sub domain.Subscription) (domain.Tariff, domain.SubscriptionPeriod, int64, error) {
	if sub.PendingTariffID != nil && sub.PendingPeriod != nil {
		pendingTariff, err := s.tariffs.WithTx(tx).GetByID(ctx, *sub.PendingTariffID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return domain.Tariff{}, "", 0, ErrTariffNotFound
			}
			return domain.Tariff{}, "", 0, fmt.Errorf("get pending tariff: %w", err)
		}
		amount := pendingTariff.MonthlyPriceKopecks
		if *sub.PendingPeriod == domain.PeriodYear {
			amount = pendingTariff.YearlyPriceKopecks
		}
		return pendingTariff, *sub.PendingPeriod, amount, nil
	}

	currentTariff, err := s.tariffs.WithTx(tx).GetByID(ctx, sub.TariffID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Tariff{}, "", 0, ErrTariffNotFound
		}
		return domain.Tariff{}, "", 0, fmt.Errorf("get current tariff: %w", err)
	}

	period := domain.PeriodMonth
	lastPayment, err := s.subscriptionPayments.WithTx(tx).GetLastSucceededBySubscriptionID(ctx, sub.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return domain.Tariff{}, "", 0, fmt.Errorf("get last succeeded payment: %w", err)
	}
	if err == nil {
		period = lastPayment.Period
	}

	amount := currentTariff.MonthlyPriceKopecks
	if period == domain.PeriodYear {
		amount = currentTariff.YearlyPriceKopecks
	}
	return currentTariff, period, amount, nil
}

func (s *BillingService) applySuccessfulRenewal(ctx context.Context, tx transaction.Tx, sub domain.Subscription, renewalTariff domain.Tariff, period domain.SubscriptionPeriod, now time.Time) error {
	if sub.TariffID == renewalTariff.ID {
		if err := sub.ApplyRenewal(period, now); err != nil {
			return fmt.Errorf("apply renewal: %w", err)
		}
	} else {
		currentTariff, err := s.tariffs.WithTx(tx).GetByID(ctx, sub.TariffID)
		if err != nil {
			return fmt.Errorf("get current tariff for change: %w", err)
		}
		if err := sub.ApplyTariffChange(currentTariff, renewalTariff, period, now); err != nil {
			return fmt.Errorf("apply tariff change: %w", err)
		}
		if s.propertyArchiver != nil {
			if err := s.propertyArchiver.ArchiveExcessProperties(ctx, sub.UserID, renewalTariff.ActivePropertyLimit); err != nil {
				return fmt.Errorf("archive excess properties after downgrade: %w", err)
			}
		}
	}

	if err := s.subscriptions.WithTx(tx).Update(ctx, sub); err != nil {
		return fmt.Errorf("update subscription after renewal: %w", err)
	}
	return nil
}

func (s *BillingService) applyFreeRenewalOrDowngrade(ctx context.Context, tx transaction.Tx, sub *domain.Subscription, renewalTariff domain.Tariff, period domain.SubscriptionPeriod, now time.Time) error {
	// The free basic tariff has no validity period and cannot be auto-renewed.
	if renewalTariff.Name == domain.TariffBasic {
		applyBasicDowngrade(sub, renewalTariff.ID)
		if err := s.subscriptions.WithTx(tx).Update(ctx, *sub); err != nil {
			return fmt.Errorf("update subscription after free downgrade to basic: %w", err)
		}
		if s.propertyArchiver != nil {
			if err := s.propertyArchiver.ArchiveExcessProperties(ctx, sub.UserID, renewalTariff.ActivePropertyLimit); err != nil {
				return fmt.Errorf("archive excess properties after free downgrade to basic: %w", err)
			}
		}
		return nil
	}

	if sub.TariffID == renewalTariff.ID {
		if err := sub.ApplyRenewal(period, now); err != nil {
			return fmt.Errorf("apply free renewal: %w", err)
		}
	} else {
		currentTariff, err := s.tariffs.WithTx(tx).GetByID(ctx, sub.TariffID)
		if err != nil {
			return fmt.Errorf("get current tariff for free change: %w", err)
		}
		if err := sub.ApplyTariffChange(currentTariff, renewalTariff, period, now); err != nil {
			return fmt.Errorf("apply free tariff change: %w", err)
		}
		if s.propertyArchiver != nil {
			if err := s.propertyArchiver.ArchiveExcessProperties(ctx, sub.UserID, renewalTariff.ActivePropertyLimit); err != nil {
				return fmt.Errorf("archive excess properties after free downgrade: %w", err)
			}
		}
	}
	if err := s.subscriptions.WithTx(tx).Update(ctx, *sub); err != nil {
		return fmt.Errorf("update subscription after free renewal: %w", err)
	}
	return nil
}

func applyBasicDowngrade(sub *domain.Subscription, basicTariffID uuid.UUID) {
	sub.TariffID = basicTariffID
	sub.Status = domain.SubscriptionStatusActive
	sub.ValidUntil = nil
	sub.AutoRenewEnabled = false
	sub.PendingTariffID = nil
	sub.PendingChangeAt = nil
	sub.PendingPeriod = nil
}

func (s *BillingService) transitionToGrace(sub *domain.Subscription, now time.Time) {
	sub.Status = domain.SubscriptionStatusGrace
	graceUntil := now.Add(gracePeriod)
	sub.ValidUntil = &graceUntil
}

func (s *BillingService) expireNonRenewingSubscription(ctx context.Context, sub domain.Subscription, basicTariff domain.Tariff, now time.Time) error {
	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	sub, err = s.subscriptions.WithTx(tx).GetByIDForUpdate(ctx, sub.ID)
	if err != nil {
		return fmt.Errorf("get subscription for update: %w", err)
	}

	if sub.Status != domain.SubscriptionStatusActive || sub.AutoRenewEnabled || sub.ValidUntil == nil || sub.ValidUntil.After(now) {
		return nil
	}

	applyBasicDowngrade(&sub, basicTariff.ID)
	if err := s.subscriptions.WithTx(tx).Update(ctx, sub); err != nil {
		return fmt.Errorf("update subscription after non-renewing expiry: %w", err)
	}

	if s.propertyArchiver != nil {
		if err := s.propertyArchiver.ArchiveExcessProperties(ctx, sub.UserID, basicTariff.ActivePropertyLimit); err != nil {
			return fmt.Errorf("archive excess properties after non-renewing expiry: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// ProcessScheduledChanges applies scheduled tariff changes (usually downgrades to
// the free basic tariff) whose pending_change_at has been reached. Returns the
// number of subscriptions processed.
func (s *BillingService) ProcessScheduledChanges(ctx context.Context, now time.Time) (int, error) {
	processed := 0
	for {
		subs, err := s.subscriptions.ListPendingChanges(ctx, now.UTC(), renewalBatchSize)
		if err != nil {
			return processed, fmt.Errorf("list subscriptions with pending change: %w", err)
		}
		if len(subs) == 0 {
			break
		}
		for _, sub := range subs {
			if err := s.applyScheduledChange(ctx, sub, now.UTC()); err != nil {
				s.log.ErrorContext(ctx, "apply scheduled change failed",
					slog.String("subscription_id", sub.ID.String()),
					slog.String("user_id", sub.UserID.String()),
					slog.String("error", err.Error()))
				continue
			}
			processed++
		}
		if len(subs) < renewalBatchSize {
			break
		}
	}
	return processed, nil
}

func (s *BillingService) applyScheduledChange(ctx context.Context, sub domain.Subscription, now time.Time) error {
	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	sub, err = s.subscriptions.WithTx(tx).GetByIDForUpdate(ctx, sub.ID)
	if err != nil {
		return fmt.Errorf("get subscription for update: %w", err)
	}

	if sub.Status != domain.SubscriptionStatusActive || sub.PendingTariffID == nil || sub.PendingChangeAt == nil || sub.PendingChangeAt.After(now) {
		return nil
	}

	pendingTariff, err := s.tariffs.WithTx(tx).GetByID(ctx, *sub.PendingTariffID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrTariffNotFound
		}
		return fmt.Errorf("get pending tariff: %w", err)
	}

	period := domain.PeriodMonth
	if sub.PendingPeriod != nil {
		period = *sub.PendingPeriod
	}

	amount := pendingTariff.MonthlyPriceKopecks
	if period == domain.PeriodYear {
		amount = pendingTariff.YearlyPriceKopecks
	}

	// Only free scheduled changes are applied automatically. Paid scheduled
	// changes are not supported in the MVP and are left for manual handling.
	if amount > 0 {
		return nil
	}

	if err := s.applyFreeRenewalOrDowngrade(ctx, tx, &sub, pendingTariff, period, now); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// ProcessExpiredGrace downgrades subscriptions whose grace period has ended to
// the free basic tariff and archives properties that exceed the basic limit.
// Returns the number of subscriptions processed.
func (s *BillingService) ProcessExpiredGrace(ctx context.Context, now time.Time) (int, error) {
	basicTariff, err := s.tariffs.GetByName(ctx, domain.TariffBasic)
	if err != nil {
		return 0, fmt.Errorf("get basic tariff: %w", err)
	}

	processed := 0
	for {
		subs, err := s.subscriptions.ListInExpiredGrace(ctx, now.UTC(), graceBatchSize)
		if err != nil {
			return processed, fmt.Errorf("list subscriptions in expired grace: %w", err)
		}
		if len(subs) == 0 {
			break
		}
		for _, sub := range subs {
			if err := s.downgradeToBasic(ctx, sub, basicTariff, now.UTC()); err != nil {
				s.log.ErrorContext(ctx, "downgrade to basic after grace failed",
					slog.String("subscription_id", sub.ID.String()),
					slog.String("user_id", sub.UserID.String()),
					slog.String("error", err.Error()))
				continue
			}
			processed++
		}
		if len(subs) < graceBatchSize {
			break
		}
	}
	return processed, nil
}

func (s *BillingService) downgradeToBasic(ctx context.Context, sub domain.Subscription, basicTariff domain.Tariff, now time.Time) error {
	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	sub, err = s.subscriptions.WithTx(tx).GetByIDForUpdate(ctx, sub.ID)
	if err != nil {
		return fmt.Errorf("get subscription for update: %w", err)
	}

	if sub.Status != domain.SubscriptionStatusGrace || sub.ValidUntil == nil || sub.ValidUntil.After(now) {
		return nil
	}

	applyBasicDowngrade(&sub, basicTariff.ID)
	if err := s.subscriptions.WithTx(tx).Update(ctx, sub); err != nil {
		return fmt.Errorf("update subscription after grace downgrade: %w", err)
	}

	if s.propertyArchiver != nil {
		if err := s.propertyArchiver.ArchiveExcessProperties(ctx, sub.UserID, basicTariff.ActivePropertyLimit); err != nil {
			return fmt.Errorf("archive excess properties after grace downgrade: %w", err)
		}
	}

	return tx.Commit(ctx)
}
