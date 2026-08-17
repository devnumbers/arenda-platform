package application

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

// stubMethodProvider is a configurable provider for the payment-method tests:
// it satisfies the consumer-side provider slices of PaymentMethodService and
// PaymentService — including the fake card-binding confirmation hook — with
// injectable outcomes.
type stubMethodProvider struct {
	mu sync.Mutex

	bindCalls int
	bindRes   BindMethodResult
	bindErr   error

	pollCalls int
	pollState MethodBindingState
	pollErr   error

	removeCalls int
	removeErr   error

	// startedBindings records the request keys BindPaymentMethod handed out,
	// so ConfirmCardBinding can distinguish started from unknown bindings.
	startedBindings map[string]bool

	parseEvent WebhookEvent
}

func newStubMethodProvider() *stubMethodProvider {
	return &stubMethodProvider{startedBindings: make(map[string]bool)}
}

func (p *stubMethodProvider) Name() domain.PaymentProvider { return "fake" }

func (p *stubMethodProvider) BindPaymentMethod(_ context.Context, _ BindMethodRequest) (BindMethodResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.bindCalls++
	if p.bindRes.BindingID != "" && p.bindErr == nil {
		p.startedBindings[p.bindRes.BindingID] = true
	}
	return p.bindRes, p.bindErr
}

func (p *stubMethodProvider) PaymentMethodBinding(_ context.Context, _ string) (MethodBindingState, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pollCalls++
	return p.pollState, p.pollErr
}

func (p *stubMethodProvider) PaymentStatus(_ context.Context, _ uuid.UUID, _ string) (PaymentStatusResult, error) {
	return PaymentStatusResult{}, nil
}

// RefundPayment satisfies the payment-finalizer slice the payment service
// consumes; the method-method tests never refund, so it answers a plain
// failure.
func (p *stubMethodProvider) RefundPayment(_ context.Context, req RefundRequest) (RefundResult, error) {
	_ = req
	return RefundResult{}, errors.New("stubMethodProvider: refund is not programmed")
}

func (p *stubMethodProvider) AddPaymentMethodFromToken(_ context.Context, customerRef, token string) (SavedMethod, error) {
	if customerRef == "" || token == "" {
		return SavedMethod{}, errors.New("stub: customer ref and token are required")
	}
	return SavedMethod{
		ProviderMethodID: "card_from_token",
		ChargeToken:      token,
		MaskedPan:        "4111********1111",
		ExpDate:          "1230",
		CustomerRef:      customerRef,
	}, nil
}

func (p *stubMethodProvider) RemovePaymentMethod(_ context.Context, _, _ string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.removeCalls++
	return p.removeErr
}

func (p *stubMethodProvider) ParseWebhook(_ context.Context, _ []byte) (WebhookEvent, error) {
	return p.parseEvent, nil
}

func (p *stubMethodProvider) WebhookAck() []byte { return []byte(`{"status":"ok"}`) }

// Compile-time checks against the consumer-side provider slices.
var (
	_ methodBindingProvider    = (*stubMethodProvider)(nil)
	_ paymentFinalizerProvider = (*stubMethodProvider)(nil)
)

// methodHarness wires the payment-method and payment services over the
// in-memory stores, a capture audit recorder, a fixed clock and the stub
// provider (issue #251).
type methodHarness struct {
	methods  *PaymentMethodService
	payments *PaymentService
	subs     *SubscriptionService
	provider *stubMethodProvider
	stores   *fakeStores
	audit    *captureRecorder
	now      time.Time
	tariffs  []domain.Tariff
}

func newMethodHarness(t *testing.T) *methodHarness {
	t.Helper()
	tariffs := testTariffs()
	stores := newFakeStores(tariffs...)
	audit := &captureRecorder{}
	now := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	factory := stores.factory(audit)
	provider := newStubMethodProvider()
	cfg := Config{CardBindingTTL: 24 * time.Hour}
	return &methodHarness{
		methods: NewPaymentMethodService(factory, provider, PaymentMethodServiceConfig{
			Config: cfg,
			Clock:  fakeClock{now: now},
			Log:    slog.New(slog.DiscardHandler),
		}),
		payments: NewPaymentService(factory, provider, PaymentServiceConfig{Clock: fakeClock{now: now}, Log: slog.New(slog.DiscardHandler)}),
		subs: NewSubscriptionService(factory, SubscriptionServiceConfig{
			Clock:  fakeClock{now: now},
			Config: cfg,
			Logger: slog.New(slog.DiscardHandler),
		}),
		provider: provider,
		stores:   stores,
		audit:    audit,
		now:      now,
		tariffs:  tariffs,
	}
}

