package tkassa

import (
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// TestMapStatus pins the full payment-status mapping, including the deliberate
// decisions: REVERSED and PARTIAL_REVERSED from GetState/webhook are failed
// (ADR 0017 §Polling — a reversal of unclear origin is not a refund for our
// subscription payment), unknown statuses default to pending, and
// PARTIAL_REFUNDED lands in refunded because the new domain has no
// partial-refund status (ADR 0037, issue #246 §3.1).
func TestMapStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status string
		want   domain.PaymentStatus
	}{
		{statusNew, domain.PaymentStatusPending},
		{statusFormShowed, domain.PaymentStatusPending},
		{statusAuthorizing, domain.PaymentStatusPending},
		{statusAuthorized, domain.PaymentStatusPending},
		{status3DSChecking, domain.PaymentStatusPending},
		{status3DSChecked, domain.PaymentStatusPending},
		{statusConfirming, domain.PaymentStatusPending},
		{statusReversing, domain.PaymentStatusPending},
		{statusRefunding, domain.PaymentStatusPending},
		{statusAsyncRefunding, domain.PaymentStatusPending},
		{statusPayChecking, domain.PaymentStatusPending},
		{statusConfirmChecking, domain.PaymentStatusPending},
		{statusChecking, domain.PaymentStatusPending},
		{statusChecked, domain.PaymentStatusPending},
		{statusCompleting, domain.PaymentStatusPending},
		{statusCompleted, domain.PaymentStatusPending},
		{statusPreauthorizing, domain.PaymentStatusPending},
		{statusProcessing, domain.PaymentStatusPending},
		{statusUnknown, domain.PaymentStatusPending},
		{statusUnknown + "_NEW", domain.PaymentStatusPending}, // Unknown defaults to pending.
		{statusConfirmed, domain.PaymentStatusSucceeded},
		{statusRefunded, domain.PaymentStatusRefunded},
		{statusPartialRefunded, domain.PaymentStatusRefunded},
		{statusRejected, domain.PaymentStatusFailed},
		{statusAuthFail, domain.PaymentStatusFailed},
		{statusDeadlineExpired, domain.PaymentStatusFailed},
		{statusCanceled, domain.PaymentStatusFailed},
		{statusReversed, domain.PaymentStatusFailed},
		{statusPartialReversed, domain.PaymentStatusFailed},
		{status3DSFailed, domain.PaymentStatusFailed},
	}
	for _, tt := range tests {
		if got := mapStatus(tt.status); got != tt.want {
			t.Errorf("mapStatus(%q) = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestMapCancelStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status string
		want   domain.PaymentStatus
	}{
		{statusRefunded, domain.PaymentStatusRefunded},
		{statusReversed, domain.PaymentStatusRefunded},
		{statusPartialRefunded, domain.PaymentStatusRefunded},
		{statusPartialReversed, domain.PaymentStatusRefunded},
		{statusNew, domain.PaymentStatusPending},
		{statusAuthorized, domain.PaymentStatusPending},
		{statusAuthorizing, domain.PaymentStatusPending},
		{status3DSChecking, domain.PaymentStatusPending},
		{status3DSChecked, domain.PaymentStatusPending},
		{statusConfirming, domain.PaymentStatusPending},
		{statusFormShowed, domain.PaymentStatusPending},
		{statusReversing, domain.PaymentStatusRefunding},
		{statusRefunding, domain.PaymentStatusRefunding},
		{statusAsyncRefunding, domain.PaymentStatusRefunding},
		// Definitive dead ends: the charge is intact or the payment died with
		// no money returned — the saga reverts its reservation.
		{statusConfirmed, domain.PaymentStatusFailed},
		{statusRejected, domain.PaymentStatusFailed},
		{statusAuthFail, domain.PaymentStatusFailed},
		{statusCanceled, domain.PaymentStatusFailed},
		{statusDeadlineExpired, domain.PaymentStatusFailed},
		{status3DSFailed, domain.PaymentStatusFailed},
		// Undetermined outcomes keep the refunding reservation under the
		// reconciliation watchdog instead of failing the refund (spec #419).
		{statusUnknown, domain.PaymentStatusRefunding},
		{statusPayChecking, domain.PaymentStatusRefunding},
		{statusConfirmChecking, domain.PaymentStatusRefunding},
		{"WHATEVER", domain.PaymentStatusRefunding},
	}
	for _, tt := range tests {
		if got := mapCancelStatus(tt.status); got != tt.want {
			t.Errorf("mapCancelStatus(%q) = %v, want %v", tt.status, got, tt.want)
		}
	}
}

// TestMapAddCardStateStatus pins the collapse of the eight provider binding
// statuses into the neutral pending/completed/failed states.
func TestMapAddCardStateStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status string
		want   application.MethodBindingStatus
	}{
		{"NEW", application.MethodBindingPending},
		{"FORM_SHOWED", application.MethodBindingPending},
		{"3DS_CHECKING", application.MethodBindingPending},
		{"3DS_CHECKED", application.MethodBindingPending},
		{"AUTHORIZING", application.MethodBindingPending},
		{"AUTHORIZED", application.MethodBindingPending},
		{"COMPLETED", application.MethodBindingCompleted},
		{"REJECTED", application.MethodBindingFailed},
	}
	for _, tt := range tests {
		got, err := mapAddCardStateStatus(tt.status)
		if err != nil {
			t.Errorf("mapAddCardStateStatus(%q) error: %v", tt.status, err)
			continue
		}
		if got != tt.want {
			t.Errorf("mapAddCardStateStatus(%q) = %v, want %v", tt.status, got, tt.want)
		}
	}
	if _, err := mapAddCardStateStatus("NOT_A_STATUS"); err == nil {
		t.Error("expected error for unknown binding status")
	}
}
