package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	auditdomain "github.com/nambers/arenda-planform/apps/backend/internal/audit/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
)

// The admin payment operations of issue #254: the three-phase refund saga, the
// manual provider sync and the admin payment views. Everything lives on
// PaymentService — the owner of payment state transitions — beside the webhook
// finalization it shares its seams with.

// refundActor identifies who applied a refund for the transition log and the
// audit record: the admin flow passes the acting admin, the webhook and the
// reconciliation worker pass the system. The audit actor derives from the
// initiator (auditRoleOfInitiator), so the two vocabularies cannot disagree.
type refundActor struct {
	initiator   domain.TransitionInitiator
	initiatorID *uuid.UUID
}

// adminRefundActor attributes a refund to the acting admin.
func adminRefundActor(adminID uuid.UUID) refundActor {
	return refundActor{
		initiator:   domain.InitiatorAdmin,
		initiatorID: &adminID,
	}
}

// systemRefundActor attributes a refund to the platform itself — the refund
// webhook and the reconciliation worker.
func systemRefundActor() refundActor {
	return refundActor{
		initiator: domain.InitiatorSystem,
	}
}

// runRefundTx runs work like runInTx plus the lifecycle bridges bound to the
// transaction — the transaction shape of every refund application, because the
// refund downgrades the subscription and the excess archiving belongs to the
// same commit as the payment change.
func (s *PaymentService) runRefundTx(ctx context.Context, work func(*txStores) error) error {
	return s.runInTxWithBridges(ctx, s.archiverSource, s.slotSource, work)
}

// RefundPayment refunds a subscription payment in full through the provider
// (issue #254, ADR 0037) as a three-phase saga:
//
//  1. Reserve — one transaction locks the payment row and moves it into the
//     internal refunding state; a concurrent refund loses the reservation race
//     and never reaches the provider, so a payment is refunded at most once.
//  2. Provider call — outside any transaction, so no row lock spans the
//     external request. A definitive refusal or a non-refund answer rolls the
//     reservation back in a short compensation transaction; an in-flight or
//     uncertain outcome keeps it under the сторож of the reconciliation
//     worker (ReconcileStaleRefunds).
//  3. Finalize — one transaction with the lifecycle bridges marks the payment
//     refunded and downgrades the subscription to the basic tariff with its
//     excess properties archived, the transition-log entry and the audit
//     record attributing everything to the acting admin.
func (s *PaymentService) RefundPayment(ctx context.Context, adminID, paymentID uuid.UUID) error {
	reservation, err := s.reserveRefund(ctx, paymentID)
	if err != nil {
		return err
	}

	result, err := s.provider.RefundPayment(ctx, RefundRequest{
		PaymentID:         paymentID,
		ProviderPaymentID: reservation.providerPaymentID,
		AmountKopecks:     reservation.amountKopecks,
	})
	if err != nil {
		if errors.Is(err, ErrProviderDuplicateOperation) {
			// A network duplicate of a refund that may already have happened:
			// the provider's status is the source of truth (ADR 0010), so
			// resolve from it instead of failing or reverting blindly.
			return s.resolveDuplicateRefund(ctx, paymentID, reservation.prevStatus, adminID)
		}
		s.log.ErrorContext(ctx, "provider refund failed",
			slog.String("payment_id", paymentID.String()),
			slog.String("error", sanitize.Error(err)))
		s.revertRefundReservationBestEffort(ctx, paymentID, reservation.prevStatus)
		return fmt.Errorf("provider refund: %w", err)
	}

	switch result.Status {
	case domain.PaymentStatusRefunded:
		if result.RefundedAmountKopecks != reservation.amountKopecks {
			// The system always refunds the full amount, so a provider
			// answering with a smaller refunded amount is an anomaly: keep the
			// reservation for manual review instead of recording a full refund
			// that did not happen (the reconciliation worker still resolves
			// the payment from the provider status later).
			s.log.WarnContext(ctx, "provider refunded less than the full amount; keeping the reservation for review",
				slog.String("payment_id", paymentID.String()),
				slog.Int64("requested_amount_kopecks", reservation.amountKopecks),
				slog.Int64("refunded_amount_kopecks", result.RefundedAmountKopecks))
			return nil
		}
		return s.finalizeRefund(ctx, paymentID, adminRefundActor(adminID))

	case domain.PaymentStatusRefunding:
		// The provider accepted the refund but has not settled it yet. The
		// reservation stays: reverting would cancel a refund in flight, and
		// the reconciliation worker finalizes or reverts from the provider
		// state once it settles.
		s.log.InfoContext(ctx, "provider accepted the refund; waiting for settlement",
			slog.String("payment_id", paymentID.String()))
		return nil

	default:
		s.log.ErrorContext(ctx, "provider refund returned a non-refund status",
			slog.String("payment_id", paymentID.String()),
			slog.String("provider_status", string(result.Status)))
		s.revertRefundReservationBestEffort(ctx, paymentID, reservation.prevStatus)
		return fmt.Errorf("%w: provider refund returned status %q", domain.ErrInvalidPaymentStatus, result.Status)
	}
}