// seedMethodSubscription creates a subscription for a fresh user so binding
// completion has a renewal target to link.
func (h *methodHarness) seedMethodSubscription(t *testing.T) (uuid.UUID, domain.Subscription) {
	t.Helper()
	userID := uuid.Must(uuid.NewV7())
	sub, err := domain.NewBasicSubscription(userID, h.tariffs[0].ID)
	if err != nil {
		t.Fatalf("NewBasicSubscription() error = %v", err)
	}
	if _, err := h.stores.subscriptions.Create(t.Context(), sub); err != nil {
		t.Fatalf("seed Create() error = %v", err)
	}
	return userID, sub
}

// openSession starts a binding through the service with the stub provider and
// returns the persisted session.
func (h *methodHarness) openSession(t *testing.T, userID uuid.UUID) domain.CardBindingSession {
	t.Helper()
	h.provider.bindRes = BindMethodResult{
		FormURL:   "https://pay.example/bind/" + userID.String(),
		BindingID: "req_" + userID.String(),
	}
	result, err := h.methods.AddPaymentMethod(t.Context(), userID, AddPaymentMethodRequest{})
	if err != nil {
		t.Fatalf("AddPaymentMethod(binding) error = %v", err)
	}
	if result.ConfirmURL == "" || result.PaymentMethod != nil {
		t.Fatalf("AddPaymentMethod result = %+v, want confirm url only", result)
	}
	session, err := h.stores.bindings.GetByRequestKeyForUpdate(t.Context(), "fake", "req_"+userID.String())
	if err != nil {
		t.Fatalf("GetByRequestKeyForUpdate() error = %v", err)
	}
	return session
}

// methodBoundEvent prepares the provider stub to parse into a method-bound
// notification and returns it.
func (h *methodHarness) methodBoundEvent(bindingID string) *MethodBoundNotification {
	n := &MethodBoundNotification{
		BindingID: bindingID,
		Method: SavedMethod{
			ProviderMethodID: "card_" + bindingID,
			ChargeToken:      "token_" + bindingID,
			MaskedPan:        "4111********1111",
			ExpDate:          "1230",
			CustomerRef:      "ignored",
		},
	}
	h.provider.parseEvent = WebhookEvent{MethodBound: n}
	return n
}

// activeMethod returns the user's single active method or fails when the
// invariant is violated.
func (h *methodHarness) activeMethod(t *testing.T, userID uuid.UUID) domain.PaymentMethod {
	t.Helper()
	list, err := h.stores.methods.ListByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("ListByUserID() error = %v", err)
	}
	var active []domain.PaymentMethod
	for _, m := range list {
		if m.IsActive {
			active = append(active, m)
		}
	}
	if len(active) != 1 {
		t.Fatalf("active methods = %d, want exactly 1 (list: %+v)", len(active), list)
	}
	return active[0]
}

// TestAddPaymentMethod_TokenCreatesActivatesAndLinks proves the synchronous
// token path reaches the binding end state directly: the method is saved, is
// the single active one, and the subscription charges it. Migrated from the
// pre-rewrite test TestBilling_AddPaymentMethod.
func TestAddPaymentMethod_TokenCreatesActivatesAndLinks(t *testing.T) {
	h := newMethodHarness(t)
	userID, sub := h.seedMethodSubscription(t)

	result, err := h.methods.AddPaymentMethod(t.Context(), userID, AddPaymentMethodRequest{ProviderToken: "raw_token_1"})
	if err != nil {
		t.Fatalf("AddPaymentMethod(token) error = %v", err)
	}
	if result.PaymentMethod == nil || result.ConfirmURL != "" {
		t.Fatalf("result = %+v, want payment method only", result)
	}
	if result.PaymentMethod.ProviderToken != "raw_token_1" {
		t.Errorf("provider token = %q, want the raw token", result.PaymentMethod.ProviderToken)
	}
	if result.PaymentMethod.ProviderCardID == "" || result.PaymentMethod.DisplayMask == "" {
		t.Errorf("method = %+v, want the provider-issued card fields", result.PaymentMethod)
	}

	active := h.activeMethod(t, userID)
	if active.ID != result.PaymentMethod.ID {
		t.Errorf("active method = %s, want the created one", active.ID)
	}
	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.ActivePaymentMethodID == nil || *stored.ActivePaymentMethodID != active.ID {
		t.Fatalf("subscription active method = %v, want %v (was %v)", stored.ActivePaymentMethodID, active.ID, sub.ActivePaymentMethodID)
	}

	entries := h.audit.recorded()
	if len(entries) != 1 || entries[0].Action != "payment_method.added" {
		t.Fatalf("audit entries = %+v, want one payment_method.added", entries)
	}
}

