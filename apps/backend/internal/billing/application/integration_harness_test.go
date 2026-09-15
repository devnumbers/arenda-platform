//go:build integration

package application_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	auditpg "github.com/nambers/arenda-planform/apps/backend/internal/audit/adapters/postgres"
	auditapp "github.com/nambers/arenda-planform/apps/backend/internal/audit/application"
	billinghttp "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/http"
	paymentfake "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/payment/fake"
	billingpg "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/postgres"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	pgdb "github.com/nambers/arenda-planform/apps/backend/internal/platform/database/postgres"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/database/testdb"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/encryption"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/actor"
)

// integrationBaseTime anchors the fake clock so TTL and validity behaviour is
// deterministic across tests.
var integrationBaseTime = time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)

// integrationEncryptionKey is a fixed 32-byte hex key so payment-method tokens
// are encrypted with a real AES encryptor in tests.
const integrationEncryptionKey = "19bcc5ae5940668759add6f4475c21ce21298b58e3437f4b9a2881cafc4e3e2a"

// mutableClock is a fake clock.Clock whose Now can be advanced mid-test.
type mutableClock struct{ now time.Time }

func (c *mutableClock) Now() time.Time { return c.now }

// integrationHarness wires the billing services to real PostgreSQL
// repositories through a postgres-backed Unit-of-Work, sharing one fake clock
// and one logger. Each test builds a fresh harness over a clean (truncated)
// database via testdb.Setup (testcontainers PostgreSQL 18 or TEST_DATABASE_URL).
type integrationHarness struct {
	t     *testing.T
	pool  *pgxpool.Pool
	clock *mutableClock

	tariffs       *billingpg.TariffRepository
	subscriptions *billingpg.SubscriptionRepository
	transitions   *billingpg.SubscriptionTransitionRepository
	payments      *billingpg.SubscriptionPaymentRepository
	methods       *billingpg.PaymentMethodRepository
	bindings      *billingpg.CardBindingSessionRepository
	encryptor     encryption.Encryptor
	// Provider is the fake adapter itself (not just the port) so integration
	// tests can drive provider-side state the flows cannot (issue #251
	// binding polling).
	provider *paymentfake.Provider

	services          billingapp.Services
	subscriptionsSvc  *billingapp.SubscriptionService
	paymentsSvc       *billingapp.PaymentService
	paymentMethodsSvc *billingapp.PaymentMethodService
	tariffsSvc        *billingapp.TariffService
	onboarding        *billingapp.OnboardingService
	limiter           *billingapp.SubscriptionPropertyLimiter
	// NewSubscriptionService builds one more subscription service over the
	// harness's shared factory — the rig-less railguard variant among others.
	newSubscriptionService func(billingapp.SubscriptionServiceConfig) *billingapp.SubscriptionService
	// FakeConfirms mounts the local confirmation endpoints the way the wiring
	// does under the fake provider (issue #287): the fake adapter reports the
	// completed entry, the confirmed event runs through the production
	// webhook path.
	fakeConfirms http.Handler
}

// newIntegrationHarness builds a fresh harness over a clean database. The
// returned clock is anchored at integrationBaseTime. The payment flows run
// against the fake provider adapter (issue #250): it implements the provider
// port exactly like the real adapter, so integration tests drive payments the
// way the local environment does. Payment-method tokens are encrypted at rest
// with a real AES encryptor (issue #251).
func newIntegrationHarness(t *testing.T) *integrationHarness {
	t.Helper()

	pool := testdb.Setup(t)
	// Each harness runs on a private clone of the migrated template, whose
	// migrations already seed the canonical tariff plans; seedTariffs is an
	// idempotent guard (ON CONFLICT DO NOTHING) pinning those plans.
	seedTariffs(t, pool)
	clk := &mutableClock{now: integrationBaseTime}
	logger := slog.New(slog.DiscardHandler)

	encryptor, err := encryption.NewEncryptor(integrationEncryptionKey)
	if err != nil {
		t.Fatalf("init encryptor: %v", err)
	}
	tariffs := billingpg.NewTariffRepository(pool, 0, clk)
	subscriptions := billingpg.NewSubscriptionRepository(pool)
	transitions := billingpg.NewSubscriptionTransitionRepository(pool)
	paymentsRepo := billingpg.NewSubscriptionPaymentRepository(pool, encryptor)
	methodsRepo := billingpg.NewPaymentMethodRepository(pool, encryptor)
	bindingsRepo := billingpg.NewCardBindingSessionRepository(pool)
	uow := pgdb.NewUoW(pool, logger)
	audit := auditapp.NewService(auditpg.NewWriter(pool), clk)

	provider := paymentfake.NewProvider("http://localhost:8080", logger, clk, nil)

	factory := billingapp.NewTxStoreFactory(tariffs, subscriptions, transitions, paymentsRepo, methodsRepo, bindingsRepo, audit, uow)
	services := billingapp.NewServices(factory, billingapp.ServicesConfig{
		Config:            billingapp.DefaultConfig(),
		Clock:             clk,
		Logger:            logger,
		Provider:          provider,
		AdminPayments:     paymentsRepo,
		TimeTravelEnabled: true,
	})

	confirmHandlers := billinghttp.NewFakeConfirmHandlers(provider, services.Payments, logger, "https://web.test", true)
	confirmRouter := chi.NewRouter()
	confirmHandlers.MountRoutes(confirmRouter)

	return &integrationHarness{
		t:                 t,
		pool:              pool,
		clock:             clk,
		tariffs:           tariffs,
		subscriptions:     subscriptions,
		transitions:       transitions,
		payments:          paymentsRepo,
		methods:           methodsRepo,
		bindings:          bindingsRepo,
		encryptor:         encryptor,
		provider:          provider,
		services:          services,
		subscriptionsSvc:  services.Subscriptions,
		paymentsSvc:       services.Payments,
		paymentMethodsSvc: services.PaymentMethods,
		tariffsSvc:        services.Tariffs,
		onboarding:        services.Onboarding,
		limiter:           services.Limiter,
		newSubscriptionService: func(cfg billingapp.SubscriptionServiceConfig) *billingapp.SubscriptionService {
			return billingapp.NewSubscriptionService(factory, cfg)
		},
		fakeConfirms: confirmRouter,
	}
}

