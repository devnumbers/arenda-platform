// Package dadata implements a minimal DaData address suggestion client.
package dadata

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
)

const (
	defaultTimeout   = 10 * time.Second
	maxSuggestions   = 10
	maxResponseBytes = 1 << 20 // 1 MiB
)

// Config holds the external DaData API settings.
type Config struct {
	BaseURL   string
	APIKey    string
	SecretKey string
	Timeout   time.Duration
	Logger    *slog.Logger
}

// Client calls the DaData suggestion API.
type Client struct {
	baseURL   string
	apiKey    string
	secretKey string
	client    *http.Client
	log       *slog.Logger
}

type suggestRequest struct {
	Query string `json:"query"`
	Count int    `json:"count"`
}

type suggestResponse struct {
	Suggestions []struct {
		Value string `json:"value"`
		Data  struct {
			City string `json:"city"`
		} `json:"data"`
	} `json:"suggestions"`
}

// NewClient creates a DaData client instance.
// If timeout is zero, a 10s default is used.
func NewClient(cfg Config) *Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}
	return &Client{
		baseURL:   strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:    cfg.APIKey,
		secretKey: cfg.SecretKey,
		client:    &http.Client{Timeout: cfg.Timeout},
		log:       cfg.Logger,
	}
}

// SuggestAddresses returns address hints from DaData for the given query.
func (c *Client) SuggestAddresses(ctx context.Context, query string) ([]propertiesapp.AddressSuggestion, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("%w: address query is empty", propertiesapp.ErrInvalidInput)
	}

	body := suggestRequest{Query: query, Count: maxSuggestions}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, fmt.Errorf("dadata: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, &buf)
	if err != nil {
		return nil, fmt.Errorf("dadata: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Token "+c.apiKey)
	if c.secretKey != "" {
		req.Header.Set("X-Secret", c.secretKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: dadata request failed: %w", propertiesapp.ErrAddressSuggestFailed, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		c.log.WarnContext(ctx, "dadata suggestion upstream error",
			slog.Int("status", resp.StatusCode),
			slog.Int("query_len", len(query)),
			slog.Int("count", 0),
		)
		return nil, fmt.Errorf("%w: dadata returned HTTP %d", propertiesapp.ErrAddressSuggestFailed, resp.StatusCode)
	}

	var out suggestResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&out); err != nil {
		c.log.WarnContext(ctx, "dadata suggestion decode error",
			slog.String("error", err.Error()),
			slog.Int("query_len", len(query)),
			slog.Int("status", resp.StatusCode),
		)
		return nil, fmt.Errorf("%w: decode response: %w", propertiesapp.ErrAddressSuggestFailed, err)
	}

	suggestions := make([]propertiesapp.AddressSuggestion, 0, len(out.Suggestions))
	for _, s := range out.Suggestions {
		suggestions = append(suggestions, propertiesapp.AddressSuggestion{
			Value: s.Value,
			City:  s.Data.City,
		})
	}

	c.log.InfoContext(ctx, "dadata suggestion success",
		slog.Int("query_len", len(query)),
		slog.Int("status", resp.StatusCode),
		slog.Int("count", len(suggestions)),
	)

	return suggestions, nil
}