// bankFormStubProvider hides the raw-token capability the way a bank-form
// provider (tkassa) structurally does: it delegates everything the shared stub
// offers but does not implement RawTokenMethodAcceptor.
type bankFormStubProvider struct{ stub *stubMethodProvider }

func (p bankFormStubProvider) Name() domain.PaymentProvider { return p.stub.Name() }

func (p bankFormStubProvider) BindPaymentMethod(ctx context.Context, req BindMethodRequest) (BindMethodResult, error) {
	return p.stub.BindPaymentMethod(ctx, req)
}

func (p bankFormStubProvider) PaymentMethodBinding(ctx context.Context, bindingID string) (MethodBindingState, error) {
	return p.stub.PaymentMethodBinding(ctx, bindingID)
}

func (p bankFormStubProvider) PaymentStatus(ctx context.Context, paymentID uuid.UUID, providerPaymentID string) (PaymentStatusResult, error) {
	return p.stub.PaymentStatus(ctx, paymentID, providerPaymentID)
}

func (p bankFormStubProvider) RemovePaymentMethod(ctx context.Context, customerRef, providerMethodID string) error {
	return p.stub.RemovePaymentMethod(ctx, customerRef, providerMethodID)
}

func (p bankFormStubProvider) ParseWebhook(ctx context.Context, payload []byte) (WebhookEvent, error) {
	return p.stub.ParseWebhook(ctx, payload)
}

func (p bankFormStubProvider) WebhookAck() []byte { return p.stub.WebhookAck() }

// TestAddPaymentMethod_TokenIgnoredByBankFormProvider proves the contract line
// "providerToken ... Ignored for providers that use a bank-form flow such as
// T-Kassa": without the raw-token capability the token never becomes a
// chargeable method — the binding flow runs instead, so a client-supplied
// token cannot inject an arbitrary charge target.
func TestAddPaymentMethod_TokenIgnoredByBankFormProvider(t *testing.T) {
	h := newMethodHarness(t)
	userID, _ := h.seedMethodSubscription(t)
	h.provider.bindRes = BindMethodResult{
		FormURL:   "https://pay.example/bind/1",
		BindingID: "req_bank_form",
	}
	methods := NewPaymentMethodService(
		h.stores.factory(h.audit),
		bankFormStubProvider{stub: h.provider},
		PaymentMethodServiceConfig{
			Config: Config{CardBindingTTL: 24 * time.Hour},
			Clock:  fakeClock{now: h.now},
			Log:    slog.New(slog.DiscardHandler),
		},
	)

	result, err := methods.AddPaymentMethod(t.Context(), userID, AddPaymentMethodRequest{ProviderToken: "forged_token"})
	if err != nil {
		t.Fatalf("AddPaymentMethod() error = %v", err)
	}
	if result.PaymentMethod != nil || result.ConfirmURL == "" {
		t.Fatalf("result = %+v, want the binding branch (no method from the raw token)", result)
	}

	stored, err := h.stores.methods.ListByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("ListByUserID() error = %v", err)
	}
	if len(stored) != 0 {
		t.Fatalf("methods = %d, want 0 (the raw token must not create a method)", len(stored))
	}
	sessions, err := h.stores.bindings.ListOpenByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("ListOpenByUserID() error = %v", err)
	}
	if len(sessions) != 1 || sessions[0].RequestKey != "req_bank_form" {
		t.Fatalf("sessions = %+v, want the started binding session", sessions)
	}
}

// TestAddPaymentMethod_BindingStartsSessionWithTTL proves the bank-form path
// (issue #251): the provider binding is initiated with the user as customer
// reference, the session row carries the request key and the configured TTL,
// and the confirmation URL is the answer. Migrated from the pre-rewrite tests
// TestBilling_AddPaymentMethod_TkassaReturnsConfirmURL and
// TestBilling_AddPaymentMethod_TkassaPersistsPendingBinding — the placeholder
// row is now a first-class session.
func TestAddPaymentMethod_BindingStartsSessionWithTTL(t *testing.T) {
	h := newMethodHarness(t)
	userID, _ := h.seedMethodSubscription(t)

	session := h.openSession(t, userID)

	if session.UserID != userID || session.Provider != "fake" {
		t.Errorf("session = user %v provider %q, want the caller's", session.UserID, session.Provider)
	}
	if session.Status != domain.CardBindingNew {
		t.Errorf("session status = %q, want new", session.Status)
	}
	wantExpiry := h.now.Add(24 * time.Hour)
	if !session.ExpiresAt.Equal(wantExpiry) {
		t.Errorf("expires at = %v, want %v (CardBindingTTL)", session.ExpiresAt, wantExpiry)
	}
	h.provider.mu.Lock()
	calls := h.provider.bindCalls
	h.provider.mu.Unlock()
	if calls != 1 {
		t.Fatalf("provider BindPaymentMethod calls = %d, want 1", calls)
	}

	methods, err := h.stores.methods.ListByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("ListByUserID() error = %v", err)
	}
	if len(methods) != 0 {
		t.Errorf("methods after binding start = %d, want 0 (nothing before confirmation)", len(methods))
	}
}

