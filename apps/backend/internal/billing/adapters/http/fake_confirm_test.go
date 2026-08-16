package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// stubFakeProvider is a func-backed FakeConfirmProvider — the fake adapter's
// confirmation capability at this consumer (ADR 0035).
type stubFakeProvider struct {
	confirm     func(ctx context.Context, internalPaymentID string) (billingapp.WebhookEvent, error)
	confirmBind func(ctx context.Context, requestKey string) (billingapp.WebhookEvent, error)
	status      func(ctx context.Context, paymentID uuid.UUID, providerPaymentID string) (billingapp.PaymentStatusResult, error)
}

func (p *stubFakeProvider) ConfirmPayment(ctx context.Context, internalPaymentID string) (billingapp.WebhookEvent, error) {
	if p.confirm != nil {
		return p.confirm(ctx, internalPaymentID)
	}
	return billingapp.WebhookEvent{}, errors.New("unexpected ConfirmPayment call")
}

func (p *stubFakeProvider) ConfirmCardBinding(ctx context.Context, requestKey string) (billingapp.WebhookEvent, error) {
	if p.confirmBind != nil {
		return p.confirmBind(ctx, requestKey)
	}
	return billingapp.WebhookEvent{}, errors.New("unexpected ConfirmCardBinding call")
}

func (p *stubFakeProvider) PaymentStatus(ctx context.Context, paymentID uuid.UUID, providerPaymentID string) (billingapp.PaymentStatusResult, error) {
	if p.status != nil {
		return p.status(ctx, paymentID, providerPaymentID)
	}
	return billingapp.PaymentStatusResult{}, errors.New("unexpected PaymentStatus call")
}

// stubConfirmBackend is a func-backed FakeConfirmBackend — the application
// capabilities the local confirmation flow consumes.
type stubConfirmBackend struct {
	getPayment func(ctx context.Context, paymentID uuid.UUID) (domain.SubscriptionPayment, error)
	applyEvent func(ctx context.Context, event billingapp.WebhookEvent) error
}

func (b *stubConfirmBackend) GetPayment(ctx context.Context, paymentID uuid.UUID) (domain.SubscriptionPayment, error) {
	if b.getPayment != nil {
		return b.getPayment(ctx, paymentID)
	}
	return domain.SubscriptionPayment{}, errors.New("unexpected GetPayment call")
}

func (b *stubConfirmBackend) ApplyProviderEvent(ctx context.Context, event billingapp.WebhookEvent) error {
	if b.applyEvent != nil {
		return b.applyEvent(ctx, event)
	}
	return errors.New("unexpected ApplyProviderEvent call")
}

// newFakeConfirmRouter mounts the confirmation handlers exactly the way the
// wiring does, so the tests drive the endpoints through their real routes.
func newFakeConfirmRouter(provider FakeConfirmProvider, backend FakeConfirmBackend) http.Handler {
	h := NewFakeConfirmHandlers(provider, backend, nil)
	r := chi.NewRouter()
	r.Post(FakePaymentConfirmRoute, h.ConfirmPayment)
	r.Post(FakeCardBindingConfirmRoute, h.ConfirmCardBinding)
	return r
}

func postConfirm(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, http.NoBody))
	return w
}

func pendingPayment() domain.SubscriptionPayment {
	providerPaymentID := "fake_123"
	return domain.SubscriptionPayment{
		ID:                uuid.New(),
		Status:            domain.PaymentStatusPending,
		ProviderPaymentID: &providerPaymentID,
		AmountKopecks:     99000,
	}
}

// TestFakeConfirmPayment_AppliesConfirmedEvent proves the happy path: the
// payment is read, the fake provider completes it and the confirmed event is
// applied through the same synchronous path a webhook takes (issue #287).
func TestFakeConfirmPayment_AppliesConfirmedEvent(t *testing.T) {
	payment := pendingPayment()
	confirmed := billingapp.WebhookEvent{Payment: &billingapp.PaymentNotification{
		InternalPaymentID: payment.ID,
		ProviderPaymentID: *payment.ProviderPaymentID,
		Status:            domain.PaymentStatusSucceeded,
	}}

	var applied *billingapp.WebhookEvent
	h := newFakeConfirmRouter(
		&stubFakeProvider{confirm: func(_ context.Context, id string) (billingapp.WebhookEvent, error) {
			if id != payment.ID.String() {
				t.Errorf("ConfirmPayment called with %q, want %q", id, payment.ID.String())
			}
			return confirmed, nil
		}},
		&stubConfirmBackend{
			getPayment: func(_ context.Context, id uuid.UUID) (domain.SubscriptionPayment, error) {
				if id != payment.ID {
					t.Errorf("GetPayment called with %v, want %v", id, payment.ID)
				}
				return payment, nil
			},
			applyEvent: func(_ context.Context, event billingapp.WebhookEvent) error {
				applied = &event
				return nil
			},
		},
	)

	w := postConfirm(t, h, "/internal/fake-subscription-payment/"+payment.ID.String()+"/confirm")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if applied == nil || applied.Payment == nil || applied.Payment.InternalPaymentID != payment.ID {
		t.Errorf("applied event = %+v, want the confirmed payment event", applied)
	}
}

