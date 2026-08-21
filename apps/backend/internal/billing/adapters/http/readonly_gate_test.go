package http

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
)

// gateTariffRepo is an in-memory billingapp.TariffRepository for the gate tests.
type gateTariffRepo struct{ tariff domain.Tariff }

func (r *gateTariffRepo) GetByID(context.Context, uuid.UUID) (domain.Tariff, error) {
	return r.tariff, nil
}

func (r *gateTariffRepo) GetByName(context.Context, domain.TariffName) (domain.Tariff, error) {
	return r.tariff, nil
}

func (r *gateTariffRepo) List(context.Context) ([]domain.Tariff, error) {
	return nil, nil
}

func (r *gateTariffRepo) ListAll(context.Context) ([]domain.Tariff, error) {
	return nil, nil
}

// The write methods are unused by the gate; they satisfy the port (issue #256).
func (r *gateTariffRepo) Create(_ context.Context, tariff domain.Tariff) (domain.Tariff, error) {
	return tariff, nil
}

func (r *gateTariffRepo) Update(_ context.Context, tariff domain.Tariff) (domain.Tariff, error) {
	return tariff, nil
}

func (r *gateTariffRepo) Invalidate(context.Context) error { return nil }

func (r *gateTariffRepo) WithTx(transaction.Tx) (billingapp.TariffRepository, error) { return r, nil }

// gateSubscriptionRepo is an in-memory billingapp.SubscriptionRepository.
type gateSubscriptionRepo struct {
	sub domain.Subscription
	err error
}

func (r *gateSubscriptionRepo) GetByUserID(context.Context, uuid.UUID) (domain.Subscription, error) {
	return r.sub, r.err
}

func (r *gateSubscriptionRepo) GetByUserIDForUpdate(ctx context.Context, id uuid.UUID) (domain.Subscription, error) {
	return r.GetByUserID(ctx, id)
}

// The worker selection listing is unused by the gate; empty results satisfy
// the port.
func (r *gateSubscriptionRepo) List(context.Context, billingapp.SubscriptionSelection) ([]domain.Subscription, error) {
	return nil, nil
}

func (r *gateSubscriptionRepo) Create(_ context.Context, sub domain.Subscription) (domain.Subscription, error) {
	return sub, nil
}

func (r *gateSubscriptionRepo) Update(context.Context, domain.Subscription) error { return nil }

func (r *gateSubscriptionRepo) WithTx(transaction.Tx) (billingapp.SubscriptionRepository, error) {
	return r, nil
}

// gateTransitionRepo is an in-memory billingapp.SubscriptionTransitionRepository.
type gateTransitionRepo struct{}

func (r *gateTransitionRepo) Append(context.Context, domain.Transition) error { return nil }

func (r *gateTransitionRepo) ListBySubscriptionID(context.Context, uuid.UUID) ([]domain.Transition, error) {
	return nil, nil
}

func (r *gateTransitionRepo) WithTx(transaction.Tx) (billingapp.SubscriptionTransitionRepository, error) {
	return r, nil
}

// gatePaymentRepo is an in-memory billingapp.SubscriptionPaymentRepository.
type gatePaymentRepo struct{}

func (r *gatePaymentRepo) Create(_ context.Context, p domain.SubscriptionPayment) (domain.SubscriptionPayment, error) {
	return p, nil
}

func (r *gatePaymentRepo) GetByID(context.Context, uuid.UUID) (domain.SubscriptionPayment, error) {
	return domain.SubscriptionPayment{}, billingapp.ErrNotFound
}

func (r *gatePaymentRepo) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.SubscriptionPayment, error) {
	return r.GetByID(ctx, id)
}

func (r *gatePaymentRepo) ListByUserID(context.Context, uuid.UUID) ([]domain.SubscriptionPayment, error) {
	return nil, nil
}

func (r *gatePaymentRepo) ListPendingByUserID(context.Context, uuid.UUID) ([]domain.SubscriptionPayment, error) {
	return nil, nil
}

