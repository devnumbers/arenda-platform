// Package stream adapts the realtime context to the shared SSE transport
// (карта #714, тикет #716; ADR 0062): it resolves each coarse change's
// audience and hands the entity.changed frames to the platform hub. The
// transport is best-effort — the clients re-read the state through their
// APIs, the frames only wake them up.
package stream

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/sse"
	realtimeapp "github.com/nambers/arenda-planform/apps/backend/internal/realtime/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/realtime/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/clock"
	"github.com/nambers/arenda-planform/apps/backend/internal/shared/sanitize"
)

// EventEntityChanged is the stream's single coarse event name (ADR 0062 §2):
// every frame of this stream rides it, the browser dispatches it to its
// entity.changed listener.
const EventEntityChanged = "entity.changed"

// envelopeVersion is the envelope schema version (ADR 0060 §5): breaking
// payload changes bump it, additive ones keep it.
const envelopeVersion = 1

// Publisher implements realtimeapp.Publisher over the platform hub: it
// dedups the transaction's change pairs, resolves each pair's audience
// through derived property access at the moment of publication (ADR 0028)
// and fans the frame out to every recipient's open connections.
type Publisher struct {
	hub      *sse.Hub
	audience realtimeapp.FrameAudience
	clk      clock.Clock
	log      *slog.Logger
}

// The adapter conforms to the consumer-declared port (CODING_STANDARDS).
var _ realtimeapp.Publisher = (*Publisher)(nil)

// NewPublisher creates the stream publisher over the shared hub. A nil
// audience resolver or logger is a wiring mistake and panics on first use;
// the composition root always wires both.
func NewPublisher(hub *sse.Hub, audience realtimeapp.FrameAudience, clk clock.Clock, log *slog.Logger) *Publisher {
	return &Publisher{hub: hub, audience: audience, clk: clk, log: log}
}

// EntityChanged publishes one entity.changed frame per distinct change pair
// (ADR 0062 §3): the per-transaction dedup makes a bulk operation whose
// callers hand over repeated pairs emit one frame per pair. An audience
// failure skips that pair's frame — logged, never propagated: the committed
// mutation is long done and the reconnect-and-reread contract absorbs a lost
// frame.
func (p *Publisher) EntityChanged(ctx context.Context, actor uuid.UUID, changes ...domain.Change) {
	type pairKey struct {
		entity     domain.Entity
		propertyID string // "" is the owner-book pair; the ids dedup by value.
	}
	seen := make(map[pairKey]struct{}, len(changes))
	unique := make([]domain.Change, 0, len(changes))
	for _, c := range changes {
		key := pairKey{entity: c.Entity}
		if c.PropertyID != nil {
			key.propertyID = c.PropertyID.String()
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, c)
	}

	for _, c := range unique {
		recipients, err := p.recipients(ctx, actor, c)
		if err != nil {
			p.log.ErrorContext(ctx, "realtime: resolve frame audience failed",
				slog.String("entity", string(c.Entity)),
				slog.String("property_id", changePropertyID(c)),
				slog.String("error", sanitize.Error(err)))
			continue
		}
		if len(recipients) == 0 {
			continue
		}
		p.publish(ctx, recipients, c)
	}
}

// recipients resolves the change's audience: the object's readers by derived
// access, the actor alone for the owner-book changes (a nil PropertyID).
func (p *Publisher) recipients(ctx context.Context, actor uuid.UUID, c domain.Change) ([]uuid.UUID, error) {
	if c.PropertyID == nil {
		return []uuid.UUID{actor}, nil
	}
	readers, err := p.audience.Readers(ctx, *c.PropertyID)
	if err != nil {
		return nil, err
	}
	return readers, nil
}

// publish marshals the change into the envelope v1 frame and fans it out.
// The hub's Publish never blocks and never fails (slow consumers are
// dismissed, not errored), so there is nothing to report back.
func (p *Publisher) publish(ctx context.Context, recipients []uuid.UUID, c domain.Change) {
	data, err := json.Marshal(envelope{
		V:          envelopeVersion,
		OccurredAt: p.clk.Now().UTC().Format(time.RFC3339),
		Payload:    marshalPayload(payloadOf(c)),
	})
	if err != nil {
		// The envelope marshal of plain strings and ids cannot fail; the
		// frames are best-effort anyway.
		return
	}
	for _, userID := range recipients {
		p.hub.Publish(ctx, sse.Event{
			UserID: userID,
			Frame:  sse.Frame{Event: EventEntityChanged, Data: string(data)},
		})
	}
}

// envelope is the JSON wrapper inside every frame's data line (ADR 0060 §5):
// version, occurrence instant, per-type payload.
type envelope struct {
	V          int             `json:"v"`
	OccurredAt string          `json:"occurredAt"`
	Payload    json.RawMessage `json:"payload"`
}

// payload is the coarse frame's body (ADR 0062 §2): the dictionary name and
// the object — null for the owner-book changes. No entity data: the client
// re-reads through its API.
type payload struct {
	PropertyID *string `json:"propertyId"`
	Entity     string  `json:"entity"`
}

func payloadOf(c domain.Change) payload {
	p := payload{Entity: string(c.Entity)}
	if c.PropertyID != nil {
		id := c.PropertyID.String()
		p.PropertyID = &id
	}
	return p
}

// marshalPayload marshals a payload struct of plain strings and numbers.
// Such a marshal cannot fail, but the frames are best-effort anyway: the
// fallback is an empty payload, never an error path.
func marshalPayload(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return data
}

// changePropertyID renders the change's property id for the log line.
func changePropertyID(c domain.Change) string {
	if c.PropertyID == nil {
		return ""
	}
	return c.PropertyID.String()
}