// refundReservation is the phase-1 outcome: everything the provider call and
// the compensation steps need about the reserved payment.
type refundReservation struct {
	amountKopecks     int64
	providerPaymentID string
	prevStatus        domain.PaymentStatus
}

// reserveRefund runs the reservation transaction of the refund saga: it locks
// the payment, rejects everything a full refund cannot start from (terminal
// states, an existing reservation, a payment the provider does not know) and
// atomically moves it into refunding.
func (s *PaymentService) reserveRefund(ctx context.Context, paymentID uuid.UUID) (refundReservation, error) {
	var reservation refundReservation
	err := s.runInTx(ctx, func(stores *txStores) error {
		payment, err := stores.paymentForUpdate(ctx, paymentID)
		if err != nil {
			return err
		}
		if !payment.HasProviderReference() {
			// Without a provider reference there is nothing to refund at the
			// provider — the payment was never initiated there.
			return fmt.Errorf("%w: payment %s has no provider reference", domain.ErrInvalidPaymentStatus, paymentID)
		}
		prevStatus := payment.Status
		if err := payment.BeginRefund(s.clock.Now().UTC()); err != nil {
			return fmt.Errorf("%w: cannot refund payment with status %s", domain.ErrInvalidPaymentStatus, payment.Status)
		}
		if err := stores.payments.Update(ctx, payment); err != nil {
			return fmt.Errorf("reserve payment for refund: %w", err)
		}
		reservation = refundReservation{
			amountKopecks:     payment.AmountKopecks,
			providerPaymentID: *payment.ProviderPaymentID,
			// The pre-reservation status, captured before the mutation: the
			// compensation step restores exactly it.
			prevStatus: prevStatus,
		}
		return nil
	})
	if err != nil {
		return refundReservation{}, err
	}
	return reservation, nil
}

// revertRefundReservation rolls the refunding reservation back to the previous
// status in its own transaction — the compensation step of the saga. A payment
// that left refunding concurrently (another flow finalized or reverted it)
// keeps its persisted state.
func (s *PaymentService) revertRefundReservation(ctx context.Context, paymentID uuid.UUID, prev domain.PaymentStatus) error {
	return s.runInTx(ctx, func(stores *txStores) error {
		payment, err := stores.paymentForUpdate(ctx, paymentID)
		if err != nil {
			return err
		}
		if err := payment.RevertRefundReservation(prev, s.clock.Now().UTC()); err != nil {
			if errors.Is(err, domain.ErrInvalidPaymentStatus) {
				// Another flow already resolved the reservation; the
				// persisted state wins.
				return nil
			}
			return err
		}
		if err := stores.payments.Update(ctx, payment); err != nil {
			return fmt.Errorf("revert refund reservation: %w", err)
		}
		return nil
	})
}

