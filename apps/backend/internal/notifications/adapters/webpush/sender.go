package webpush

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
)

// Compile-time check that *Sender satisfies the application.PushSender port.
var _ application.PushSender = (*Sender)(nil)

// Default push message lifetime (RFC 8030 §5.2): the push service stores an
// undelivered message for up to 30 days. Reminders are time-sensitive, so we
// keep a high TTL so a device that is briefly offline still receives the push.
const defaultTTL = 30 * 24 * time.Hour

// httpTimeout caps a single push request (connect + TLS + server processing).
const httpTimeout = 30 * time.Second

// maxResponseRead is the body snippet read into error messages; push services
// return tiny error bodies (RFC 8030), so 1 KiB is plenty.
const maxResponseRead = 1 << 10

// maxTopicLen is the maximum Topic header length in bytes (RFC 8030 §5.4).
// Push services reject longer values with 400. A topic that is too long or
// contains characters outside [A-Za-z0-9._~-] is silently dropped: the message
// is still delivered, it just loses its collapse/replace semantics.
const maxTopicLen = 32

// Urgency header values (RFC 8030 §5.3); shared with the sender tests.
const (
	urgencyHigh   = "high"
	urgencyNormal = "normal"
)

// Sender implements application.PushSender with a stdlib-only Web Push adapter:
// it encrypts the payload (RFC 8291), signs a per-origin VAPID JWT (RFC 8292),
// and POSTs to the push service endpoint (RFC 8030), mapping the response code
// to domain errors.
type Sender struct {
	vapid   *vapidSigner
	http    *http.Client
	ttl     int
	metrics *Metrics
	logger  *slog.Logger
}

// NewSender parses the VAPID keys and builds a Sender with a keep-alive HTTP
// client. The keys are base64url-encoded: publicKey is the 65-byte uncompressed
// P-256 key, privateKey is the 32-byte scalar. Returns an error if the keys are
// inconsistent (the public key must match the one derived from the private key).
// The metrics parameter records push delivery outcomes
// (sent/gone/rate_limited/failed); nil is safe (recording becomes a no-op).
func NewSender(subject, publicKey, privateKey string, metrics *Metrics, logger *slog.Logger) (*Sender, error) {
	signer, err := newVAPIDSigner(subject, publicKey, privateKey)
	if err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Sender{
		vapid:   signer,
		http:    &http.Client{Timeout: httpTimeout},
		ttl:     int(defaultTTL.Seconds()),
		metrics: metrics,
		logger:  logger,
	}, nil
}

