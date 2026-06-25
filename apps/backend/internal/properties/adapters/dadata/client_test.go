package dadata

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	propertiesapp "github.com/nambers/arenda-planform/apps/backend/internal/properties/application"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func TestSuggestAddresses(t *testing.T) {
	cases := []struct {
		name        string
		query       string
		secretKey   string
		serverJSON  string
		serverCode  int
		serverSleep time.Duration
		ctxTimeout  time.Duration
		wantErr     error
		want        []propertiesapp.AddressSuggestion
	}{
		{
			name:  "successful suggestions",
			query: "москва тверская",
			serverJSON: `{
				"suggestions": [
					{"value": "г Москва, ул Тверская", "data": {"city": "Москва"}},
					{"value": "г Москва, Тверская пл", "data": {"city": "Москва"}}
				]
			}`,
			serverCode: http.StatusOK,
			want: []propertiesapp.AddressSuggestion{
				{Value: "г Москва, ул Тверская", City: "Москва"},
				{Value: "г Москва, Тверская пл", City: "Москва"},
			},
		},
		{
			name:      "sets X-Secret when configured",
			query:     "санкт-петербург невский",
			secretKey: "secret",
			serverJSON: `{
				"suggestions": [
					{"value": "г Санкт-Петербург, пр-кт Невский", "data": {"city": "Санкт-Петербург"}}
				]
			}`,
			serverCode: http.StatusOK,
			want: []propertiesapp.AddressSuggestion{
				{Value: "г Санкт-Петербург, пр-кт Невский", City: "Санкт-Петербург"},
			},
		},
		{
			name:    "empty query returns invalid input",
			query:   "   ",
			wantErr: propertiesapp.ErrInvalidInput,
		},
		{
			name:       "upstream non-2xx returns suggest failed",
			query:      "ленина",
			serverCode: http.StatusUnauthorized,
			serverJSON: `{"message":"unauthorized"}`,
			wantErr:    propertiesapp.ErrAddressSuggestFailed,
		},
		{
			name:        "context timeout returns deadline error",
			query:       "ленина",
			serverSleep: 500 * time.Millisecond,
			ctxTimeout:  10 * time.Millisecond,
			wantErr:     context.DeadlineExceeded,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var lastRequest *http.Request
			var lastBody string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				lastRequest = r
				body, _ := io.ReadAll(r.Body)
				lastBody = string(body)
				if tc.serverSleep > 0 {
					time.Sleep(tc.serverSleep)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.serverCode)
				if tc.serverJSON != "" {
					_, _ = w.Write([]byte(tc.serverJSON))
				}
			}))
			defer server.Close()

			client := NewClient(Config{
				BaseURL:   server.URL,
				APIKey:    "test-token",
				SecretKey: tc.secretKey,
				Timeout:   5 * time.Second,
				Logger:    discardLogger(),
			})

			ctx := context.Background()
			if tc.ctxTimeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, tc.ctxTimeout)
				defer cancel()
			}

			got, err := client.SuggestAddresses(ctx, tc.query)

			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tc.wantErr)
				}
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("expected %d suggestions, got %d", len(tc.want), len(got))
			}
			for i := range tc.want {
				if got[i].Value != tc.want[i].Value || got[i].City != tc.want[i].City {
					t.Fatalf("suggestion %d: expected %+v, got %+v", i, tc.want[i], got[i])
				}
			}

			if tc.serverCode == 0 || tc.serverCode == http.StatusOK {
				if lastRequest == nil {
					t.Fatal("expected request to reach test server")
				}
				if auth := lastRequest.Header.Get("Authorization"); auth != "Token test-token" {
					t.Fatalf("expected Authorization header %q, got %q", "Token test-token", auth)
				}
				if ct := lastRequest.Header.Get("Content-Type"); ct != "application/json" {
					t.Fatalf("expected Content-Type %q, got %q", "application/json", ct)
				}
				if accept := lastRequest.Header.Get("Accept"); accept != "application/json" {
					t.Fatalf("expected Accept %q, got %q", "application/json", accept)
				}
				if tc.secretKey != "" {
					if secret := lastRequest.Header.Get("X-Secret"); secret != tc.secretKey {
						t.Fatalf("expected X-Secret %q, got %q", tc.secretKey, secret)
					}
				} else {
					if secret := lastRequest.Header.Get("X-Secret"); secret != "" {
						t.Fatalf("expected no X-Secret header, got %q", secret)
					}
				}

				var body struct {
					Query string `json:"query"`
					Count int    `json:"count"`
				}
				if err := json.Unmarshal([]byte(lastBody), &body); err != nil {
					t.Fatalf("failed to decode request body: %v", err)
				}
				if strings.TrimSpace(tc.query) != body.Query {
					t.Fatalf("expected query %q, got %q", strings.TrimSpace(tc.query), body.Query)
				}
				if body.Count != 10 {
					t.Fatalf("expected count 10, got %d", body.Count)
				}
			}
		})
	}
}

func TestSuggestAddressesHandlesEmptySuggestions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"suggestions":[]}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		APIKey:  "test-token",
		Timeout: 5 * time.Second,
		Logger:  discardLogger(),
	})

	got, err := client.SuggestAddresses(context.Background(), "xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty suggestions, got %d", len(got))
	}
}