// revertRefundReservationBestEffort runs the compensation and swallows its
// failure: the caller is already returning the provider error, and a failed
// revert only leaves the payment reserved for the reconciliation worker.
func (s *PaymentService) revertRefundReservationBestEffort(ctx context.Context, paymentID uuid.UUID, prev domain.PaymentStatus) {
	if err := s.revertRefundReservation(ctx, paymentID, prev); err != nil {
		s.log.ErrorContext(ctx, "failed to revert the refund reservation; leaving it to the reconciliation worker",
			slog.String("payment_id", paymentID.String()),
			slog.String("prev_status", string(prev)),
			slog.String("error", sanitize.Error(err)))
	}
}

// resolveDuplicateRefund handles a provider duplicate-operation answer: the
// refund may already have happened while the provider reports the request as
// repeated. The provider's current status decides — refunded finalizes the
// refund attributed to the admin whose request reached the provider, a
// still-captured charge reverts the reservation to the payment's previous
// status, anything else keeps the reservation for the reconciliation worker.
func (s *PaymentService) resolveDuplicateRefund(ctx context.Context, paymentID uuid.UUID, prev domain.PaymentStatus, adminID uuid.UUID) error {
	status, err := s.providerStatus(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("provider status after duplicate refund: %w", err)
	}
	switch status.Status {
	case domain.PaymentStatusRefunded:
		return s.finalizeRefund(ctx, paymentID, adminRefundActor(adminID))
	case domain.PaymentStatusSucceeded:
		s.revertRefundReservationBestEffort(ctx, paymentID, prev)
		return nil
	default:
		// The provider has not settled the duplicated refund; the reservation
		// stays under the сторож of the reconciliation worker.
		return nil
	}
}

// finalizeRefund runs the finalizing transaction of the refund saga: it locks
// the payment, moves it to refunded (a payment another flow finalized first is
// an idempotent no-op — the persisted state wins) and applies the subscription
// effects of the refund with the given actor's attribution.
func (s *PaymentService) finalizeRefund(ctx context.Context, paymentID uuid.UUID, actor refundActor) error {
	return s.runRefundTx(ctx, func(stores *txStores) error {
		payment, err := stores.paymentForUpdate(ctx, paymentID)
		if err != nil {
			return err
		}
		switch payment.Status {
		case domain.PaymentStatusRefunded:
			return nil // another flow finalized first
		case domain.PaymentStatusRefunding, domain.PaymentStatusSucceeded, domain.PaymentStatusPending:
			// The reservation of this saga, or a state a concurrent flow
			// restored it to — both finalize from the provider-confirmed
			// refund.
		default:
			return fmt.Errorf("%w: payment status changed to %s during refund", domain.ErrInvalidPaymentStatus, payment.Status)
		}
		return s.applyRefundedPayment(ctx, stores, payment, s.clock.Now().UTC(), actor)
	})
}

// applyRefundedPayment marks the payment refunded and applies the
// subscription effects of the refund inside the caller's transaction: the
// subscription falls to the basic tariff (the paid time the payment bought is
// returned), the excess properties are archived and the recipient slots
// enforced, the transition log records the downgrade with the refund reason
// and the audit log records the refunded payment — both with the actor's
// attribution (issue #254).
func (s *PaymentService) applyRefundedPayment(ctx context.Context, stores *txStores, payment domain.SubscriptionPayment, now time.Time, actor refundActor) error {
	if err := payment.MarkRefunded(now); err != nil {
		return err
	}
	if err := stores.payments.Update(ctx, payment); err != nil {
		return fmt.Errorf("mark payment refunded: %w", err)
	}

	sub, err := stores.subscriptionForUpdate(ctx, payment.UserID)
	if err != nil {
		return err
	}
	basicTariff, err := stores.tariffs.GetByName(ctx, domain.TariffBasic)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return fmt.Errorf("get basic tariff for refund: %w", ErrTariffNotFound)
		}
		return fmt.Errorf("get basic tariff for refund: %w", err)
	}
	if _, err := stores.applyTransition(ctx, &sub,
		func(s *domain.Subscription) error { s.DowngradeToBasic(basicTariff.ID); return nil },
		transitionSpec{
			reason:      domain.TransitionReasonRefunded,
			initiator:   actor.initiator,
			initiatorID: actor.initiatorID,
			paymentID:   new(payment.ID),
		},
	); err != nil {
		return err
	}

	if err := stores.enforceTariffLimit(ctx, sub.UserID, basicTariff.ActivePropertyLimit, triggerRefund); err != nil {
		return fmt.Errorf("enforce tariff limit after refund: %w", err)
	}

	if err := stores.audit.Record(ctx, auditdomain.Entry{
		ActorID:    actor.initiatorID,
		ActorRole:  auditRoleOfInitiator(actor.initiator),
		Action:     auditdomain.ActionSubscriptionPaymentRefunded,
		EntityType: auditdomain.EntitySubscriptionPayment,
		EntityID:   &payment.ID,
		Context:    map[string]any{"payment_id": payment.ID, "provider": string(s.provider.Name()), "amount_kopecks": payment.AmountKopecks},
	}); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}
	return nil
}

