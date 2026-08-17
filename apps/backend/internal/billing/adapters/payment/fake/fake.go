// Package fake implements a deterministic local payment provider for
// development and tests. It satisfies the same provider-neutral port as the
// real adapter, so application-layer tests drive payments through it exactly
// like production code drives the real provider (issue #248).
package fake

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/payment"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/logger"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
)

// ProviderName is the provider identity payments persist as their provider
// (ADR 0038).
const ProviderName domain.PaymentProvider = "fake"

const (
	pendingTTL                  = 24 * time.Hour
	defaultErrorCode            = "fake_error"
	fakeFailTokenPrefix         = "fake_fail_"
	fakeProviderPaymentIDPrefix = "fake_"
	fakeBindingIDPrefix         = "fake_bind_"
	fakeMaskedPan               = "4111********1111"
	fakeExpDate                 = "1230"
)

type pendingEntry struct {
	event         application.WebhookEvent
	savedMethod   application.SavedMethod
	amountKopecks int64
	createdAt     time.Time
}

// bindingEntry is one card-binding session held by the fake provider: the
// customer it was started for, the bound method once it completed, and the
// creation time for TTL purging.
type bindingEntry struct {
	customerRef string
	method      application.SavedMethod
	completed   bool
	createdAt   time.Time
}

// Provider is a fake payment processor that keeps pending payments in memory.
type Provider struct {
	baseURL          string
	log              *slog.Logger
	clock            clock.Clock
	mu               sync.Mutex
	pending          map[string]pendingEntry
	confirmedAmounts map[string]int64
	// charges counts completed charges by internal payment id — the probe
	// tests use to prove a payment was charged exactly once.
	charges map[string]int
	// refundOutcomes programs RefundPayment answers by internal payment id
	// (issue #254): outcome.result is answered when set, otherwise outcome.err.
	refundOutcomes map[string]refundOutcome
	// paymentStates programs PaymentStatus answers by internal payment id
	// (issue #254): programmed states take precedence over the pending map —
	// the refund tests model a provider that already settled a refund.
	paymentStates map[string]application.PaymentStatusResult
	bindingStates map[string]application.MethodBindingState
	bindings      map[string]bindingEntry
	metrics       *payment.Metrics
}

// refundOutcome is one programmed RefundPayment answer: exactly one of result
// and err is used.
type refundOutcome struct {
	result *application.RefundResult
	err    error
}

// Compile-time assertions that Provider satisfies the aggregate provider port
// and each of its narrow capability interfaces.
var (
	_ application.PaymentProvider            = (*Provider)(nil)
	_ application.PaymentInitiator           = (*Provider)(nil)
	_ application.PaymentCharger             = (*Provider)(nil)
	_ application.PaymentStatusReader        = (*Provider)(nil)
	_ application.PaymentRefunder            = (*Provider)(nil)
	_ application.WebhookParser              = (*Provider)(nil)
	_ application.WebhookResponder           = (*Provider)(nil)
	_ application.PaymentMethodBinder        = (*Provider)(nil)
	_ application.PaymentMethodBindingReader = (*Provider)(nil)
	_ application.PaymentMethodRemover       = (*Provider)(nil)
	_ application.PaymentMethodLister        = (*Provider)(nil)
	_ application.ProviderNamer              = (*Provider)(nil)
)

// Name returns the provider identity used by the application layer.
func (p *Provider) Name() domain.PaymentProvider {
	return ProviderName
}

// NewProvider creates a fake provider for local development and tests.
func NewProvider(baseURL string, log *slog.Logger, clk clock.Clock, metrics *payment.Metrics) *Provider {
	return &Provider{
		baseURL:          strings.TrimRight(baseURL, "/"),
		log:              log,
		clock:            clk,
		pending:          make(map[string]pendingEntry),
		confirmedAmounts: make(map[string]int64),
		charges:          make(map[string]int),
		refundOutcomes:   make(map[string]refundOutcome),
		paymentStates:    make(map[string]application.PaymentStatusResult),
		bindingStates:    make(map[string]application.MethodBindingState),
		bindings:         make(map[string]bindingEntry),
		metrics:          metrics,
	}
}

