package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
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
	callbackBaseURL      string
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
	callbackBaseURL string,
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
		callbackBaseURL:      callbackBaseURL,
	}
}

func upgradePaymentDescription(name domain.TariffName, period domain.SubscriptionPeriod) string {
	return fmt.Sprintf("Upgrade to %s (%s)", name, period)
}

func renewalPaymentDescription(name domain.TariffName, period domain.SubscriptionPeriod) string {
	return fmt.Sprintf("Subscription renewal %s (%s)", name, period)
}

const tkassaDescriptionLimit = 140

// providerError is implemented by provider-specific errors that expose a
// machine-readable error code (e.g. T-Kassa's ErrorCode).
type providerError interface {
	error
	ProviderErrorCode() string
}

func providerErrorCode(err error) *string {
	var coder providerError
	if errors.As(err, &coder) {
		code := coder.ProviderErrorCode()
		if code != "" {
			return &code
		}
	}
	return nil
}

func truncateTkassaDescription(s string) string {
	if len(s) <= tkassaDescriptionLimit {
		return s
	}
	return s[:tkassaDescriptionLimit]
}

func tkassaCallbackURLs(baseURL string, paymentID uuid.UUID) (notification, success, fail, addCardSuccess, addCardFail string) {
	baseURL = strings.TrimRight(baseURL, "/")
	notification = baseURL + "/webhooks/payment/tkassa"
	success = fmt.Sprintf("%s/subscription/payments/%s/success", baseURL, paymentID.String())
	fail = fmt.Sprintf("%s/subscription/payments/%s/fail", baseURL, paymentID.String())
	addCardSuccess = baseURL + "/subscription/payment-methods/add-card/success"
	addCardFail = baseURL + "/subscription/payment-methods/add-card/fail"
	return
}

// ListTariffs returns all tariffs ordered by price.
func (s *BillingService) ListTariffs(ctx context.Context) ([]domain.Tariff, error) {
	list, err := s.tariffs.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tariffs: %w", err)
	}

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

	lastPayment, err := s.subscriptionPayments.GetLastSucceededBySubscriptionID(ctx, sub.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return SubscriptionView{}, fmt.Errorf("get last succeeded payment: %w", err)
	}
	if err == nil {
		period := lastPayment.Period
		sub.CurrentPeriod = &period
		view.Subscription = sub
	}

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