// ResolveRefundingFromStatus resolves a payment stuck in the refunding
// reservation from a provider-confirmed status — the shared seam of the
// reconciliation worker (its PaymentLifecycle port) and the admin sync
// (issue #254). A refunded provider payment finalizes the refund with its
// subscription effects (attributed to the system — no admin request is known
// to be in flight), a still-captured charge reverts the reservation to the
// payment's previous status (a pending one re-enters the pending-payment
// reconciliation). It reports whether the reservation was actually resolved;
// everything else waits for the provider to settle.
func (s *PaymentService) ResolveRefundingFromStatus(ctx context.Context, payment domain.SubscriptionPayment, status PaymentStatusResult) (bool, error) {
	switch status.Status {
	case domain.PaymentStatusRefunded:
		return true, s.finalizeRefund(ctx, payment.ID, systemRefundActor())
	case domain.PaymentStatusSucceeded:
		prev := payment.StatusBeforeRefundReservation()
		if err := s.revertRefundReservation(ctx, payment.ID, prev); err != nil {
			return false, err
		}
		s.log.InfoContext(ctx, "stuck refund reverted; the charge is still captured at the provider",
			slog.String("payment_id", payment.ID.String()),
			slog.String("restored_status", string(prev)))
		return true, nil
	default:
		return false, nil
	}
}

// SyncPayment reconciles one payment with the provider on an admin's request
// (issue #254): the provider's current status — the source of truth
// (ADR 0010) — is applied through the same synchronous paths the webhooks and
// the workers use, so a stuck payment resolves without waiting for the next
// worker tick. A provider that has not settled the payment yet changes
// nothing. The sync itself is audited with the acting admin regardless of the
// outcome it resolved to.
func (s *PaymentService) SyncPayment(ctx context.Context, adminID, paymentID uuid.UUID) error {
	payment, err := s.GetPayment(ctx, paymentID)
	if err != nil {
		return err
	}
	if !payment.HasProviderReference() {
		return fmt.Errorf("%w: payment %s has no provider reference", domain.ErrInvalidPaymentStatus, paymentID)
	}

	status, err := s.provider.PaymentStatus(ctx, payment.ID, *payment.ProviderPaymentID)
	if err != nil {
		return fmt.Errorf("provider status: %w", err)
	}
	providerStatus := status.Status

	if payment.Status == domain.PaymentStatusRefunding {
		// A stuck refund reservation resolves the same way the reconciliation
		// worker resolves it: refunded finalizes, a still-captured charge
		// reverts, anything else waits for the provider to settle.
		if _, err := s.ResolveRefundingFromStatus(ctx, payment, status); err != nil {
			return err
		}
	} else {
		switch providerStatus {
		case domain.PaymentStatusSucceeded, domain.PaymentStatusFailed:
			if err := s.ApplyPaymentNotification(ctx, notificationFromStatus(payment, status)); err != nil {
				return err
			}
		case domain.PaymentStatusRefunded:
			// The provider reports the money returned: apply the refund with
			// its subscription effects (idempotent — an already refunded
			// payment is a no-op) unless the local payment never reached a
			// refundable state.
			switch payment.Status {
			case domain.PaymentStatusPending, domain.PaymentStatusSucceeded:
				if err := s.finalizeRefund(ctx, paymentID, systemRefundActor()); err != nil {
					return err
				}
			case domain.PaymentStatusFailed, domain.PaymentStatusRefunded:
				// A failed charge was never captured and an already refunded
				// payment finalized before — nothing to apply.
			case domain.PaymentStatusRefunding:
				// Unreachable here: a refunding payment took the reservation
				// branch above.
			}
		default:
			// Pending (or a transitional provider state): the provider has not
			// settled the payment yet; there is nothing to apply.
		}
	}

	if err := s.runInTx(ctx, func(stores *txStores) error {
		return stores.audit.Record(ctx, auditdomain.Entry{
			ActorID:    &adminID,
			ActorRole:  auditdomain.ActorRoleAdmin,
			Action:     auditdomain.ActionSubscriptionPaymentSynced,
			EntityType: auditdomain.EntitySubscriptionPayment,
			EntityID:   &payment.ID,
			Context: map[string]any{
				"payment_id":      payment.ID,
				"provider":        string(s.provider.Name()),
				"provider_status": string(providerStatus),
				"local_status":    string(payment.Status),
			},
		})
	}); err != nil {
		return fmt.Errorf("record audit: %w", err)
	}
	return nil
}

