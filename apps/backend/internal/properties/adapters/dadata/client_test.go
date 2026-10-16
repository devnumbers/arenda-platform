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

// suggestCase configures one SuggestAddresses scenario: the upstream reply
// (code, JSON body, optional sleep), the request context timeout and the
// expected outcome.
type suggestCase struct {
	name        string
	query       string
	secretKey   string
	serverJSON  string
	serverCode  int
	serverSleep time.Duration
	ctxTimeout  time.Duration
	wantErr     error
	want        []propertiesapp.AddressSuggestion
}

// suggestRequestCapture records the last request the fake upstream received.
type suggestRequestCapture struct {
	request *http.Request
	body    string
}

// record stores a received request; it runs on the server goroutine before the
// reply is written, so the client observes the capture once its call returns.
func (c *suggestRequestCapture) record(t *testing.T, r *http.Request) {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Errorf("read request body: %v", err)
	}
	c.request = r
	c.body = string(body)
}

// newSuggestServer starts an httptest upstream answering per the case: the
// status code, the JSON body and an optional delay. It records the last
// received request for the header and body assertions.
func newSuggestServer(t *testing.T, tc suggestCase) (*httptest.Server, *suggestRequestCapture) {
	t.Helper()
	capture := &suggestRequestCapture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capture.record(t, r)
		if tc.serverSleep > 0 {
			time.Sleep(tc.serverSleep)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(tc.serverCode)
		if tc.serverJSON != "" {
			if _, err := w.Write([]byte(tc.serverJSON)); err != nil {
				t.Errorf("write server fixture body: %v", err)
			}
		}
	}))
	t.Cleanup(server.Close)
	return server, capture
}

// suggestContext builds the request context, with the per-case timeout when the
// case exercises cancellation.
func suggestContext(t *testing.T, timeout time.Duration) context.Context {
	t.Helper()
	if timeout <= 0 {
		return context.Background()
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)
	return ctx
}

// assertSuggestions compares the returned suggestions with the case fixture.
func assertSuggestions(t *testing.T, got, want []propertiesapp.AddressSuggestion) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %d suggestions, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i].Value != want[i].Value || got[i].City != want[i].City {
			t.Fatalf("suggestion %d: expected %+v, got %+v", i, want[i], got[i])
		}
	}
}

// assertSuggestHeader checks one upstream request header.
func assertSuggestHeader(t *testing.T, r *http.Request, name, want string) {
	t.Helper()
	if got := r.Header.Get(name); got != want {
		t.Fatalf("expected %s header %q, got %q", name, want, got)
	}
}

// assertSuggestRequest verifies the wire format of the request that reached the
// fake upstream: the auth headers (X-Secret only when configured) and the
// suggest JSON body.
func assertSuggestRequest(t *testing.T, capture *suggestRequestCapture, tc suggestCase) {
	t.Helper()
	if capture.request == nil {
		t.Fatal("expected request to reach test server")
	}
	assertSuggestHeader(t, capture.request, "Authorization", "Token test-token")
	assertSuggestHeader(t, capture.request, "Content-Type", "application/json")
	assertSuggestHeader(t, capture.request, "Accept", "application/json")
	if tc.secretKey != "" {
		assertSuggestHeader(t, capture.request, "X-Secret", tc.secretKey)
	} else if secret := capture.request.Header.Get("X-Secret"); secret != "" {
		t.Fatalf("expected no X-Secret header, got %q", secret)
	}

	var body struct {
		Query string `json:"query"`
		Count int    `json:"count"`
	}
	if err := json.Unmarshal([]byte(capture.body), &body); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}
	if strings.TrimSpace(tc.query) != body.Query {
		t.Fatalf("expected query %q, got %q", strings.TrimSpace(tc.query), body.Query)
	}
	if body.Count != 10 {
		t.Fatalf("expected count 10, got %d", body.Count)
	}
}

func TestSuggestAddresses(t *testing.T) {
	t.Parallel()

	cases := []suggestCase{
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
			t.Parallel()
			server, capture := newSuggestServer(t, tc)

			client := NewClient(Config{
				BaseURL:   server.URL,
				APIKey:    "test-token",
				SecretKey: tc.secretKey,
				Timeout:   5 * time.Second,
				Logger:    discardLogger(),
			})

			got, err := client.SuggestAddresses(suggestContext(t, tc.ctxTimeout), tc.query)

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

			assertSuggestions(t, got, tc.want)
			if tc.serverCode == 0 || tc.serverCode == http.StatusOK {
				assertSuggestRequest(t, capture, tc)
			}
		})
	}
}

func TestSuggestAddressesHandlesEmptySuggestions(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"suggestions":[]}`)); err != nil {
			t.Errorf("write empty-suggestions body: %v", err)
		}
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
