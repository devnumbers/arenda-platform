//go:build integration

package application_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	billingapp "github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// methodIntegrationHarness extends the shared integration harness with the
// payment-method scenarios of issue #251.
type methodIntegrationHarness struct {
	*integrationHarness
}

func newMethodIntegrationHarness(t *testing.T) *methodIntegrationHarness {
	t.Helper()
	return &methodIntegrationHarness{integrationHarness: newIntegrationHarness(t)}
}

// seedMethodUser inserts an owner with a basic subscription so binding
// completion has a renewal target to link.
func (h *methodIntegrationHarness) seedMethodUser(t *testing.T) uuid.UUID {
	t.Helper()
	userID := h.seedUser()
	tariff, err := h.tariffs.GetByName(h.ctx(), domain.TariffBasic)
	if err != nil {
		t.Fatalf("GetByName(basic): %v", err)
	}
	sub, err := domain.NewBasicSubscription(userID, tariff.ID)
	if err != nil {
		t.Fatalf("NewBasicSubscription(): %v", err)
	}
	if _, err := h.subscriptions.Create(h.ctx(), sub); err != nil {
		t.Fatalf("seed subscription: %v", err)
	}
	return userID
}

// rawColumn reads one raw text column of a payment-method row so tests can
// assert on the ciphertext actually stored at rest.
func (h *methodIntegrationHarness) rawColumn(t *testing.T, methodID uuid.UUID, column string) string {
	t.Helper()
	var value string
	if err := h.pool.QueryRow(h.ctx(),
		"SELECT "+column+"::text FROM payment_methods WHERE id = $1", methodID,
	).Scan(&value); err != nil {
		t.Fatalf("read raw %s: %v", column, err)
	}
	return value
}

// TestCardBindingFlow_EndToEndOnFakeProvider proves the headline acceptance
// criterion of issue #251 against real PostgreSQL and the fake provider
// adapter: AddPaymentMethod starts a binding session (request key + TTL), the
// local confirmation — the payer completing the provider form — flows through
// the same synchronous path as the add-card webhook, and the payment method is
// created, active, linked to the subscription and encrypted at rest.
func TestCardBindingFlow_EndToEndOnFakeProvider(t *testing.T) {
	h := newMethodIntegrationHarness(t)
	userID := h.seedMethodUser(t)

	// 1. The binding session is initiated: the answer is the form URL and the
	// session row carries the provider's request key with its TTL.
	result, err := h.paymentMethodsSvc.AddPaymentMethod(h.ctx(), userID, billingapp.AddPaymentMethodRequest{})
	if err != nil {
		t.Fatalf("AddPaymentMethod(binding): %v", err)
	}
	if result.ConfirmURL == "" || result.PaymentMethod != nil {
		t.Fatalf("result = %+v, want a confirmation URL", result)
	}
	sessions, err := h.bindings.ListOpenByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("ListOpenByUserID(): %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("open sessions = %d, want 1", len(sessions))
	}
	session := sessions[0]
	if session.RequestKey == "" {
		t.Fatal("session carries no request key")
	}
	wantExpiry := h.clock.Now().Add(billingapp.DefaultConfig().CardBindingTTL)
	if !session.ExpiresAt.Equal(wantExpiry) {
		t.Errorf("expires at = %v, want %v", session.ExpiresAt, wantExpiry)
	}

	// 2. The payer completes the form: the local confirmation drives the
	// method-bound event through the synchronous webhook path.
	h.confirmFakeCardBinding(t, session.RequestKey)

	// 3. The method exists, is the single active one and the subscription
	// charges it.
	methods, err := h.methods.ListByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("ListByUserID(): %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("methods = %d, want 1", len(methods))
	}
	method := methods[0]
	if !method.IsActive {
		t.Errorf("method is not active after binding confirmation")
	}
	if method.ProviderToken == "" || method.DisplayMask == "" {
		t.Errorf("method = %+v, want charge token and display mask", method)
	}
	sub, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID(): %v", err)
	}
	if sub.ActivePaymentMethodID == nil || *sub.ActivePaymentMethodID != method.ID {
		t.Fatalf("subscription active method = %v, want %v", sub.ActivePaymentMethodID, method.ID)
	}

	// 4. The token is encrypted at rest: the raw column is ciphertext, the
	// hash column is the HMAC, and the plaintext appears nowhere.
	if raw := h.rawColumn(t, method.ID, "provider_token"); raw == method.ProviderToken {
		t.Errorf("provider_token stored as plaintext: %q", raw)
	}
	if raw := h.rawColumn(t, method.ID, "token_hash"); raw != h.encryptor.HashToken(method.ProviderToken) {
		t.Errorf("token_hash = %q, want the HMAC of the charge token", raw)
	}

	// 5. The session is completed and a repeated confirmation changes
	// nothing.
	closed, err := h.bindings.GetByRequestKeyForUpdate(h.ctx(), "fake", session.RequestKey)
	if err != nil {
		t.Fatalf("GetByRequestKeyForUpdate(): %v", err)
	}
	if closed.Status != domain.CardBindingCompleted {
		t.Errorf("session status = %q, want completed", closed.Status)
	}
	h.confirmFakeCardBinding(t, session.RequestKey)
	methods, err = h.methods.ListByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("ListByUserID(redelivery): %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("methods after redelivery = %d, want 1", len(methods))
	}
}

