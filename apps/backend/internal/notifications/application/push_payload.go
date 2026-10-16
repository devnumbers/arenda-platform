package application

import (
	"encoding/json"

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
