package sse

import "encoding/json"

// EnvelopeVersion is the envelope schema version (ADR 0060 §5): breaking
// payload changes bump it, additive ones keep it.
const EnvelopeVersion = 1

// Envelope is the shared wire contract inside every frame's data line
// (ADR 0060 §5): version, occurrence instant, per-type payload. The wrapper
// is package-wide — every context publishing through the hub rides this one
// shape; what stays per-context is only the payload's filling (ids and
// display fields, the client re-reads state through its API), handed over
// through MarshalPayload.
type Envelope struct {
	V          int             `json:"v"`
	OccurredAt string          `json:"occurredAt"`
	Payload    json.RawMessage `json:"payload"`
}

// MarshalPayload marshals a payload struct of plain strings and numbers.
// Such a marshal cannot fail, but the frames are best-effort anyway: the
// fallback is an empty payload, never an error path.
func MarshalPayload(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return data
}
