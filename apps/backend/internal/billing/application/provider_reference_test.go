package application

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// Unit tests of the saveProviderReference module (issue #285): the single
// idempotent write of the provider's payment id both payment paths share —
// the customer-initiated tariff-change payment (CIT) and the merchant-
// initiated renewal (MIT). The paths differ only in the initiation result
// they pass (the CIT result carries the payer's form URL, the MIT one does
// not) and in what they map the returned payment to; the write itself is one
// implementation.

// providerReferenceHarness runs saveProviderReference over the in-memory
// fakes inside a real factory transaction.
type providerReferenceHarness struct {
	stores *fakeStores
	now    time.Time
}

func newProviderReferenceHarness(t *testing.T) *providerReferenceHarness {
	t.Helper()
	return &providerReferenceHarness{
		stores: newFakeStores(testTariffs()...),
		now:    time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC),
	}
}

// seedPendingPayment stores a pending month payment without a provider
// reference — the state both paths hand to the save — shaped by mutate.
func (h *providerReferenceHarness) seedPendingPayment(t *testing.T, mutate func(*domain.SubscriptionPayment)) domain.SubscriptionPayment {
	t.Helper()
	var pro domain.Tariff
	for _, tariff := range testTariffs() {
		if tariff.Name == domain.TariffPro {
			pro = tariff
		}
	}
	payment, err := domain.NewSubscriptionPayment(
		uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), pro.ID,
		domain.PeriodMonth, pro.MonthlyPriceKopecks, testProviderFake, h.now.Add(-time.Minute))
	if err != nil {
		t.Fatalf("NewSubscriptionPayment() error = %v", err)
	}
	if mutate != nil {
		mutate(&payment)
	}
	if _, err := h.stores.payments.Create(t.Context(), payment); err != nil {
		t.Fatalf("seed Create() error = %v", err)
	}
	return payment
}

// save runs one saveProviderReference call through the harness factory.
func (h *providerReferenceHarness) save(
	t *testing.T, paymentID uuid.UUID, initRes InitPaymentResult, now time.Time,
) (domain.SubscriptionPayment, error) {
	t.Helper()
	factory := h.stores.factory(nil)
	return saveProviderReference(t.Context(), factory.runInTx, paymentID, initRes, now)
}