func (s *BillingService) existingUpgradeResponse(ctx context.Context, payment domain.SubscriptionPayment) ChangeTariffResponse {
	res := ChangeTariffResponse{PaymentID: payment.ID}
	if urlProvider, ok := s.provider.(PaymentURLProvider); ok {
		if url, err := urlProvider.PaymentURL(ctx, payment.ID); err == nil {
			res.ConfirmURL = url
		}
		return res
	}
	if payment.PaymentURL != nil && *payment.PaymentURL != "" {
		res.ConfirmURL = *payment.PaymentURL
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
				// If a previous Init succeeded but the process crashed before the
				// provider reference was persisted, recover it from the provider
				// before returning. This keeps the retry path idempotent.
				if p.ProviderPaymentID == nil {
					_ = tx.Rollback(ctx)
					return s.recoverUpgradeProviderReference(ctx, p, newTariff, amount, req.Period, userID, now)
				}
				return s.existingUpgradeResponse(ctx, p), nil
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
		if errors.Is(err, ErrAlreadyExists) {
			// A concurrent request created the pending payment first. Return the
			// existing one instead of failing.
			pending, listErr := s.subscriptionPayments.WithTx(tx).ListPendingSubscriptionPaymentsByUserID(ctx, userID)
			if listErr != nil {
				return ChangeTariffResponse{}, fmt.Errorf("list pending subscription payments: %w", listErr)
			}
			for _, p := range pending {
				if p.TariffID == newTariff.ID && p.Period == period {
					_ = tx.Rollback(ctx)
					return s.existingUpgradeResponse(ctx, p), nil
				}
			}
		}
		return ChangeTariffResponse{}, fmt.Errorf("save subscription payment: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return ChangeTariffResponse{}, fmt.Errorf("commit transaction: %w", err)
	}

	notification, successURL, failURL, _, _ := tkassaCallbackURLs(s.callbackBaseURL, payment.ID)
	initRes, err := s.provider.Init(ctx, InitRequest{
		PaymentID:              payment.ID,
		AmountKopecks:          amount,
		Period:                 period,
		UserID:                 userID,
		CustomerKey:            userID.String(),
		Recurrent:              true,
		OperationInitiatorType: "1",
		NotificationURL:        notification,
		SuccessURL:             successURL,
		FailURL:                failURL,
		Description:            truncateTkassaDescription(upgradePaymentDescription(newTariff.Name, period)),
	})
	if err != nil {
		s.markPaymentFailedBestEffort(ctx, payment.ID, now)
		return ChangeTariffResponse{}, sanitize.Wrap(err, "init payment")
	}

	// The provider token is only available after Init succeeds. Persist the
	// provider reference and any newly created payment method atomically in a
	// single transaction so a crash cannot leave a pending payment without a
	// provider reference.
	payment, err = s.saveProviderInitResult(ctx, payment, initRes, userID, now)
	if err != nil {
		return ChangeTariffResponse{}, err
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
// For the fake provider the method is created synchronously from the raw token.
// For T-Kassa a bank-form flow is initiated and the confirmation URL is returned.
func (s *BillingService) AddPaymentMethod(ctx context.Context, userID uuid.UUID, req AddPaymentMethodRequest) (AddPaymentMethodResponse, error) {
	if s.provider.Name() == domain.ProviderFake {
		pm, err := domain.NewPaymentMethod(
			userID,
			s.provider.Name(),
			req.ProviderToken,
			maskToken(req.ProviderToken),
			s.clock.Now().UTC(),
		)
		if err != nil {
			return AddPaymentMethodResponse{}, fmt.Errorf("create payment method: %w", err)
		}

		tx, err := s.beginner.Begin(ctx)
		if err != nil {
			return AddPaymentMethodResponse{}, fmt.Errorf("begin transaction: %w", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		pm, err = s.paymentMethods.WithTx(tx).Create(ctx, pm)
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
// For T-Kassa, the card is detached from the provider first; local deletion is
// skipped if the provider call fails.
func (s *BillingService) DeletePaymentMethod(ctx context.Context, userID, methodID uuid.UUID) error {
	// Validate existence, ownership, active status and provider card id inside a
	// short transaction so the active check cannot race with concurrent updates.
	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	pm, err := s.paymentMethods.WithTx(tx).GetByID(ctx, methodID)
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

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit validate payment method transaction: %w", err)
	}

	if s.provider.Name() == domain.ProviderTkassa && pm.ProviderCardID != "" {
		if err := s.provider.RemoveCard(ctx, userID.String(), pm.ProviderCardID); err != nil {
			if errors.Is(err, ErrProviderCardNotFound) {
				s.log.WarnContext(ctx, "provider card already removed; continuing local deletion",
					slog.String("payment_method_id", methodID.String()),
					slog.String("provider_card_id", maskCardID(pm.ProviderCardID)))
			} else {
				return fmt.Errorf("remove provider card: %w", err)
			}
		}
	}

	tx, err = s.beginner.Begin(ctx)
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

// GetPayment returns a single subscription payment for admin view.
func (s *BillingService) GetPayment(ctx context.Context, paymentID uuid.UUID) (AdminSubscriptionPaymentView, error) {
	p, err := s.subscriptionPayments.GetByIDAdmin(ctx, paymentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return AdminSubscriptionPaymentView{}, ErrPaymentNotFound
		}
		return AdminSubscriptionPaymentView{}, fmt.Errorf("get payment: %w", err)
	}

	tariff, err := s.tariffs.GetByID(ctx, p.Payment.TariffID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return AdminSubscriptionPaymentView{}, ErrTariffNotFound
		}
		return AdminSubscriptionPaymentView{}, fmt.Errorf("get payment tariff: %w", err)
	}

	return AdminSubscriptionPaymentView{
		Payment:   p.Payment,
		Tariff:    tariff,
		UserPhone: p.UserPhone,
	}, nil
}

// ListAllPayments returns all subscription payments for admin view.
func (s *BillingService) ListAllPayments(ctx context.Context, filters ListAllPaymentsFilters) ([]AdminSubscriptionPaymentView, int64, error) {
	if filters.Limit <= 0 {
		filters.Limit = 20
	}
	if filters.Limit > 100 {
		filters.Limit = 100
	}
	if filters.Offset < 0 {
		filters.Offset = 0
	}

	if filters.Status != "" &&
		filters.Status != string(domain.PaymentStatusPending) &&
		filters.Status != string(domain.PaymentStatusSucceeded) &&
		filters.Status != string(domain.PaymentStatusFailed) &&
		filters.Status != string(domain.PaymentStatusRefunded) &&
		filters.Status != string(domain.PaymentStatusPartialRefunded) {
		return nil, 0, fmt.Errorf("%w: invalid status filter", ErrInvalidFilter)
	}

	payments, total, err := s.subscriptionPayments.ListAll(ctx, filters.Status, filters.UserID, filters.Limit, filters.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list all payments: %w", err)
	}

	tariffs, err := s.tariffs.List(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list tariffs: %w", err)
	}
	tariffByID := make(map[uuid.UUID]domain.Tariff, len(tariffs))
	for _, t := range tariffs {
		tariffByID[t.ID] = t
	}

	views := make([]AdminSubscriptionPaymentView, 0, len(payments))
	for _, p := range payments {
		t, ok := tariffByID[p.Payment.TariffID]
		if !ok {
			return nil, 0, fmt.Errorf("payment %s references unknown tariff %s", p.Payment.ID, p.Payment.TariffID)
		}
		views = append(views, AdminSubscriptionPaymentView{
			Payment:   p.Payment,
			Tariff:    t,
			UserPhone: p.UserPhone,
		})
	}

	return views, total, nil
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
		if payment.Status == domain.PaymentStatusSucceeded {
			s.applySubscriptionRenewalAndArchive(ctx, payment)
		}
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

	if err := s.applyPaymentResult(ctx, tx, &payment, payload); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit fake payment confirmation transaction: %w", err)
	}

	s.applySubscriptionRenewalAndArchive(ctx, payment)
	return nil
}

// WebhookResponse returns the provider-specific response body that must be sent
// back after a webhook is handled.
func (s *BillingService) WebhookResponse() []byte {
	return s.provider.WebhookResponse()
}

func isAddCardNotificationType(notificationType string) bool {
	return notificationType == "NotificationAddCard" || notificationType == "AddCard"
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

	s.log.InfoContext(ctx, "processing payment webhook",
		slog.String("provider", providerName),
		slog.String("notification_type", result.NotificationType),
		slog.String("status", string(result.Status)),
		slog.String("provider_payment_id", result.ProviderPaymentID),
		slog.String("customer_key", result.CustomerKey),
		slog.String("request_key", result.RequestKey),
	)

	// Standalone card binding webhook: upsert the saved card and exit.
	if isAddCardNotificationType(result.NotificationType) {
		userID, err := uuid.Parse(result.CustomerKey)
		if err != nil {
			return fmt.Errorf("invalid customer key: %w", err)
		}

		pm, err := s.buildPaymentMethodFromWebhook(userID, result)
		if err != nil {
			return err
		}

		tx, err := s.beginner.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin transaction: %w", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		pm, err = s.paymentMethods.WithTx(tx).UpsertByTokenHash(ctx, pm)
		if err != nil {
			return fmt.Errorf("upsert add card payment method: %w", err)
		}

		// A successful AddCard webhook means the saved token is valid and should
		// become the active payment method for future renewals.
		if err = s.paymentMethods.WithTx(tx).SetActive(ctx, userID, pm.ID); err != nil {
			return fmt.Errorf("activate add card payment method: %w", err)
		}

		return tx.Commit(ctx)
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
		// Refund webhooks may arrive after a payment has already succeeded.
		// In that case we must process the refund instead of ignoring it.
		if result.Status != domain.PaymentStatusRefunded && result.Status != domain.PaymentStatusPartialRefunded {
			if payment.Status == domain.PaymentStatusSucceeded {
				s.applySubscriptionRenewalAndArchive(ctx, payment)
			}
			return nil
		}
	}

	switch result.Status {
	case domain.PaymentStatusPending:
		// AUTHORIZED: credentials received, payment stays pending until CONFIRMED.
		pm, err := s.upsertPaymentMethodFromWebhook(ctx, tx, payment.UserID, result)
		if err != nil {
			return err
		}

		if payment.ProviderPaymentID == nil || *payment.ProviderPaymentID == "" {
			if _, updateErr := s.subscriptionPayments.WithTx(tx).UpdateProviderPaymentID(ctx, payment.ID, result.ProviderPaymentID); updateErr != nil {
				return fmt.Errorf("update provider payment id: %w", updateErr)
			}
		}

		if payment.PaymentMethodID == nil {
			if _, updateErr := s.subscriptionPayments.WithTx(tx).UpdatePaymentMethodAndProviderID(ctx, payment.ID, pm.ID, result.ProviderPaymentID); updateErr != nil {
				return fmt.Errorf("update payment method id: %w", updateErr)
			}
		}

		return tx.Commit(ctx)

	case domain.PaymentStatusSucceeded, domain.PaymentStatusFailed, domain.PaymentStatusRefunded, domain.PaymentStatusPartialRefunded:
		if err := s.applyPaymentResult(ctx, tx, &payment, result); err != nil {
			return err
		}

		switch result.Status {
		case domain.PaymentStatusFailed:
			sub, err := s.subscriptions.WithTx(tx).GetByIDForUpdate(ctx, payment.SubscriptionID)
			if err != nil {
				return fmt.Errorf("get subscription for failed webhook: %w", err)
			}
			if sub.TariffID == payment.TariffID {
				s.transitionToGrace(&sub, s.clock.Now().UTC())
				if err := s.subscriptions.WithTx(tx).Update(ctx, sub); err != nil {
					return fmt.Errorf("transition subscription to grace after failed webhook: %w", err)
				}
			}

		case domain.PaymentStatusRefunded, domain.PaymentStatusPartialRefunded:
			if err := s.applyRefundToSubscription(ctx, tx, payment.SubscriptionID); err != nil {
				return err
			}
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit webhook transaction: %w", err)
		}

		if result.Status == domain.PaymentStatusSucceeded {
			s.applySubscriptionRenewalAndArchive(ctx, payment)
		}
		return nil

	default:
		return fmt.Errorf("unsupported webhook status: %s", result.Status)
	}
}

// RefundPayment cancels/refunds a succeeded subscription payment through the provider
// and immediately downgrades the subscription to basic. The provider HTTP call is
// made outside of any database transaction so a slow provider cannot hold a row
// lock for an unbounded time.
func (s *BillingService) RefundPayment(ctx context.Context, paymentID uuid.UUID, amountKopecks *int64) error {
	// First short transaction: load the payment with a row lock, validate that it
	// can be refunded, and commit immediately.
	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin refund payment transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	payment, err := s.subscriptionPayments.WithTx(tx).GetByIDForUpdate(ctx, paymentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrPaymentNotFound
		}
		return fmt.Errorf("get payment for refund: %w", err)
	}

	if payment.Status != domain.PaymentStatusSucceeded && payment.Status != domain.PaymentStatusPending {
		return fmt.Errorf("%w: cannot refund payment with status %s", domain.ErrInvalidPaymentStatus, payment.Status)
	}

	refundAmount := payment.AmountKopecks
	if amountKopecks != nil {
		if *amountKopecks <= 0 || *amountKopecks > payment.AmountKopecks {
			return fmt.Errorf("%w: refund amount must be between 1 and %d kopecks", domain.ErrInvalidAmount, payment.AmountKopecks)
		}
		refundAmount = *amountKopecks
	}

	if payment.ProviderPaymentID == nil || *payment.ProviderPaymentID == "" {
		return fmt.Errorf("%w: payment has no provider payment id", domain.ErrInvalidPaymentStatus)
	}

	providerPaymentID := *payment.ProviderPaymentID
	userID := payment.UserID
	subscriptionID := payment.SubscriptionID

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit refund validation transaction: %w", err)
	}

	// Provider call happens outside the transaction so the database connection is
	// not held during an unbounded external HTTP request.
	cancelRes, err := s.provider.Cancel(ctx, CancelRequest{
		PaymentID:         paymentID,
		ProviderPaymentID: providerPaymentID,
		AmountKopecks:     refundAmount,
	})
	if err != nil {
		s.log.ErrorContext(ctx, "provider cancel failed",
			slog.String("payment_id", paymentID.String()),
			slog.String("subscription_id", subscriptionID.String()),
			slog.String("user_id", userID.String()),
			slog.Int64("refund_amount_kopecks", refundAmount),
			slog.String("error", sanitize.Error(err)))
		return fmt.Errorf("provider cancel: %w", err)
	}

	if cancelRes.Status != domain.PaymentStatusRefunded && cancelRes.Status != domain.PaymentStatusPartialRefunded {
		return fmt.Errorf("provider cancel returned non-refund status: %s", cancelRes.Status)
	}

	// Second short transaction: reload the payment under lock, re-check that it is
	// still succeeded (it may have been refunded by a concurrent webhook), apply
	// the refund and downgrade the subscription.
	resultTx, err := s.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin refund result transaction: %w", err)
	}
	defer func() { _ = resultTx.Rollback(ctx) }()

	payment, err = s.subscriptionPayments.WithTx(resultTx).GetByIDForUpdate(ctx, paymentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrPaymentNotFound
		}
		return fmt.Errorf("get payment for refund result: %w", err)
	}

	if payment.Status != domain.PaymentStatusSucceeded && payment.Status != domain.PaymentStatusPending {
		return fmt.Errorf("%w: payment status changed to %s during refund", domain.ErrInvalidPaymentStatus, payment.Status)
	}

	if err := s.subscriptionPayments.WithTx(resultTx).MarkRefunded(ctx, payment.ID, cancelRes.Status, cancelRes.RefundedAmountKopecks, s.clock.Now().UTC()); err != nil {
		return fmt.Errorf("mark payment refunded: %w", err)
	}

	if err := s.applyRefundToSubscription(ctx, resultTx, payment.SubscriptionID); err != nil {
		return err
	}

	if err := resultTx.Commit(ctx); err != nil {
		return fmt.Errorf("commit refund payment transaction: %w", err)
	}

	s.log.InfoContext(ctx, "payment refunded",
		slog.String("payment_id", paymentID.String()),
		slog.String("subscription_id", subscriptionID.String()),
		slog.String("user_id", userID.String()),
		slog.Int64("refund_amount_kopecks", cancelRes.RefundedAmountKopecks))

	return nil
}

