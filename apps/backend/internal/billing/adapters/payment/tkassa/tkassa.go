// Package tkassa implements the T-Kassa payment provider HTTP adapter.
package tkassa

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/billing/domain"
)

const (
	defaultBaseURL                = "https://rest-api-test.tinkoff.ru/v2/"
	defaultTimeout                = 30 * time.Second
	webhookOK                     = "OK"
	recurrentYes                  = "Y"
	payTypeOneStage               = "O"
	checkType3DSHold              = "3DSHOLD"
	firstPaymentInitiatorType     = "1" // CIT CC (customer-initiated credential-on-file first payment)
	renewalInitiatorType          = "R" // MIT COF recurring
	notificationTypeAddCard       = "NotificationAddCard"
	notificationTypeAddCardLegacy = "AddCard"
	maxResponseBytes              = 1 << 20 // 1 MiB
	maxDescriptionLength          = 140     // T-Kassa limit for Description field
)

// ProviderError is returned when T-Kassa responds with Success=false and a non-zero ErrorCode.
type ProviderError struct {
	Method    string
	ErrorCode string
	Message   string
	Details   string
}

func (e *ProviderError) Error() string {
	msg := fmt.Sprintf("tkassa: %s failed with error code %s", e.Method, e.ErrorCode)
	if e.Message != "" {
		msg += ": " + e.Message
	}
	if e.Details != "" {
		msg += " (" + e.Details + ")"
	}
	return msg
}

// ProviderErrorCode exposes the T-Kassa ErrorCode so application-layer error
// handlers can persist it without importing this package.
func (e *ProviderError) ProviderErrorCode() string { return e.ErrorCode }

// Provider is a T-Kassa payment adapter.
type Provider struct {
	baseURL     string
	terminalKey string
	password    string
	client      *http.Client
	log         *slog.Logger
}

// baseResponse is embedded in all T-Kassa API responses.
type baseResponse struct {
	Success     bool   `json:"Success"`
	ErrorCode   string `json:"ErrorCode"`
	Message     string `json:"Message"`
	Details     string `json:"Details"`
	TerminalKey string `json:"TerminalKey"`
	Status      string `json:"Status"`
}

func (b baseResponse) Base() baseResponse { return b }

type responseWithBase interface {
	Base() baseResponse
}

type initResponse struct {
	baseResponse
	PaymentID  string `json:"PaymentId"`
	PaymentURL string `json:"PaymentURL"`
	OrderID    string `json:"OrderId"`
	Amount     int64  `json:"Amount"`
}

type chargeResponse struct {
	baseResponse
	PaymentID string `json:"PaymentId"`
	OrderID   string `json:"OrderId"`
	Amount    int64  `json:"Amount"`
}

type getStateResponse struct {
	baseResponse
	PaymentID string `json:"PaymentId"`
	OrderID   string `json:"OrderId"`
	Amount    int64  `json:"Amount"`
}

type addCustomerResponse struct {
	baseResponse
	CustomerKey string `json:"CustomerKey"`
}

type addCardResponse struct {
	baseResponse
	PaymentURL string `json:"PaymentURL"`
	RequestKey string `json:"RequestKey"`
}

type removeCardResponse struct {
	baseResponse
	CardID      string `json:"CardId"`
	CustomerKey string `json:"CustomerKey"`
}

// NewProvider creates a T-Kassa provider instance.
// If baseURL is empty, the sandbox URL is used. If timeout is zero, a 30s default is used.
func NewProvider(baseURL, terminalKey, password string, timeout time.Duration, log *slog.Logger) *Provider {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/") + "/"
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &Provider{
		baseURL:     baseURL,
		terminalKey: terminalKey,
		password:    password,
		client:      &http.Client{Timeout: timeout},
		log:         log,
	}
}

// Name returns the provider identity used by the application layer.
func (p *Provider) Name() domain.PaymentProvider {
	return domain.ProviderTkassa
}

