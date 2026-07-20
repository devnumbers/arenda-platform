package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// SubscriptionService manages the current user's subscription and tariff changes.
type SubscriptionService struct {
	deps     subscriptionServiceDeps
	provider SubscriptionPaymentProvider
}

// NewSubscriptionService creates a SubscriptionService.
func NewSubscriptionService(deps subscriptionServiceDeps, provider SubscriptionPaymentProvider) *SubscriptionService {
	return &SubscriptionService{deps: deps, provider: provider}
}

// GetSubscription returns the current subscription with its tariff and active payment method.
func (s *SubscriptionService) GetSubscription(ctx context.Context, userID uuid.UUID) (SubscriptionView, error) {
	sub, err := s.deps.subscriptions.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return SubscriptionView{}, ErrSubscriptionNotFound
		}
		return SubscriptionView{}, fmt.Errorf("get subscription: %w", err)
	}

	tariff, err := s.deps.tariffs.GetByID(ctx, sub.TariffID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return SubscriptionView{}, ErrTariffNotFound
		}
		return SubscriptionView{}, fmt.Errorf("get subscription tariff: %w", err)
	}

	view := SubscriptionView{Subscription: sub, Tariff: tariff}

	lastPayment, err := s.deps.subscriptionPayments.GetLastSucceededBySubscriptionID(ctx, sub.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return SubscriptionView{}, fmt.Errorf("get last succeeded payment: %w", err)
	}
	if err == nil {
		period := lastPayment.Period
		view.CurrentPeriod = &period
	}

	if sub.PendingTariffID != nil {
		pending, err := s.deps.tariffs.GetByID(ctx, *sub.PendingTariffID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return SubscriptionView{}, ErrTariffNotFound
			}
			return SubscriptionView{}, fmt.Errorf("get pending tariff: %w", err)
		}
		view.PendingTariff = &pending
	}

	if sub.ActivePaymentMethodID != nil {
		pm, err := s.deps.paymentMethods.GetByID(ctx, *sub.ActivePaymentMethodID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				s.deps.log.WarnContext(ctx, "subscription references missing active payment method",
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

func (s *SubscriptionService) existingUpgradeResponse(ctx context.Context, payment domain.SubscriptionPayment) ChangeTariffResponse {
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
func (s *SubscriptionService) ChangeTariff(ctx context.Context, userID uuid.UUID, req ChangeTariffRequest) (ChangeTariffResponse, error) {
	newTariff, err := s.deps.tariffs.GetByName(ctx, req.TariffName)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ChangeTariffResponse{}, ErrTariffNotFound
		}
		return ChangeTariffResponse{}, fmt.Errorf("get tariff: %w", err)
	}

	tx, err := s.deps.beginner.Begin(ctx)
	if err != nil {
		return ChangeTariffResponse{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txSubscriptions, err := s.deps.subscriptions.WithTx(tx)
	if err != nil {
		return ChangeTariffResponse{}, fmt.Errorf("bind subscriptions transaction: %w", err)
	}
	txTariffs, err := s.deps.tariffs.WithTx(tx)
	if err != nil {
		return ChangeTariffResponse{}, fmt.Errorf("bind tariffs transaction: %w", err)
	}
	txSubscriptionPayments, err := s.deps.subscriptionPayments.WithTx(tx)
	if err != nil {
		return ChangeTariffResponse{}, fmt.Errorf("bind subscription payments transaction: %w", err)
	}

	sub, err := txSubscriptions.GetByUserIDForUpdate(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ChangeTariffResponse{}, ErrSubscriptionNotFound
		}
		return ChangeTariffResponse{}, fmt.Errorf("get subscription: %w", err)
	}

	if sub.Source != domain.SubscriptionSourcePaid {
		return ChangeTariffResponse{}, domain.ErrInvalidSubscriptionState
	}

	now := s.deps.clock.Now().UTC()

	// A same-tariff change is normally rejected, but while the subscription is
	// in grace a same-tariff payment acts as a manual renewal: it goes through
	// the upgrade-style payment flow and the webhook path applies it via
	// ApplyRenewal (see flows.go).
	sameTariffGraceRenewal := false
	if sub.TariffID == newTariff.ID {
		switch {
		case sub.IsInGrace(now):
			sameTariffGraceRenewal = true
		case sub.Status == domain.SubscriptionStatusGrace:
			// The grace period has already expired; the subscription awaits the
			// worker downgrade to basic and can no longer initiate payments.
			return ChangeTariffResponse{}, domain.ErrInvalidSubscriptionState
		default:
			return ChangeTariffResponse{}, ErrAlreadyOnTariff
		}
	}

	currentTariff, err := txTariffs.GetByID(ctx, sub.TariffID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ChangeTariffResponse{}, ErrTariffNotFound
		}
		return ChangeTariffResponse{}, fmt.Errorf("get current tariff: %w", err)
	}

	changeType := domain.ClassifyTariffChange(currentTariff, newTariff)
	if changeType == domain.TariffChangeSame && !sameTariffGraceRenewal {
		return ChangeTariffResponse{}, ErrInvalidTariffChange
	}

	amount := newTariff.MonthlyPriceKopecks
	if req.Period == domain.PeriodYear {
		amount = newTariff.YearlyPriceKopecks
	}

	canInitiate := sub.CanInitiatePayment(now)
	isRecoveryUpgrade := changeType == domain.TariffChangeUpgrade &&
		sub.Status == domain.SubscriptionStatusCancelled
	if !canInitiate && !isRecoveryUpgrade {
		return ChangeTariffResponse{}, domain.ErrInvalidSubscriptionState
	}

	if changeType == domain.TariffChangeUpgrade || sameTariffGraceRenewal {
		// Return an existing pending upgrade payment for the same tariff and
		// period instead of creating a duplicate. The partial unique index on
		// pending payments is the durable backstop for races.
		pending, err := txSubscriptionPayments.ListPendingSubscriptionPaymentsByUserID(ctx, userID)
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
	// Paid and free downgrades are deferred until the end of the paid period;
	// the charge happens when the scheduled change is applied by the worker.
	// See ADR 0008, section 3.
	if err := sub.ScheduleDowngrade(currentTariff, newTariff, req.Period, *sub.ValidUntil); err != nil {
		return ChangeTariffResponse{}, err
	}
	if err := txSubscriptions.Update(ctx, sub); err != nil {
		return ChangeTariffResponse{}, fmt.Errorf("schedule downgrade: %w", err)
	}
	if err := s.deps.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &userID,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionSubscriptionTariffChanged,
		EntityType: auditdomain.EntitySubscription,
		EntityID:   &sub.ID,
		Context:    map[string]any{"from_tariff_id": currentTariff.ID, "to_tariff_id": newTariff.ID},
	}); err != nil {
		return ChangeTariffResponse{}, fmt.Errorf("record audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return ChangeTariffResponse{}, fmt.Errorf("commit schedule downgrade transaction: %w", err)
	}

	return ChangeTariffResponse{}, nil
}

// paymentRedirectTTL bounds how long a user-facing payment form stays
// payable. After the deadline T-Kassa moves the payment to
// DEADLINE_EXPIRED and the user retries via the existing recovery flow.
// See docs/plans/2026-07-13-tkassa-redirect-due-date-design.md.
const paymentRedirectTTL = 15 * time.Minute

func (s *SubscriptionService) changeTariffUpgrade(
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

	now := s.deps.clock.Now().UTC()
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
	txSubscriptionPayments, err := s.deps.subscriptionPayments.WithTx(tx)
	if err != nil {
		return ChangeTariffResponse{}, fmt.Errorf("bind subscription payments transaction: %w", err)
	}
	payment, err = txSubscriptionPayments.Create(ctx, payment)
	if err != nil {
		if errors.Is(err, ErrAlreadyExists) {
			// A concurrent request created the pending payment first. Return the
			// existing one instead of failing.
			pending, listErr := txSubscriptionPayments.ListPendingSubscriptionPaymentsByUserID(ctx, userID)
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
	// The pending payment this transaction commits is the persisted form of the
	// user's tariff-change decision, so the tariff change is audited here. The
	// payment itself is audited by the webhook/sync paths that finalize it.
	if err := s.deps.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &userID,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionSubscriptionTariffChanged,
		EntityType: auditdomain.EntitySubscription,
		EntityID:   &sub.ID,
		Context:    map[string]any{"from_tariff_id": sub.TariffID, "to_tariff_id": newTariff.ID},
	}); err != nil {
		return ChangeTariffResponse{}, fmt.Errorf("record audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return ChangeTariffResponse{}, fmt.Errorf("commit transaction: %w", err)
	}

	notification, successURL, failURL := tkassaCallbackURLs(s.deps.callbackBaseURL, payment.ID)
	initRes, err := s.provider.Init(ctx, InitRequest{
		PaymentID:              payment.ID,
		AmountKopecks:          amount,
		Period:                 period,
		UserID:                 userID,
		CustomerKey:            userID.String(),
		Recurrent:              true,
		OperationInitiatorType: InitiatorTypeCITCC, // parent payment of the CC/COF chain that obtains the RebillId
		NotificationURL:        notification,
		SuccessURL:             successURL,
		FailURL:                failURL,
		RedirectDueDate:        now.Add(paymentRedirectTTL),
		Description:            truncateTkassaDescription(upgradePaymentDescription(newTariff.Name, period)),
	})
	if err != nil {
		markPaymentFailedBestEffort(ctx, markFailedDeps{
			beginner:             s.deps.beginner,
			subscriptionPayments: s.deps.subscriptionPayments,
			log:                  s.deps.log,
		}, payment.ID, now)
		return ChangeTariffResponse{}, sanitize.Wrap(err, "init payment")
	}

	// The provider token is only available after Init succeeds. Persist the
	// provider reference and any newly created payment method atomically in a
	// single transaction so a crash cannot leave a pending payment without a
	// provider reference.
	payment, err = saveProviderInitResult(ctx, saveProviderInitDeps{
		beginner:             s.deps.beginner,
		subscriptionPayments: s.deps.subscriptionPayments,
		paymentMethods:       s.deps.paymentMethods,
		provider:             s.provider,
		log:                  s.deps.log,
	}, payment, initRes, userID, now)
	if err != nil {
		return ChangeTariffResponse{}, err
	}

	return ChangeTariffResponse{
		PaymentID:  payment.ID,
		ConfirmURL: initRes.PaymentURL,
	}, nil
}

// recoverUpgradeProviderReference recovers a provider reference for an existing
// pending upgrade payment whose provider_payment_id was lost after a crash.
// It relies on the provider's idempotent Init to reconstruct the reference.
func (s *SubscriptionService) recoverUpgradeProviderReference(ctx context.Context, payment domain.SubscriptionPayment, newTariff domain.Tariff, amount int64, period domain.SubscriptionPeriod, userID uuid.UUID, now time.Time) (ChangeTariffResponse, error) {
	notification, successURL, failURL := tkassaCallbackURLs(s.deps.callbackBaseURL, payment.ID)
	initRes, err := s.provider.Init(ctx, InitRequest{
		PaymentID:              payment.ID,
		AmountKopecks:          amount,
		Period:                 period,
		UserID:                 userID,
		CustomerKey:            userID.String(),
		Recurrent:              true,
		OperationInitiatorType: InitiatorTypeCITCC, // parent payment of the CC/COF chain that obtains the RebillId
		NotificationURL:        notification,
		SuccessURL:             successURL,
		FailURL:                failURL,
		RedirectDueDate:        now.Add(paymentRedirectTTL),
		Description:            truncateTkassaDescription(upgradePaymentDescription(newTariff.Name, period)),
	})
	if err != nil {
		markPaymentFailedBestEffort(ctx, markFailedDeps{
			beginner:             s.deps.beginner,
			subscriptionPayments: s.deps.subscriptionPayments,
			log:                  s.deps.log,
		}, payment.ID, now)
		return ChangeTariffResponse{}, sanitize.Wrap(err, "recover provider reference")
	}

	payment, err = saveProviderInitResult(ctx, saveProviderInitDeps{
		beginner:             s.deps.beginner,
		subscriptionPayments: s.deps.subscriptionPayments,
		paymentMethods:       s.deps.paymentMethods,
		provider:             s.provider,
		log:                  s.deps.log,
	}, payment, initRes, userID, now)
	if err != nil {
		return ChangeTariffResponse{}, fmt.Errorf("recover provider reference: %w", err)
	}

	return ChangeTariffResponse{
		PaymentID:  payment.ID,
		ConfirmURL: initRes.PaymentURL,
	}, nil
}

// CancelSubscription terminates the paid subscription. The current tariff remains
// valid until valid_until, after which the worker downgrades the subscription to
// basic.
func (s *SubscriptionService) CancelSubscription(ctx context.Context, userID uuid.UUID) error {
	tx, err := s.deps.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txSubscriptions, err := s.deps.subscriptions.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind subscriptions transaction: %w", err)
	}

	sub, err := txSubscriptions.GetByUserIDForUpdate(ctx, userID)
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

	if err := txSubscriptions.Update(ctx, sub); err != nil {
		return fmt.Errorf("update subscription: %w", err)
	}

	if err := s.deps.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &userID,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionSubscriptionCancelled,
		EntityType: auditdomain.EntitySubscription,
		EntityID:   &sub.ID,
	}); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit cancel subscription transaction: %w", err)
	}
	return nil
}

// ToggleAutoRenew enables or disables automatic subscription renewal.
func (s *SubscriptionService) ToggleAutoRenew(ctx context.Context, userID uuid.UUID, enabled bool) error {
	tx, err := s.deps.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txSubscriptions, err := s.deps.subscriptions.WithTx(tx)
	if err != nil {
		return fmt.Errorf("bind subscriptions transaction: %w", err)
	}

	sub, err := txSubscriptions.GetByUserIDForUpdate(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrSubscriptionNotFound
		}
		return fmt.Errorf("get subscription: %w", err)
	}

	if err := sub.SetAutoRenew(enabled); err != nil {
		return err
	}

	if err := txSubscriptions.Update(ctx, sub); err != nil {
		return fmt.Errorf("update subscription: %w", err)
	}

	if err := s.deps.audit.WithTx(tx).Record(ctx, auditdomain.Entry{
		ActorID:    &userID,
		ActorRole:  auditdomain.ActorRoleOwner,
		Action:     auditdomain.ActionSubscriptionAutoRenewToggled,
		EntityType: auditdomain.EntitySubscription,
		EntityID:   &sub.ID,
		Context:    map[string]any{"enabled": enabled},
	}); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit toggle auto-renew transaction: %w", err)
	}
	return nil
}
