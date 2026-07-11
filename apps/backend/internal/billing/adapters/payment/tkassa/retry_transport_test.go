package tkassa

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type timeoutNetError struct {
	errorString string
}

func (e *timeoutNetError) Error() string   { return e.errorString }
func (e *timeoutNetError) Temporary() bool { return false }
func (e *timeoutNetError) Timeout() bool   { return true }

type nonTimeoutNetError struct {
	errorString string
}

func (e *nonTimeoutNetError) Error() string   { return e.errorString }
func (e *nonTimeoutNetError) Temporary() bool { return true }
func (e *nonTimeoutNetError) Timeout() bool   { return false }

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

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
}

func TestRetryTransportRetryThenSuccess(t *testing.T) {
	base := &countingRoundTripper{
		failures: 2,
		err:      &timeoutNetError{errorString: "timeout"},
	}
	tr := newRetryTransport(base, 3, 1*time.Millisecond, 10*time.Millisecond)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error after retries: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if base.calls.Load() != 3 {
		t.Fatalf("expected 3 calls (1 initial + 2 retries), got %d", base.calls.Load())
	}
}

func TestRetryTransportExhaustsRetries(t *testing.T) {
	retriableErr := &timeoutNetError{errorString: "boom"}
	base := &fakeRoundTripper{err: retriableErr}
	tr := newRetryTransport(base, 2, 1*time.Millisecond, 5*time.Millisecond)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", nil)
	resp, err := tr.RoundTrip(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
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
	retriableErr := &timeoutNetError{errorString: "boom"}
	base := &fakeRoundTripper{err: retriableErr}
	tr := newRetryTransport(base, 0, 1*time.Millisecond, 5*time.Millisecond)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", nil)
	resp, err := tr.RoundTrip(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
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

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", nil)
	resp, err := tr.RoundTrip(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil {
		t.Fatalf("expected error")
	}
	if base.calls.Load() != 1 {
		t.Fatalf("expected 1 call for permanent error, got %d", base.calls.Load())
	}
}

func TestRetryTransportPreservesBodyAcrossRetries(t *testing.T) {
	wantBody := `{"key":"value"}`
	base := &bodyCapturingRoundTripper{
		failures: 2,
		err:      &timeoutNetError{errorString: "timeout"},
	}
	tr := newRetryTransport(base, 3, 1*time.Millisecond, 10*time.Millisecond)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "http://example.com", bytes.NewReader([]byte(wantBody)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error after retries: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if base.calls.Load() != 3 {
		t.Fatalf("expected 3 calls (1 initial + 2 retries), got %d", base.calls.Load())
	}
	for i, got := range base.bodies {
		if got != wantBody {
			t.Fatalf("attempt %d body = %q, want %q", i+1, got, wantBody)
		}
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

func TestRetryTransportJitterApplied(t *testing.T) {
	retriableErr := &timeoutNetError{errorString: "boom"}
	base := &fakeRoundTripper{err: retriableErr}
	tr := newRetryTransport(base, 1, 1*time.Millisecond, 100*time.Millisecond)

	// Deterministic jitter: record the input it receives and return a fixed,
	// near-zero sleep so the test stays fast and wall-clock independent.
	const fixedSleep = time.Microsecond
	var (
		called    bool
		receivedD time.Duration
	)
	tr.jitter = func(d time.Duration) time.Duration {
		called = true
		receivedD = d
		return fixedSleep
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com", nil)
	resp, err := tr.RoundTrip(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil {
		t.Fatalf("expected error")
	}
	// maxRetries=1 with an always-failing base yields exactly one retry
	// (1 initial + 1 retry) and therefore exactly one sleep at attempt 0.
	if base.calls.Load() != 2 {
		t.Fatalf("expected 2 calls (1 initial + 1 retry), got %d", base.calls.Load())
	}
	if !called {
		t.Fatalf("expected jitter func to be invoked exactly once")
	}
	// The single sleep corresponds to attempt 0, so jitter must receive the
	// deterministic backoff(0) value; its return (fixedSleep) is what gets slept.
	if receivedD != tr.backoff(0) {
		t.Fatalf("jitter input = %v, want %v", receivedD, tr.backoff(0))
	}
}

func TestRetryTransportBackoffOverflowCap(t *testing.T) {
	tr := &retryTransport{baseDelay: time.Hour, maxDelay: 10 * time.Minute}
	// shift is capped at 30; time.Hour * (1<<30) exceeds int64 and wraps to a
	// negative value. The overflow guard must clamp the result to maxDelay and
	// never return a negative (or zero) duration.
	got := tr.backoff(30)
	if got != tr.maxDelay {
		t.Fatalf("backoff(30) = %v, want maxDelay %v", got, tr.maxDelay)
	}
	if got < 0 {
		t.Fatalf("backoff(30) returned negative duration %v (int64 overflow)", got)
	}
}

func TestRetryTransportCanRetry(t *testing.T) {
	timeout := &timeoutNetError{errorString: "timeout"}
	cases := []struct {
		name         string
		method       string
		err          error
		wroteRequest bool
		want         bool
	}{
		{"nil error", "GetState", nil, false, false},
		{"context canceled", "GetState", context.Canceled, false, false},
		{"non-timeout net error", "GetState", &nonTimeoutNetError{errorString: "temp"}, false, false},
		{"permanent net error", "GetState", &permanentNetError{errorString: "perm"}, false, false},
		{"plain error", "GetState", errors.New("plain"), false, false},
		{"read method retries timeout after send", "GetState", timeout, true, true},
		{"idempotent Init retries timeout after send", "Init", timeout, true, true},
		{"Charge retries timeout before send", "Charge", timeout, false, true},
		{"Charge never retries after send", "Charge", timeout, true, false},
		{"Cancel never retries after send", "Cancel", timeout, true, false},
		{"AddCard never retries after send", "AddCard", timeout, true, false},
		{"RemoveCard never retries after send", "RemoveCard", timeout, true, false},
		{"AddCustomer never retries after send", "AddCustomer", timeout, true, false},
		{"unknown method is conservative after send", "SomeFutureMethod", timeout, true, false},
		{"unknown method retries before send", "SomeFutureMethod", timeout, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := canRetry(tc.method, tc.err, tc.wroteRequest)
			if got != tc.want {
				t.Errorf("canRetry(%q, %v, %v) = %v, want %v", tc.method, tc.err, tc.wroteRequest, got, tc.want)
			}
		})
	}
}

// wroteRequestRoundTripper fires the request's httptrace WroteRequest callback
// before failing, simulating an error that happened after the request bytes
// were sent to the provider.
type wroteRequestRoundTripper struct {
	calls atomic.Int32
	err   error
}

func (w *wroteRequestRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	w.calls.Add(1)
	if trace := httptrace.ContextClientTrace(req.Context()); trace != nil && trace.WroteRequest != nil {
		trace.WroteRequest(httptrace.WroteRequestInfo{})
	}
	return nil, w.err
}

func TestRetryTransportChargeNotRetriedAfterRequestSent(t *testing.T) {
	base := &wroteRequestRoundTripper{err: &timeoutNetError{errorString: "read timeout"}}
	tr := newRetryTransport(base, 3, 1*time.Millisecond, 10*time.Millisecond)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://securepay.tinkoff.ru/v2/Charge", nil)
	resp, err := tr.RoundTrip(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil {
		t.Fatalf("expected error")
	}
	if base.calls.Load() != 1 {
		t.Fatalf("Charge must not be retried after the request was sent: got %d calls, want 1", base.calls.Load())
	}
}

func TestRetryTransportInitRetriedAfterRequestSent(t *testing.T) {
	base := &wroteRequestRoundTripper{err: &timeoutNetError{errorString: "read timeout"}}
	tr := newRetryTransport(base, 2, 1*time.Millisecond, 10*time.Millisecond)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://securepay.tinkoff.ru/v2/Init", nil)
	resp, err := tr.RoundTrip(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil {
		t.Fatalf("expected error")
	}
	if base.calls.Load() != 3 {
		t.Fatalf("Init is idempotent by OrderId and must be retried: got %d calls, want 3", base.calls.Load())
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

type bodyCapturingRoundTripper struct {
	calls    atomic.Int32
	failures int32
	err      error
	mu       sync.Mutex
	bodies   []string
}

func (b *bodyCapturingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	call := b.calls.Add(1)

	var body string
	if req.Body != nil {
		data, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		body = string(data)
	}

	b.mu.Lock()
	b.bodies = append(b.bodies, body)
	b.mu.Unlock()

	if call <= b.failures {
		return nil, b.err
	}
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

// Ensure fakeRoundTripper satisfies the interface at compile time.
var _ http.RoundTripper = (*fakeRoundTripper)(nil)
var _ http.RoundTripper = (*countingRoundTripper)(nil)

// Ensure net.Error implementations satisfy the interface at compile time.
var _ net.Error = (*timeoutNetError)(nil)
var _ net.Error = (*nonTimeoutNetError)(nil)
var _ net.Error = (*permanentNetError)(nil)