// Init starts a new payment through T-Kassa.
func (p *Provider) Init(ctx context.Context, req application.InitRequest) (application.InitResult, error) {
	operationInitiatorType := req.OperationInitiatorType
	if operationInitiatorType == "" {
		operationInitiatorType = firstPaymentInitiatorType
	}

	body := map[string]any{
		"TerminalKey":     p.terminalKey,
		"Amount":          req.AmountKopecks,
		"OrderId":         req.PaymentID.String(),
		"CustomerKey":     req.CustomerKey,
		"PayType":         payTypeOneStage,
		"NotificationURL": req.NotificationURL,
		"SuccessURL":      req.SuccessURL,
		"FailURL":         req.FailURL,
		"DATA": map[string]string{
			"OperationInitiatorType": operationInitiatorType,
		},
	}
	if req.Recurrent {
		body["Recurrent"] = recurrentYes
	}
	if req.Description != "" {
		body["Description"] = truncateDescription(req.Description, maxDescriptionLength)
	}

	p.log.InfoContext(ctx, "tkassa init",
		"internal_payment_id", req.PaymentID.String(),
		"user_id", req.UserID.String(),
		"amount_kopecks", req.AmountKopecks,
	)

	var resp initResponse
	if err := p.post(ctx, "Init", body, &resp); err != nil {
		return application.InitResult{}, err
	}

	return application.InitResult{
		ProviderPaymentID: resp.PaymentID,
		PaymentURL:        resp.PaymentURL,
		SavedToken:        "",
		CustomerKey:       req.CustomerKey,
		Status:            mapStatus(resp.Status),
	}, nil
}

// Charge performs a recurrent charge through T-Kassa.
// Amount is intentionally omitted from the request body: T-Kassa takes the
// charge amount from the original Init call.
func (p *Provider) Charge(ctx context.Context, req application.ChargeRequest) (application.ChargeResult, error) {
	body := map[string]any{
		"TerminalKey": p.terminalKey,
		"PaymentId":   req.ProviderPaymentID,
		"RebillId":    req.Token,
	}

	p.log.InfoContext(ctx, "tkassa charge",
		"internal_payment_id", req.PaymentID.String(),
		"provider_payment_id", req.ProviderPaymentID,
		"amount_kopecks", req.AmountKopecks,
	)

	var resp chargeResponse
	if err := p.post(ctx, "Charge", body, &resp); err != nil {
		return application.ChargeResult{}, err
	}

	return application.ChargeResult{
		ProviderPaymentID: resp.PaymentID,
		Status:            mapStatus(resp.Status),
	}, nil
}

// Status queries the current status of a payment through T-Kassa.
func (p *Provider) Status(ctx context.Context, paymentID uuid.UUID, providerPaymentID string) (domain.PaymentStatus, error) {
	body := map[string]any{
		"TerminalKey": p.terminalKey,
		"PaymentId":   providerPaymentID,
	}

	p.log.InfoContext(ctx, "tkassa get state",
		"internal_payment_id", paymentID.String(),
		"provider_payment_id", providerPaymentID,
	)

	var resp getStateResponse
	if err := p.post(ctx, "GetState", body, &resp); err != nil {
		return "", err
	}

	return mapStatus(resp.Status), nil
}

// Cancel refunds or cancels a payment through T-Kassa.
func (p *Provider) Cancel(ctx context.Context, req application.CancelRequest) (application.CancelResult, error) {
	body := map[string]any{
		"TerminalKey": p.terminalKey,
		"PaymentId":   req.ProviderPaymentID,
	}
	if req.AmountKopecks > 0 {
		body["Amount"] = req.AmountKopecks
	}

	p.log.InfoContext(ctx, "tkassa cancel",
		"internal_payment_id", req.PaymentID.String(),
		"provider_payment_id", req.ProviderPaymentID,
		"amount_kopecks", req.AmountKopecks,
	)

	var resp cancelResponse
	if err := p.post(ctx, "Cancel", body, &resp); err != nil {
		return application.CancelResult{}, err
	}

	if !resp.Success {
		return application.CancelResult{}, &ProviderError{
			Method:    "Cancel",
			ErrorCode: resp.ErrorCode,
			Message:   resp.Message,
			Details:   resp.Details,
		}
	}

	status := mapCancelStatus(resp.Status)
	refundedAmount := req.AmountKopecks
	// Prefer the amount reported by T-Kassa when both original and new amounts
	// are present. Fall back to the requested amount only when the response does
	// not contain them.
	if resp.OriginalAmount > 0 || resp.NewAmount > 0 {
		refundedAmount = resp.OriginalAmount - resp.NewAmount
		if refundedAmount < 0 {
			refundedAmount = 0
		}
	}

	return application.CancelResult{
		ProviderPaymentID:     resp.PaymentID,
		Status:                status,
		RefundedAmountKopecks: refundedAmount,
	}, nil
}