// InitPayment creates a pending payment and returns a confirmation URL.
// Calling InitPayment twice with the same internal payment id is idempotent:
// the existing provider payment id and confirmation URL are returned.
func (p *Provider) InitPayment(ctx context.Context, req application.InitPaymentRequest) (res application.InitPaymentResult, err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "Init", status, time.Since(start))
	}()

	if req.PaymentID == uuid.Nil {
		return application.InitPaymentResult{}, errors.New("fake: payment id is required")
	}
	if req.AmountKopecks <= 0 {
		return application.InitPaymentResult{}, errors.New("fake: amount must be positive")
	}
	if !req.Initiator.Valid() {
		return application.InitPaymentResult{}, fmt.Errorf("fake: unknown operation initiator %q", req.Initiator)
	}

	log := logger.WithCorrelation(ctx, p.log)

	p.mu.Lock()
	defer p.mu.Unlock()
	p.purgeLocked()

	if entry, ok := p.pending[req.PaymentID.String()]; ok {
		log.InfoContext(ctx, "fake payment init is idempotent",
			"provider_payment_id", entry.event.Payment.ProviderPaymentID,
			"internal_payment_id", req.PaymentID.String(),
		)
		return application.InitPaymentResult{
			ProviderPaymentID: entry.event.Payment.ProviderPaymentID,
			PaymentURL:        p.confirmURL(entry.event.Payment.InternalPaymentID),
			Status:            domain.PaymentStatusPending,
			SavedMethod:       &entry.savedMethod,
		}, nil
	}

	providerPaymentID := fakeProviderPaymentIDPrefix + uuid.Must(uuid.NewV7()).String()
	savedMethod := application.SavedMethod{
		ChargeToken: "fake_token_" + uuid.Must(uuid.NewV7()).String(),
		CustomerRef: req.CustomerRef,
	}

	p.pending[req.PaymentID.String()] = pendingEntry{
		event: application.WebhookEvent{
			Payment: &application.PaymentNotification{
				ProviderPaymentID: providerPaymentID,
				InternalPaymentID: req.PaymentID,
				Status:            domain.PaymentStatusPending,
			},
		},
		savedMethod:   savedMethod,
		amountKopecks: req.AmountKopecks,
		createdAt:     p.clock.Now().UTC(),
	}

	log.InfoContext(ctx, "fake payment initialized",
		"provider_payment_id", providerPaymentID,
		"internal_payment_id", req.PaymentID.String(),
		"customer_ref", req.CustomerRef,
		"amount_kopecks", req.AmountKopecks,
		"period", req.Period,
	)

	// The fake provider binds the charge token to the payment synchronously —
	// the counterpart of the token-delivery webhook of a real provider.
	return application.InitPaymentResult{
		ProviderPaymentID: providerPaymentID,
		PaymentURL:        p.confirmURL(req.PaymentID),
		Status:            domain.PaymentStatusPending,
		SavedMethod:       &savedMethod,
	}, nil
}

func (p *Provider) confirmURL(internalPaymentID uuid.UUID) string {
	return fmt.Sprintf("%s/internal/fake-subscription-payment/%s/confirm", p.baseURL, internalPaymentID.String())
}

// PaymentURL returns the confirmation URL for a previously initialized fake payment.
// It is a fake-specific extra used by the local confirmation handler, not part
// of the provider port.
func (p *Provider) PaymentURL(internalPaymentID uuid.UUID) string {
	return p.confirmURL(internalPaymentID)
}

// PaymentStatus returns the provider-side status of a payment. If the payment
// is not found in the pending map it is assumed to have been completed and
// succeeded. A status programmed via SetPaymentState takes precedence.
func (p *Provider) PaymentStatus(ctx context.Context, paymentID uuid.UUID, providerPaymentID string) (res application.PaymentStatusResult, err error) {
	start := time.Now()
	defer func() {
		recStatus := "ok"
		if err != nil {
			recStatus = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "Status", recStatus, time.Since(start))
	}()

	_ = providerPaymentID
	p.mu.Lock()
	defer p.mu.Unlock()
	p.purgeLocked()

	if state, ok := p.paymentStates[paymentID.String()]; ok {
		return state, nil
	}
	if entry, ok := p.pending[paymentID.String()]; ok {
		res := application.PaymentStatusResult{Status: entry.event.Payment.Status}
		if entry.event.Payment.Status == domain.PaymentStatusFailed {
			res.ErrorCode = defaultErrorCode
		}
		return res, nil
	}
	return application.PaymentStatusResult{Status: domain.PaymentStatusSucceeded}, nil
}