// TestPaymentMethodSync_CompletesOpenBindingWithoutWebhook proves the sync
// endpoint resolves an open binding by provider polling when the webhook was
// not delivered: the provider completed its side, the session completes via
// polling and the method becomes active (issue #251).
func TestPaymentMethodSync_CompletesOpenBindingWithoutWebhook(t *testing.T) {
	h := newMethodIntegrationHarness(t)
	userID := h.seedMethodUser(t)

	if _, err := h.paymentMethodsSvc.AddPaymentMethod(h.ctx(), userID, billingapp.AddPaymentMethodRequest{}); err != nil {
		t.Fatalf("AddPaymentMethod(binding): %v", err)
	}
	sessions, err := h.bindings.ListOpenByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("ListOpenByUserID(): %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("open sessions = %d, want 1", len(sessions))
	}
	requestKey := sessions[0].RequestKey

	// No confirmation happened: the poll reports pending, nothing resolves.
	methods, err := h.paymentMethodsSvc.SyncPaymentMethods(h.ctx(), userID)
	if err != nil {
		t.Fatalf("SyncPaymentMethods(pending): %v", err)
	}
	if len(methods) != 0 {
		t.Fatalf("methods = %d, want 0 while the binding is pending", len(methods))
	}
	open, err := h.bindings.GetByRequestKeyForUpdate(h.ctx(), "fake", requestKey)
	if err != nil {
		t.Fatalf("GetByRequestKeyForUpdate(): %v", err)
	}
	if open.Status != domain.CardBindingNew {
		t.Fatalf("session status = %q, want still new", open.Status)
	}

	// The provider completes its side but the webhook never arrives (the
	// state is programmed directly, exactly like a lost delivery): the sync
	// must heal the flow by polling.
	h.provider.SetBindingState(requestKey, billingapp.MethodBindingState{
		Status: billingapp.MethodBindingCompleted,
		Method: &billingapp.SavedMethod{
			ProviderMethodID: "card_healed",
			ChargeToken:      "token_healed",
			MaskedPan:        "4222********2222",
			ExpDate:          "1230",
		},
	})

	methods, err = h.paymentMethodsSvc.SyncPaymentMethods(h.ctx(), userID)
	if err != nil {
		t.Fatalf("SyncPaymentMethods(completed): %v", err)
	}
	if len(methods) != 1 || !methods[0].IsActive {
		t.Fatalf("methods = %+v, want one active healed by polling", methods)
	}
	if methods[0].ProviderToken != "token_healed" {
		t.Errorf("charge token = %q, want the polled method's", methods[0].ProviderToken)
	}
	sub, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID(): %v", err)
	}
	if sub.ActivePaymentMethodID == nil || *sub.ActivePaymentMethodID != methods[0].ID {
		t.Fatalf("subscription active method = %v, want %v", sub.ActivePaymentMethodID, methods[0].ID)
	}
	closed, err := h.bindings.GetByRequestKeyForUpdate(h.ctx(), "fake", requestKey)
	if err != nil {
		t.Fatalf("GetByRequestKeyForUpdate(): %v", err)
	}
	if closed.Status != domain.CardBindingCompleted {
		t.Errorf("session status = %q, want completed", closed.Status)
	}

	// A repeated sync is an idempotent no-op.
	methods, err = h.paymentMethodsSvc.SyncPaymentMethods(h.ctx(), userID)
	if err != nil {
		t.Fatalf("SyncPaymentMethods(repeat): %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("methods after repeated sync = %d, want 1", len(methods))
	}
}

