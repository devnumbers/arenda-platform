package webpush

import (
	"context"
	"crypto/ecdh"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
)

// newTestSender builds a Sender with freshly generated VAPID keys pointing at
// the given test server's origin.
func newTestSender(t *testing.T) *Sender {
	t.Helper()
	privB64, pubB64 := generateTestVAPIDKeys(t)
	s, err := NewSender("mailto:test@example.com", pubB64, privB64, nil, nil)
	if err != nil {
		t.Fatalf("new sender: %v", err)
	}
	return s
}

// newTestSubscription builds a subscription whose endpoint points at the test
// server, with a freshly generated P-256 key pair so encryption succeeds.
func newTestSubscription(t *testing.T, endpoint string) domain.PushSubscription {
	t.Helper()
	priv, err := ecdh.P256().GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate p256 key: %v", err)
	}
	auth := make([]byte, 16)
	return domain.PushSubscription{
		Endpoint: endpoint,
		P256dh:   base64.RawURLEncoding.EncodeToString(priv.PublicKey().Bytes()),
		Auth:     base64.RawURLEncoding.EncodeToString(auth),
	}
}

func TestSend_Success(t *testing.T) {
	var gotHeaders http.Header
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	sender := newTestSender(t)
	sub := newTestSubscription(t, srv.URL+"/fcm/send/abc")
	payload := application.PushPayload{Title: "Test", Body: "Hello", Tag: "tag1", URL: "/x"}

	if err := sender.Send(t.Context(), sub, payload); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	// Verify required headers are present.
	if got := gotHeaders.Get("Content-Encoding"); got != "aes128gcm" {
		t.Errorf("Content-Encoding = %q, want aes128gcm", got)
	}
	if got := gotHeaders.Get("TTL"); got != "2592000" {
		t.Errorf("TTL = %q, want 2592000", got)
	}
	if got := gotHeaders.Get("Topic"); got != "tag1" {
		t.Errorf("Topic = %q, want tag1", got)
	}
	if got := gotHeaders.Get("Urgency"); got != "normal" {
		t.Errorf("Urgency = %q, want normal", got)
	}
	authHeader := gotHeaders.Get("Authorization")
	if authHeader == "" || authHeader[:6] != "vapid " {
		t.Errorf("Authorization = %q, want 'vapid t=...,k=...'", authHeader)
	}

	// Verify the body is a valid RFC 8188 message: header is at least 86 bytes
	// and the idlen byte is 65.
	if len(gotBody) <= headerLen {
		t.Fatalf("body too short: %d bytes", len(gotBody))
	}
	if gotBody[20] != p256UncompressedSize {
		t.Errorf("idlen byte = %d, want %d", gotBody[20], p256UncompressedSize)
	}
}

func TestSend_ResponseCodeMapping(t *testing.T) {
	cases := []struct {
		name      string
		status    int
		wantErr   error
		wantInMsg string
	}{
		{"201 created", http.StatusCreated, nil, ""},
		{"200 ok", http.StatusOK, nil, ""},
		{"404 not found", http.StatusNotFound, application.ErrSubscriptionGone, ""},
		{"410 gone", http.StatusGone, application.ErrSubscriptionGone, ""},
		{"413 too large", http.StatusRequestEntityTooLarge, application.ErrPushPayloadTooLarge, ""},
		{"429 rate limited", http.StatusTooManyRequests, application.ErrRateLimited, ""},
		{"400 bad request", http.StatusBadRequest, nil, "status 400"},
		{"403 forbidden", http.StatusForbidden, nil, "status 403"},
		{"500 server error", http.StatusInternalServerError, nil, "status 500"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tc.status == http.StatusTooManyRequests {
					w.Header().Set("Retry-After", "60")
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte("error details"))
			}))
			defer srv.Close()

			sender := newTestSender(t)
			sub := newTestSubscription(t, srv.URL+"/push/abc")

			err := sender.Send(t.Context(), sub, application.PushPayload{Title: "T", Body: "B"})
			switch {
			case tc.wantErr != nil:
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("expected %v, got %v", tc.wantErr, err)
				}
			case tc.wantInMsg != "":
				if err == nil || !strings.Contains(err.Error(), tc.wantInMsg) {
					t.Errorf("expected error containing %q, got %v", tc.wantInMsg, err)
				}
			default:
				if err != nil {
					t.Errorf("expected nil error, got %v", err)
				}
			}
		})
	}
}