// cancelResponse is the T-Kassa response for the Cancel method.
type cancelResponse struct {
	baseResponse
	OrderID        string `json:"OrderId"`
	PaymentID      string `json:"PaymentId"`
	OriginalAmount int64  `json:"OriginalAmount"`
	NewAmount      int64  `json:"NewAmount"`
}

// InitAddCard initializes attaching a new card to a T-Kassa customer.
func (p *Provider) InitAddCard(ctx context.Context, req application.InitAddCardRequest) (application.InitAddCardResult, error) {
	// The T-Kassa AddCard schema requires the customer to already exist, so we
	// call AddCustomer first. If the customer was already created (for example,
	// by a previous Init payment), T-Kassa returns ErrorCode 7. That is not a
	// fatal error for the card-binding flow, so we proceed to AddCard.
	customerBody := map[string]any{
		"TerminalKey": p.terminalKey,
		"CustomerKey": req.CustomerKey,
	}
	var customerResp addCustomerResponse
	if err := p.post(ctx, "AddCustomer", customerBody, &customerResp); err != nil {
		var providerErr *ProviderError
		if errors.As(err, &providerErr) && providerErr.ErrorCode == "7" {
			p.log.InfoContext(ctx, "tkassa customer already exists, proceeding to add card",
				"customer_key", req.CustomerKey,
			)
		} else {
			return application.InitAddCardResult{}, fmt.Errorf("tkassa: add customer failed: %w", err)
		}
	}

	checkType := req.CheckType
	if checkType == "" {
		checkType = checkType3DSHold
	}

	// The AddCard schema only supports TerminalKey, CustomerKey, CheckType, IP and
	// ResidentState. Return URLs are configured in the terminal, not in the request.
	cardBody := map[string]any{
		"TerminalKey": p.terminalKey,
		"CustomerKey": req.CustomerKey,
		"CheckType":   checkType,
	}

	p.log.InfoContext(ctx, "tkassa add card",
		"customer_key", req.CustomerKey,
	)

	var cardResp addCardResponse
	if err := p.post(ctx, "AddCard", cardBody, &cardResp); err != nil {
		return application.InitAddCardResult{}, err
	}

	return application.InitAddCardResult{
		PaymentURL:  cardResp.PaymentURL,
		RequestKey:  cardResp.RequestKey,
		CustomerKey: req.CustomerKey,
	}, nil
}

// RemoveCard detaches a card from a T-Kassa customer.
func (p *Provider) RemoveCard(ctx context.Context, customerKey, cardID string) error {
	body := map[string]any{
		"TerminalKey": p.terminalKey,
		"CustomerKey": customerKey,
		"CardId":      cardID,
	}

	p.log.InfoContext(ctx, "tkassa remove card",
		"customer_key", customerKey,
	)

	var resp removeCardResponse
	if err := p.post(ctx, "RemoveCard", body, &resp); err != nil {
		if isCardNotFoundError(err) {
			return application.ErrProviderCardNotFound
		}
		return err
	}
	return nil
}

// WebhookResponse returns the fixed success response T-Kassa expects HTTP
// handlers to send back after receiving a webhook.
func (p *Provider) WebhookResponse() []byte {
	return []byte(webhookOK)
}

