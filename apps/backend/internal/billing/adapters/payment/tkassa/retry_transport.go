package tkassa

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"time"
)

// retryTransport wraps an http.RoundTripper and retries requests on timeout
// network errors. It does not retry on HTTP 4xx/5xx from the provider because
// those are business/provider-level outcomes, and it does not retry when the
// caller has already canceled the context.
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
		_ = req.Body.Close()
		if err != nil {
			return nil, err
		}
	}

	for attempt := 0; attempt <= t.maxRetries; attempt++ {
		attemptReq := req.Clone(req.Context())
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
		if !isRetriable(err) || attempt == t.maxRetries {
			return nil, err
		}
		time.Sleep(t.jitter(t.backoff(attempt)))
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

func isRetriable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	return false
}