// storedPayment loads the payment from the fake repository.
func (h *providerReferenceHarness) storedPayment(t *testing.T, paymentID uuid.UUID) domain.SubscriptionPayment {
	t.Helper()
	payment, err := h.stores.payments.GetByID(t.Context(), paymentID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	return payment
}

// TestSaveProviderReference_CITAndMITShapes proves the two initiation shapes
// flow through the same write as parameters: the CIT result carries the
// payer's form URL and persists both the provider id and the URL, the MIT
// result carries no URL and persists the provider id alone. Both leave the
// payment pending with the save's timestamp.
func TestSaveProviderReference_CITAndMITShapes(t *testing.T) {
	for _, tc := range []struct {
		name    string
		initRes InitPaymentResult
		wantURL string // Empty means the initiation carried no URL to persist.
	}{
		{
			name:    "cit persists reference and payer url",
			initRes: InitPaymentResult{ProviderPaymentID: "prov_cit_1", PaymentURL: "https://pay/1"},
			wantURL: "https://pay/1",
		},
		{
			name:    "mit persists reference without url",
			initRes: InitPaymentResult{ProviderPaymentID: "prov_mit_1"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newProviderReferenceHarness(t)
			seeded := h.seedPendingPayment(t, nil)

			saved, err := h.save(t, seeded.ID, tc.initRes, h.now)
			if err != nil {
				t.Fatalf("saveProviderReference() error = %v", err)
			}
			stored := h.storedPayment(t, seeded.ID)

			for _, payment := range []domain.SubscriptionPayment{saved, stored} {
				if !payment.HasProviderReference() || *payment.ProviderPaymentID != tc.initRes.ProviderPaymentID {
					t.Errorf("provider payment id = %v, want %q", payment.ProviderPaymentID, tc.initRes.ProviderPaymentID)
				}
				if tc.wantURL == "" && payment.HasPaymentURL() {
					t.Errorf("payment url = %v, want none for an initiation without one", payment.PaymentURL)
				}
				if tc.wantURL != "" && (!payment.HasPaymentURL() || *payment.PaymentURL != tc.wantURL) {
					t.Errorf("payment url = %v, want %q", payment.PaymentURL, tc.wantURL)
				}
				if payment.Status != domain.PaymentStatusPending {
					t.Errorf("status = %q, want the pending unchanged", payment.Status)
				}
				if !payment.UpdatedAt.Equal(h.now) {
					t.Errorf("updated at = %v, want the save's %v", payment.UpdatedAt, h.now)
				}
			}
		})
	}
}

// TestSaveProviderReference_SecondWriteKeepsFirstReference proves the
// idempotency of a repeated save: a second initiation result — what a retry
// after a crash or a concurrent flow produces — never overwrites the
// reference, the URL or the timestamp the first save persisted.
func TestSaveProviderReference_SecondWriteKeepsFirstReference(t *testing.T) {
	h := newProviderReferenceHarness(t)
	seeded := h.seedPendingPayment(t, nil)

	if _, err := h.save(t, seeded.ID, InitPaymentResult{ProviderPaymentID: "prov_first", PaymentURL: "https://pay/first"}, h.now); err != nil {
		t.Fatalf("first saveProviderReference() error = %v", err)
	}

	saved, err := h.save(t, seeded.ID,
		InitPaymentResult{ProviderPaymentID: "prov_second", PaymentURL: "https://pay/second"}, h.now.Add(time.Hour))
	if err != nil {
		t.Fatalf("second saveProviderReference() error = %v", err)
	}
	stored := h.storedPayment(t, seeded.ID)

	for _, payment := range []domain.SubscriptionPayment{saved, stored} {
		if *payment.ProviderPaymentID != "prov_first" {
			t.Errorf("provider payment id = %q, want the first save's %q", *payment.ProviderPaymentID, "prov_first")
		}
		if !payment.HasPaymentURL() || *payment.PaymentURL != "https://pay/first" {
			t.Errorf("payment url = %v, want the first save's", payment.PaymentURL)
		}
		if !payment.UpdatedAt.Equal(h.now) {
			t.Errorf("updated at = %v, want the first save's %v", payment.UpdatedAt, h.now)
		}
	}
}

// TestSaveProviderReference_FinalizedPaymentWins proves the concurrent-race
// guard: a payment a webhook finalized before the save lands is returned as
// persisted — no error, no reference written over the final state.
func TestSaveProviderReference_FinalizedPaymentWins(t *testing.T) {
	h := newProviderReferenceHarness(t)
	seeded := h.seedPendingPayment(t, func(p *domain.SubscriptionPayment) {
		if err := p.MarkSucceeded(h.now.Add(-time.Minute)); err != nil {
			t.Fatalf("MarkSucceeded() error = %v", err)
		}
	})

	saved, err := h.save(t, seeded.ID, InitPaymentResult{ProviderPaymentID: "prov_late", PaymentURL: "https://pay/late"}, h.now)
	if err != nil {
		t.Fatalf("saveProviderReference() error = %v", err)
	}
	if saved.Status != domain.PaymentStatusSucceeded || saved.HasProviderReference() {
		t.Errorf("returned payment = %q/referenced %t, want the persisted succeeded without a reference",
			saved.Status, saved.HasProviderReference())
	}
	if stored := h.storedPayment(t, seeded.ID); stored.HasProviderReference() {
		t.Errorf("stored payment got a reference over the final state: %v", stored.ProviderPaymentID)
	}
}

// TestSaveProviderReference_MissingPaymentFails proves the miss narrows to
// the payment-not-found error of the shared lock step.
func TestSaveProviderReference_MissingPaymentFails(t *testing.T) {
	h := newProviderReferenceHarness(t)

	_, err := h.save(t, uuid.Must(uuid.NewV7()), InitPaymentResult{ProviderPaymentID: "prov_x"}, h.now)
	if !errors.Is(err, ErrPaymentNotFound) {
		t.Fatalf("saveProviderReference() error = %v, want ErrPaymentNotFound", err)
	}
}

// TestSaveProviderReference_RejectedReferenceWritesNothing proves the domain
// rejection of a referenceless initiation result fails the save before
// anything is written: the stored payment keeps its pre-save shape.
func TestSaveProviderReference_RejectedReferenceWritesNothing(t *testing.T) {
	h := newProviderReferenceHarness(t)
	seeded := h.seedPendingPayment(t, nil)

	_, err := h.save(t, seeded.ID, InitPaymentResult{PaymentURL: "https://pay/nothing"}, h.now)
	if !errors.Is(err, domain.ErrInvalidPayment) {
		t.Fatalf("saveProviderReference() error = %v, want the domain's ErrInvalidPayment", err)
	}
	stored := h.storedPayment(t, seeded.ID)
	if stored.HasProviderReference() || stored.HasPaymentURL() {
		t.Errorf("stored payment = referenced %t/url %t, want the untouched pending", stored.HasProviderReference(), stored.HasPaymentURL())
	}
	if !stored.UpdatedAt.Equal(seeded.UpdatedAt) {
		t.Errorf("updated at = %v, want the seeded %v", stored.UpdatedAt, seeded.UpdatedAt)
	}
}