// TestPaymentMethodRepository_DuplicateTokenConvergesByHash proves the
// UNIQUE (user_id, token_hash) invariant at the database level: re-binding
// the same card converges on one row instead of duplicating (issue #251 AC).
func TestPaymentMethodRepository_DuplicateTokenConvergesByHash(t *testing.T) {
	h := newMethodIntegrationHarness(t)
	userID := h.seedMethodUser(t)

	first, err := domain.NewPaymentMethod(userID, "fake", "token_same", h.clock.Now())
	if err != nil {
		t.Fatalf("NewPaymentMethod(): %v", err)
	}
	first.ProviderCardID = "card_1"
	first.DisplayMask = "4111********1111"
	saved, err := h.methods.UpsertByTokenHash(h.ctx(), first)
	if err != nil {
		t.Fatalf("UpsertByTokenHash(first): %v", err)
	}

	second, err := domain.NewPaymentMethod(userID, "fake", "token_same", h.clock.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("NewPaymentMethod(): %v", err)
	}
	second.ProviderCardID = "card_2"
	converged, err := h.methods.UpsertByTokenHash(h.ctx(), second)
	if err != nil {
		t.Fatalf("UpsertByTokenHash(duplicate): %v", err)
	}
	if converged.ID != saved.ID {
		t.Fatalf("duplicate upsert created a second row: %s vs %s", converged.ID, saved.ID)
	}
	if converged.ProviderCardID != "card_2" {
		t.Errorf("card id = %q, want refreshed to card_2", converged.ProviderCardID)
	}

	list, err := h.methods.ListByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("ListByUserID(): %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("methods = %d, want 1", len(list))
	}
	if got := h.countRows("SELECT COUNT(*) FROM payment_methods WHERE user_id = $1", userID); got != 1 {
		t.Fatalf("payment_methods rows = %d, want 1", got)
	}
}