// providerStatus queries the provider-side status of a payment by its id,
// keeping the caller free of the provider reference plumbing.
func (s *PaymentService) providerStatus(ctx context.Context, paymentID uuid.UUID) (PaymentStatusResult, error) {
	payment, err := s.payments.GetByID(ctx, paymentID)
	if err != nil {
		return PaymentStatusResult{}, err
	}
	return s.provider.PaymentStatus(ctx, payment.ID, *payment.ProviderPaymentID)
}

// notificationFromStatus builds the payment notification that routes a
// provider-confirmed outcome through the synchronous application path the
// webhook flow uses — the shared seam of the admin sync and the
// reconciliation workers.
func notificationFromStatus(payment domain.SubscriptionPayment, status PaymentStatusResult) *PaymentNotification {
	var errorCode *string
	if status.Status == domain.PaymentStatusFailed && status.ErrorCode != "" {
		errorCode = &status.ErrorCode
	}
	return &PaymentNotification{
		InternalPaymentID: payment.ID,
		ProviderPaymentID: *payment.ProviderPaymentID,
		Status:            status.Status,
		ErrorCode:         errorCode,
		AmountKopecks:     payment.AmountKopecks,
	}
}

// adminPaymentSortFields is the sort whitelist of the admin payment listing,
// in API (camelCase) naming — mirroring the SQL CASE arms of
// ListSubscriptionPaymentsAdmin. Keep both sides in sync.
var adminPaymentSortFields = []string{"createdAt", "amountKopecks", "status"}

// normalizeAdminPaymentFilters validates and defaults the listing filters:
// pagination bounds, the sort whitelist and the status vocabularies. An
// unknown value answers ErrInvalidFilter instead of silently matching nothing.
func normalizeAdminPaymentFilters(filters AdminPaymentFilters) (AdminPaymentFilters, error) {
	if filters.Limit <= 0 {
		filters.Limit = 20
	}
	if filters.Limit > 100 {
		filters.Limit = 100
	}
	if filters.Offset < 0 {
		filters.Offset = 0
	}

	sort := strings.TrimSpace(filters.Sort)
	if sort != "" && !slices.Contains(adminPaymentSortFields, sort) {
		return filters, fmt.Errorf("%w: unsupported sort field %q (allowed: %s)", ErrInvalidFilter, sort, strings.Join(adminPaymentSortFields, ", "))
	}
	filters.Sort = sort
	order := strings.TrimSpace(filters.Order)
	if order == "" {
		order = "desc"
	}
	if order != "asc" && order != "desc" {
		return filters, fmt.Errorf("%w: unsupported sort order %q (allowed: asc, desc)", ErrInvalidFilter, order)
	}
	filters.Order = order

	filters.Status = strings.TrimSpace(filters.Status)
	if filters.Status != "" && !validAdminPaymentStatus(filters.Status) {
		return filters, fmt.Errorf("%w: unknown payment status %q", ErrInvalidFilter, filters.Status)
	}
	filters.SubscriptionStatus = strings.TrimSpace(filters.SubscriptionStatus)
	switch filters.SubscriptionStatus {
	case "":
	case string(domain.SubscriptionStatusActive), string(domain.SubscriptionStatusGrace), string(domain.SubscriptionStatusCancelled):
	default:
		return filters, fmt.Errorf("%w: unknown subscription status %q", ErrInvalidFilter, filters.SubscriptionStatus)
	}
	filters.UserPhone = strings.TrimSpace(filters.UserPhone)
	return filters, nil
}