// ChargePayment performs a merchant-initiated charge using a saved token.
// The charge resolves the provider-side payment the way a bank provider's
// recurring charge does: the outcome is final and PaymentStatus reports it, so
// callers recover a crash between the charge and its local application by
// re-reading the status instead of charging again.
func (p *Provider) ChargePayment(ctx context.Context, req application.ChargeRequest) (res application.ChargeResult, err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "Charge", status, time.Since(start))
	}()

	if req.PaymentID == uuid.Nil {
		return application.ChargeResult{}, errors.New("fake: payment id is required")
	}
	if req.AmountKopecks <= 0 {
		return application.ChargeResult{}, errors.New("fake: amount must be positive")
	}

	log := logger.WithCorrelation(ctx, p.log)

	providerPaymentID := fakeProviderPaymentIDPrefix + uuid.Must(uuid.NewV7()).String()

	p.mu.Lock()
	defer p.mu.Unlock()

	if entry, ok := p.pending[req.PaymentID.String()]; ok {
		entry.event.Payment.ProviderPaymentID = providerPaymentID
		if strings.HasPrefix(req.ChargeToken, fakeFailTokenPrefix) {
			entry.event.Payment.Status = domain.PaymentStatusFailed
		} else {
			entry.event.Payment.Status = domain.PaymentStatusSucceeded
		}
		p.pending[req.PaymentID.String()] = entry
	}

	if strings.HasPrefix(req.ChargeToken, fakeFailTokenPrefix) {
		log.InfoContext(ctx, "fake charge failed",
			"provider_payment_id", providerPaymentID,
			"internal_payment_id", req.PaymentID.String(),
		)
		return application.ChargeResult{
			ProviderPaymentID: providerPaymentID,
			Status:            domain.PaymentStatusFailed,
			ErrorCode:         defaultErrorCode,
		}, nil
	}

	log.InfoContext(ctx, "fake charge succeeded",
		"provider_payment_id", providerPaymentID,
		"internal_payment_id", req.PaymentID.String(),
	)
	p.confirmedAmounts[providerPaymentID] = req.AmountKopecks
	p.confirmedAmounts[req.PaymentID.String()] = req.AmountKopecks
	p.charges[req.PaymentID.String()]++
	return application.ChargeResult{
		ProviderPaymentID: providerPaymentID,
		Status:            domain.PaymentStatusSucceeded,
	}, nil
}

// ChargeCount reports how many charges completed for the internal payment id
// — the double-charge probe of tests and local debugging.
func (p *Provider) ChargeCount(internalPaymentID uuid.UUID) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.charges[internalPaymentID.String()]
}

// BindPaymentMethod starts a card-binding session: the "bank form" is the
// local confirmation endpoint, so local end-to-end runs drive the whole
// binding flow (issue #251). Calling it twice returns independent sessions,
// exactly like re-opening the provider form.
func (p *Provider) BindPaymentMethod(ctx context.Context, req application.BindMethodRequest) (res application.BindMethodResult, err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "BindPaymentMethod", status, time.Since(start))
	}()

	if req.CustomerRef == "" {
		return application.BindMethodResult{}, errors.New("fake: customer ref is required")
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.purgeLocked()

	bindingID := fakeBindingIDPrefix + uuid.Must(uuid.NewV7()).String()
	p.bindings[bindingID] = bindingEntry{
		customerRef: req.CustomerRef,
		createdAt:   p.clock.Now().UTC(),
	}

	logger.WithCorrelation(ctx, p.log).InfoContext(ctx, "fake card binding initialized",
		"binding_id", bindingID,
		"customer_ref", req.CustomerRef,
	)
	return application.BindMethodResult{
		FormURL:   p.bindingConfirmURL(bindingID),
		BindingID: bindingID,
	}, nil
}

