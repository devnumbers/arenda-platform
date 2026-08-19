package tkassa

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptrace"
	"path"
	"sync/atomic"
	"time"
)

// retryTransport wraps an http.RoundTripper and retries requests on timeout
// network errors. It does not retry on HTTP 4xx/5xx from the provider because
// those are business/provider-level outcomes, and it does not retry when the
// caller has already canceled the context.
//
// Retry safety is per-method: T-Kassa write operations (Charge first of all)
// are not idempotent — the Charge body carries only PaymentId/RebillId without
// an OrderId, so repeating it after a read timeout would charge the card
// twice. Non-idempotent methods are retried only when the request never left
// the client (tracked via httptrace WroteRequest). Read and idempotent
// methods (GetState, Init-by-OrderId) are retried on any timeout.
type retryTransport struct {
	base       http.RoundTripper
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
	// jitter maps the deterministic backoff delay to the actual sleep duration.
	// It defaults to the identity so callers (and tests) that do not configure
	// it observe the exact backoff values; production wiring installs full-jitter.
	jitter func(time.Duration) time.Duration
}

// retryOnTimeoutMethods may be retried even after the request bytes reached
// the provider: GetState is a pure read and Init is idempotent by OrderId —
// T-Kassa returns the existing payment when the same OrderId is initialized
// again. Every other method (Charge, Cancel, AddCustomer, AddCard,
// RemoveCard, and anything unknown) is retried only before the request is
// sent.
var retryOnTimeoutMethods = map[string]bool{
	"Init":     true,
	"GetState": true,
}

func newRetryTransport(base http.RoundTripper, maxRetries int, baseDelay, maxDelay time.Duration) *retryTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &retryTransport{
		base:       base,
		maxRetries: maxRetries,
		baseDelay:  baseDelay,
		maxDelay:   maxDelay,
		jitter:     func(d time.Duration) time.Duration { return d },
	}
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	// Capture the request body once so it can be replayed on each retry.
	// http.Request.Clone performs only a shallow copy of Body, so a fresh
	// ReadCloser is required for every attempt.
	var body []byte
	if req.Body != nil {
		body, err = io.ReadAll(req.Body)
		closeErr := req.Body.Close()
		if err != nil {
			// The read error is the more informative failure; a concurrent
			// close error is intentionally dropped in its favor.
			return nil, err
		}
		// The body bytes are captured, but a stream that fails to close is a
		// transport anomaly worth surfacing rather than discarding.
		if closeErr != nil {
			return nil, fmt.Errorf("tkassa: close request body: %w", closeErr)
		}
	}

	method := path.Base(req.URL.Path)

	for attempt := 0; attempt <= t.maxRetries; attempt++ {
		// Track whether the request bytes were actually written to the wire:
		// a timeout after WroteRequest means the provider may already be
		// processing the call, so non-idempotent methods must not retry.
		var wroteRequest atomic.Bool
		trace := &httptrace.ClientTrace{
			WroteRequest: func(httptrace.WroteRequestInfo) { wroteRequest.Store(true) },
		}
		attemptReq := req.Clone(httptrace.WithClientTrace(req.Context(), trace))
		if body != nil {
			attemptReq.Body = io.NopCloser(bytes.NewReader(body))
			attemptReq.ContentLength = int64(len(body))
			attemptReq.GetBody = func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(body)), nil
			}
		}

		resp, err = t.base.RoundTrip(attemptReq)
		if err == nil {
			return resp, nil
		}
		if !canRetry(method, err, wroteRequest.Load()) || attempt == t.maxRetries {
			return nil, err
		}
		if sleepErr := sleepContext(req.Context(), t.jitter(t.backoff(attempt))); sleepErr != nil {
			return nil, sleepErr
		}
	}
	return resp, err
}

func (t *retryTransport) backoff(attempt int) time.Duration {
	shift := min(attempt, 30)
	d := t.baseDelay * (1 << shift)
	// Clamp to maxDelay both when the delay exceeds the cap and when a large
	// baseDelay shifted high overflows int64 and wraps to zero or negative.
	if d <= 0 || d > t.maxDelay {
		d = t.maxDelay
	}
	return d
}

// cryptoJitter draws the full-jitter sleep uniformly from [0, d) using
// crypto/rand. The cryptographic source is not about secrecy — gosec G404
// rejects math/rand for this outright — and retry sleeps are rare, so its
// cost is irrelevant. A non-positive delay or a read failure degrades to no
// sleep instead of panicking or failing the retry loop.
func cryptoJitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(d)))
	if err != nil {
		return 0
	}
	return time.Duration(n.Int64())
}

// sleepContext waits for d or until ctx is done, whichever happens first, so
// a canceled request does not sit out the whole backoff delay.
func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// canRetry reports whether a failed RoundTrip may be safely repeated.
//
// Only timeout network errors are retriable, and context.Canceled never is.
// Read/idempotent methods retry on any such timeout; non-idempotent writes
// retry only when the request was never written to the wire.
func canRetry(method string, err error, wroteRequest bool) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		return false
	}
	if retryOnTimeoutMethods[method] {
		return true
	}
	return !wroteRequest
}