// TestWebhook_MethodBoundCreatesActivatesAndLinks proves the add-card branch
// of the synchronous webhook (issue #251): one delivery creates the method
// from the notification, activates it, links the subscription and closes the
// session. Migrated from the pre-rewrite tests
// TestBilling_HandleWebhook_TkassaAddCard and
// TestBilling_HandleWebhook_AddCardLinksActivePaymentMethodToSubscription.
func TestWebhook_MethodBoundCreatesActivatesAndLinks(t *testing.T) {
	h := newMethodHarness(t)
	userID, _ := h.seedMethodSubscription(t)
	session := h.openSession(t, userID)
	n := h.methodBoundEvent(session.RequestKey)

	if err := h.payments.HandleWebhook(t.Context(), "fake", []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook() error = %v", err)
	}

	active := h.activeMethod(t, userID)
	if active.ProviderToken != n.Method.ChargeToken {
		t.Errorf("charge token = %q, want the notification's", active.ProviderToken)
	}
	if active.ProviderCardID != n.Method.ProviderMethodID || active.DisplayMask != n.Method.MaskedPan || active.ExpDate != n.Method.ExpDate {
		t.Errorf("method = %+v, want the notification's card fields", active)
	}
	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.ActivePaymentMethodID == nil || *stored.ActivePaymentMethodID != active.ID {
		t.Fatalf("subscription active method = %v, want %v", stored.ActivePaymentMethodID, active.ID)
	}
	closed, err := h.stores.bindings.GetByRequestKeyForUpdate(t.Context(), "fake", session.RequestKey)
	if err != nil {
		t.Fatalf("GetByRequestKeyForUpdate() error = %v", err)
	}
	if closed.Status != domain.CardBindingCompleted {
		t.Errorf("session status = %q, want completed", closed.Status)
	}
	if entries := h.audit.recorded(); len(entries) != 1 || entries[0].Action != "payment_method.added" {
		t.Fatalf("audit entries = %+v, want one payment_method.added", entries)
	}
}

// TestWebhook_MethodBoundRedeliveryIsIdempotent proves a repeated add-card
// delivery is a no-op: one method row, one active method, no extra audit
// entry (issue #251 AC).
func TestWebhook_MethodBoundRedeliveryIsIdempotent(t *testing.T) {
	h := newMethodHarness(t)
	userID, _ := h.seedMethodSubscription(t)
	session := h.openSession(t, userID)
	h.methodBoundEvent(session.RequestKey)

	for range 2 {
		if err := h.payments.HandleWebhook(t.Context(), "fake", []byte(`{}`)); err != nil {
			t.Fatalf("HandleWebhook() error = %v", err)
		}
	}

	list, err := h.stores.methods.ListByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("ListByUserID() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("methods after redelivery = %d, want 1", len(list))
	}
	h.activeMethod(t, userID) // asserts exactly one active
	if entries := h.audit.recorded(); len(entries) != 1 {
		t.Fatalf("audit entries = %d, want 1 (no duplicate on redelivery)", len(entries))
	}
}

// TestWebhook_MethodBoundExpiredSessionCreatesNoCard proves a session past
// its TTL never produces a payment method, whatever the provider reports
// (issue #251 AC): the session is rejected and the webhook answered as
// processed.
func TestWebhook_MethodBoundExpiredSessionCreatesNoCard(t *testing.T) {
	h := newMethodHarness(t)
	userID, _ := h.seedMethodSubscription(t)
	session := h.openSession(t, userID)
	// Expire the session behind the service's back.
	expired := session
	expired.ExpiresAt = h.now.Add(-time.Minute)
	if err := h.stores.bindings.UpdateStatus(t.Context(), expired); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	h.methodBoundEvent(session.RequestKey)

	if err := h.payments.HandleWebhook(t.Context(), "fake", []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook() error = %v (expired sessions answer processed)", err)
	}

	methods, err := h.stores.methods.ListByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("ListByUserID() error = %v", err)
	}
	if len(methods) != 0 {
		t.Fatalf("methods = %d, want 0 from an expired session", len(methods))
	}
	closed, err := h.stores.bindings.GetByRequestKeyForUpdate(t.Context(), "fake", session.RequestKey)
	if err != nil {
		t.Fatalf("GetByRequestKeyForUpdate() error = %v", err)
	}
	if closed.Status != domain.CardBindingRejected {
		t.Errorf("session status = %q, want rejected", closed.Status)
	}
}