// TestPaymentMethodRepository_OneActivePerUserAndDeleteGuards proves the
// partial unique index keeps exactly one active method per user, activation
// switches atomically, and the subscription FK blocks deleting the referenced
// method (issue #251 AC).
func TestPaymentMethodRepository_OneActivePerUserAndDeleteGuards(t *testing.T) {
	h := newMethodIntegrationHarness(t)
	userID := h.seedMethodUser(t)

	first, err := domain.NewPaymentMethod(userID, "fake", "token_1", h.clock.Now())
	if err != nil {
		t.Fatalf("NewPaymentMethod(): %v", err)
	}
	first, err = h.methods.UpsertByTokenHash(h.ctx(), first)
	if err != nil {
		t.Fatalf("UpsertByTokenHash(first): %v", err)
	}
	second, err := domain.NewPaymentMethod(userID, "fake", "token_2", h.clock.Now())
	if err != nil {
		t.Fatalf("NewPaymentMethod(): %v", err)
	}
	second, err = h.methods.UpsertByTokenHash(h.ctx(), second)
	if err != nil {
		t.Fatalf("UpsertByTokenHash(second): %v", err)
	}

	if err := h.methods.SetActive(h.ctx(), userID, first.ID); err != nil {
		t.Fatalf("SetActive(first): %v", err)
	}
	if err := h.methods.SetActive(h.ctx(), userID, second.ID); err != nil {
		t.Fatalf("SetActive(second): %v", err)
	}
	if got := h.countRows(
		"SELECT COUNT(*) FROM payment_methods WHERE user_id = $1 AND is_active", userID,
	); got != 1 {
		t.Fatalf("active methods = %d, want exactly 1", got)
	}
	reloaded, err := h.methods.GetByID(h.ctx(), first.ID)
	if err != nil {
		t.Fatalf("GetByID(first): %v", err)
	}
	if reloaded.IsActive {
		t.Error("first method must be inactive after the switch")
	}

	// Link the subscription to the active method; the FK must protect it.
	sub, err := h.subscriptions.GetByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("GetByUserID(): %v", err)
	}
	sub.SetActivePaymentMethod(second.ID)
	if err := h.subscriptions.Update(h.ctx(), sub); err != nil {
		t.Fatalf("Update(subscription): %v", err)
	}
	if err := h.methods.Delete(h.ctx(), userID, second.ID); err == nil {
		t.Fatal("deleting the subscription-referenced method must fail")
	}

	// The unreferenced inactive method deletes cleanly.
	if err := h.methods.Delete(h.ctx(), userID, first.ID); err != nil {
		t.Fatalf("Delete(inactive): %v", err)
	}
}

// TestCardBindingSessionRepository_RoundTrip proves the session persistence
// port against real PostgreSQL: create with TTL, lock by request key, list
// open sessions per user, and the status transition (issue #251).
func TestCardBindingSessionRepository_RoundTrip(t *testing.T) {
	h := newMethodIntegrationHarness(t)
	userID := h.seedMethodUser(t)
	other := h.seedMethodUser(t)
	now := h.clock.Now()

	session, err := domain.NewCardBindingSession(userID, "fake", "req_key_1", now.Add(24*time.Hour), now)
	if err != nil {
		t.Fatalf("NewCardBindingSession(): %v", err)
	}
	if _, err := h.bindings.Create(h.ctx(), session); err != nil {
		t.Fatalf("Create(): %v", err)
	}

	// The request key is unique per provider.
	dup, err := domain.NewCardBindingSession(other, "fake", "req_key_1", now.Add(24*time.Hour), now)
	if err != nil {
		t.Fatalf("NewCardBindingSession(): %v", err)
	}
	if _, err := h.bindings.Create(h.ctx(), dup); err == nil {
		t.Fatal("duplicate (provider, request_key) must be rejected")
	}

	loaded, err := h.bindings.GetByRequestKeyForUpdate(h.ctx(), "fake", "req_key_1")
	if err != nil {
		t.Fatalf("GetByRequestKeyForUpdate(): %v", err)
	}
	if loaded.UserID != userID || loaded.Status != domain.CardBindingNew {
		t.Fatalf("loaded = %+v, want the created session", loaded)
	}

	open, err := h.bindings.ListOpenByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("ListOpenByUserID(): %v", err)
	}
	if len(open) != 1 || open[0].RequestKey != "req_key_1" {
		t.Fatalf("open sessions = %+v, want the created one", open)
	}

	if err := loaded.MarkCompleted(now.Add(time.Minute)); err != nil {
		t.Fatalf("MarkCompleted(): %v", err)
	}
	if err := h.bindings.UpdateStatus(h.ctx(), loaded); err != nil {
		t.Fatalf("UpdateStatus(): %v", err)
	}
	open, err = h.bindings.ListOpenByUserID(h.ctx(), userID)
	if err != nil {
		t.Fatalf("ListOpenByUserID(after completion): %v", err)
	}
	if len(open) != 0 {
		t.Fatalf("open sessions after completion = %d, want 0", len(open))
	}
	if _, err := h.bindings.GetByRequestKeyForUpdate(h.ctx(), "tkassa", "req_key_1"); err == nil {
		t.Fatal("request key lookup must be provider-scoped")
	}
}
