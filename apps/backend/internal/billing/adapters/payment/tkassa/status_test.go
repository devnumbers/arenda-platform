package tkassa

import (
	"testing"

	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// TestMapStatus pins the full payment-status mapping, including the deliberate
// decisions: REVERSED from GetState/webhook is failed (ADR 0017), unknown
// statuses default to pending, and the PARTIAL_* outcomes land in refunded
// because the new domain has no partial-refund status (ADR 0037, issue #246
// §3.1).
func TestMapStatus(t *testing.T) {
	tests := []struct {
		status string
		want   domain.PaymentStatus
	}{
		{"NEW", domain.PaymentStatusPending},
		{"FORM_SHOWED", domain.PaymentStatusPending},
		{"AUTHORIZING", domain.PaymentStatusPending},
		{"AUTHORIZED", domain.PaymentStatusPending},
		{"3DS_CHECKING", domain.PaymentStatusPending},
		{"3DS_CHECKED", domain.PaymentStatusPending},
		{"CONFIRMING", domain.PaymentStatusPending},
		{"REVERSING", domain.PaymentStatusPending},
		{"REFUNDING", domain.PaymentStatusPending},
		{"ASYNC_REFUNDING", domain.PaymentStatusPending},
		{"PAY_CHECKING", domain.PaymentStatusPending},
		{"CONFIRM_CHECKING", domain.PaymentStatusPending},
		{"CHECKING", domain.PaymentStatusPending},
		{"CHECKED", domain.PaymentStatusPending},
		{"COMPLETING", domain.PaymentStatusPending},
		{"COMPLETED", domain.PaymentStatusPending},
		{"PREAUTHORIZING", domain.PaymentStatusPending},
		{"PROCESSING", domain.PaymentStatusPending},
		{"UNKNOWN", domain.PaymentStatusPending},
		{"SOMETHING_NEW", domain.PaymentStatusPending}, // unknown defaults to pending
		{"CONFIRMED", domain.PaymentStatusSucceeded},
		{"REFUNDED", domain.PaymentStatusRefunded},
		{"PARTIAL_REFUNDED", domain.PaymentStatusRefunded},
		{"REJECTED", domain.PaymentStatusFailed},
		{"AUTH_FAIL", domain.PaymentStatusFailed},
		{"DEADLINE_EXPIRED", domain.PaymentStatusFailed},
		{"CANCELED", domain.PaymentStatusFailed},
		{"REVERSED", domain.PaymentStatusFailed},
		{"PARTIAL_REVERSED", domain.PaymentStatusRefunded},
		{"3DS_FAILED", domain.PaymentStatusFailed},
	}
	for _, tt := range tests {
		if got := mapStatus(tt.status); got != tt.want {
			t.Errorf("mapStatus(%q) = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestMapCancelStatus(t *testing.T) {
	tests := []struct {
		status string
		want   domain.PaymentStatus
	}{
		{"REFUNDED", domain.PaymentStatusRefunded},
		{"REVERSED", domain.PaymentStatusRefunded},
		{"PARTIAL_REFUNDED", domain.PaymentStatusRefunded},
		{"PARTIAL_REVERSED", domain.PaymentStatusRefunded},
		{"NEW", domain.PaymentStatusPending},
		{"AUTHORIZED", domain.PaymentStatusPending},
		{"AUTHORIZING", domain.PaymentStatusPending},
		{"3DS_CHECKING", domain.PaymentStatusPending},
		{"3DS_CHECKED", domain.PaymentStatusPending},
		{"CONFIRMING", domain.PaymentStatusPending},
		{"FORM_SHOWED", domain.PaymentStatusPending},
		{"REVERSING", domain.PaymentStatusRefunding},
		{"REFUNDING", domain.PaymentStatusRefunding},
		{"ASYNC_REFUNDING", domain.PaymentStatusRefunding},
		{"CONFIRMED", domain.PaymentStatusFailed}, // refund not confirmed — deliberate default
		{"WHATEVER", domain.PaymentStatusFailed},
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