// TestFakeConfirmPayment_NotFoundMapsTo404 proves an unknown payment answers
// the contract's 404.
func TestFakeConfirmPayment_NotFoundMapsTo404(t *testing.T) {
	h := newFakeConfirmRouter(
		nil,
		&stubConfirmBackend{getPayment: func(context.Context, uuid.UUID) (domain.SubscriptionPayment, error) {
			return domain.SubscriptionPayment{}, billingapp.ErrPaymentNotFound
		}},
	)

	w := postConfirm(t, h, "/internal/fake-subscription-payment/"+uuid.New().String()+"/confirm")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", w.Code, w.Body.String())
	}
}

// TestFakeConfirmPayment_FinalizedIsNoOp proves confirming a finalized payment
// never reaches the provider — the idempotency the local flow guarantees.
func TestFakeConfirmPayment_FinalizedIsNoOp(t *testing.T) {
	payment := pendingPayment()
	payment.Status = domain.PaymentStatusSucceeded
	h := newFakeConfirmRouter(
		&stubFakeProvider{confirm: func(context.Context, string) (billingapp.WebhookEvent, error) {
			t.Error("provider must not be called for a finalized payment")
			return billingapp.WebhookEvent{}, nil
		}},
		&stubConfirmBackend{
			getPayment: func(context.Context, uuid.UUID) (domain.SubscriptionPayment, error) {
				return payment, nil
			},
			applyEvent: func(context.Context, billingapp.WebhookEvent) error {
				t.Error("no event may be applied for a finalized payment")
				return nil
			},
		},
	)

	w := postConfirm(t, h, "/internal/fake-subscription-payment/"+payment.ID.String()+"/confirm")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}

// TestFakeConfirmPayment_LostProviderEntryFallsBackToStatus proves a
// confirmation the provider cannot resolve (a purged or already-settled entry)
// falls back to the provider status as the source of truth.
func TestFakeConfirmPayment_LostProviderEntryFallsBackToStatus(t *testing.T) {
	payment := pendingPayment()
	var applied *billingapp.WebhookEvent
	h := newFakeConfirmRouter(
		&stubFakeProvider{
			confirm: func(context.Context, string) (billingapp.WebhookEvent, error) {
				return billingapp.WebhookEvent{}, billingapp.ErrProviderPaymentNotFound
			},
			status: func(_ context.Context, paymentID uuid.UUID, providerPaymentID string) (billingapp.PaymentStatusResult, error) {
				if paymentID != payment.ID || providerPaymentID != *payment.ProviderPaymentID {
					t.Errorf("PaymentStatus called with %v/%q, want %v/%q", paymentID, providerPaymentID, payment.ID, *payment.ProviderPaymentID)
				}
				return billingapp.PaymentStatusResult{Status: domain.PaymentStatusSucceeded}, nil
			},
		},
		&stubConfirmBackend{
			getPayment: func(context.Context, uuid.UUID) (domain.SubscriptionPayment, error) {
				return payment, nil
			},
			applyEvent: func(_ context.Context, event billingapp.WebhookEvent) error {
				applied = &event
				return nil
			},
		},
	)

	w := postConfirm(t, h, "/internal/fake-subscription-payment/"+payment.ID.String()+"/confirm")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if applied == nil || applied.Payment == nil ||
		applied.Payment.Status != domain.PaymentStatusSucceeded ||
		applied.Payment.ProviderPaymentID != *payment.ProviderPaymentID {
		t.Errorf("applied event = %+v, want the synthesized succeeded notification", applied)
	}
}

