package application

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
)

// MaxPayloadBytes is the practical ceiling for a Web Push payload. RFC 8030
// allows 4078 bytes of encrypted content; after aes128gcm overhead the safe
// application-payload budget is 3993 bytes (research #174, web.dev). We keep a
// margin under that.
const MaxPayloadBytes = 3993

// PushPayload is the JSON body sent to the service worker: the SW reads these
// fields and calls showNotification. It is intentionally minimal so the
// encrypted payload stays well under the 3993-byte ceiling. JSON field tags
// are defined once in marshalPushPayload (MarshalJSON overrides default
// encoding, so tags on this struct would be dead weight).
type PushPayload struct {
	Title     string
	Body      string
	Tag       string
	URL       string
	EventType domain.EventType
}

// NewPushPayload builds a push payload from a reminder. The tag collapses
// repeated notifications of the same kind on the same target so the user sees
// one current item instead of a stack of stale ones (RFC 8030 §5.4 topic). The
// URL routes the user to the relevant cabinet page on tap.
func NewPushPayload(r domain.Reminder) PushPayload {
	return PushPayload{
		Title:     r.MessageTitle,
		Body:      r.MessageBody,
		Tag:       pushTag(r),
		URL:       pushURL(r),
		EventType: r.EventType,
	}
}

// pushTag builds a collapse-key scoped to the event type and target so a newer
// push of the same kind replaces a pending older one (RFC 8030 §5.4). The tag
// is short and URL-safe (alphanumeric + ':' + '-' as required by push services
// for the Topic header).
func pushTag(r domain.Reminder) string {
	targetID := pushTargetID(r)
	if targetID == uuid.Nil {
		return string(r.EventType)
	}
	return fmt.Sprintf("%s:%s", r.EventType, targetID)
}

// pushTargetID returns the id of the concrete target the reminder is attached
// to, preferring the most specific target available.
func pushTargetID(r domain.Reminder) uuid.UUID {
	if r.OperationID != nil {
		return *r.OperationID
	}
	if r.LeaseID != nil {
		return *r.LeaseID
	}
	return uuid.Nil
}

// pushURL builds the relative cabinet path the user lands on when they tap the
// push. Operations and leases point at the property card (their detail is
// visible there).
func pushURL(r domain.Reminder) string {
	if r.PropertyID != nil {
		return fmt.Sprintf("/properties/%s", r.PropertyID)
	}
	return "/calendar"
}

// MarshalJSON encodes the payload as compact JSON. If the encoded size would
// exceed MaxPayloadBytes the Body is truncated to fit; this is a defensive
// measure since reminder titles/bodies are short in practice.
func (p PushPayload) MarshalJSON() ([]byte, error) {
	data, err := marshalPushPayload(p)
	if err != nil {
		return nil, err
	}

	if len(data) <= MaxPayloadBytes {
		return data, nil
	}

	// Truncate Body to fit under the ceiling. The overhead is the JSON keys +
	// tag + url + eventType, so we shrink Body by the excess plus a margin.
	excess := len(data) - MaxPayloadBytes
	truncated := p
	truncated.Body = truncateBytes(p.Body, max(len(p.Body)-excess, 0))

	return marshalPushPayload(truncated)
}

// marshalPushPayload encodes the payload fields to compact JSON with stable
// field order and omitempty on optional fields.
func marshalPushPayload(p PushPayload) ([]byte, error) {
	return json.Marshal(struct {
		Title     string           `json:"title"`
		Body      string           `json:"body"`
		Tag       string           `json:"tag,omitempty"`
		URL       string           `json:"url,omitempty"`
		EventType domain.EventType `json:"eventType"`
	}{
		Title:     p.Title,
		Body:      p.Body,
		Tag:       p.Tag,
		URL:       p.URL,
		EventType: p.EventType,
	})
}

// truncateBytes cuts s to at most n bytes without splitting a multi-byte UTF-8
// rune.
func truncateBytes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	// Walk back to a rune boundary.
	for n > 0 && !isRuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// isRuneStart reports whether the byte is the first byte of a UTF-8 encoded
// code point (continuation bytes start with 0b10xxxxxx).
func isRuneStart(b byte) bool {
	return b&0xC0 != 0x80
}