// confirmFakePayment completes a pending payment through the local
// confirmation endpoint (the fake provider's counterpart of the payer
// completing the payment form).
func (h *integrationHarness) confirmFakePayment(t *testing.T, paymentID uuid.UUID) {
	t.Helper()
	h.postFakeConfirm(t, strings.ReplaceAll(billinghttp.FakePaymentConfirmRoute, "{id}", paymentID.String()))
}

// confirmFakeCardBinding completes a binding session through the local
// confirmation endpoint (the fake provider's counterpart of the payer
// completing the bank form).
func (h *integrationHarness) confirmFakeCardBinding(t *testing.T, requestKey string) {
	t.Helper()
	h.postFakeConfirm(t, strings.ReplaceAll(billinghttp.FakeCardBindingConfirmRoute, "{requestKey}", requestKey))
}

// postFakeConfirm drives one local confirmation request and fails the test on
// anything but 200.
func (h *integrationHarness) postFakeConfirm(t *testing.T, path string) {
	t.Helper()
	w := httptest.NewRecorder()
	h.fakeConfirms.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, http.NoBody))
	if w.Code != http.StatusOK {
		t.Fatalf("POST %s: status = %d, want 200; body: %s", path, w.Code, w.Body.String())
	}
}

// ctx returns a background context for the harness.
func (h *integrationHarness) ctx() context.Context { return context.Background() }

// seedTariffs inserts the canonical tariff plans, mirroring the migration seed
// (issue #245). It is idempotent.
func seedTariffs(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
INSERT INTO tariffs (id, name, active_property_limit, monthly_price_kopecks, yearly_price_kopecks, is_active)
VALUES
    (uuidv7(), 'basic', 1, 0, 0, true),
    (uuidv7(), 'pro', 5, 49000, 440000, true),
    (uuidv7(), 'business', -1, 99000, 890000, true)
ON CONFLICT (name) DO NOTHING`)
	if err != nil {
		t.Fatalf("seed tariffs: %v", err)
	}
}

// seedUserCounter disambiguates seeded phones: two uuidv7 ids minted in the
// same millisecond would otherwise collide on users_phone_key.
var seedUserCounter atomic.Int64

// seedUser inserts a users row (the subscriptions FK target) and returns its id.
func (h *integrationHarness) seedUser() uuid.UUID {
	return h.seedUserWithRole(actor.RoleOwner)
}

// seedUserWithRole inserts a users row with an explicit role (the audit log's
// actor FK target); the admin-gate composition test seeds the acting admin
// this way (issue #424).
func (h *integrationHarness) seedUserWithRole(role actor.Role) uuid.UUID {
	h.t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		h.t.Fatalf("new uuid: %v", err)
	}
	phone := fmt.Sprintf("+7999%010d", seedUserCounter.Add(1)%10000000000)
	if _, err := h.pool.Exec(h.ctx(),
		"INSERT INTO users (id, phone, role) VALUES ($1, $2, $3)", id, phone, role,
	); err != nil {
		h.t.Fatalf("seed user: %v", err)
	}
	return id
}

// countRows runs a count query so tests can assert on raw table state.
func (h *integrationHarness) countRows(query string, args ...any) int {
	h.t.Helper()
	var n int
	if err := h.pool.QueryRow(h.ctx(), query, args...).Scan(&n); err != nil {
		h.t.Fatalf("count %q: %v", query, err)
	}
	return n
}
