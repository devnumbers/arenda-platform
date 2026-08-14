package tkassa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"time"
)

const (
	defaultTimeout   = 30 * time.Second
	maxResponseBytes = 1 << 20 // 1 MiB
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

// post signs body and sends it as a JSON POST request to a T-Kassa method.
func (p *Provider) post(ctx context.Context, method string, body map[string]any, out any) error {
	reqBody := maps.Clone(body)
	reqBody["Token"] = sign(body, p.password)
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
