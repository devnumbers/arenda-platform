package tkassa

import (
	"fmt"

	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// mapStatus maps T-Kassa payment statuses to the domain status model.
// REVERSED/PARTIAL_REVERSED here come from GetState/webhook: the provider
// reports a reversal of the payment (a hold cancelled before capture, or a
// dispute/chargeback after it) whose outcome for our subscription payment is
// that the money left the merchant, so both map to refunded and the refund
// application seam brings the subscription to the money-consistent state
// (issue #432, ADR 0017 §Polling). A reversal of a never-captured charge is a
// no-op there — a failed payment has nothing to refund. In mapCancelStatus
// the same statuses answer a Cancel call we made ourselves, so they equally
// mean the refund we requested succeeded and map to refunded.
// PARTIAL_REFUNDED maps to refunded: the new domain has no partial-refund
// status (ADR 0037 — refunds are full-amount only), and a provider-side
// partial outcome still ends the refund flow with the payment in its terminal
// refunded state.
func mapStatus(status string) domain.PaymentStatus {
	switch status {
	case statusNew, statusAuthorized, statusAuthorizing, status3DSChecking,
		status3DSChecked, statusConfirming, statusFormShowed, statusAsyncRefunding:
		return domain.PaymentStatusPending
	case statusReversing, statusRefunding:
		// Transitional statuses, waiting for the final one.
		return domain.PaymentStatusPending
	case statusPayChecking, statusConfirmChecking, statusChecking, statusChecked,
		statusCompleting, statusCompleted, statusPreauthorizing, statusProcessing,
		statusUnknown:
		// Transitional or unknown-outcome statuses: the payment result is not
		// final yet, so treat them as pending.
		return domain.PaymentStatusPending
	case statusConfirmed:
		return domain.PaymentStatusSucceeded
	case statusRefunded, statusPartialRefunded, statusReversed, statusPartialReversed:
		return domain.PaymentStatusRefunded
	case statusRejected, statusAuthFail, statusCanceled, statusDeadlineExpired,
		status3DSFailed:
		return domain.PaymentStatusFailed
	default:
		return domain.PaymentStatusPending
	}
}

// mapCancelStatus maps the T-Kassa Cancel response statuses to domain refund
// statuses. REVERSED means the operation was cancelled before completion
// (e.g. a pending/NEW payment), so it is treated as a full refund for our
// domain model. Statuses that definitively leave the charge intact (CONFIRMED)
// or end the payment with no money returned (REJECTED, CANCELED, AUTH_FAIL,
// DEADLINE_EXPIRED, 3DS_FAILED) fail the refund so the saga reverts its
// reservation. Everything else — UNKNOWN, the result-unknown transitional
// statuses (PAY_CHECKING, CONFIRM_CHECKING) and anything outside the
// vocabulary — leaves the refund outcome undetermined: the money may still be
// returning, so the reservation stays under the reconciliation watchdog
// instead of failing the refund (spec #419).
func mapCancelStatus(status string) domain.PaymentStatus {
	switch status {
	case statusRefunded, statusReversed, statusPartialRefunded, statusPartialReversed:
		return domain.PaymentStatusRefunded
	case statusNew, statusAuthorized, statusAuthorizing, status3DSChecking,
		status3DSChecked, statusConfirming, statusFormShowed:
		return domain.PaymentStatusPending
	case statusReversing, statusRefunding, statusAsyncRefunding:
		// The provider accepted the refund but has not settled it yet: the refund
		// is in flight, so keep the internal refunding reservation instead of
		// failing it. ASYNC_REFUNDING is an async-acquiring refund the provider
		// settles in the background.
		return domain.PaymentStatusRefunding
	case statusConfirmed, statusRejected, statusAuthFail, statusCanceled,
		statusDeadlineExpired, status3DSFailed:
		return domain.PaymentStatusFailed
	default:
		return domain.PaymentStatusRefunding
	}
}

// mapAddCardStateStatus maps the eight GetAddCardState response statuses to
// the provider-neutral binding state: the intermediate challenge and
// authorization steps all collapse into pending, because the application
// cannot act on anything finer.
func mapAddCardStateStatus(status string) (application.MethodBindingStatus, error) {
	switch status {
	case statusNew, statusFormShowed, status3DSChecking, status3DSChecked,
		statusAuthorizing, statusAuthorized:
		return application.MethodBindingPending, nil
	case statusCompleted:
		return application.MethodBindingCompleted, nil
	case statusRejected:
		return application.MethodBindingFailed, nil
	default:
		return "", fmt.Errorf("tkassa: unknown add card status %q", status)
	}
}

// T-Kassa payment status constants.
const (
	statusNew             = "NEW"
	statusFormShowed      = "FORM_SHOWED"
	statusAuthorizing     = "AUTHORIZING"
	statusAuthorized      = "AUTHORIZED"
	statusAuthFail        = "AUTH_FAIL"
	statusRejected        = "REJECTED"
	status3DSChecking     = "3DS_CHECKING"
	status3DSChecked      = "3DS_CHECKED"
	status3DSFailed       = "3DS_FAILED"
	statusConfirming      = "CONFIRMING"
	statusConfirmed       = "CONFIRMED"
	statusReversing       = "REVERSING"
	statusReversed        = "REVERSED"
	statusPartialReversed = "PARTIAL_REVERSED"
	statusRefunding       = "REFUNDING"
	statusAsyncRefunding  = "ASYNC_REFUNDING"
	statusRefunded        = "REFUNDED"
	statusPartialRefunded = "PARTIAL_REFUNDED"
	statusDeadlineExpired = "DEADLINE_EXPIRED"
	statusCanceled        = "CANCELED"
	statusPayChecking     = "PAY_CHECKING"
	statusConfirmChecking = "CONFIRM_CHECKING"
	statusChecking        = "CHECKING"
	statusChecked         = "CHECKED"
	statusCompleting      = "COMPLETING"
	statusCompleted       = "COMPLETED"
	statusPreauthorizing  = "PREAUTHORIZING"
	statusProcessing      = "PROCESSING"
	statusUnknown         = "UNKNOWN"
)