// ParseWebhook parses and verifies a T-Kassa webhook payload.
func (p *Provider) ParseWebhook(ctx context.Context, payload []byte) (application.WebhookPayload, error) {
	_ = ctx

	if err := verifyToken(payload, p.password); err != nil {
		return application.WebhookPayload{}, err
	}

	data, err := unmarshalWebhook(payload)
	if err != nil {
		return application.WebhookPayload{}, err
	}

	if getString(data, "TerminalKey") != p.terminalKey {
		return application.WebhookPayload{}, errors.New("tkassa: webhook terminal key mismatch")
	}

	notificationType := getString(data, "NotificationType")
	switch notificationType {
	case notificationTypeAddCard, notificationTypeAddCardLegacy:
		if !isAddCardSuccessful(data) {
			return application.WebhookPayload{}, errors.New("tkassa: add card webhook ignored: binding not successful")
		}
		return application.WebhookPayload{
			NotificationType: notificationType,
			CustomerKey:      getString(data, "CustomerKey"),
			RequestKey:       getString(data, "RequestKey"),
			RebillID:         getString(data, "RebillId"),
			CardID:           getString(data, "CardId"),
			Pan:              getString(data, "Pan"),
			ExpDate:          getString(data, "ExpDate"),
		}, nil
	case "", "NotificationPayment":
		// Payment notifications either omit NotificationType or explicitly set
		// it to NotificationPayment. Continue parsing as a payment webhook below.
	default:
		return application.WebhookPayload{}, fmt.Errorf("tkassa: unknown notification type %q", notificationType)
	}

	orderID := getString(data, "OrderId")
	internalPaymentID, _ := uuid.Parse(orderID)

	status := mapStatus(getString(data, "Status"))
	errorCode := getString(data, "ErrorCode")
	var errorCodePtr *string
	if errorCode != "" && errorCode != "0" {
		errorCodePtr = &errorCode
	}

	return application.WebhookPayload{
		InternalPaymentID: internalPaymentID,
		ProviderPaymentID: getString(data, "PaymentId"),
		Status:            status,
		ErrorCode:         errorCodePtr,
		AmountKopecks:     getInt64(data, "Amount"),
		RebillID:          getString(data, "RebillId"),
		CardID:            getString(data, "CardId"),
		Pan:               getString(data, "Pan"),
		ExpDate:           getString(data, "ExpDate"),
		CustomerKey:       getString(data, "CustomerKey"),
	}, nil
}

// post sends a signed JSON POST request to a T-Kassa method and decodes the response.
func (p *Provider) post(ctx context.Context, method string, body map[string]any, out any) error {
	reqBody := cloneBody(body)
	reqBody["Token"] = sign(body, p.password)

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(reqBody); err != nil {
		return fmt.Errorf("tkassa: encode %s request: %w", method, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+method, &buf)
	if err != nil {
		return fmt.Errorf("tkassa: create %s request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("tkassa: %s request failed: %w", method, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodySnippet, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("tkassa: %s returned HTTP %d: %s", method, resp.StatusCode, string(bodySnippet))
	}

	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(out); err != nil {
		return fmt.Errorf("tkassa: decode %s response: %w", method, err)
	}

	if r, ok := out.(responseWithBase); ok {
		base := r.Base()
		if !base.Success && base.ErrorCode != "" && base.ErrorCode != "0" {
			return &ProviderError{
				Method:    method,
				ErrorCode: base.ErrorCode,
				Message:   base.Message,
				Details:   base.Details,
			}
		}
	}

	return nil
}

// sign computes the T-Kassa request/webhook token.
// It excludes the keys Token, DATA, Data and Receipt, adds the password, sorts keys
// lexicographically and concatenates the stringified values before SHA-256 hashing.
func sign(data map[string]any, password string) string {
	keys := make([]string, 0, len(data)+1)
	for k := range data {
		if k == "Token" || k == "DATA" || k == "Data" || k == "Receipt" {
			continue
		}
		keys = append(keys, k)
	}
	keys = append(keys, "Password")
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		if k == "Password" {
			sb.WriteString(password)
			continue
		}
		if shouldSkipValue(data[k]) {
			continue
		}
		sb.WriteString(stringifyValue(data[k]))
	}

	h := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(h[:])
}

func shouldSkipValue(v any) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case string:
		return val == ""
	case map[string]any, []any:
		return true
	}
	return false
}

