// Package tkassa implements the T-Kassa payment provider adapter: the only
// place in the billing module that knows the provider's wire protocol. The
// adapter lives behind the provider-neutral application port (issue #248,
// ADR 0007/0038) and is split by concern: client.go (HTTP plumbing), sign.go
// (request/webhook tokens), webhook.go (notification parsing), status.go
// (status mapping), errors.go (error classification), description.go (payment
// descriptions) and retry_transport.go (retry semantics).
package tkassa

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/payment"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/payment/tkassa/spec"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/logger"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// ProviderName is the provider identity payments persist as their provider
// (ADR 0038).
const ProviderName domain.PaymentProvider = "tkassa"

// instrumentationName is the OpenTelemetry tracer scope for this adapter.
const instrumentationName = "github.com/nambers/arenda-planform/apps/backend/internal/billing/adapters/payment/tkassa"

// tracer is the package-level OpenTelemetry tracer used to span provider operations.
var tracer = otel.Tracer(instrumentationName)

// Callback route contract with the platform edge: the paths the adapter
// builds provider callbacks from (issue #248 — URL construction is provider
// concern). The HTTP layer serves them under these public paths: Caddy
// routes /api and /webhooks prefixes to the backend on stage/prod, the
// Next.js dev rewrite does the same locally, and /subscription/* return
// pages are frontend routes.
const (
	// Public webhook endpoint T-Kassa posts status notifications to
	// (payment and card binding alike).
	notificationPath = "/webhooks/payment/tkassa"
	// Browser return URL format after the payment form (success and fail),
	// parameterized by the internal payment id.
	paymentResultPathFmt = "/subscription/payments/%s/"
	// Browser return URLs after the card-binding form: success and fail.
	bindingReturnSuccessPath = "/api/subscription/payment-methods/add-card/success"
	bindingReturnFailPath    = "/api/subscription/payment-methods/add-card/fail"
)

// Config is the T-Kassa adapter configuration. Every field is required unless
// documented otherwise; in particular BaseURL has no default — the test URL
// is deliberately not implicit (issue #248), so running against an
// unconfigured endpoint fails fast instead of silently hitting the sandbox.
type Config struct {
	// BaseURL is the provider API base URL (must end with /v2/).
	BaseURL string
	// TerminalKey and Password are the terminal credentials from T-Business.
	TerminalKey string
	Password    string
	// AppBaseURL is the platform's public base URL; notification and return
	// URLs are built from it.
	AppBaseURL string
	// Timeout bounds one provider HTTP request; zero means 30s.
	Timeout time.Duration
	// MaxRetries is the number of retry attempts after the first request
	// (zero disables retries). Retries follow the per-method safety rules in
	// retry_transport.go.
	MaxRetries int
	// RetryBaseDelay and RetryMaxDelay shape the exponential backoff between
	// retries.
	RetryBaseDelay time.Duration
	RetryMaxDelay  time.Duration
}