// validAdminPaymentStatus accepts the payment statuses of the API contract,
// including the legacy partial_refunded: no row can carry it any more
// (ADR 0037), so filtering by it simply matches nothing.
func validAdminPaymentStatus(status string) bool {
	switch domain.PaymentStatus(status) {
	case domain.PaymentStatusPending,
		domain.PaymentStatusSucceeded,
		domain.PaymentStatusFailed,
		domain.PaymentStatusRefunded,
		domain.PaymentStatusRefunding:
		return true
	case "partial_refunded":
		return true
	}
	return false
}

// ListAdminPayments returns the cross-user payment listing for the admin
// screen with the total count of the filtered set (issue #254).
func (s *PaymentService) ListAdminPayments(ctx context.Context, filters AdminPaymentFilters) ([]AdminSubscriptionPaymentView, int64, error) {
	if s.adminPayments == nil {
		return nil, 0, errors.New("admin payment listing is not wired")
	}
	filters, err := normalizeAdminPaymentFilters(filters)
	if err != nil {
		return nil, 0, err
	}
	rows, total, err := s.adminPayments.ListAdminPayments(ctx, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("list admin payments: %w", err)
	}
	tariffs, err := s.tariffs.ListAll(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list tariffs: %w", err)
	}
	tariffByID := make(map[uuid.UUID]domain.Tariff, len(tariffs))
	for _, t := range tariffs {
		tariffByID[t.ID] = t
	}
	views := make([]AdminSubscriptionPaymentView, 0, len(rows))
	for _, row := range rows {
		tariff, ok := tariffByID[row.Payment.TariffID]
		if !ok {
			return nil, 0, fmt.Errorf("payment %s references unknown tariff %s", row.Payment.ID, row.Payment.TariffID)
		}
		views = append(views, AdminSubscriptionPaymentView{Payment: row.Payment, Tariff: tariff, UserPhone: row.UserPhone})
	}
	return views, total, nil
}

// GetAdminPayment returns one payment for the admin detail view with the
// payer's phone resolved (issue #254).
func (s *PaymentService) GetAdminPayment(ctx context.Context, paymentID uuid.UUID) (AdminSubscriptionPaymentView, error) {
	if s.adminPayments == nil {
		return AdminSubscriptionPaymentView{}, errors.New("admin payment listing is not wired")
	}
	row, err := s.adminPayments.GetAdminPayment(ctx, paymentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return AdminSubscriptionPaymentView{}, ErrPaymentNotFound
		}
		return AdminSubscriptionPaymentView{}, fmt.Errorf("get admin payment: %w", err)
	}
	tariff, err := s.tariffs.GetByID(ctx, row.Payment.TariffID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return AdminSubscriptionPaymentView{}, fmt.Errorf("payment %s references unknown tariff %s: %w", row.Payment.ID, row.Payment.TariffID, ErrTariffNotFound)
		}
		return AdminSubscriptionPaymentView{}, fmt.Errorf("get payment tariff: %w", err)
	}
	return AdminSubscriptionPaymentView{Payment: row.Payment, Tariff: tariff, UserPhone: row.UserPhone}, nil
}