// Send encrypts the payload and delivers it as a Web Push message to the
// subscription's endpoint. The response code is mapped to domain errors:
// ErrSubscriptionGone (404/410), ErrPushPayloadTooLarge (413), ErrRateLimited
// (429). A non-nil error otherwise means the send failed; the reminder worker
// treats push as best-effort (research #174 §5).
func (s *Sender) Send(ctx context.Context, sub domain.PushSubscription, payload application.PushPayload) error {
	plaintext, err := payload.MarshalJSON()
	if err != nil {
		return fmt.Errorf("webpush: marshal payload: %w", err)
	}

	msg, err := encryptPayload(sub, plaintext)
	if err != nil {
		return fmt.Errorf("webpush: encrypt payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Endpoint, bytes.NewReader(msg.Bytes()))
	if err != nil {
		return fmt.Errorf("webpush: build request: %w", err)
	}
	req.ContentLength = int64(len(msg.Bytes()))

	auth, err := s.vapid.authorizationHeader(sub.Endpoint)
	if err != nil {
		return fmt.Errorf("webpush: vapid authorization: %w", err)
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("TTL", strconv.Itoa(s.ttl))
	if topic := validTopic(payload.Tag); topic != "" {
		req.Header.Set("Topic", topic)
	}
	req.Header.Set("Urgency", urgencyForEventType(payload.EventType))

	resp, err := s.http.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return fmt.Errorf("webpush: post to push service: %w", err)
	}
	defer func() {
		// The response status is already mapped, so a close failure cannot
		// fail the dispatch — but it does break connection reuse, which is
		// worth a warning.
		if closeErr := resp.Body.Close(); closeErr != nil {
			s.logger.WarnContext(ctx, "webpush: close response body",
				"error", sanitize.Error(closeErr))
		}
	}()

	return s.mapResponse(ctx, resp)
}

// mapResponse translates the push service HTTP status code into a domain error
// (RFC 8030 §7). Success is 201 Created (the message was accepted for delivery,
// not necessarily delivered — research #174 §5).
func (s *Sender) mapResponse(ctx context.Context, resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusCreated, http.StatusOK:
		s.metrics.RecordDispatch(ctx, outcomeSent)
		return nil
	case http.StatusNotFound, http.StatusGone:
		// RFC 8030 §7.3: the subscription is no longer valid and must be deleted.
		s.metrics.RecordDispatch(ctx, outcomeGone)
		return application.ErrSubscriptionGone
	case http.StatusRequestEntityTooLarge:
		// RFC 8030 §7.2: the encrypted body exceeded the service's limit.
		s.metrics.RecordDispatch(ctx, outcomeFailed)
		return application.ErrPushPayloadTooLarge
	case http.StatusTooManyRequests:
		// RFC 8030 §8.4: honour Retry-After. The reminder worker stops sending
		// to this recipient's remaining devices when it sees ErrRateLimited.
		s.metrics.RecordDispatch(ctx, outcomeRateLimited)
		retryAfter, ok := parseRetryAfter(resp.Header.Get("Retry-After"))
		if ok {
			s.logger.WarnContext(ctx, "webpush rate limited",
				"retry_after", retryAfter, "endpoint", sanitize.String(resp.Request.URL.String()))
			return &RateLimitedError{RetryAfter: retryAfter}
		}
		s.logger.WarnContext(ctx, "webpush rate limited (no Retry-After)",
			"endpoint", sanitize.String(resp.Request.URL.String()))
		return application.ErrRateLimited
	default:
		s.metrics.RecordDispatch(ctx, outcomeFailed)
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseRead))
		if readErr != nil {
			return fmt.Errorf("webpush: push service returned status %d (response body read failed: %w)",
				resp.StatusCode, readErr)
		}
		return fmt.Errorf("webpush: push service returned status %d: %s",
			resp.StatusCode, sanitize.String(string(body)))
	}
}

// urgencyForEventType maps the reminder event type to a Web Push Urgency header
// value (RFC 8030 §5.3). Overdue and requires-action events are time-critical
// (the user is already late) and get "high" so the device wakes immediately;
// upcoming reminders and free reminders get "normal" to save battery (research
// #174 §4).
func urgencyForEventType(eventType domain.EventType) string {
	switch eventType {
	case domain.EventOperationOverdue, domain.EventLeaseRequiresAction:
		return urgencyHigh
	default:
		return urgencyNormal
	}
}

// validTopic validates a Topic header value against RFC 8030 §5.4: at most
// maxTopicLen bytes from the URL/filename-safe alphabet [A-Za-z0-9._~-]. A tag
// that fails validation is dropped (empty return) — the push is still sent, it
// just loses its collapse/replace semantics rather than being rejected with 400.
func validTopic(tag string) string {
	if len(tag) == 0 || len(tag) > maxTopicLen {
		return ""
	}
	for i := range len(tag) {
		if !isTopicByte(tag[i]) {
			return ""
		}
	}
	return tag
}

// isTopicByte reports whether b is allowed in a Topic header value (RFC 8030
// §5.4: a token from the URL/filename-safe alphabet).
func isTopicByte(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z',
		b >= 'A' && b <= 'Z',
		b >= '0' && b <= '9':
		return true
	case b == '.', b == '_', b == '~', b == '-':
		return true
	}
	return false
}

// RateLimitedError wraps application.ErrRateLimited with the Retry-After value
// the push service returned (RFC 8030 §8.4), so the caller can honour it
// instead of guessing a backoff delay.
type RateLimitedError struct {
	RetryAfter time.Duration
}

func (e *RateLimitedError) Error() string {
	return fmt.Sprintf("webpush: rate limited (retry after %s)", e.RetryAfter)
}

func (e *RateLimitedError) Unwrap() error { return application.ErrRateLimited }

// parseRetryAfter parses a Retry-After header (RFC 7231 §7.1.3): either a
// delta-seconds integer or an HTTP-date. A missing or invalid value returns
// ok=false.
func parseRetryAfter(v string) (time.Duration, bool) {
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second, true
	}
	if t, err := http.ParseTime(v); err == nil {
		return time.Until(t), true
	}
	return 0, false
}