// applyRefundToSubscription downgrades the subscription to basic after a refund.
func (s *BillingService) applyRefundToSubscription(ctx context.Context, tx transaction.Tx, subscriptionID uuid.UUID) error {
	sub, err := s.subscriptions.WithTx(tx).GetByIDForUpdate(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("get subscription for refund: %w", err)
	}
	basicTariff, err := s.tariffs.GetByName(ctx, domain.TariffBasic)
	if err != nil {
		return fmt.Errorf("get basic tariff for refund: %w", err)
	}
	applyBasicDowngrade(&sub, basicTariff.ID)
	if err := s.subscriptions.WithTx(tx).Update(ctx, sub); err != nil {
		return fmt.Errorf("downgrade subscription to basic after refund: %w", err)
	}
	if s.propertyArchiver != nil {
		if err := s.propertyArchiver.ArchiveExcessProperties(ctx, tx, sub.UserID, basicTariff.ActivePropertyLimit); err != nil {
			return fmt.Errorf("archive excess properties after refund: %w", err)
		}
	}
	return nil
}

func (s *BillingService) applyPaymentResult(
	ctx context.Context,
	tx transaction.Tx,
	payment *domain.SubscriptionPayment,
	payload WebhookPayload,
) error {
	if payment.ProviderPaymentID != nil && *payment.ProviderPaymentID != payload.ProviderPaymentID {
		return fmt.Errorf("provider payment id mismatch: expected %q, got %q", *payment.ProviderPaymentID, payload.ProviderPaymentID)
	}

	// If the provider payment id was not persisted during Init (e.g. recovery
	// path or concurrent webhook), store it from the webhook payload before
	// finalizing the payment.
	if payment.ProviderPaymentID == nil && payload.ProviderPaymentID != "" {
		updated, err := s.subscriptionPayments.WithTx(tx).UpdateProviderPaymentID(ctx, payment.ID, payload.ProviderPaymentID)
		if err != nil {
			return fmt.Errorf("persist provider payment id from webhook: %w", err)
		}
		payment.ProviderPaymentID = updated.ProviderPaymentID
	}

	now := s.clock.Now().UTC()
	switch payload.Status {
	case domain.PaymentStatusSucceeded:
		if err := s.subscriptionPayments.WithTx(tx).MarkSucceeded(ctx, payment.ID, now); err != nil {
			return fmt.Errorf("mark payment succeeded: %w", err)
		}
		payment.Status = domain.PaymentStatusSucceeded
		payment.UpdatedAt = now
		payment.SucceededAt = &now

		// Payment method activation is part of the critical transaction: a
		// succeeded payment means the saved token is valid and should become the
		// active method. Subscription renewal/tariff change runs afterwards as a
		// best-effort step so a domain failure cannot roll back the charge.
		methodID := payment.PaymentMethodID
		if payload.RebillID != "" {
			pm, err := s.upsertPaymentMethodFromWebhook(ctx, tx, payment.UserID, payload)
			if err != nil {
				return err
			}
			methodID = &pm.ID
			if payment.PaymentMethodID == nil {
				if _, updateErr := s.subscriptionPayments.WithTx(tx).UpdatePaymentMethodAndProviderID(ctx, payment.ID, pm.ID, payload.ProviderPaymentID); updateErr != nil {
					return fmt.Errorf("update payment method id: %w", updateErr)
				}
			}
		}

		if methodID != nil {
			if err := s.paymentMethods.WithTx(tx).SetActive(ctx, payment.UserID, *methodID); err != nil {
				return fmt.Errorf("activate payment method: %w", err)
			}
		}

	case domain.PaymentStatusFailed:
		if err := s.subscriptionPayments.WithTx(tx).MarkFailed(ctx, payment.ID, payload.ErrorCode, now); err != nil {
			return fmt.Errorf("mark payment failed: %w", err)
		}
		payment.Status = domain.PaymentStatusFailed
		payment.ErrorCode = payload.ErrorCode
		payment.UpdatedAt = now

	case domain.PaymentStatusRefunded, domain.PaymentStatusPartialRefunded:
		if err := s.subscriptionPayments.WithTx(tx).MarkRefunded(ctx, payment.ID, payload.Status, payload.AmountKopecks, now); err != nil {
			return fmt.Errorf("mark payment refunded: %w", err)
		}
		payment.Status = payload.Status
		payment.RefundedAmountKopecks = &payload.AmountKopecks
		payment.UpdatedAt = now

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

func maskCardID(cardID string) string {
	if len(cardID) <= 4 {
		return "****"
	}
	return cardID[:2] + "****" + cardID[len(cardID)-4:]
}

func (s *BillingService) buildPaymentMethodFromWebhook(userID uuid.UUID, payload WebhookPayload) (domain.PaymentMethod, error) {
	pm, err := domain.NewPaymentMethod(
		userID,
		s.provider.Name(),
		payload.RebillID,
		payload.Pan,
		s.clock.Now().UTC(),
	)
	if err != nil {
		return domain.PaymentMethod{}, fmt.Errorf("create payment method from webhook: %w", err)
	}
	pm.ProviderCardID = payload.CardID
	pm.ExpDate = payload.ExpDate
	return pm, nil
}

func (s *BillingService) upsertPaymentMethodFromWebhook(ctx context.Context, tx transaction.Tx, userID uuid.UUID, payload WebhookPayload) (domain.PaymentMethod, error) {
	pm, err := s.buildPaymentMethodFromWebhook(userID, payload)
	if err != nil {
		return domain.PaymentMethod{}, err
	}
	pm, err = s.paymentMethods.WithTx(tx).UpsertByTokenHash(ctx, pm)
	if err != nil {
		return domain.PaymentMethod{}, fmt.Errorf("upsert payment method from webhook: %w", err)
	}
	return pm, nil
}

// saveProviderInitResult persists the provider reference and any newly created
// payment method atomically in a single transaction. If the transaction cannot
// be committed, the pending payment is marked failed so it does not stay
// unfinished.
func (s *BillingService) saveProviderInitResult(ctx context.Context, payment domain.SubscriptionPayment, initRes InitResult, userID uuid.UUID, now time.Time) (domain.SubscriptionPayment, error) {
	paymentID := payment.ID

	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		s.markPaymentFailedBestEffort(ctx, paymentID, now)
		return domain.SubscriptionPayment{}, fmt.Errorf("begin provider result transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Reload the payment under lock: the provider call happened outside of a
	// transaction, so the row must be locked before the reference is persisted.
	payment, err = s.subscriptionPayments.WithTx(tx).GetByIDForUpdate(ctx, paymentID)
	if err != nil {
		s.markPaymentFailedBestEffort(ctx, paymentID, now)
		return domain.SubscriptionPayment{}, fmt.Errorf("get payment for provider result: %w", err)
	}
	if payment.IsFinalized() {
		// The payment was already finalized by a concurrent webhook or another
		// request. Return the current state without overwriting it.
		return payment, nil
	}
	if payment.Status != domain.PaymentStatusPending {
		s.markPaymentFailedBestEffort(ctx, paymentID, now)
		return domain.SubscriptionPayment{}, domain.ErrInvalidPaymentStatus
	}

	if initRes.SavedToken != "" {
		pm, err := domain.NewPaymentMethod(
			userID,
			s.provider.Name(),
			initRes.SavedToken,
			maskToken(initRes.SavedToken),
			now,
		)
		if err != nil {
			s.markPaymentFailedBestEffort(ctx, paymentID, now)
			return domain.SubscriptionPayment{}, fmt.Errorf("create payment method: %w", err)
		}
		pm, err = s.paymentMethods.WithTx(tx).Create(ctx, pm)
		if err != nil {
			s.markPaymentFailedBestEffort(ctx, paymentID, now)
			return domain.SubscriptionPayment{}, fmt.Errorf("save payment method: %w", err)
		}
		payment, err = s.subscriptionPayments.WithTx(tx).UpdatePaymentMethodAndProviderID(ctx, paymentID, pm.ID, initRes.ProviderPaymentID)
		if err != nil {
			s.markPaymentFailedBestEffort(ctx, paymentID, now)
			return domain.SubscriptionPayment{}, fmt.Errorf("update payment method and provider payment id: %w", err)
		}
	} else {
		payment, err = s.subscriptionPayments.WithTx(tx).UpdateProviderPaymentID(ctx, paymentID, initRes.ProviderPaymentID)
		if err != nil {
			s.markPaymentFailedBestEffort(ctx, paymentID, now)
			return domain.SubscriptionPayment{}, fmt.Errorf("update provider payment id: %w", err)
		}
	}

	if initRes.PaymentURL != "" {
		payment, err = s.subscriptionPayments.WithTx(tx).UpdatePaymentURL(ctx, paymentID, initRes.PaymentURL)
		if err != nil {
			s.markPaymentFailedBestEffort(ctx, paymentID, now)
			return domain.SubscriptionPayment{}, fmt.Errorf("update payment url: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		s.markPaymentFailedBestEffort(ctx, paymentID, now)
		return domain.SubscriptionPayment{}, fmt.Errorf("commit provider result transaction: %w", err)
	}

	return payment, nil
}

// markPaymentFailedBestEffort marks a pending payment as failed in a separate
// transaction. It is used when a transaction that should have finalized the
// payment has already failed and we need to avoid leaving the record pending.
func (s *BillingService) markPaymentFailedBestEffort(ctx context.Context, paymentID uuid.UUID, now time.Time) {
	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to begin transaction for best-effort payment failure mark",
			slog.String("payment_id", paymentID.String()),
			slog.String("error", sanitize.Error(err)))
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := s.subscriptionPayments.WithTx(tx).MarkFailed(ctx, paymentID, nil, now); err != nil {
		s.log.ErrorContext(ctx, "failed to mark payment failed in best-effort transaction",
			slog.String("payment_id", paymentID.String()),
			slog.String("error", sanitize.Error(err)))
		return
	}

	if err := tx.Commit(ctx); err != nil {
		s.log.ErrorContext(ctx, "failed to commit best-effort payment failure mark",
			slog.String("payment_id", paymentID.String()),
			slog.String("error", sanitize.Error(err)))
	}
}

// recoverUpgradeProviderReference recovers a provider reference for an existing
// pending upgrade payment whose provider_payment_id was lost after a crash.
// It relies on the provider's idempotent Init to reconstruct the reference.
func (s *BillingService) recoverUpgradeProviderReference(ctx context.Context, payment domain.SubscriptionPayment, newTariff domain.Tariff, amount int64, period domain.SubscriptionPeriod, userID uuid.UUID, now time.Time) (ChangeTariffResponse, error) {
	notification, successURL, failURL, _, _ := tkassaCallbackURLs(s.callbackBaseURL, payment.ID)
	initRes, err := s.provider.Init(ctx, InitRequest{
		PaymentID:              payment.ID,
		AmountKopecks:          amount,
		Period:                 period,
		UserID:                 userID,
		CustomerKey:            userID.String(),
		Recurrent:              true,
		OperationInitiatorType: "1",
		NotificationURL:        notification,
		SuccessURL:             successURL,
		FailURL:                failURL,
		Description:            truncateTkassaDescription(upgradePaymentDescription(newTariff.Name, period)),
	})
	if err != nil {
		s.markPaymentFailedBestEffort(ctx, payment.ID, now)
		return ChangeTariffResponse{}, sanitize.Wrap(err, "recover provider reference")
	}

	payment, err = s.saveProviderInitResult(ctx, payment, initRes, userID, now)
	if err != nil {
		return ChangeTariffResponse{}, fmt.Errorf("recover provider reference: %w", err)
	}

	return ChangeTariffResponse{
		PaymentID:  payment.ID,
		ConfirmURL: initRes.PaymentURL,
	}, nil
}

const (
	renewalBatchSize = 100
	graceBatchSize   = 100
	gracePeriod      = 7 * 24 * time.Hour
)

const (
	pendingUpgradeStalenessThreshold = 5 * time.Minute
	pendingPaymentStalenessThreshold = pendingUpgradeStalenessThreshold
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
					slog.String("error", sanitize.Error(err)))
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
					slog.String("error", sanitize.Error(err)))
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
					slog.String("error", sanitize.Error(err)))
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
	// First transaction: lock the subscription, resolve the renewal terms, and
	// persist a pending payment. This transaction is committed *before* the
	// external provider call so the database connection is not held during an
	// unbounded HTTP request.
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

	// If a previous succeeded payment for the same renewal tariff was not fully
	// applied to the subscription (e.g. the best-effort renewal step failed),
	// reconcile it now instead of creating a duplicate payment.
	lastSucceeded, err := s.subscriptionPayments.WithTx(tx).GetLastSucceededBySubscriptionID(ctx, sub.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("get last succeeded payment: %w", err)
	}
	if err == nil && lastSucceeded.TariffID == renewalTariff.ID && !s.isSubscriptionRenewalApplied(sub, lastSucceeded) {
		_ = tx.Rollback(ctx)
		s.applySubscriptionRenewalBestEffort(ctx, sub.ID, lastSucceeded, now)
		return nil
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

	// Look for an existing pending renewal payment for the same terms. If a
	// previous run created the pending row but crashed before saving the provider
	// reference, recover it idempotently instead of creating a duplicate.
	var payment domain.SubscriptionPayment
	pendingPayments, err := s.subscriptionPayments.WithTx(tx).ListPendingSubscriptionPaymentsByUserID(ctx, sub.UserID)
	if err != nil {
		return fmt.Errorf("list pending renewal payments: %w", err)
	}
	for _, p := range pendingPayments {
		if p.SubscriptionID == sub.ID && p.TariffID == renewalTariff.ID && p.Period == period {
			payment = p
			break
		}
	}

	// If we reuse a pending renewal payment but the active card has changed,
	// update the payment method reference so the payment record matches the
	// token that will actually be charged.
	if payment.ID != uuid.Nil && (payment.PaymentMethodID == nil || *payment.PaymentMethodID != pm.ID) {
		payment, err = s.subscriptionPayments.WithTx(tx).UpdatePaymentMethodID(ctx, payment.ID, pm.ID)
		if err != nil {
			return fmt.Errorf("update pending renewal payment method: %w", err)
		}
	}

	if payment.ID == uuid.Nil {
		payment, err = domain.NewSubscriptionPayment(
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
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit pending renewal payment transaction: %w", err)
	}

	// Recover a missing provider reference idempotently before charging.
	if payment.ProviderPaymentID == nil || *payment.ProviderPaymentID == "" {
		notification, successURL, failURL, _, _ := tkassaCallbackURLs(s.callbackBaseURL, payment.ID)
		initRes, err := s.provider.Init(ctx, InitRequest{
			PaymentID:              payment.ID,
			AmountKopecks:          amount,
			Period:                 period,
			UserID:                 sub.UserID,
			CustomerKey:            sub.UserID.String(),
			Recurrent:              true,
			OperationInitiatorType: "R",
			NotificationURL:        notification,
			SuccessURL:             successURL,
			FailURL:                failURL,
			Description:            truncateTkassaDescription(renewalPaymentDescription(renewalTariff.Name, period)),
		})
		if err != nil {
			if recErr := s.recoverRenewalFailure(ctx, payment.ID, sub.ID, "", providerErrorCode(err), now); recErr != nil {
				return fmt.Errorf("provider init error: %w; recovery failed: %w", err, recErr)
			}
			s.log.ErrorContext(ctx, "provider init failed; subscription moved to grace",
				slog.String("subscription_id", sub.ID.String()),
				slog.String("payment_id", payment.ID.String()),
				slog.String("error", sanitize.Error(err)))
			return nil
		}

		payment, err = s.saveProviderInitResult(ctx, payment, initRes, sub.UserID, now)
		if err != nil {
			return fmt.Errorf("save provider init result: %w", err)
		}
	}

	chargeProviderPaymentID := ""
	if payment.ProviderPaymentID != nil {
		chargeProviderPaymentID = *payment.ProviderPaymentID
	}

	// If a previous run already registered a provider reference, query the
	// provider status before re-charging to avoid duplicate charges.
	if chargeProviderPaymentID != "" {
		status, statusErr := s.provider.Status(ctx, payment.ID, chargeProviderPaymentID)
		if statusErr == nil {
			switch status {
			case domain.PaymentStatusSucceeded:
				return s.markRenewalSucceededAndApply(ctx, payment, sub.ID, now)
			case domain.PaymentStatusFailed:
				return s.markRenewalFailedAndGrace(ctx, payment.ID, sub.ID, nil, now)
			}
		}
	}

	chargeResult, chargeErr := s.provider.Charge(ctx, ChargeRequest{
		PaymentID:         payment.ID,
		AmountKopecks:     amount,
		Token:             pm.ProviderToken,
		ProviderPaymentID: chargeProviderPaymentID,
	})
	if chargeErr != nil {
		hint := chargeResult.ProviderPaymentID
		if hint == "" {
			hint = chargeProviderPaymentID
		}
		if recErr := s.recoverRenewalFailure(ctx, payment.ID, sub.ID, hint, providerErrorCode(chargeErr), now); recErr != nil {
			return fmt.Errorf("provider charge error: %w; recovery failed: %w", chargeErr, recErr)
		}
		s.log.ErrorContext(ctx, "provider charge failed; subscription moved to grace",
			slog.String("subscription_id", sub.ID.String()),
			slog.String("payment_id", payment.ID.String()),
			slog.String("error", sanitize.Error(chargeErr)))
		return nil
	}

	// Second transaction: reload the payment under lock and apply the provider
	// result. The lock guards against concurrent webhook updates.
	resultTx, err := s.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin post-charge transaction: %w", err)
	}
	defer func() { _ = resultTx.Rollback(ctx) }()

	// Recovery opens its own transaction, so it must never run while resultTx
	// is still open. This helper rolls back resultTx first; the deferred
	// rollback will then return ErrTxDone, which is safely ignored.
	recoverWithClosedTx := func() error {
		_ = resultTx.Rollback(ctx)
		return s.recoverRenewalFailure(ctx, payment.ID, sub.ID, chargeResult.ProviderPaymentID, nil, now)
	}

	payment, err = s.subscriptionPayments.WithTx(resultTx).GetByIDForUpdate(ctx, payment.ID)
	if err != nil {
		if recErr := recoverWithClosedTx(); recErr != nil {
			return fmt.Errorf("get payment for update after charge: %w; recovery failed: %w", err, recErr)
		}
		return fmt.Errorf("get payment for update after charge: %w", err)
	}

	if payment.Status != domain.PaymentStatusPending {
		// A concurrent webhook already finalized the payment; the provider
		// response is stale relative to the database state.
		return nil
	}

	if chargeResult.ProviderPaymentID != "" {
		if _, updateErr := s.subscriptionPayments.WithTx(resultTx).UpdateProviderPaymentID(ctx, payment.ID, chargeResult.ProviderPaymentID); updateErr != nil {
			if recErr := recoverWithClosedTx(); recErr != nil {
				return fmt.Errorf("update renewal provider payment id: %w; recovery failed: %w", updateErr, recErr)
			}
			return fmt.Errorf("update renewal provider payment id: %w", updateErr)
		}
	}

	switch chargeResult.Status {
	case domain.PaymentStatusSucceeded:
		if err := s.subscriptionPayments.WithTx(resultTx).MarkSucceeded(ctx, payment.ID, now); err != nil {
			if recErr := recoverWithClosedTx(); recErr != nil {
				return fmt.Errorf("mark renewal payment succeeded: %w; recovery failed: %w", err, recErr)
			}
			return fmt.Errorf("mark renewal payment succeeded: %w", err)
		}
		payment.Status = domain.PaymentStatusSucceeded
		payment.UpdatedAt = now
		payment.SucceededAt = &now
		if err := resultTx.Commit(ctx); err != nil {
			return fmt.Errorf("commit post-charge transaction: %w", err)
		}
		// Subscription renewal/tariff change and property archiving are
		// best-effort compensating operations: they must not roll back a payment
		// that has already been charged.
		sub, renewalTariff, oldTariffID, ok := s.applySubscriptionRenewalBestEffort(ctx, sub.ID, payment, now)
		if ok && oldTariffID != sub.TariffID {
			s.archiveExcessPropertiesBestEffort(ctx, sub.UserID, renewalTariff.ActivePropertyLimit)
		}
		return nil
	case domain.PaymentStatusFailed:
		if err := s.subscriptionPayments.WithTx(resultTx).MarkFailed(ctx, payment.ID, providerErrorCode(chargeErr), now); err != nil {
			if recErr := recoverWithClosedTx(); recErr != nil {
				return fmt.Errorf("mark renewal payment failed: %w; recovery failed: %w", err, recErr)
			}
			return fmt.Errorf("mark renewal payment failed: %w", err)
		}
		sub, err = s.subscriptions.WithTx(resultTx).GetByIDForUpdate(ctx, sub.ID)
		if err != nil {
			if recErr := recoverWithClosedTx(); recErr != nil {
				return fmt.Errorf("get subscription for update after failed charge: %w; recovery failed: %w", err, recErr)
			}
			return fmt.Errorf("get subscription for update after failed charge: %w", err)
		}
		s.transitionToGrace(&sub, now)
		if err := s.subscriptions.WithTx(resultTx).Update(ctx, sub); err != nil {
			if recErr := recoverWithClosedTx(); recErr != nil {
				return fmt.Errorf("transition to grace after failed renewal: %w; recovery failed: %w", err, recErr)
			}
			return fmt.Errorf("transition to grace after failed renewal: %w", err)
		}
	case domain.PaymentStatusPending:
		// The provider will finalize the charge asynchronously via a webhook.
		// The pending payment is already persisted; leave the subscription active
		// and wait for the webhook.
		return resultTx.Commit(ctx)
	default:
		if recErr := recoverWithClosedTx(); recErr != nil {
			return fmt.Errorf("unexpected charge status %s; recovery failed: %w", chargeResult.Status, recErr)
		}
		return fmt.Errorf("unexpected charge status: %s", chargeResult.Status)
	}

	return resultTx.Commit(ctx)
}

// recoverRenewalFailure resolves an uncertain renewal outcome. It persists any
// provider payment id hint, reloads the payment, and then queries the provider
// for the payment status *outside* of a database transaction. Only after the
// status is known does it open a short transaction to finalize the payment and,
// if necessary, move the subscription to grace.
func (s *BillingService) recoverRenewalFailure(ctx context.Context, paymentID, subscriptionID uuid.UUID, providerPaymentIDHint string, errorCode *string, now time.Time) error {
	// Persist the hint in a dedicated transaction so the status query can use it,
	// even if the payment row is currently locked by another request.
	if providerPaymentIDHint != "" {
		if err := s.persistProviderPaymentIDHint(ctx, paymentID, providerPaymentIDHint); err != nil {
			return err
		}
	}

	payment, err := s.subscriptionPayments.GetByID(ctx, paymentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return s.transitionSubscriptionToGrace(ctx, subscriptionID, now)
		}
		return fmt.Errorf("get payment for recovery: %w", err)
	}

	if payment.Status == domain.PaymentStatusSucceeded || payment.Status != domain.PaymentStatusPending {
		// Nothing more to do for finalized or unexpected statuses.
		return nil
	}

	if payment.ProviderPaymentID == nil {
		return s.markRenewalFailedAndGrace(ctx, paymentID, subscriptionID, errorCode, now)
	}

	status, err := s.provider.Status(ctx, paymentID, *payment.ProviderPaymentID)
	if err != nil || status == domain.PaymentStatusPending {
		statusStr := string(status)
		if statusStr == "" {
			statusStr = "unknown"
		}
		logAttrs := []any{
			slog.String("subscription_id", subscriptionID.String()),
			slog.String("payment_id", paymentID.String()),
			slog.String("status", statusStr),
		}
		if err != nil {
			logAttrs = append(logAttrs, slog.String("error", sanitize.Error(err)))
		}
		s.log.WarnContext(ctx, "provider status unknown during renewal recovery; leaving subscription active and payment pending", logAttrs...)
		return nil
	}

	if status == domain.PaymentStatusFailed {
		return s.markRenewalFailedAndGrace(ctx, paymentID, subscriptionID, errorCode, now)
	}

	if status != domain.PaymentStatusSucceeded {
		s.log.WarnContext(ctx, "unexpected provider status during renewal recovery; leaving subscription active and payment pending",
			slog.String("subscription_id", subscriptionID.String()),
			slog.String("payment_id", paymentID.String()),
			slog.String("status", string(status)))
		return nil
	}

	return s.markRenewalSucceededAndApply(ctx, payment, subscriptionID, now)
}

func (s *BillingService) persistProviderPaymentIDHint(ctx context.Context, paymentID uuid.UUID, providerPaymentIDHint string) error {
	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin provider payment id hint transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	payment, err := s.subscriptionPayments.WithTx(tx).GetByIDForUpdate(ctx, paymentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return fmt.Errorf("get payment for provider payment id hint: %w", err)
	}
	if payment.ProviderPaymentID == nil || *payment.ProviderPaymentID != providerPaymentIDHint {
		if _, updateErr := s.subscriptionPayments.WithTx(tx).UpdateProviderPaymentID(ctx, paymentID, providerPaymentIDHint); updateErr != nil {
			return fmt.Errorf("update provider payment id hint: %w", updateErr)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit provider payment id hint transaction: %w", err)
	}
	return nil
}

func (s *BillingService) markRenewalFailedAndGrace(ctx context.Context, paymentID, subscriptionID uuid.UUID, errorCode *string, now time.Time) error {
	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin recovery transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := s.subscriptionPayments.WithTx(tx).MarkFailed(ctx, paymentID, errorCode, now); err != nil {
		return fmt.Errorf("mark payment failed in recovery: %w", err)
	}

	sub, err := s.subscriptions.WithTx(tx).GetByIDForUpdate(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("get subscription for recovery: %w", err)
	}
	s.transitionToGrace(&sub, now)
	if err := s.subscriptions.WithTx(tx).Update(ctx, sub); err != nil {
		return fmt.Errorf("transition subscription to grace in recovery: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit recovery transaction: %w", err)
	}

	s.log.ErrorContext(ctx, "recovered from renewal failure: subscription moved to grace",
		slog.String("subscription_id", subscriptionID.String()),
		slog.String("payment_id", paymentID.String()))
	return nil
}

func (s *BillingService) markRenewalSucceededAndApply(ctx context.Context, payment domain.SubscriptionPayment, subscriptionID uuid.UUID, now time.Time) error {
	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin recovery transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := s.subscriptionPayments.WithTx(tx).MarkSucceeded(ctx, payment.ID, now); err != nil {
		return fmt.Errorf("mark payment succeeded in recovery: %w", err)
	}
	payment.Status = domain.PaymentStatusSucceeded
	payment.UpdatedAt = now
	payment.SucceededAt = &now
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit recovery transaction: %w", err)
	}

	sub, renewalTariff, oldTariffID, ok := s.applySubscriptionRenewalBestEffort(ctx, subscriptionID, payment, now)
	if ok && oldTariffID != sub.TariffID {
		s.archiveExcessPropertiesBestEffort(ctx, sub.UserID, renewalTariff.ActivePropertyLimit)
	}
	return nil
}

func (s *BillingService) transitionSubscriptionToGrace(ctx context.Context, subscriptionID uuid.UUID, now time.Time) error {
	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin recovery transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	sub, err := s.subscriptions.WithTx(tx).GetByIDForUpdate(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("get subscription for recovery: %w", err)
	}
	s.transitionToGrace(&sub, now)
	if err := s.subscriptions.WithTx(tx).Update(ctx, sub); err != nil {
		return fmt.Errorf("transition subscription to grace in recovery: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit recovery transaction: %w", err)
	}

	s.log.ErrorContext(ctx, "recovered from renewal failure: subscription moved to grace",
		slog.String("subscription_id", subscriptionID.String()))
	return nil
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

func (s *BillingService) applyRenewalChanges(ctx context.Context, tx transaction.Tx, sub domain.Subscription, payment domain.SubscriptionPayment, now time.Time) (domain.Subscription, domain.Tariff, error) {
	renewalTariff, err := s.tariffs.WithTx(tx).GetByID(ctx, payment.TariffID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.Subscription{}, domain.Tariff{}, ErrTariffNotFound
		}
		return domain.Subscription{}, domain.Tariff{}, fmt.Errorf("get renewal tariff: %w", err)
	}

	if sub.TariffID == renewalTariff.ID {
		if err := sub.ApplyRenewal(payment.Period, now); err != nil {
			return domain.Subscription{}, domain.Tariff{}, fmt.Errorf("apply renewal: %w", err)
		}
	} else {
		currentTariff, err := s.tariffs.WithTx(tx).GetByID(ctx, sub.TariffID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return domain.Subscription{}, domain.Tariff{}, ErrTariffNotFound
			}
			return domain.Subscription{}, domain.Tariff{}, fmt.Errorf("get current tariff for change: %w", err)
		}
		if err := sub.ApplyTariffChange(currentTariff, renewalTariff, payment.Period, now); err != nil {
			return domain.Subscription{}, domain.Tariff{}, fmt.Errorf("apply tariff change: %w", err)
		}
	}

	if err := s.subscriptions.WithTx(tx).Update(ctx, sub); err != nil {
		return domain.Subscription{}, domain.Tariff{}, fmt.Errorf("update subscription after renewal: %w", err)
	}
	return sub, renewalTariff, nil
}