// bindingConfirmURL builds the local confirmation URL of a binding session —
// the fake counterpart of the provider's bank form.
func (p *Provider) bindingConfirmURL(bindingID string) string {
	return fmt.Sprintf("%s/internal/fake-card-binding/%s/confirm", p.baseURL, bindingID)
}

// AddPaymentMethodFromToken accepts a raw charge token synchronously — the
// capability bank-form providers do not offer (the application gates the
// token path on it). The fake stores the card itself, so local runs and tests
// can drive the whole method flow without the binding form.
func (p *Provider) AddPaymentMethodFromToken(ctx context.Context, customerRef, token string) (res application.SavedMethod, err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "AddPaymentMethodFromToken", status, time.Since(start))
	}()

	if customerRef == "" {
		return application.SavedMethod{}, errors.New("fake: customer ref is required")
	}
	if token == "" {
		return application.SavedMethod{}, errors.New("fake: charge token is required")
	}
	return application.SavedMethod{
		ProviderMethodID: "fake_card_" + uuid.Must(uuid.NewV7()).String(),
		ChargeToken:      token,
		MaskedPan:        fakeMaskedPan,
		ExpDate:          fakeExpDate,
		CustomerRef:      customerRef,
	}, nil
}

// PaymentMethodBinding returns the state of a binding session: programmed
// states (SetBindingState) take precedence so tests can force outcomes, real
// entries are pending until confirmed and completed afterwards. An unknown or
// purged binding id resolves to ErrProviderBindingNotFound — the provider has
// forgotten the session, which the sync treats as expired.
func (p *Provider) PaymentMethodBinding(ctx context.Context, bindingID string) (res application.MethodBindingState, err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "PaymentMethodBinding", status, time.Since(start))
	}()

	p.mu.Lock()
	defer p.mu.Unlock()
	p.purgeLocked()
	if state, ok := p.bindingStates[bindingID]; ok {
		return state, nil
	}
	entry, ok := p.bindings[bindingID]
	if !ok {
		return application.MethodBindingState{}, fmt.Errorf("fake: binding not found: %w", application.ErrProviderBindingNotFound)
	}
	if entry.completed {
		return application.MethodBindingState{
			Status: application.MethodBindingCompleted,
			Method: &entry.method,
		}, nil
	}
	return application.MethodBindingState{Status: application.MethodBindingPending}, nil
}

// RemovePaymentMethod is a no-op for the fake provider.
func (p *Provider) RemovePaymentMethod(ctx context.Context, customerRef, providerMethodID string) (err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "RemovePaymentMethod", status, time.Since(start))
	}()

	_ = customerRef
	_ = providerMethodID
	return nil
}

// ListPaymentMethods always returns an empty list: the fake provider creates
// payment methods synchronously from tokens and has no provider-side method
// storage.
func (p *Provider) ListPaymentMethods(ctx context.Context, customerRef string) (methods []application.SavedMethod, err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "ListPaymentMethods", status, time.Since(start))
	}()

	_ = customerRef
	return []application.SavedMethod{}, nil
}

// SetBindingState programs the PaymentMethodBinding result for a binding id.
// It is the fake counterpart of provider-side binding state and lets tests
// force outcomes (failed bindings, foreign methods) the local confirmation
// flow cannot produce.
func (p *Provider) SetBindingState(bindingID string, state application.MethodBindingState) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.bindingStates[bindingID] = state
}

