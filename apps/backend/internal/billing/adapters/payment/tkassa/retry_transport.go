package tkassa

import (
	"errors"
	"net"
	"net/http"
	"time"
)

// retryTransport wraps an http.RoundTripper and retries idempotent requests on
// temporary network errors. It does not retry on HTTP 4xx/5xx from the provider
// because those are business/provider-level outcomes.
type retryTransport struct {
	base       http.RoundTripper
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
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
	}
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt <= t.maxRetries; attempt++ {
		resp, err = t.base.RoundTrip(req.Clone(req.Context()))
		if err == nil {
			return resp, nil
		}
		if !isRetriable(err) || attempt == t.maxRetries {
			return nil, err
		}
		time.Sleep(t.backoff(attempt))
	}
	return resp, err
}

func (t *retryTransport) backoff(attempt int) time.Duration {
	d := t.baseDelay * (1 << attempt)
	if d > t.maxDelay {
		return t.maxDelay
	}
	return d
}

func isRetriable(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Temporary() || netErr.Timeout()
	}
	return false
}