func stringifyValue(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case int:
		return strconv.FormatInt(int64(val), 10)
	case int8:
		return strconv.FormatInt(int64(val), 10)
	case int16:
		return strconv.FormatInt(int64(val), 10)
	case int32:
		return strconv.FormatInt(int64(val), 10)
	case int64:
		return strconv.FormatInt(val, 10)
	case uint:
		return strconv.FormatUint(uint64(val), 10)
	case uint8:
		return strconv.FormatUint(uint64(val), 10)
	case uint16:
		return strconv.FormatUint(uint64(val), 10)
	case uint32:
		return strconv.FormatUint(uint64(val), 10)
	case uint64:
		return strconv.FormatUint(val, 10)
	case float64:
		if val == math.Trunc(val) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case json.Number:
		return val.String()
	case map[string]any, []any:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

func cloneBody(body map[string]any) map[string]any {
	cloned := make(map[string]any, len(body)+1)
	for k, v := range body {
		cloned[k] = v
	}
	return cloned
}

func verifyToken(payload []byte, password string) error {
	data, err := unmarshalWebhook(payload)
	if err != nil {
		return err
	}

	expected := sign(data, password)
	actual, _ := data["Token"].(string)

	if !hmac.Equal([]byte(expected), []byte(actual)) {
		return errors.New("tkassa: invalid webhook token")
	}
	return nil
}

func unmarshalWebhook(payload []byte) (map[string]any, error) {
	var data map[string]any
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.UseNumber()
	if err := dec.Decode(&data); err != nil {
		return nil, fmt.Errorf("tkassa: invalid webhook payload: %w", err)
	}
	return data, nil
}

func getString(data map[string]any, key string) string {
	v, ok := data[key]
	if !ok {
		return ""
	}
	return stringifyValue(v)
}

func getInt64(data map[string]any, key string) int64 {
	v, ok := data[key]
	if !ok {
		return 0
	}
	s := stringifyValue(v)
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func isAddCardSuccessful(data map[string]any) bool {
	if status := getString(data, "Status"); status != "SUCCESS" {
		return false
	}
	success, _ := data["Success"].(bool)
	return success
}

// mapStatus maps T-Kassa payment statuses to the domain status model.
func mapStatus(status string) domain.PaymentStatus {
	switch status {
	case statusNew, statusAuthorized, statusAuthorizing, status3DSChecking,
		status3DSChecked, statusConfirming, statusFormShowed, statusAsyncRefunding:
		return domain.PaymentStatusPending
	case statusConfirmed:
		return domain.PaymentStatusSucceeded
	case statusRefunded:
		return domain.PaymentStatusRefunded
	case statusPartialRefunded:
		return domain.PaymentStatusPartialRefunded
	case statusRejected, statusAuthFail, statusCanceled, statusDeadlineExpired,
		statusReversed, statusPartialReversed, status3DSFailed:
		return domain.PaymentStatusFailed
	default:
		return domain.PaymentStatusPending
	}
}

// mapCancelStatus maps the T-Kassa Cancel response statuses to domain refund statuses.
// REVERSED means the operation was cancelled before completion (e.g. a pending/NEW payment),
// so it is treated as a full refund for our domain model.
func mapCancelStatus(status string) domain.PaymentStatus {
	switch status {
	case statusRefunded, statusReversed:
		return domain.PaymentStatusRefunded
	case statusPartialRefunded, statusPartialReversed:
		return domain.PaymentStatusPartialRefunded
	case statusNew, statusAuthorized, statusAuthorizing, status3DSChecking,
		status3DSChecked, statusConfirming, statusFormShowed, statusAsyncRefunding:
		return domain.PaymentStatusPending
	default:
		return domain.PaymentStatusFailed
	}
}

// T-Kassa payment status constants.
const (
	statusNew             = "NEW"
	statusFormShowed      = "FORM_SHOWED"
	statusAuthorizing     = "AUTHORIZING"
	statusAuthorized      = "AUTHORIZED"
	statusAuthFail        = "AUTH_FAIL"
	statusRejected        = "REJECTED"
	status3DSChecking     = "3DS_CHECKING"
	status3DSChecked      = "3DS_CHECKED"
	status3DSFailed       = "3DS_FAILED"
	statusConfirming      = "CONFIRMING"
	statusConfirmed       = "CONFIRMED"
	statusReversing       = "REVERSING"
	statusReversed        = "REVERSED"
	statusPartialReversed = "PARTIAL_REVERSED"
	statusRefunding       = "REFUNDING"
	statusAsyncRefunding  = "ASYNC_REFUNDING"
	statusRefunded        = "REFUNDED"
	statusPartialRefunded = "PARTIAL_REFUNDED"
	statusDeadlineExpired = "DEADLINE_EXPIRED"
	statusCanceled        = "CANCELED"
)

func isCardNotFoundError(err error) bool {
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) {
		return false
	}
	switch providerErr.ErrorCode {
	case "107", "231":
		return true
	}
	return false
}

func truncateDescription(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