// ConfirmCardBinding completes a previously started binding and returns the
// add-card notification for it. It is used by the local confirmation handler
// and is not part of the provider port. Confirming a completed binding is
// idempotent: the same notification is returned again, and the application's
// session state makes reprocessing a no-op.
func (p *Provider) ConfirmCardBinding(ctx context.Context, requestKey string) (res application.WebhookEvent, err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "ConfirmCardBinding", status, time.Since(start))
	}()

	p.mu.Lock()
	defer p.mu.Unlock()
	p.purgeLocked()
	entry, ok := p.bindings[requestKey]
	if !ok {
		// The provider no longer tracks the session (TTL purge or an unknown
		// key); the sentinel lets the caller answer not-found.
		return application.WebhookEvent{}, fmt.Errorf("fake: binding not found: %w", application.ErrProviderBindingNotFound)
	}
	if !entry.completed {
		entry.method = application.SavedMethod{
			ProviderMethodID: "fake_card_" + uuid.Must(uuid.NewV7()).String(),
			ChargeToken:      "fake_token_" + uuid.Must(uuid.NewV7()).String(),
			MaskedPan:        fakeMaskedPan,
			ExpDate:          fakeExpDate,
			CustomerRef:      entry.customerRef,
		}
		entry.completed = true
		p.bindings[requestKey] = entry
	}

	logger.WithCorrelation(ctx, p.log).InfoContext(ctx, "fake card binding confirmed",
		"binding_id", requestKey,
		"customer_ref", entry.customerRef,
	)
	return application.WebhookEvent{
		MethodBound: &application.MethodBoundNotification{
			BindingID: requestKey,
			Method:    entry.method,
		},
	}, nil
}

// RefundPayment refunds a finalized fake payment. For the fake provider every
// refund succeeds and reports the full amount back, so callers do not need to
// track it separately. An outcome programmed via SetRefundOutcome takes
// precedence, so tests can model refused, in-flight and partial refunds.
func (p *Provider) RefundPayment(ctx context.Context, req application.RefundRequest) (res application.RefundResult, err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "RefundPayment", status, time.Since(start))
	}()

	if req.PaymentID == uuid.Nil {
		return application.RefundResult{}, errors.New("fake: payment id is required")
	}
	if req.ProviderPaymentID == "" {
		return application.RefundResult{}, errors.New("fake: provider payment id is required")
	}

	log := logger.WithCorrelation(ctx, p.log)

	if outcome, ok := p.programmedRefundOutcome(req.PaymentID.String()); ok {
		if outcome.err != nil {
			log.InfoContext(ctx, "fake payment refund failed (programmed)",
				"provider_payment_id", req.ProviderPaymentID,
				"internal_payment_id", req.PaymentID.String(),
			)
			return application.RefundResult{}, outcome.err
		}
		return *outcome.result, nil
	}

	refundedAmount := req.AmountKopecks
	if refundedAmount == 0 {
		p.mu.Lock()
		defer p.mu.Unlock()
		if amount, ok := p.confirmedAmounts[req.ProviderPaymentID]; ok {
			refundedAmount = amount
		} else if amount, ok := p.confirmedAmounts[req.PaymentID.String()]; ok {
			refundedAmount = amount
		}
	}

	log.InfoContext(ctx, "fake payment refunded",
		"provider_payment_id", req.ProviderPaymentID,
		"internal_payment_id", req.PaymentID.String(),
		"amount_kopecks", refundedAmount,
	)

	return application.RefundResult{
		ProviderPaymentID:     req.ProviderPaymentID,
		Status:                domain.PaymentStatusRefunded,
		RefundedAmountKopecks: refundedAmount,
	}, nil
}

// SetRefundOutcome programs the RefundPayment answer for an internal payment
// id — the fake counterpart of provider-side refund states local flows cannot
// produce (refused refunds, in-flight settlements, partial amounts). It is not
// part of the provider port.
func (p *Provider) SetRefundOutcome(internalPaymentID string, result *application.RefundResult, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.refundOutcomes[internalPaymentID] = refundOutcome{result: result, err: err}
}

// programmedRefundOutcome returns the programmed refund answer of a payment.
func (p *Provider) programmedRefundOutcome(internalPaymentID string) (refundOutcome, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	outcome, ok := p.refundOutcomes[internalPaymentID]
	return outcome, ok
}

// SetPaymentState programs the PaymentStatus answer for an internal payment
// id, overriding the pending map — the refund tests model a provider that
// already settled (or refused) a refund. It is not part of the provider port.
func (p *Provider) SetPaymentState(internalPaymentID string, state application.PaymentStatusResult) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paymentStates[internalPaymentID] = state
}

// WebhookAck returns the fixed success response the fake provider expects
// HTTP handlers to send back after receiving a webhook.
func (p *Provider) WebhookAck() []byte {
	return []byte(`{"status":"ok"}`)
}