// applySubscriptionRenewalBestEffort applies the subscription side of a
// successful payment in a separate transaction. This keeps MarkSucceeded in the
// critical transaction path: if the renewal/tariff change fails, the payment
// stays succeeded and the error is logged for manual review.
func (s *BillingService) applySubscriptionRenewalBestEffort(ctx context.Context, subscriptionID uuid.UUID, payment domain.SubscriptionPayment, now time.Time) (domain.Subscription, domain.Tariff, uuid.UUID, bool) {
	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to begin transaction for best-effort subscription renewal",
			slog.String("subscription_id", subscriptionID.String()),
			slog.String("payment_id", payment.ID.String()),
			slog.String("error", sanitize.Error(err)))
		return domain.Subscription{}, domain.Tariff{}, uuid.UUID{}, false
	}
	defer func() { _ = tx.Rollback(ctx) }()

	sub, err := s.subscriptions.WithTx(tx).GetByIDForUpdate(ctx, subscriptionID)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to load subscription for best-effort renewal",
			slog.String("subscription_id", subscriptionID.String()),
			slog.String("payment_id", payment.ID.String()),
			slog.String("error", sanitize.Error(err)))
		return domain.Subscription{}, domain.Tariff{}, uuid.UUID{}, false
	}
	oldTariffID := sub.TariffID

	// Idempotency: if the subscription already reflects this payment, skip.
	if s.isSubscriptionRenewalApplied(sub, payment) {
		return sub, domain.Tariff{}, oldTariffID, true
	}

	if payment.PaymentMethodID != nil {
		sub.ActivePaymentMethodID = payment.PaymentMethodID
	}

	sub, renewalTariff, err := s.applyRenewalChanges(ctx, tx, sub, payment, now)
	if err != nil {
		s.log.ErrorContext(ctx, "best-effort subscription renewal failed",
			slog.String("subscription_id", subscriptionID.String()),
			slog.String("payment_id", payment.ID.String()),
			slog.String("error", sanitize.Error(err)))
		return domain.Subscription{}, domain.Tariff{}, uuid.UUID{}, false
	}

	if err := tx.Commit(ctx); err != nil {
		s.log.ErrorContext(ctx, "failed to commit best-effort subscription renewal transaction",
			slog.String("subscription_id", subscriptionID.String()),
			slog.String("payment_id", payment.ID.String()),
			slog.String("error", sanitize.Error(err)))
		return domain.Subscription{}, domain.Tariff{}, uuid.UUID{}, false
	}

	return sub, renewalTariff, oldTariffID, true
}