// TestWebhook_MethodBoundUnknownRequestKeyIsProcessedNoop proves a
// notification no local session matches is answered as processed (a retry
// cannot fix it) and creates nothing.
func TestWebhook_MethodBoundUnknownRequestKeyIsProcessedNoop(t *testing.T) {
	h := newMethodHarness(t)
	userID, _ := h.seedMethodSubscription(t)
	h.methodBoundEvent("req_never_initiated")

	if err := h.payments.HandleWebhook(t.Context(), "fake", []byte(`{}`)); err != nil {
		t.Fatalf("HandleWebhook() error = %v, want processed no-op", err)
	}

	methods, err := h.stores.methods.ListByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("ListByUserID() error = %v", err)
	}
	if len(methods) != 0 {
		t.Fatalf("methods = %d, want 0", len(methods))
	}
}

// TestSyncPaymentMethods_CompletesOpenBinding proves the polling path resolves
// a completed binding exactly like the webhook would (issue #251): the method
// is created and active, the subscription points at it and the session is
// closed. Migrated from the pre-rewrite
// TestBilling_SyncPaymentMethods_CompletesPendingBindingViaGetAddCardState.
func TestSyncPaymentMethods_CompletesOpenBinding(t *testing.T) {
	h := newMethodHarness(t)
	userID, _ := h.seedMethodSubscription(t)
	session := h.openSession(t, userID)
	h.provider.pollState = MethodBindingState{
		Status: MethodBindingCompleted,
		Method: &SavedMethod{
			ProviderMethodID: "card_1",
			ChargeToken:      "token_1",
			MaskedPan:        "4111********1111",
		},
	}

	methods, err := h.methods.SyncPaymentMethods(t.Context(), userID)
	if err != nil {
		t.Fatalf("SyncPaymentMethods() error = %v", err)
	}
	if len(methods) != 1 || !methods[0].IsActive {
		t.Fatalf("methods = %+v, want one active", methods)
	}
	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.ActivePaymentMethodID == nil || *stored.ActivePaymentMethodID != methods[0].ID {
		t.Fatalf("subscription active method = %v, want %v", stored.ActivePaymentMethodID, methods[0].ID)
	}
	closed, err := h.stores.bindings.GetByRequestKeyForUpdate(t.Context(), "fake", session.RequestKey)
	if err != nil {
		t.Fatalf("GetByRequestKeyForUpdate() error = %v", err)
	}
	if closed.Status != domain.CardBindingCompleted {
		t.Errorf("session status = %q, want completed", closed.Status)
	}
}

// TestSyncPaymentMethods_RejectedBindingClosed proves a provider-rejected
// binding closes its session without a payment method. Migrated from the
// pre-rewrite TestBilling_SyncPaymentMethods_RejectedBindingDropped.
func TestSyncPaymentMethods_RejectedBindingClosed(t *testing.T) {
	h := newMethodHarness(t)
	userID, _ := h.seedMethodSubscription(t)
	session := h.openSession(t, userID)
	h.provider.pollState = MethodBindingState{Status: MethodBindingFailed, ErrorCode: "53"}

	if _, err := h.methods.SyncPaymentMethods(t.Context(), userID); err != nil {
		t.Fatalf("SyncPaymentMethods() error = %v", err)
	}

	closed, err := h.stores.bindings.GetByRequestKeyForUpdate(t.Context(), "fake", session.RequestKey)
	if err != nil {
		t.Fatalf("GetByRequestKeyForUpdate() error = %v", err)
	}
	if closed.Status != domain.CardBindingRejected {
		t.Errorf("session status = %q, want rejected", closed.Status)
	}
	methods, err := h.stores.methods.ListByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("ListByUserID() error = %v", err)
	}
	if len(methods) != 0 {
		t.Fatalf("methods = %d, want 0 from a rejected binding", len(methods))
	}
}