// Provider is a T-Kassa payment adapter.
type Provider struct {
	baseURL     string
	terminalKey string
	password    string
	appBaseURL  string
	client      *http.Client
	log         *slog.Logger
	metrics     *payment.Metrics
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

// NewProvider creates a T-Kassa provider instance. If MaxRetries is positive,
// the HTTP client wraps the default transport with an exponential backoff
// retry layer for temporary network errors.
func NewProvider(cfg Config, log *slog.Logger, metrics *payment.Metrics) (*Provider, error) {
	if cfg.BaseURL == "" {
		return nil, errors.New("tkassa: base URL is required — the provider endpoint must be configured explicitly")
	}
	if cfg.TerminalKey == "" {
		return nil, errors.New("tkassa: terminal key is required")
	}
	if cfg.Password == "" {
		return nil, errors.New("tkassa: password is required")
	}
	if cfg.AppBaseURL == "" {
		return nil, errors.New("tkassa: app base URL is required — provider callbacks cannot be built without it")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	base := http.DefaultTransport
	if cfg.MaxRetries > 0 {
		rt := newRetryTransport(base, cfg.MaxRetries, cfg.RetryBaseDelay, cfg.RetryMaxDelay)
		// Full-jitter: the actual sleep is drawn uniformly from [0, d], where d
		// is the deterministic exponential-backoff delay. This desynchronizes
		// concurrent retries and avoids thundering-herd spikes against T-Kassa.
		rt.jitter = cryptoJitter
		base = rt
	}
	instrumentedTransport := otelhttp.NewTransport(base,
		otelhttp.WithSpanNameFormatter(func(_ string, _ *http.Request) string {
			return "tkassa-http"
		}),
	)
	client := &http.Client{
		Timeout:   timeout,
		Transport: instrumentedTransport,
	}
	return &Provider{
		baseURL:     strings.TrimRight(cfg.BaseURL, "/") + "/",
		terminalKey: cfg.TerminalKey,
		password:    cfg.Password,
		appBaseURL:  strings.TrimRight(cfg.AppBaseURL, "/"),
		client:      client,
		log:         log,
		metrics:     metrics,
	}, nil
}

// Name returns the provider identity used by the application layer.
func (p *Provider) Name() domain.PaymentProvider {
	return ProviderName
}

// notificationURL builds the webhook URL T-Kassa posts payment and card
// binding notifications to.
func (p *Provider) notificationURL() string {
	return p.appBaseURL + notificationPath
}

// paymentResultURL builds the browser return URL (kind "success"/"fail")
// after the payment form.
func (p *Provider) paymentResultURL(paymentID uuid.UUID, kind string) string {
	return p.appBaseURL + fmt.Sprintf(paymentResultPathFmt, paymentID) + kind
}

// toSpecOperationInitiatorType maps the port's neutral initiator to the
// T-Kassa OperationInitiatorType enum. The mapping is total and the empty
// initiator is rejected: the port contract makes the initiator an explicit
// mandatory parameter (issue #246 §6) — defaulting it silently produced
// wrong CIT/MIT routing.
func toSpecOperationInitiatorType(i application.Initiator) (spec.CommonOperationInitiatorType, error) {
	switch i {
	case application.InitiatorCustomer:
		// "1" — CIT CC: the parent payment of a CC/COF chain entered on the
		// provider form.
		return spec.CommonOperationInitiatorTypeN1, nil
	case application.InitiatorMerchant:
		// "R" — MIT COF Recurring: a recurring charge on saved credentials.
		return spec.CommonOperationInitiatorTypeR, nil
	default:
		return "", fmt.Errorf("tkassa: unknown operation initiator %q", i)
	}
}

// InitPayment starts a new payment through T-Kassa.
func (p *Provider) InitPayment(ctx context.Context, req application.InitPaymentRequest) (res application.InitPaymentResult, err error) {
	start := time.Now()
	defer func() {
		p.metrics.RecordRequest(ctx, "tkassa", methodInit, metricStatus(err), time.Since(start))
	}()

	ctx, span := tracer.Start(ctx, "tkassa.InitPayment")
	defer span.End()

	span.SetAttributes(
		attribute.String("provider", "tkassa"),
		attribute.String("internal_payment_id", req.PaymentID.String()),
		attribute.Int64("amount_kopecks", req.AmountKopecks),
	)

	log := logger.WithCorrelation(ctx, p.log)

	initiatorType, err := toSpecOperationInitiatorType(req.Initiator)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return application.InitPaymentResult{}, err
	}

	var data spec.InitRequest_DATA
	if err := data.FromCommon(spec.Common{OperationInitiatorType: &initiatorType}); err != nil {
		buildErr := fmt.Errorf("tkassa: build init DATA: %w", err)
		span.RecordError(buildErr)
		span.SetStatus(codes.Error, buildErr.Error())
		return application.InitPaymentResult{}, buildErr
	}

	payType := spec.O
	initReq := spec.InitRequest{
		TerminalKey:     p.terminalKey,
		Amount:          req.AmountKopecks,
		OrderId:         req.PaymentID.String(),
		CustomerKey:     &req.CustomerRef,
		PayType:         &payType,
		NotificationURL: new(p.notificationURL()),
		SuccessURL:      new(p.paymentResultURL(req.PaymentID, "success")),
		FailURL:         new(p.paymentResultURL(req.PaymentID, "fail")),
		DATA:            &data,
	}
	// Recurrent=Y marks the parent payment of a save-method chain: T-Kassa
	// issues the charge token (RebillId) the later merchant-initiated charges
	// run on. It is only valid on customer-initiated payments (error 1126
	// otherwise).
	if req.SaveMethod {
		recurrent := spec.Y
		initReq.Recurrent = &recurrent
	}
	description := truncateDescription(paymentDescription(req.Purpose))
	initReq.Description = &description
	if !req.FormDeadline.IsZero() {
		deadline := req.FormDeadline.UTC()
		initReq.RedirectDueDate = &deadline
	}

	body, err := bodyFromStruct(initReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return application.InitPaymentResult{}, err
	}

	log.InfoContext(ctx, "tkassa init",
		"internal_payment_id", req.PaymentID.String(),
		"customer_ref", req.CustomerRef,
		"amount_kopecks", req.AmountKopecks,
	)

	var resp initResponse
	if err := p.post(ctx, methodInit, body, &resp); err != nil {
		err = classifyProviderError(err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return application.InitPaymentResult{}, err
	}

	return application.InitPaymentResult{
		ProviderPaymentID: resp.PaymentID,
		PaymentURL:        resp.PaymentURL,
		Status:            mapStatus(resp.Status),
	}, nil
}

// ChargePayment performs a merchant-initiated charge through T-Kassa.
// Amount is intentionally absent from the request body: T-Kassa takes the
// charge amount from the original Init call.
func (p *Provider) ChargePayment(ctx context.Context, req application.ChargeRequest) (res application.ChargeResult, err error) {
	start := time.Now()
	defer func() {
		p.metrics.RecordRequest(ctx, "tkassa", methodCharge, metricStatus(err), time.Since(start))
	}()

	ctx, span := tracer.Start(ctx, "tkassa.ChargePayment")
	defer span.End()

	span.SetAttributes(
		attribute.String("provider", "tkassa"),
		attribute.String("internal_payment_id", req.PaymentID.String()),
		attribute.String("provider_payment_id", req.ProviderPaymentID),
		attribute.Int64("amount_kopecks", req.AmountKopecks),
	)

	log := logger.WithCorrelation(ctx, p.log)

	body, err := bodyFromStruct(spec.ChargeRequest{
		TerminalKey: p.terminalKey,
		PaymentId:   req.ProviderPaymentID,
		RebillId:    req.ChargeToken,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return application.ChargeResult{}, err
	}

	log.InfoContext(ctx, "tkassa charge",
		"internal_payment_id", req.PaymentID.String(),
		"provider_payment_id", req.ProviderPaymentID,
		"amount_kopecks", req.AmountKopecks,
	)

	var resp chargeResponse
	if err := p.post(ctx, methodCharge, body, &resp); err != nil {
		err = classifyProviderError(err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return application.ChargeResult{}, err
	}

	res = application.ChargeResult{
		ProviderPaymentID: resp.PaymentID,
		Status:            mapStatus(resp.Status),
	}
	if res.Status == domain.PaymentStatusFailed && resp.ErrorCode != "0" {
		res.ErrorCode = resp.ErrorCode
	}
	return res, nil
}

// PaymentStatus queries the current status of a payment through T-Kassa.
func (p *Provider) PaymentStatus(
	ctx context.Context, paymentID uuid.UUID, providerPaymentID string,
) (res application.PaymentStatusResult, err error) {
	start := time.Now()
	defer func() {
		p.metrics.RecordRequest(ctx, "tkassa", "Status", metricStatus(err), time.Since(start))
	}()

	ctx, span := tracer.Start(ctx, "tkassa.PaymentStatus")
	defer span.End()

	span.SetAttributes(
		attribute.String("provider", "tkassa"),
		attribute.String("internal_payment_id", paymentID.String()),
		attribute.String("provider_payment_id", providerPaymentID),
	)

	log := logger.WithCorrelation(ctx, p.log)

	body, err := bodyFromStruct(spec.GetStateRequest{
		TerminalKey: p.terminalKey,
		PaymentId:   providerPaymentID,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return application.PaymentStatusResult{}, err
	}

	log.InfoContext(ctx, "tkassa get state",
		"internal_payment_id", paymentID.String(),
		"provider_payment_id", providerPaymentID,
	)

	var resp getStateResponse
	if err := p.post(ctx, methodGetState, body, &resp); err != nil {
		err = classifyProviderError(err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return application.PaymentStatusResult{}, err
	}

	res = application.PaymentStatusResult{
		Status:      mapStatus(resp.Status),
		ChargeToken: resp.RebillID,
	}
	if res.Status == domain.PaymentStatusFailed && resp.ErrorCode != "0" {
		res.ErrorCode = resp.ErrorCode
	}
	return res, nil
}

// RefundPayment refunds a payment through T-Kassa. The refund is always
// full-amount; ExternalRequestId carries the internal payment id so network
// duplicates are deduplicated by the provider instead of risking a second
// refund (issue #246 §2.3).
func (p *Provider) RefundPayment(ctx context.Context, req application.RefundRequest) (res application.RefundResult, err error) {
	start := time.Now()
	defer func() {
		p.metrics.RecordRequest(ctx, "tkassa", methodCancel, metricStatus(err), time.Since(start))
	}()

	ctx, span := tracer.Start(ctx, "tkassa.RefundPayment")
	defer span.End()

	span.SetAttributes(
		attribute.String("provider", "tkassa"),
		attribute.String("internal_payment_id", req.PaymentID.String()),
		attribute.String("provider_payment_id", req.ProviderPaymentID),
		attribute.Int64("amount_kopecks", req.AmountKopecks),
	)

	log := logger.WithCorrelation(ctx, p.log)

	externalRequestID := req.PaymentID.String()
	body, err := bodyFromStruct(spec.CancelRequest{
		TerminalKey:       p.terminalKey,
		PaymentId:         req.ProviderPaymentID,
		Amount:            &req.AmountKopecks,
		ExternalRequestId: &externalRequestID,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return application.RefundResult{}, err
	}

	log.InfoContext(ctx, "tkassa cancel",
		"internal_payment_id", req.PaymentID.String(),
		"provider_payment_id", req.ProviderPaymentID,
		"amount_kopecks", req.AmountKopecks,
	)

	var resp cancelResponse
	if err := p.post(ctx, methodCancel, body, &resp); err != nil {
		err = classifyProviderError(err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return application.RefundResult{}, err
	}

	if !resp.Success {
		err = classifyProviderError(&ProviderError{
			Method:    "Cancel",
			ErrorCode: resp.ErrorCode,
			Message:   resp.Message,
			Details:   resp.Details,
		})
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return application.RefundResult{}, err
	}

	status := mapCancelStatus(resp.Status)
	refundedAmount := req.AmountKopecks
	// Prefer the amount reported by T-Kassa when both original and new amounts
	// are present. Fall back to the requested amount only when the response does
	// not contain them.
	if resp.OriginalAmount > 0 || resp.NewAmount > 0 {
		refundedAmount = max(resp.OriginalAmount-resp.NewAmount, 0)
	}

	return application.RefundResult{
		ProviderPaymentID:     resp.PaymentID,
		Status:                status,
		RefundedAmountKopecks: refundedAmount,
	}, nil
}

// BindPaymentMethod initializes attaching a new card to a T-Kassa customer.
func (p *Provider) BindPaymentMethod(ctx context.Context, req application.BindMethodRequest) (res application.BindMethodResult, err error) {
	start := time.Now()
	defer func() {
		p.metrics.RecordRequest(ctx, "tkassa", "InitAddCard", metricStatus(err), time.Since(start))
	}()

	ctx, span := tracer.Start(ctx, "tkassa.BindPaymentMethod")
	defer span.End()

	span.SetAttributes(
		attribute.String("provider", "tkassa"),
		attribute.String("customer_ref", req.CustomerRef),
	)

	log := logger.WithCorrelation(ctx, p.log)

	// The T-Kassa AddCard schema requires the customer to already exist, so we
	// call AddCustomer first. If the customer was already created (for example,
	// by a previous Init payment), T-Kassa returns ErrorCode 7. That is not a
	// fatal error for the card-binding flow, so we proceed to AddCard.
	customerBody, err := bodyFromStruct(spec.AddCustomerRequest{
		TerminalKey: p.terminalKey,
		CustomerKey: req.CustomerRef,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return application.BindMethodResult{}, err
	}
	var customerResp addCustomerResponse
	if err := p.post(ctx, "AddCustomer", customerBody, &customerResp); err != nil {
		var providerErr *ProviderError
		if !errors.As(err, &providerErr) || providerErr.ErrorCode != "7" {
			wrappedErr := fmt.Errorf("tkassa: add customer failed: %w", classifyProviderError(err))
			span.RecordError(wrappedErr)
			span.SetStatus(codes.Error, wrappedErr.Error())
			return application.BindMethodResult{}, wrappedErr
		}
		log.InfoContext(ctx, "tkassa customer already exists, proceeding to add card",
			"customer_ref", req.CustomerRef,
		)
	}

	// 3DSHOLD checks 3DS support while binding and holds 0 RUB when the card
	// does not support 3DS, so every bound card is chargeable afterwards
	// (ADR 0017).
	checkType := spec.N3DSHOLD

	// The official AddCard schema (developer.tbank.ru/eacq/api/add-card) only
	// contains TerminalKey, CustomerKey, Token, CheckType, IP and
	// ResidentState — no URL fields at all. RedirectUrl/FailRedirectUrl are
	// legacy undocumented names that the production gateway ignores: relying
	// on them failed the bank-form redirect in prod with error 9
	// ("Переадресовываемый url пуст"). We therefore additionally send the
	// documented names SuccessAddCardURL/FailAddCardURL/NotificationURL. All
	// of these extras sit outside the schema, so the T-Kassa server excludes
	// them from token verification while sign() hashes the whole body —
	// including them in the signature produced error 204 ("Неверный токен")
	// in prod. The token is hence computed over the schema fields only and
	// the extras are appended to the request body afterwards. If the
	// per-request URLs turn out to be ignored too, the documented fallback
	// is terminal-level URLs configured via acq_help@tbank.ru (ADR 0017).
	cardBody, err := bodyFromStruct(spec.AddCardRequest{
		TerminalKey: p.terminalKey,
		CustomerKey: req.CustomerRef,
		CheckType:   &checkType,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return application.BindMethodResult{}, err
	}
	token := sign(cardBody, p.password)
	cardBody[fieldRedirectURL] = p.appBaseURL + bindingReturnSuccessPath
	cardBody[fieldFailRedirectURL] = p.appBaseURL + bindingReturnFailPath
	cardBody[fieldSuccessAddCardURL] = p.appBaseURL + bindingReturnSuccessPath
	cardBody[fieldFailAddCardURL] = p.appBaseURL + bindingReturnFailPath
	cardBody[fieldNotificationURL] = p.notificationURL()
	cardBody[fieldToken] = token

	log.InfoContext(ctx, "tkassa add card",
		"customer_ref", req.CustomerRef,
	)

	var cardResp addCardResponse
	if err := p.send(ctx, methodAddCard, cardBody, &cardResp); err != nil {
		err = classifyProviderError(err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return application.BindMethodResult{}, err
	}

	return application.BindMethodResult{
		FormURL:   cardResp.PaymentURL,
		BindingID: cardResp.RequestKey,
	}, nil
}

// PaymentMethodBinding queries the state of a card-binding session started by
// BindPaymentMethod.
func (p *Provider) PaymentMethodBinding(ctx context.Context, bindingID string) (res application.MethodBindingState, err error) {
	start := time.Now()
	defer func() {
		p.metrics.RecordRequest(ctx, "tkassa", "GetAddCardState", metricStatus(err), time.Since(start))
	}()

	ctx, span := tracer.Start(ctx, "tkassa.PaymentMethodBinding")
	defer span.End()

	span.SetAttributes(
		attribute.String("provider", "tkassa"),
		attribute.String("binding_id", bindingID),
	)

	log := logger.WithCorrelation(ctx, p.log)

	body, err := bodyFromStruct(spec.GetAddCardStateRequest{
		TerminalKey: p.terminalKey,
		RequestKey:  bindingID,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return application.MethodBindingState{}, err
	}

	log.InfoContext(ctx, "tkassa get add card state",
		"binding_id", bindingID,
	)

	var resp getAddCardStateResponse
	if err := p.post(ctx, "GetAddCardState", body, &resp); err != nil {
		err = classifyProviderError(err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return application.MethodBindingState{}, err
	}

	status, err := mapAddCardStateStatus(resp.Status)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return application.MethodBindingState{}, err
	}

	state := application.MethodBindingState{
		Status:    status,
		ErrorCode: resp.ErrorCode,
	}
	if status == application.MethodBindingCompleted {
		state.Method = &application.SavedMethod{
			ProviderMethodID: resp.CardID,
			ChargeToken:      resp.RebillID,
			CustomerRef:      resp.CustomerKey,
		}
	}
	return state, nil
}

// RemovePaymentMethod detaches a card from a T-Kassa customer.
func (p *Provider) RemovePaymentMethod(ctx context.Context, customerRef, providerMethodID string) (err error) {
	start := time.Now()
	defer func() {
		p.metrics.RecordRequest(ctx, "tkassa", "RemoveCard", metricStatus(err), time.Since(start))
	}()

	ctx, span := tracer.Start(ctx, "tkassa.RemovePaymentMethod")
	defer span.End()

	span.SetAttributes(
		attribute.String("provider", "tkassa"),
		attribute.String("customer_ref", customerRef),
		attribute.String("provider_method_id", providerMethodID),
	)

	log := logger.WithCorrelation(ctx, p.log)

	body, err := bodyFromStruct(spec.RemoveCardRequest{
		TerminalKey: p.terminalKey,
		CustomerKey: customerRef,
		CardId:      providerMethodID,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	log.InfoContext(ctx, "tkassa remove card",
		"customer_ref", customerRef,
	)

	var resp removeCardResponse
	if err := p.post(ctx, "RemoveCard", body, &resp); err != nil {
		if isMethodNotFoundError(err) {
			return application.ErrProviderMethodNotFound
		}
		err = classifyProviderError(err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

// ListPaymentMethods returns the chargeable cards bound to a T-Kassa
// customer. Cards the provider reports as inactive or deleted are filtered
// out: the neutral port lists only methods later charges can run on.
func (p *Provider) ListPaymentMethods(ctx context.Context, customerRef string) (methods []application.SavedMethod, err error) {
	start := time.Now()
	defer func() {
		p.metrics.RecordRequest(ctx, "tkassa", "GetCardList", metricStatus(err), time.Since(start))
	}()

	ctx, span := tracer.Start(ctx, "tkassa.ListPaymentMethods")
	defer span.End()

	span.SetAttributes(
		attribute.String("provider", "tkassa"),
		attribute.String("customer_ref", customerRef),
	)

	log := logger.WithCorrelation(ctx, p.log)

	body, err := bodyFromStruct(spec.GetCardListRequest{
		TerminalKey: p.terminalKey,
		CustomerKey: customerRef,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	log.InfoContext(ctx, "tkassa get card list",
		"customer_ref", customerRef,
	)

	// GetCardList answers with a bare JSON array of cards on success and with
	// the usual response envelope on failure, so the base-response check inside
	// post cannot fire. Decode the raw body and handle both shapes explicitly.
	var raw json.RawMessage
	if err := p.post(ctx, "GetCardList", body, &raw); err != nil {
		err = classifyProviderError(err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	if trimmed := bytes.TrimSpace(raw); len(trimmed) == 0 || trimmed[0] != '[' {
		var base baseResponse
		if decodeErr := json.Unmarshal(raw, &base); decodeErr != nil {
			err = fmt.Errorf("tkassa: decode GetCardList response: %w", decodeErr)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		if !base.Success && base.ErrorCode != "" && base.ErrorCode != "0" {
			providerErr := &ProviderError{
				Method:    "GetCardList",
				ErrorCode: base.ErrorCode,
				Message:   base.Message,
				Details:   base.Details,
			}
			if isCustomerNotFoundError(providerErr) {
				log.InfoContext(ctx, "tkassa customer not found, treating as empty card list",
					"customer_ref", customerRef,
				)
				// Keep the *ProviderError in the chain so errors.As on it still
				// works for callers and metricStatus.
				err = fmt.Errorf("%w: %w", application.ErrProviderCustomerNotFound, providerErr)
				return nil, err
			}
			if isAccountNotFoundError(providerErr) {
				log.WarnContext(ctx, "tkassa terminal not found, treating as empty card list",
					"customer_ref", customerRef,
				)
				// Keep the *ProviderError in the chain so errors.As on it still
				// works for callers and metricStatus.
				err = fmt.Errorf("%w: %w", application.ErrProviderAccountNotFound, providerErr)
				return nil, err
			}
			err = classifyProviderError(providerErr)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		// A successful non-array envelope carries no cards.
		return []application.SavedMethod{}, nil
	}

	var items []cardListItem
	if decodeErr := json.Unmarshal(raw, &items); decodeErr != nil {
		err = fmt.Errorf("tkassa: decode GetCardList response: %w", decodeErr)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	methods = make([]application.SavedMethod, 0, len(items))
	for _, item := range items {
		if item.Status != string(cardStatusActive) {
			continue
		}
		methods = append(methods, application.SavedMethod{
			ProviderMethodID: item.CardID,
			ChargeToken:      item.RebillID,
			MaskedPan:        item.Pan,
			ExpDate:          item.ExpDate,
			CustomerRef:      customerRef,
		})
	}
	return methods, nil
}

// cardStatusActive is the T-Kassa GetCardList status of a chargeable card
// ("A"; "I" is temporarily inactive, "D" is deleted).
const cardStatusActive = "A"