// applySubscriptionRenewalAndArchive runs the best-effort subscription renewal
// and then archives excess properties when the tariff changed to a lower limit.
func (s *BillingService) applySubscriptionRenewalAndArchive(ctx context.Context, payment domain.SubscriptionPayment) {
	now := s.clock.Now().UTC()
	sub, renewalTariff, oldTariffID, ok := s.applySubscriptionRenewalBestEffort(ctx, payment.SubscriptionID, payment, now)
	if !ok {
		return
	}
	// Archiving is only relevant when the tariff changed, because only then can
	// the property limit decrease.
	if oldTariffID == sub.TariffID {
		return
	}
	s.archiveExcessPropertiesBestEffort(ctx, sub.UserID, renewalTariff.ActivePropertyLimit)
}

// isSubscriptionRenewalApplied reports whether the subscription already reflects
// the given successful payment. It is used to make best-effort renewal idempotent.
func (s *BillingService) isSubscriptionRenewalApplied(sub domain.Subscription, payment domain.SubscriptionPayment) bool {
	if sub.TariffID != payment.TariffID {
		return false
	}
	base := payment.SucceededAt
	if base == nil {
		// Backward compatibility for payments created before the succeeded_at
		// column was introduced; use creation time as a conservative fallback.
		base = &payment.CreatedAt
	}
	expectedValidUntil := base.AddDate(0, 1, 0)
	if payment.Period == domain.PeriodYear {
		expectedValidUntil = base.AddDate(1, 0, 0)
	}
	return sub.ValidUntil != nil && !sub.ValidUntil.Before(expectedValidUntil)
}

