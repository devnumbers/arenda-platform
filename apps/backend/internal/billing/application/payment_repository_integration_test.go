//go:build integration

package application_test

import (
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// TestPaymentRepository_Integration_ListSelection proves the parameterized
// payment worker selection (issue #286) against the real schema: the stale
// pending batch and the stale refunding batch pick exactly their rows — the
// provider reference is mandatory, staleness is strict, and the tariff-change
// narrowing compares the payment's target against the subscription's current
// tariff.
// PaymentSelectionSeeder seeds the worker-selection fixture rows of the
// ListSelection test: it onboards a user — optionally lifting their
// subscription onto a paid current tariff — and stores one payment in the
// given state, backdating its clocks through raw SQL the way a stuck row
// looks. Every payment gets its own user so the pending-payment unique index
// never interferes; the onboarding subscription stays on basic, so a business
// payment is a tariff-change one, while a pro payment against a pro
// subscription is not.
type paymentSelectionSeeder struct {
	h   *integrationHarness
	now time.Time
}

// payment seeds one user with one payment row in the given state.
func (s paymentSelectionSeeder) payment(
	t *testing.T, name string, currentTariff *domain.Tariff, targetTariff domain.Tariff,
	ref string, status domain.PaymentStatus, age time.Duration,
) domain.SubscriptionPayment {
	t.Helper()
	userID := s.h.seedUser()
	if err := s.h.onboarding.OnUserRegistered(s.h.ctx(), userID); err != nil {
		t.Fatalf("OnUserRegistered(%s) error = %v", name, err)
	}
	sub, err := s.h.subscriptions.GetByUserID(s.h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID(%s) error = %v", name, err)
	}
	if currentTariff != nil {
		s.liftSubscriptionOntoTariff(t, name, &sub, currentTariff)
	}
	stored, err := s.h.payments.Create(s.h.ctx(), s.newPayment(t, name, sub, targetTariff, ref, status, age))
	if err != nil {
		t.Fatalf("Create(%s) error = %v", name, err)
	}
	s.backdate(t, name, stored, age)
	return stored
}

// liftSubscriptionOntoTariff moves the onboarding subscription onto the paid
// current tariff so the payment's target can be compared against it.
func (s paymentSelectionSeeder) liftSubscriptionOntoTariff(
	t *testing.T, name string, sub *domain.Subscription, tariff *domain.Tariff,
) {
	t.Helper()
	sub.TariffID = tariff.ID
	until := s.now.Add(24 * time.Hour)
	sub.ValidUntil = &until
	sub.AutoRenewEnabled = true
	if err := s.h.subscriptions.Update(s.h.ctx(), *sub); err != nil {
		t.Fatalf("lift subscription(%s) error = %v", name, err)
	}
}

// newPayment builds the payment row: the target tariff at its monthly price,
// aged by the given staleness, with the provider reference when one is given.
func (s paymentSelectionSeeder) newPayment(
	t *testing.T, name string, sub domain.Subscription, targetTariff domain.Tariff,
	ref string, status domain.PaymentStatus, age time.Duration,
) domain.SubscriptionPayment {
	t.Helper()
	payment, err := domain.NewSubscriptionPayment(
		sub.UserID, sub.ID, targetTariff.ID, domain.PeriodMonth,
		targetTariff.MonthlyPriceKopecks, testProviderFake, s.now.Add(-age))
	if err != nil {
		t.Fatalf("NewSubscriptionPayment(%s) error = %v", name, err)
	}
	if ref != "" {
		if err := payment.SaveProviderReference(ref, "", s.now.Add(-age)); err != nil {
			t.Fatalf("SaveProviderReference(%s) error = %v", name, err)
		}
	}
	if status != domain.PaymentStatusPending {
		payment.Status = status
	}
	return payment
}

// runTriggerStatement runs one ALTER TABLE statement on the updated_at
// trigger, failing with the given action verb ("park" or "restore").
func (s paymentSelectionSeeder) runTriggerStatement(t *testing.T, verb, name, statement string) {
	t.Helper()
	if _, err := s.h.pool.Exec(s.h.ctx(), statement); err != nil {
		t.Fatalf("%s updated_at trigger(%s): %v", verb, name, err)
	}
}

// backdate parks the updated_at trigger for one statement and rewrites the
// payment's clocks to the aged moment, restoring the trigger right after.
func (s paymentSelectionSeeder) backdate(
	t *testing.T, name string, payment domain.SubscriptionPayment, age time.Duration,
) {
	t.Helper()
	if age <= 0 {
		return
	}
	aged := s.now.Add(-age)
	// The updated_at trigger would overwrite the backdating; park it
	// for the statement and restore it right after.
	s.runTriggerStatement(t, "park", name,
		`ALTER TABLE subscription_payments DISABLE TRIGGER trg_subscription_payments_updated_at`)
	_, err := s.h.pool.Exec(s.h.ctx(),
		`UPDATE subscription_payments SET created_at = $1, updated_at = $1 WHERE id = $2`, aged, payment.ID)
	s.runTriggerStatement(t, "restore", name,
		`ALTER TABLE subscription_payments ENABLE TRIGGER trg_subscription_payments_updated_at`)
	if err != nil {
		t.Fatalf("age %s: %v", name, err)
	}
}

func TestPaymentRepository_Integration_ListSelection(t *testing.T) {
	h := newIntegrationHarness(t)
	now := h.clock.Now()

	pro, err := h.tariffs.GetByName(h.ctx(), domain.TariffPro)
	if err != nil {
		t.Fatalf("GetByName(pro) error = %v", err)
	}
	business, err := h.tariffs.GetByName(h.ctx(), domain.TariffBusiness)
	if err != nil {
		t.Fatalf("GetByName(business) error = %v", err)
	}

	seeder := paymentSelectionSeeder{h: h, now: now}
	const stale = 10 * time.Minute
	staleUpgrade := seeder.payment(t, "stale pending upgrade", nil, business, "prov_stale_upgrade", domain.PaymentStatusPending, stale)
	staleRenewal := seeder.payment(t, "stale pending renewal", &pro, pro, "prov_stale_renewal", domain.PaymentStatusPending, stale)
	seeder.payment(t, "fresh pending", nil, business, "prov_fresh", domain.PaymentStatusPending, time.Minute)
	seeder.payment(t, "stale pending without reference", nil, business, "", domain.PaymentStatusPending, stale)
	staleRefunding := seeder.payment(t, "stale refunding", nil, pro, "prov_refunding", domain.PaymentStatusRefunding, stale)
	seeder.payment(t, "fresh refunding", nil, pro, "prov_refunding_fresh", domain.PaymentStatusRefunding, time.Minute)
	seeder.payment(t, "stale succeeded", nil, pro, "prov_succeeded", domain.PaymentStatusSucceeded, stale)

	pendingSel := billingapp.PaymentSelection{
		Status:        domain.PaymentStatusPending,
		CreatedBefore: new(now.Add(-5 * time.Minute)),
		Limit:         100,
	}
	upgradeSel := pendingSel
	upgradeSel.TariffChangeOnly = true
	refundingSel := billingapp.PaymentSelection{
		Status:        domain.PaymentStatusRefunding,
		UpdatedBefore: new(now.Add(-5 * time.Minute)),
		Limit:         100,
	}

	for _, tc := range []struct {
		name string
		sel  billingapp.PaymentSelection
		want []uuid.UUID
	}{
		{name: "stale pending with reference", sel: pendingSel, want: []uuid.UUID{staleUpgrade.ID, staleRenewal.ID}},
		{name: "stale pending tariff changes", sel: upgradeSel, want: []uuid.UUID{staleUpgrade.ID}},
		{name: "stale refunding", sel: refundingSel, want: []uuid.UUID{staleRefunding.ID}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			found, err := h.payments.List(h.ctx(), tc.sel)
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			got := make([]uuid.UUID, 0, len(found))
			for _, p := range found {
				got = append(got, p.ID)
			}
			// Same-id ties make the uuidv7 order non-deterministic inside one
			// batch; membership is the contract, order belongs to the phase
			// clocks.
			if len(got) != len(tc.want) {
				t.Fatalf("List() = %v, want exactly %v", got, tc.want)
			}
			for _, id := range tc.want {
				if !slices.Contains(got, id) {
					t.Errorf("List() = %v, want %v among them", got, id)
				}
			}
		})
	}
}