// TestFakeConfirmPayment_LostEntryPendingStatusIsNoOp proves a lost entry the
// provider has not settled yet changes nothing.
func TestFakeConfirmPayment_LostEntryPendingStatusIsNoOp(t *testing.T) {
	payment := pendingPayment()
	h := newFakeConfirmRouter(
		&stubFakeProvider{
			confirm: func(context.Context, string) (billingapp.WebhookEvent, error) {
				return billingapp.WebhookEvent{}, billingapp.ErrProviderPaymentNotFound
			},
			status: func(context.Context, uuid.UUID, string) (billingapp.PaymentStatusResult, error) {
				return billingapp.PaymentStatusResult{Status: domain.PaymentStatusPending}, nil
			},
		},
		&stubConfirmBackend{
			getPayment: func(context.Context, uuid.UUID) (domain.SubscriptionPayment, error) {
				return payment, nil
			},
			applyEvent: func(context.Context, billingapp.WebhookEvent) error {
				t.Error("no event may be applied for an unsettled provider status")
				return nil
			},
		},
	)

	w := postConfirm(t, h, "/internal/fake-subscription-payment/"+payment.ID.String()+"/confirm")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
}

// TestFakeConfirmPayment_ProviderErrorMapsTo500 proves a provider failure
// without a fallback answers 500.
func TestFakeConfirmPayment_ProviderErrorMapsTo500(t *testing.T) {
	payment := pendingPayment()
	h := newFakeConfirmRouter(
		&stubFakeProvider{confirm: func(context.Context, string) (billingapp.WebhookEvent, error) {
			return billingapp.WebhookEvent{}, errors.New("provider is down")
		}},
		&stubConfirmBackend{
			getPayment: func(context.Context, uuid.UUID) (domain.SubscriptionPayment, error) {
				return payment, nil
			},
		},
	)

	w := postConfirm(t, h, "/internal/fake-subscription-payment/"+payment.ID.String()+"/confirm")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body: %s", w.Code, w.Body.String())
	}
}

// TestFakeConfirmPayment_InvalidUUIDMapsTo400 proves a malformed payment id is
// a request defect, not a server failure.
func TestFakeConfirmPayment_InvalidUUIDMapsTo400(t *testing.T) {
	h := newFakeConfirmRouter(nil, nil)

	w := postConfirm(t, h, "/internal/fake-subscription-payment/not-a-uuid/confirm")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}
}

// TestFakeConfirmCardBinding_AppliesConfirmedEvent proves the binding flow:
// the fake adapter completes the session and the method-bound event is applied
// through the synchronous webhook path (issue #287).
func TestFakeConfirmCardBinding_AppliesConfirmedEvent(t *testing.T) {
	confirmed := billingapp.WebhookEvent{MethodBound: &billingapp.MethodBoundNotification{
		BindingID: "fake_bind_1",
		Method:    billingapp.SavedMethod{ChargeToken: "token_1"},
	}}
	var applied *billingapp.WebhookEvent
	h := newFakeConfirmRouter(
		&stubFakeProvider{confirmBind: func(_ context.Context, key string) (billingapp.WebhookEvent, error) {
			if key != "fake_bind_1" {
				t.Errorf("ConfirmCardBinding called with %q, want %q", key, "fake_bind_1")
			}
			return confirmed, nil
		}},
		&stubConfirmBackend{applyEvent: func(_ context.Context, event billingapp.WebhookEvent) error {
			applied = &event
			return nil
		}},
	)

	w := postConfirm(t, h, "/internal/fake-card-binding/fake_bind_1/confirm")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if applied == nil || applied.MethodBound == nil || applied.MethodBound.BindingID != "fake_bind_1" {
		t.Errorf("applied event = %+v, want the method-bound notification", applied)
	}
}

// TestFakeConfirmCardBinding_UnknownKeyMapsTo404 proves an unknown or expired
// request key answers 404.
func TestFakeConfirmCardBinding_UnknownKeyMapsTo404(t *testing.T) {
	h := newFakeConfirmRouter(
		&stubFakeProvider{confirmBind: func(context.Context, string) (billingapp.WebhookEvent, error) {
			return billingapp.WebhookEvent{}, billingapp.ErrProviderBindingNotFound
		}},
		nil,
	)

	w := postConfirm(t, h, "/internal/fake-card-binding/unknown/confirm")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", w.Code, w.Body.String())
	}
}
