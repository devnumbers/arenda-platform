package tkassa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/logger"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
)

const (
	defaultTimeout   = 30 * time.Second
	maxResponseBytes = 1 << 20 // 1 MiB.
)

// Wire field names used outside generated spec types: sign() key exclusions,
// webhook map access, and the AddCard URL extras appended after signing
// (ADR 0017). Request and response structs pin the same names in their json
// tags; these constants serve the map-level access only.
const (
	fieldToken             = "Token"
	fieldTerminalKey       = "TerminalKey"
	fieldAmount            = "Amount"
	fieldOrderID           = "OrderId"
	fieldSuccess           = "Success"
	fieldPaymentID         = "PaymentId"
	fieldErrorCode         = "ErrorCode"
	fieldKey               = "Key"
	fieldValue             = "Value"
	fieldStatus            = "Status"
	fieldPan               = "Pan"
	fieldExpDate           = "ExpDate"
	fieldCardID            = "CardId"
	fieldRebillID          = "RebillId"
	fieldRequestKey        = "RequestKey"
	fieldNotificationType  = "NotificationType"
	fieldDATA              = "DATA"
	fieldData              = "Data"
	fieldReceipt           = "Receipt"
	fieldRedirectURL       = "RedirectUrl"
	fieldFailRedirectURL   = "FailRedirectUrl"
	fieldSuccessAddCardURL = "SuccessAddCardURL"
	fieldFailAddCardURL    = "FailAddCardURL"
	fieldNotificationURL   = "NotificationURL"
)

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
	RebillID  string `json:"RebillId"`
	Pan       string `json:"Pan"`
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

// getAddCardStateResponse is the T-Kassa response for the GetAddCardState
// method.
type getAddCardStateResponse struct {
	baseResponse
	CardID      string `json:"CardId"`
	RebillID    string `json:"RebillId"`
	CustomerKey string `json:"CustomerKey"`
	RequestKey  string `json:"RequestKey"`
}

type removeCardResponse struct {
	baseResponse
	CardID      string `json:"CardId"`
	CustomerKey string `json:"CustomerKey"`
}

// cancelResponse is the T-Kassa response for the Cancel method.
type cancelResponse struct {
	baseResponse
	OrderID        string `json:"OrderId"`
	PaymentID      string `json:"PaymentId"`
	OriginalAmount int64  `json:"OriginalAmount"`
	NewAmount      int64  `json:"NewAmount"`
}

// cardListItem is a single card entry in the T-Kassa GetCardList response.
type cardListItem struct {
	CardID   string `json:"CardId"`
	Pan      string `json:"Pan"`
	ExpDate  string `json:"ExpDate"`
	Status   string `json:"Status"`
	RebillID string `json:"RebillId"`
}

// bodyFromStruct converts a generated spec request struct into the map shape
// the signer and HTTP layer use. The JSON round-trip with UseNumber keeps
// numbers as json.Number, so sign() stringifies values byte-identically to
// the hand-built maps this replaced.
func bodyFromStruct(v any) (map[string]any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("tkassa: marshal request: %w", err)
	}
	var body map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&body); err != nil {
		return nil, fmt.Errorf("tkassa: decode request map: %w", err)
	}
	return body, nil
}

// failedBaseResponseError turns a Success=false response envelope into the
// call error. A provider answer without an error code is defective — it is a
// plain call failure (spec #419), not a business outcome, so the metric
// convention counts it as an integration error; a coded answer is a
// *ProviderError for callers to classify.
func failedBaseResponseError(method string, base baseResponse) error {
	if base.ErrorCode == "" || base.ErrorCode == "0" {
		return fmt.Errorf("tkassa: %s returned Success=false without error code", method)
	}
	return &ProviderError{
		Method:    method,
		ErrorCode: base.ErrorCode,
		Message:   base.Message,
		Details:   base.Details,
	}
}

// post signs body and sends it as a JSON POST request to a T-Kassa method.
func (p *Provider) post(ctx context.Context, method string, body map[string]any, out any) error {
	reqBody := maps.Clone(body)
	reqBody[fieldToken] = sign(body, p.password)
	return p.send(ctx, method, reqBody, out)
}

// send posts a prepared request body (Token already set) to a T-Kassa method
// and decodes the response.
func (p *Provider) send(ctx context.Context, method string, reqBody map[string]any, out any) error {
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
	defer func() {
		// The response is already decoded by now, so a close failure cannot
		// fail the call — but it does break connection reuse, which is worth
		// a warning on a payment-processing path.
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.WithCorrelation(ctx, p.log).WarnContext(ctx, "tkassa: close response body",
				slog.String("method", method),
				slog.String("error", sanitize.Error(closeErr)))
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodySnippet, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if readErr != nil {
			return fmt.Errorf("tkassa: %s returned HTTP %d (response body read failed: %w)", method, resp.StatusCode, readErr)
		}
		return fmt.Errorf("tkassa: %s returned HTTP %d: %s", method, resp.StatusCode, string(bodySnippet))
	}

	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(out); err != nil {
		return fmt.Errorf("tkassa: decode %s response: %w", method, err)
	}

	if r, ok := out.(responseWithBase); ok {
		if base := r.Base(); !base.Success {
			return failedBaseResponseError(method, base)
		}
	}

	return nil
}