// The reconciliation selection listing is unused by the gate; empty results
// satisfy the port.
func (r *gatePaymentRepo) List(context.Context, billingapp.PaymentSelection) ([]domain.SubscriptionPayment, error) {
	return nil, nil
}

func (r *gatePaymentRepo) Update(context.Context, domain.SubscriptionPayment) error { return nil }

func (r *gatePaymentRepo) WithTx(transaction.Tx) (billingapp.SubscriptionPaymentRepository, error) {
	return r, nil
}

// newGate builds the readonly-gate adapter over an in-memory subscription
// service. The txStoreFactory is unexported but constructible and passable
// from this package (ADR 0033 factory pattern).
func newGate(sub domain.Subscription, subErr error, now time.Time) *MutationGate {
	tariff := domain.Tariff{ID: sub.TariffID, Name: domain.TariffBasic, ActivePropertyLimit: 1, IsActive: true}
	factory := billingapp.NewTxStoreFactory(
		&gateTariffRepo{tariff: tariff},
		&gateSubscriptionRepo{sub: sub, err: subErr},
		&gateTransitionRepo{},
		&gatePaymentRepo{},
		nil,
		nil,
		nil,
		nil,
	)
	return NewMutationGate(billingapp.NewSubscriptionService(factory, billingapp.SubscriptionServiceConfig{}), clockOn(now))
}

type clockOn time.Time

func (c clockOn) Now() time.Time { return time.Time(c) }

// TestMutationGate_BasicSubscriptionAllowsMutations proves the acceptance
// criterion of issue #245: the basic subscription created at onboarding allows
// data mutations through the readonly gate.
func TestMutationGate_BasicSubscriptionAllowsMutations(t *testing.T) {
	t.Parallel()
	sub, err := domain.NewBasicSubscription(uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()))
	if err != nil {
		t.Fatalf("NewBasicSubscription() error = %v", err)
	}
	gate := newGate(sub, nil, time.Now())

	ok, err := gate.CanMutateData(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("CanMutateData() error = %v", err)
	}
	if !ok {
		t.Error("CanMutateData() = false, want true for the basic subscription")
	}
}

// TestMutationGate_NoSubscriptionAllowsMutations proves a user without a
// subscription is treated as mutable: the gate only blocks, never grants.
func TestMutationGate_NoSubscriptionAllowsMutations(t *testing.T) {
	t.Parallel()
	gate := newGate(domain.Subscription{}, billingapp.ErrSubscriptionNotFound, time.Now())

	ok, err := gate.CanMutateData(t.Context(), uuid.Must(uuid.NewV7()))
	if err != nil {
		t.Fatalf("CanMutateData() error = %v", err)
	}
	if !ok {
		t.Error("CanMutateData() = false, want true without a subscription")
	}
}

// TestMutationGate_ExpiredGraceBlocksMutations proves the gate blocks when the
// grace window has passed (ADR 0008 readonly mode).
func TestMutationGate_ExpiredGraceBlocksMutations(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	graceUntil := now.Add(-time.Hour)
	sub := domain.Subscription{
		ID:         uuid.Must(uuid.NewV7()),
		UserID:     uuid.Must(uuid.NewV7()),
		TariffID:   uuid.Must(uuid.NewV7()),
		Status:     domain.SubscriptionStatusGrace,
		ValidUntil: &graceUntil,
	}
	gate := newGate(sub, nil, now)

	ok, err := gate.CanMutateData(t.Context(), sub.UserID)
	if err != nil {
		t.Fatalf("CanMutateData() error = %v", err)
	}
	if ok {
		t.Error("CanMutateData() = true, want false for expired grace")
	}
}

// TestMutationGate_InfrastructureErrorPropagates proves a lookup failure is
// returned rather than silently allowing or blocking.
func TestMutationGate_InfrastructureErrorPropagates(t *testing.T) {
	t.Parallel()
	boom := errors.New("connection reset")
	gate := newGate(domain.Subscription{}, boom, time.Now())

	if _, err := gate.CanMutateData(t.Context(), uuid.Must(uuid.NewV7())); !errors.Is(err, boom) {
		t.Fatalf("CanMutateData() error = %v, want %v", err, boom)
	}
}