// TestSyncPaymentMethods_ExpiredSessionClosedWithoutPolling proves an expired
// session is closed locally without a provider call: the form is gone, the
// binding can never complete. Migrated from the pre-rewrite
// TestBilling_SyncPaymentMethods_ExpiredPlaceholderDroppedFreshKept.
func TestSyncPaymentMethods_ExpiredSessionClosedWithoutPolling(t *testing.T) {
	h := newMethodHarness(t)
	userID, _ := h.seedMethodSubscription(t)
	fresh := h.openSession(t, userID)
	expired, err := domain.NewCardBindingSession(userID, "fake", "req_expired", h.now.Add(-time.Hour), h.now.Add(-time.Minute))
	if err != nil {
		t.Fatalf("NewCardBindingSession() error = %v", err)
	}
	if _, err := h.stores.bindings.Create(t.Context(), expired); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	h.provider.pollState = MethodBindingState{Status: MethodBindingPending}

	if _, err := h.methods.SyncPaymentMethods(t.Context(), userID); err != nil {
		t.Fatalf("SyncPaymentMethods() error = %v", err)
	}

	h.provider.mu.Lock()
	polls := h.provider.pollCalls
	h.provider.mu.Unlock()
	if polls != 1 {
		t.Fatalf("provider polls = %d, want 1 (only the fresh session)", polls)
	}
	closedExpired, err := h.stores.bindings.GetByRequestKeyForUpdate(t.Context(), "fake", "req_expired")
	if err != nil {
		t.Fatalf("GetByRequestKeyForUpdate(expired) error = %v", err)
	}
	if closedExpired.Status != domain.CardBindingRejected {
		t.Errorf("expired session status = %q, want rejected", closedExpired.Status)
	}
	openFresh, err := h.stores.bindings.GetByRequestKeyForUpdate(t.Context(), "fake", fresh.RequestKey)
	if err != nil {
		t.Fatalf("GetByRequestKeyForUpdate(fresh) error = %v", err)
	}
	if openFresh.Status != domain.CardBindingNew {
		t.Errorf("fresh session status = %q, want still new (pending at the provider)", openFresh.Status)
	}
}

// TestSyncPaymentMethods_UnknownRequestKeyClosesSession proves a binding the
// provider has forgotten (error 502 semantics) is closed as expired.
// Migrated from the pre-rewrite
// TestBilling_SyncPaymentMethods_UnknownRequestKeyDroppedAsExpired.
func TestSyncPaymentMethods_UnknownRequestKeyClosesSession(t *testing.T) {
	h := newMethodHarness(t)
	userID, _ := h.seedMethodSubscription(t)
	session := h.openSession(t, userID)
	h.provider.pollErr = ErrProviderBindingNotFound

	if _, err := h.methods.SyncPaymentMethods(t.Context(), userID); err != nil {
		t.Fatalf("SyncPaymentMethods() error = %v", err)
	}

	closed, err := h.stores.bindings.GetByRequestKeyForUpdate(t.Context(), "fake", session.RequestKey)
	if err != nil {
		t.Fatalf("GetByRequestKeyForUpdate() error = %v", err)
	}
	if closed.Status != domain.CardBindingRejected {
		t.Errorf("session status = %q, want rejected", closed.Status)
	}
}

// TestSyncPaymentMethods_TransientPollErrorKeepsSessionOpen proves a provider
// outage keeps the session resolvable: a later sync (or the webhook) can
// still complete it.
func TestSyncPaymentMethods_TransientPollErrorKeepsSessionOpen(t *testing.T) {
	h := newMethodHarness(t)
	userID, _ := h.seedMethodSubscription(t)
	session := h.openSession(t, userID)
	h.provider.pollErr = errors.New("provider is down")

	if _, err := h.methods.SyncPaymentMethods(t.Context(), userID); err != nil {
		t.Fatalf("SyncPaymentMethods() error = %v (a poll failure must not fail the sync)", err)
	}

	open, err := h.stores.bindings.GetByRequestKeyForUpdate(t.Context(), "fake", session.RequestKey)
	if err != nil {
		t.Fatalf("GetByRequestKeyForUpdate() error = %v", err)
	}
	if open.Status != domain.CardBindingNew {
		t.Errorf("session status = %q, want still new", open.Status)
	}
}

// TestActivatePaymentMethod_SwitchesSingleActiveAndLinks proves explicit
// activation switches the single active method and repoints the subscription
// (issue #251 AC: exactly one active). Migrated from the pre-rewrite
// TestBilling_SetActivePaymentMethodUsesTransaction.
func TestActivatePaymentMethod_SwitchesSingleActiveAndLinks(t *testing.T) {
	h := newMethodHarness(t)
	userID, _ := h.seedMethodSubscription(t)
	first, err := h.methods.AddPaymentMethod(t.Context(), userID, AddPaymentMethodRequest{ProviderToken: "token_first"})
	if err != nil {
		t.Fatalf("AddPaymentMethod(first) error = %v", err)
	}
	second, err := h.methods.AddPaymentMethod(t.Context(), userID, AddPaymentMethodRequest{ProviderToken: "token_second"})
	if err != nil {
		t.Fatalf("AddPaymentMethod(second) error = %v", err)
	}

	if err := h.methods.ActivatePaymentMethod(t.Context(), userID, first.PaymentMethod.ID); err != nil {
		t.Fatalf("ActivatePaymentMethod(first) error = %v", err)
	}
	active := h.activeMethod(t, userID)
	if active.ID != first.PaymentMethod.ID {
		t.Fatalf("active = %s, want first", active.ID)
	}

	if err := h.methods.ActivatePaymentMethod(t.Context(), userID, second.PaymentMethod.ID); err != nil {
		t.Fatalf("ActivatePaymentMethod(second) error = %v", err)
	}
	active = h.activeMethod(t, userID)
	if active.ID != second.PaymentMethod.ID {
		t.Fatalf("active = %s, want second after switch", active.ID)
	}
	stored, err := h.stores.subscriptions.GetByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("GetByUserID() error = %v", err)
	}
	if stored.ActivePaymentMethodID == nil || *stored.ActivePaymentMethodID != second.PaymentMethod.ID {
		t.Fatalf("subscription active method = %v, want second", stored.ActivePaymentMethodID)
	}

	entries := h.audit.recorded()
	activated := 0
	for _, e := range entries {
		if e.Action == "payment_method.activated" {
			activated++
		}
	}
	if activated != 2 {
		t.Errorf("activation audit entries = %d, want 2", activated)
	}
}