func TestSend_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Block so the context cancellation surfaces.
		select {}
	}))
	defer srv.Close()

	sender := newTestSender(t)
	sub := newTestSubscription(t, srv.URL+"/push/abc")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := sender.Send(ctx, sub, application.PushPayload{Title: "T", Body: "B"})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestNewSender_NilLoggerDefaultsToDefault(t *testing.T) {
	privB64, pubB64 := generateTestVAPIDKeys(t)
	s, err := NewSender("mailto:test@example.com", pubB64, privB64, nil, nil)
	if err != nil {
		t.Fatalf("new sender: %v", err)
	}
	if s.logger == nil {
		t.Error("logger must not be nil (defaults to slog.Default())")
	}
}

func TestSend_RateLimitedCarriesRetryAfter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	sender := newTestSender(t)
	sub := newTestSubscription(t, srv.URL+"/push/abc")

	err := sender.Send(t.Context(), sub, application.PushPayload{Title: "T", Body: "B"})
	var rlErr *RateLimitedError
	if !errors.As(err, &rlErr) {
		t.Fatalf("expected *RateLimitedError, got %v", err)
	}
	if rlErr.RetryAfter != 120*time.Second {
		t.Errorf("RetryAfter = %v, want 120s", rlErr.RetryAfter)
	}
	if !errors.Is(err, application.ErrRateLimited) {
		t.Error("RateLimitedError must unwrap to application.ErrRateLimited")
	}
}

func TestSend_RateLimitedNoRetryAfter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	sender := newTestSender(t)
	sub := newTestSubscription(t, srv.URL+"/push/abc")

	err := sender.Send(t.Context(), sub, application.PushPayload{Title: "T", Body: "B"})
	if !errors.Is(err, application.ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
	var rlErr *RateLimitedError
	if errors.As(err, &rlErr) {
		t.Error("bare ErrRateLimited (no Retry-After) should not wrap into *RateLimitedError")
	}
}

func TestValidTopic(t *testing.T) {
	cases := []struct {
		tag  string
		want string
	}{
		{"", ""},
		{"abc", "abc"},
		{"operation_overdue", "operation_overdue"},
		{"a.b-c~d", "a.b-c~d"},
		{strings.Repeat("a", maxTopicLen), strings.Repeat("a", maxTopicLen)},
		{strings.Repeat("a", maxTopicLen+1), ""},
		{"bad space", ""},
		{"bad:colon", ""},
		{"привет", ""},
	}
	for _, tc := range cases {
		t.Run(tc.tag, func(t *testing.T) {
			got := validTopic(tc.tag)
			if tc.tag == "" {
				if got != "" {
					t.Errorf("validTopic(%q) = %q, want empty", tc.tag, got)
				}
				return
			}
			if got != tc.want {
				t.Errorf("validTopic(%q) = %q, want %q", tc.tag, got, tc.want)
			}
		})
	}
}

func TestParseRetryAfter(t *testing.T) {
	cases := []struct {
		input   string
		wantDur time.Duration
		wantOk  bool
	}{
		{"", 0, false},
		{"60", 60 * time.Second, true},
		{"0", 0, true},
		{"not-a-number", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			d, ok := parseRetryAfter(tc.input)
			if ok != tc.wantOk {
				t.Errorf("parseRetryAfter(%q) ok = %v, want %v", tc.input, ok, tc.wantOk)
			}
			if ok && d != tc.wantDur {
				t.Errorf("parseRetryAfter(%q) = %v, want %v", tc.input, d, tc.wantDur)
			}
		})
	}
}

func TestUrgencyForEventType(t *testing.T) {
	cases := []struct {
		eventType domain.EventType
		want      string
	}{
		{domain.EventOperationDue, "normal"},
		{domain.EventOperationOverdue, "high"},
		{domain.EventLeaseExpiring, "normal"},
		{domain.EventLeaseRequiresAction, "high"},
		{domain.EventFreeReminder, "normal"},
	}
	for _, tc := range cases {
		t.Run(string(tc.eventType), func(t *testing.T) {
			if got := urgencyForEventType(tc.eventType); got != tc.want {
				t.Errorf("urgencyForEventType(%q) = %q, want %q", tc.eventType, got, tc.want)
			}
		})
	}
}
