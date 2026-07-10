package tkassa

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type temporaryNetError struct {
	errorString string
	timeout     bool
}

func (e *temporaryNetError) Error() string   { return e.errorString }
func (e *temporaryNetError) Temporary() bool { return true }
func (e *temporaryNetError) Timeout() bool   { return e.timeout }

type permanentNetError struct {
	errorString string
}

func (e *permanentNetError) Error() string   { return e.errorString }
func (e *permanentNetError) Temporary() bool { return false }
func (e *permanentNetError) Timeout() bool   { return false }

func TestRetryTransportSuccessFirstAttempt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	base := http.DefaultTransport
	tr := newRetryTransport(base, 3, 1*time.Millisecond, 10*time.Millisecond)
	client := &http.Client{Transport: tr}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
}

func TestRetryTransportRetryThenSuccess(t *testing.T) {
	base := &countingRoundTripper{
		failures: 2,
		err:      &temporaryNetError{errorString: "timeout", timeout: true},
	}
	tr := newRetryTransport(base, 3, 1*time.Millisecond, 10*time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error after retries: %v", err)
	}
	_ = resp.Body.Close()
	if base.calls.Load() != 3 {
		t.Fatalf("expected 3 calls (1 initial + 2 retries), got %d", base.calls.Load())
	}
}

func TestRetryTransportExhaustsRetries(t *testing.T) {
	retriableErr := &temporaryNetError{errorString: "boom"}
	base := &fakeRoundTripper{err: retriableErr}
	tr := newRetryTransport(base, 2, 1*time.Millisecond, 5*time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	_, err := tr.RoundTrip(req)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, retriableErr) {
		t.Fatalf("expected retriable error, got %v", err)
	}
	if base.calls.Load() != 3 {
		t.Fatalf("expected 3 calls (1 initial + 2 retries), got %d", base.calls.Load())
	}
}

func TestRetryTransportNoRetryWhenDisabled(t *testing.T) {
	retriableErr := &temporaryNetError{errorString: "boom"}
	base := &fakeRoundTripper{err: retriableErr}
	tr := newRetryTransport(base, 0, 1*time.Millisecond, 5*time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	_, err := tr.RoundTrip(req)
	if err == nil {
		t.Fatalf("expected error")
	}
	if base.calls.Load() != 1 {
		t.Fatalf("expected 1 call when retries disabled, got %d", base.calls.Load())
	}
}

func TestRetryTransportNoRetryOnPermanentError(t *testing.T) {
	permanentErr := &permanentNetError{errorString: "permanent"}
	base := &fakeRoundTripper{err: permanentErr}
	tr := newRetryTransport(base, 3, 1*time.Millisecond, 5*time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	_, err := tr.RoundTrip(req)
	if err == nil {
		t.Fatalf("expected error")
	}
	if base.calls.Load() != 1 {
		t.Fatalf("expected 1 call for permanent error, got %d", base.calls.Load())
	}
}

func TestRetryTransportBackoffExponential(t *testing.T) {
	tr := &retryTransport{baseDelay: 10 * time.Millisecond, maxDelay: 100 * time.Millisecond}
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 10 * time.Millisecond},
		{1, 20 * time.Millisecond},
		{2, 40 * time.Millisecond},
		{3, 80 * time.Millisecond},
		{4, 100 * time.Millisecond},
		{5, 100 * time.Millisecond},
	}
	for _, tc := range cases {
		got := tr.backoff(tc.attempt)
		if got != tc.want {
			t.Errorf("backoff(%d) = %v, want %v", tc.attempt, got, tc.want)
		}
	}
}

func TestRetryTransportIsRetriable(t *testing.T) {
	if isRetriable(nil) {
		t.Fatalf("nil error should not be retriable")
	}
	if !isRetriable(&temporaryNetError{errorString: "temp"}) {
		t.Fatalf("temporary net error should be retriable")
	}
	if !isRetriable(&temporaryNetError{errorString: "timeout", timeout: true}) {
		t.Fatalf("timeout net error should be retriable")
	}
	if isRetriable(&permanentNetError{errorString: "perm"}) {
		t.Fatalf("permanent net error should not be retriable")
	}
	if isRetriable(errors.New("plain")) {
		t.Fatalf("plain error should not be retriable")
	}
}

type fakeRoundTripper struct {
	calls atomic.Int32
	err   error
}

func (f *fakeRoundTripper) RoundTrip(_ *http.Request) (*http.Response, error) {
	f.calls.Add(1)
	return nil, f.err
}

type countingRoundTripper struct {
	calls    atomic.Int32
	failures int32
	err      error
}

func (c *countingRoundTripper) RoundTrip(_ *http.Request) (*http.Response, error) {
	call := c.calls.Add(1)
	if call <= c.failures {
		return nil, c.err
	}
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

// Ensure fakeRoundTripper satisfies the interface at compile time.
var _ http.RoundTripper = (*fakeRoundTripper)(nil)
var _ http.RoundTripper = (*countingRoundTripper)(nil)

// Ensure net.Error implementations satisfy the interface at compile time.
var _ net.Error = (*temporaryNetError)(nil)
var _ net.Error = (*permanentNetError)(nil)