// TestActivatePaymentMethod_MissingOrForeignIsNotFound proves activation of a
// missing method or a method of another user answers ErrPaymentMethodNotFound.
func TestActivatePaymentMethod_MissingOrForeignIsNotFound(t *testing.T) {
	h := newMethodHarness(t)
	userID, _ := h.seedMethodSubscription(t)
	other, _ := h.seedMethodSubscription(t)
	owned, err := h.methods.AddPaymentMethod(t.Context(), other, AddPaymentMethodRequest{ProviderToken: "token_other"})
	if err != nil {
		t.Fatalf("AddPaymentMethod(other) error = %v", err)
	}

	if err := h.methods.ActivatePaymentMethod(t.Context(), userID, uuid.Must(uuid.NewV7())); !errors.Is(err, ErrPaymentMethodNotFound) {
		t.Errorf("activate missing: err = %v, want ErrPaymentMethodNotFound", err)
	}
	if err := h.methods.ActivatePaymentMethod(t.Context(), userID, owned.PaymentMethod.ID); !errors.Is(err, ErrPaymentMethodNotFound) {
		t.Errorf("activate foreign: err = %v, want ErrPaymentMethodNotFound", err)
	}
}

// TestDeletePaymentMethod_RejectsActiveUntilAnotherActivated proves the
// active method cannot be deleted while it is the charge target, and becomes
// deletable after another method is activated (issue #251 AC). Migrated from
// the pre-rewrite TestBilling_DeletePaymentMethod_RejectsActiveMethodInSingleTransaction.
func TestDeletePaymentMethod_RejectsActiveUntilAnotherActivated(t *testing.T) {
	h := newMethodHarness(t)
	userID, _ := h.seedMethodSubscription(t)
	first, err := h.methods.AddPaymentMethod(t.Context(), userID, AddPaymentMethodRequest{ProviderToken: "token_first"})
	if err != nil {
		t.Fatalf("AddPaymentMethod(first) error = %v", err)
	}

	if err := h.methods.DeletePaymentMethod(t.Context(), userID, first.PaymentMethod.ID); !errors.Is(err, ErrPaymentMethodInUse) {
		t.Fatalf("delete active: err = %v, want ErrPaymentMethodInUse", err)
	}

	second, err := h.methods.AddPaymentMethod(t.Context(), userID, AddPaymentMethodRequest{ProviderToken: "token_second"})
	if err != nil {
		t.Fatalf("AddPaymentMethod(second) error = %v", err)
	}
	if err := h.methods.ActivatePaymentMethod(t.Context(), userID, first.PaymentMethod.ID); err != nil {
		t.Fatalf("ActivatePaymentMethod(first) error = %v", err)
	}
	// second is inactive now — deletable.
	if err := h.methods.DeletePaymentMethod(t.Context(), userID, second.PaymentMethod.ID); err != nil {
		t.Fatalf("DeletePaymentMethod(inactive) error = %v, want nil", err)
	}
	list, err := h.stores.methods.ListByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("ListByUserID() error = %v", err)
	}
	if len(list) != 1 || list[0].ID != first.PaymentMethod.ID {
		t.Fatalf("methods after delete = %+v, want only the first", list)
	}
}