// ParseWebhook parses the fake provider webhook JSON payload.
func (p *Provider) ParseWebhook(_ context.Context, payload []byte) (application.WebhookEvent, error) {
	var raw struct {
		ProviderPaymentID string `json:"provider_payment_id"`
		InternalPaymentID string `json:"internal_payment_id"`
		Status            string `json:"status"`
		ErrorCode         string `json:"error_code,omitempty"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return application.WebhookEvent{}, fmt.Errorf("fake: invalid webhook payload: %w", err)
	}

	internalPaymentID, err := uuid.Parse(raw.InternalPaymentID)
	if err != nil {
		return application.WebhookEvent{}, fmt.Errorf("fake: invalid internal payment id: %w", err)
	}

	status := domain.PaymentStatus(raw.Status)
	if status != domain.PaymentStatusSucceeded && status != domain.PaymentStatusFailed {
		return application.WebhookEvent{}, fmt.Errorf("fake: unsupported webhook status %q", raw.Status)
	}

	var errorCode *string
	if status == domain.PaymentStatusFailed {
		code := raw.ErrorCode
		if code == "" {
			code = defaultErrorCode
		}
		errorCode = &code
	}

	return application.WebhookEvent{
		Payment: &application.PaymentNotification{
			ProviderPaymentID: raw.ProviderPaymentID,
			InternalPaymentID: internalPaymentID,
			Status:            status,
			ErrorCode:         errorCode,
		},
	}, nil
}

// ConfirmPayment completes a previously initialized fake payment as
// succeeded. It is used by the local fake confirmation HTTP handler and is
// not part of the provider port.
func (p *Provider) ConfirmPayment(ctx context.Context, internalPaymentID string) (res application.WebhookEvent, err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "ConfirmPayment", status, time.Since(start))
	}()

	return p.confirm(internalPaymentID, false, nil)
}

// ConfirmPaymentFailed completes a previously initialized fake payment as
// failed.
func (p *Provider) ConfirmPaymentFailed(ctx context.Context, internalPaymentID string, errorCode *string) (res application.WebhookEvent, err error) {
	start := time.Now()
	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}
		p.metrics.RecordRequest(ctx, "fake", "ConfirmPaymentFailed", status, time.Since(start))
	}()

	return p.confirm(internalPaymentID, true, errorCode)
}

func (p *Provider) confirm(internalPaymentID string, failed bool, errorCode *string) (application.WebhookEvent, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.purgeLocked()
	entry, ok := p.pending[internalPaymentID]
	if ok {
		delete(p.pending, internalPaymentID)
	}
	if !ok {
		// The provider no longer tracks the entry (TTL purge or an earlier
		// confirmation); the sentinel lets the caller fall back to the
		// provider status as the source of truth.
		return application.WebhookEvent{}, fmt.Errorf("fake: payment not found: %w", application.ErrProviderPaymentNotFound)
	}

	status := domain.PaymentStatusSucceeded
	if failed {
		status = domain.PaymentStatusFailed
		if errorCode == nil {
			defaultCode := defaultErrorCode
			errorCode = &defaultCode
		}
	} else {
		errorCode = nil
		p.confirmedAmounts[internalPaymentID] = entry.amountKopecks
	}

	event := application.WebhookEvent{
		Payment: &application.PaymentNotification{
			ProviderPaymentID: entry.event.Payment.ProviderPaymentID,
			InternalPaymentID: entry.event.Payment.InternalPaymentID,
			Status:            status,
			ErrorCode:         errorCode,
			AmountKopecks:     entry.amountKopecks,
		},
	}
	if !failed {
		method := entry.savedMethod
		event.Payment.SavedMethod = &method
	}
	return event, nil
}

// purgeLocked removes pending payments and open bindings older than
// pendingTTL. p.mu must be held.
func (p *Provider) purgeLocked() {
	now := p.clock.Now().UTC()
	for id, entry := range p.pending {
		if now.Sub(entry.createdAt) > pendingTTL {
			delete(p.pending, id)
		}
	}
	for id, entry := range p.bindings {
		if !entry.completed && now.Sub(entry.createdAt) > pendingTTL {
			delete(p.bindings, id)
		}
	}
}