func (s *BillingService) archiveExcessPropertiesBestEffort(ctx context.Context, userID uuid.UUID, limit int) {
	if s.propertyArchiver == nil {
		return
	}

	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to begin transaction for best-effort property archiving",
			slog.String("user_id", userID.String()),
			slog.String("error", sanitize.Error(err)))
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := s.propertyArchiver.ArchiveExcessProperties(ctx, tx, userID, limit); err != nil {
		s.log.ErrorContext(ctx, "best-effort property archiving failed",
			slog.String("user_id", userID.String()),
			slog.String("error", sanitize.Error(err)))
		return
	}

	if err := tx.Commit(ctx); err != nil {
		s.log.ErrorContext(ctx, "failed to commit best-effort property archiving transaction",
			slog.String("user_id", userID.String()),
			slog.String("error", sanitize.Error(err)))
	}
}

func (s *BillingService) applyFreeRenewalOrDowngrade(ctx context.Context, tx transaction.Tx, sub *domain.Subscription, renewalTariff domain.Tariff, period domain.SubscriptionPeriod, now time.Time) error {
	// The free basic tariff has no validity period and cannot be auto-renewed.
	if renewalTariff.Name == domain.TariffBasic {
		applyBasicDowngrade(sub, renewalTariff.ID)
		if err := s.subscriptions.WithTx(tx).Update(ctx, *sub); err != nil {
			return fmt.Errorf("update subscription after free downgrade to basic: %w", err)
		}
		if s.propertyArchiver != nil {
			if err := s.propertyArchiver.ArchiveExcessProperties(ctx, tx, sub.UserID, renewalTariff.ActivePropertyLimit); err != nil {
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
			if err := s.propertyArchiver.ArchiveExcessProperties(ctx, tx, sub.UserID, renewalTariff.ActivePropertyLimit); err != nil {
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
	// Never shorten an already-paid validity period. This protects against
	// races or failed renewals overwriting a future valid_until.
	if sub.ValidUntil == nil || graceUntil.After(*sub.ValidUntil) {
		sub.ValidUntil = &graceUntil
	}
}

// expireNonRenewingSubscription downgrades an expired subscription to the free
// basic tariff. It handles both active non-renewing subscriptions and cancelled
// subscriptions whose retained validity period has ended.
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

	if sub.ValidUntil == nil || sub.ValidUntil.After(now) {
		return nil
	}
	if sub.Status != domain.SubscriptionStatusActive && sub.Status != domain.SubscriptionStatusCancelled {
		return nil
	}
	if sub.Status == domain.SubscriptionStatusActive && sub.AutoRenewEnabled {
		return nil
	}

	applyBasicDowngrade(&sub, basicTariff.ID)
	if err := s.subscriptions.WithTx(tx).Update(ctx, sub); err != nil {
		return fmt.Errorf("update subscription after non-renewing expiry: %w", err)
	}

	if s.propertyArchiver != nil {
		if err := s.propertyArchiver.ArchiveExcessProperties(ctx, tx, sub.UserID, basicTariff.ActivePropertyLimit); err != nil {
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
					slog.String("error", sanitize.Error(err)))
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
					slog.String("error", sanitize.Error(err)))
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
		if err := s.propertyArchiver.ArchiveExcessProperties(ctx, tx, sub.UserID, basicTariff.ActivePropertyLimit); err != nil {
			return fmt.Errorf("archive excess properties after grace downgrade: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// ProcessPendingUpgradePayments queries the provider for pending upgrade
// payments that have been stale for longer than the configured threshold and
// finalizes them based on the provider status. Returns the number of payments
// that were checked with the provider.
func (s *BillingService) ProcessPendingUpgradePayments(ctx context.Context, now time.Time) (int, error) {
	processed := 0
	createdBefore := now.Add(-pendingUpgradeStalenessThreshold).UTC()

	for {
		payments, err := s.subscriptionPayments.ListPendingUpgradePayments(ctx, createdBefore, renewalBatchSize)
		if err != nil {
			return processed, fmt.Errorf("list pending upgrade payments: %w", err)
		}
		if len(payments) == 0 {
			break
		}

		for _, payment := range payments {
			if payment.ProviderPaymentID == nil || *payment.ProviderPaymentID == "" {
				continue
			}

			status, statusErr := s.provider.Status(ctx, payment.ID, *payment.ProviderPaymentID)
			processed++

			if statusErr != nil {
				s.log.WarnContext(ctx, "failed to query provider status for pending upgrade payment",
					slog.String("payment_id", payment.ID.String()),
					slog.String("error", sanitize.Error(statusErr)))
				continue
			}

			switch status {
			case domain.PaymentStatusSucceeded:
				if err := s.markRenewalSucceededAndApply(ctx, payment, payment.SubscriptionID, now.UTC()); err != nil {
					s.log.ErrorContext(ctx, "failed to apply succeeded upgrade payment",
						slog.String("payment_id", payment.ID.String()),
						slog.String("error", sanitize.Error(err)))
				}
			case domain.PaymentStatusFailed:
				s.markPaymentFailedBestEffort(ctx, payment.ID, now.UTC())
			case domain.PaymentStatusPending:
				// Provider has not finalized the payment yet; leave it pending.
			default:
				s.log.WarnContext(ctx, "unexpected provider status for pending upgrade payment",
					slog.String("payment_id", payment.ID.String()),
					slog.String("status", string(status)))
			}
		}

		if len(payments) < renewalBatchSize {
			break
		}
	}

	return processed, nil
}

// SyncPendingPayment queries the provider for the current status of a single
// pending subscription payment and finalizes it based on the response.
//
// Succeeded payments are marked as such and the subscription renewal/tariff
// change is applied best-effort afterwards. Because T-Kassa GetState does not
// return card tokens, a synthetic WebhookPayload without card data is used;
// no new payment method is saved on this path.
//
// Failed payments are marked as failed. If the payment was for the current
// subscription tariff (a renewal), the subscription is moved to a grace period.
// Upgrade payments that fail leave the subscription on its current tariff.
//
// Payments that are not in pending status cannot be synced and return an
// invalid-status error.
func (s *BillingService) SyncPendingPayment(ctx context.Context, paymentID uuid.UUID) error {
	payment, err := s.subscriptionPayments.GetByID(ctx, paymentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrPaymentNotFound
		}
		return fmt.Errorf("get payment: %w", err)
	}

	if payment.Status != domain.PaymentStatusPending {
		return fmt.Errorf("%w: cannot sync payment with status %s", domain.ErrInvalidPaymentStatus, payment.Status)
	}
	if payment.ProviderPaymentID == nil || *payment.ProviderPaymentID == "" {
		return fmt.Errorf("%w: payment has no provider payment id", domain.ErrInvalidPaymentStatus)
	}

	status, err := s.provider.Status(ctx, paymentID, *payment.ProviderPaymentID)
	if err != nil {
		return fmt.Errorf("provider status: %w", err)
	}

	switch status {
	case domain.PaymentStatusPending:
		// Nothing to finalize. The provider.Status contract only returns the
		// status, so a missing provider payment id cannot be recovered here.
		return nil
	case domain.PaymentStatusSucceeded, domain.PaymentStatusFailed, domain.PaymentStatusRefunded, domain.PaymentStatusPartialRefunded:
		return s.finalizeSyncedPayment(ctx, payment, status)
	default:
		return fmt.Errorf("unexpected provider status: %s", status)
	}
}

func (s *BillingService) finalizeSyncedPayment(ctx context.Context, payment domain.SubscriptionPayment, status domain.PaymentStatus) error {
	tx, err := s.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	payment, err = s.subscriptionPayments.WithTx(tx).GetByIDForUpdate(ctx, payment.ID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrPaymentNotFound
		}
		return fmt.Errorf("get payment for update: %w", err)
	}

	if payment.Status != domain.PaymentStatusPending {
		// A concurrent webhook or another sync already finalized the payment.
		return nil
	}

	payload := WebhookPayload{
		ProviderPaymentID: *payment.ProviderPaymentID,
		InternalPaymentID: payment.ID,
		Status:            status,
		AmountKopecks:     payment.AmountKopecks,
	}

	if err := s.applyPaymentResult(ctx, tx, &payment, payload); err != nil {
		return err
	}

	switch status {
	case domain.PaymentStatusFailed:
		sub, err := s.subscriptions.WithTx(tx).GetByIDForUpdate(ctx, payment.SubscriptionID)
		if err != nil {
			return fmt.Errorf("get subscription for failed payment sync: %w", err)
		}
		if sub.TariffID == payment.TariffID {
			s.transitionToGrace(&sub, s.clock.Now().UTC())
			if err := s.subscriptions.WithTx(tx).Update(ctx, sub); err != nil {
				return fmt.Errorf("transition subscription to grace after failed payment sync: %w", err)
			}
		}

	case domain.PaymentStatusRefunded, domain.PaymentStatusPartialRefunded:
		if err := s.applyRefundToSubscription(ctx, tx, payment.SubscriptionID); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit sync payment transaction: %w", err)
	}

	if status == domain.PaymentStatusSucceeded {
		s.applySubscriptionRenewalAndArchive(ctx, payment)
	}

	return nil
}

// ReconcilePendingPayments checks all pending subscription payments that have
// been stuck longer than the staleness threshold with the provider in batches
// and finalizes them via SyncPendingPayment. Returns the number of payments
// successfully synced.
func (s *BillingService) ReconcilePendingPayments(ctx context.Context, now time.Time) (int, error) {
	processed := 0
	createdBefore := now.Add(-pendingPaymentStalenessThreshold).UTC()

	for {
		payments, err := s.subscriptionPayments.ListPendingPayments(ctx, createdBefore, renewalBatchSize)
		if err != nil {
			return processed, fmt.Errorf("list pending payments: %w", err)
		}
		if len(payments) == 0 {
			break
		}

		for _, payment := range payments {
			if err := s.SyncPendingPayment(ctx, payment.ID); err != nil {
				s.log.ErrorContext(ctx, "failed to sync pending payment",
					slog.String("payment_id", payment.ID.String()),
					slog.String("error", sanitize.Error(err)))
				continue
			}
			processed++
		}

		if len(payments) < renewalBatchSize {
			break
		}
	}

	return processed, nil
}