// TestDeletePaymentMethod_DetachesProviderCardBestEffort proves the
// provider-side card is detached after the local deletion and a provider
// failure (even a hard one) does not fail the delete. Migrated from the
// pre-rewrite TestBilling_DeletePaymentMethod_TkassaRemoveCardFailureIsBestEffort.
func TestDeletePaymentMethod_DetachesProviderCardBestEffort(t *testing.T) {
	h := newMethodHarness(t)
	userID, _ := h.seedMethodSubscription(t)
	added, err := h.methods.AddPaymentMethod(t.Context(), userID, AddPaymentMethodRequest{ProviderToken: "token_1"})
	if err != nil {
		t.Fatalf("AddPaymentMethod() error = %v", err)
	}
	// Replace the active method so the first becomes deletable.
	second, err := h.methods.AddPaymentMethod(t.Context(), userID, AddPaymentMethodRequest{ProviderToken: "token_2"})
	if err != nil {
		t.Fatalf("AddPaymentMethod(second) error = %v", err)
	}
	if err := h.methods.ActivatePaymentMethod(t.Context(), userID, second.PaymentMethod.ID); err != nil {
		t.Fatalf("ActivatePaymentMethod(second) error = %v", err)
	}
	h.provider.removeErr = errors.New("provider is down")

	if err := h.methods.DeletePaymentMethod(t.Context(), userID, added.PaymentMethod.ID); err != nil {
		t.Fatalf("DeletePaymentMethod() error = %v, want nil despite the provider failure", err)
	}
	h.provider.mu.Lock()
	calls := h.provider.removeCalls
	h.provider.mu.Unlock()
	if calls != 1 {
		t.Fatalf("provider RemovePaymentMethod calls = %d, want 1 (the deleted card is detached best-effort)", calls)
	}
	// The local row is gone regardless.
	if _, err := h.stores.methods.GetByID(t.Context(), added.PaymentMethod.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("get deleted method error = %v, want ErrNotFound", err)
	}
}

// TestDeletePaymentMethod_NotFoundOrForeign proves deleting a missing method
// or a method of another user answers ErrPaymentMethodNotFound.
func TestDeletePaymentMethod_NotFoundOrForeign(t *testing.T) {
	h := newMethodHarness(t)
	userID, _ := h.seedMethodSubscription(t)
	other, _ := h.seedMethodSubscription(t)
	owned, err := h.methods.AddPaymentMethod(t.Context(), other, AddPaymentMethodRequest{ProviderToken: "token_other"})
	if err != nil {
		t.Fatalf("AddPaymentMethod(other) error = %v", err)
	}

	if err := h.methods.DeletePaymentMethod(t.Context(), userID, uuid.Must(uuid.NewV7())); !errors.Is(err, ErrPaymentMethodNotFound) {
		t.Errorf("delete missing: err = %v, want ErrPaymentMethodNotFound", err)
	}
	if err := h.methods.DeletePaymentMethod(t.Context(), userID, owned.PaymentMethod.ID); !errors.Is(err, ErrPaymentMethodNotFound) {
		t.Errorf("delete foreign: err = %v, want ErrPaymentMethodNotFound", err)
	}
}

// TestAddPaymentMethod_DuplicateTokenConverges proves re-adding the same card
// converges on one row by token hash instead of duplicating (issue #251 AC).
func TestAddPaymentMethod_DuplicateTokenConverges(t *testing.T) {
	h := newMethodHarness(t)
	userID, _ := h.seedMethodSubscription(t)

	for range 2 {
		if _, err := h.methods.AddPaymentMethod(t.Context(), userID, AddPaymentMethodRequest{ProviderToken: "same_token"}); err != nil {
			t.Fatalf("AddPaymentMethod() error = %v", err)
		}
	}

	list, err := h.stores.methods.ListByUserID(t.Context(), userID)
	if err != nil {
		t.Fatalf("ListByUserID() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("methods = %d, want 1 (token-hash convergence)", len(list))
	}
}

// TestGetSubscription_IncludesActivePaymentMethod proves the subscription
// view resolves the active method (issue #251). Migrated from the pre-rewrite
// TestBilling_GetSubscriptionWithActivePaymentMethod.
func TestGetSubscription_IncludesActivePaymentMethod(t *testing.T) {
	h := newMethodHarness(t)
	userID, _ := h.seedMethodSubscription(t)
	added, err := h.methods.AddPaymentMethod(t.Context(), userID, AddPaymentMethodRequest{ProviderToken: "token_1"})
	if err != nil {
		t.Fatalf("AddPaymentMethod() error = %v", err)
	}

	view, err := h.subs.GetSubscription(t.Context(), userID)
	if err != nil {
		t.Fatalf("GetSubscription() error = %v", err)
	}
	if view.ActivePaymentMethod == nil || view.ActivePaymentMethod.ID != added.PaymentMethod.ID {
		t.Fatalf("view active method = %+v, want the created one", view.ActivePaymentMethod)
	}
}

// The local card-binding confirmation moved to the HTTP adapter level
// (issue #287); the method-bound application path itself is covered by the
// webhook-delivery tests above and the integration tests.
