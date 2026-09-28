package stream_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/sse"
	"github.com/nambers/arenda-planform/apps/backend/internal/realtime/adapters/stream"
	realtimeapp "github.com/nambers/arenda-planform/apps/backend/internal/realtime/application"
	"github.com/nambers/arenda-planform/apps/backend/internal/realtime/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAudience is a scripted FrameAudience: a listed property resolves to its
// readers, any other one fails with err (nil err = empty audience).
type fakeAudience struct {
	readers map[uuid.UUID][]uuid.UUID
	err     error
}

func (f fakeAudience) Readers(_ context.Context, propertyID uuid.UUID) ([]uuid.UUID, error) {
	if readers, ok := f.readers[propertyID]; ok {
		return readers, nil
	}
	return nil, f.err
}

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC) }

// subscribe opens a connection for the user and hands the frame channel back.
func subscribe(t *testing.T, hub *sse.Hub, userID uuid.UUID) <-chan sse.Frame {
	t.Helper()
	frames, unsubscribe := hub.Subscribe(context.Background(), userID)
	t.Cleanup(unsubscribe)
	return frames
}

// readFrame waits for the next frame on the channel.
func readFrame(t *testing.T, frames <-chan sse.Frame) sse.Frame {
	t.Helper()
	select {
	case f := <-frames:
		return f
	case <-time.After(2 * time.Second):
		t.Fatalf("no frame within 2s")
		return sse.Frame{}
	}
}

// readNoFrame asserts silence on the channel.
func readNoFrame(t *testing.T, frames <-chan sse.Frame) {
	t.Helper()
	select {
	case f := <-frames:
		t.Fatalf("unexpected frame: %+v", f)
	case <-time.After(50 * time.Millisecond):
	}
}

func newPublisher(hub *sse.Hub, audience realtimeapp.FrameAudience) *stream.Publisher {
	return stream.NewPublisher(hub, audience, fixedClock{}, slog.Default())
}

func TestEntityChangedDeliversFramesToAudience(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	owner := uuid.Must(uuid.NewV7())
	member := uuid.Must(uuid.NewV7())
	outsider := uuid.Must(uuid.NewV7())
	prop := uuid.Must(uuid.NewV7())

	hub := sse.NewHub(nil)
	ownerFrames := subscribe(t, hub, owner)
	memberFrames := subscribe(t, hub, member)
	outsiderFrames := subscribe(t, hub, outsider)

	p := newPublisher(hub, fakeAudience{readers: map[uuid.UUID][]uuid.UUID{prop: {owner, member}}})

	p.EntityChanged(ctx, actorOf(owner), domain.On(domain.EntityPayments, prop))

	for _, frames := range []<-chan sse.Frame{ownerFrames, memberFrames} {
		frame := readFrame(t, frames)
		assert.Equal(t, "entity.changed", frame.Event)
		assert.Equal(t, "payments", entityOf(t, frame.Data))
		assert.Equal(t, prop.String(), propertyIDOf(t, frame.Data))
	}
	readNoFrame(t, outsiderFrames)
}

// TestEntityChangedActorIsOwnAudienceForOwnerBook pins the nil-property
// contract (ADR 0052): the owner-book change's audience is the actor alone —
// the book has no members — and the payload's propertyId is null.
func TestEntityChangedActorIsOwnAudienceForOwnerBook(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	actor := uuid.Must(uuid.NewV7())
	other := uuid.Must(uuid.NewV7())

	hub := sse.NewHub(nil)
	actorFrames := subscribe(t, hub, actor)
	otherFrames := subscribe(t, hub, other)

	p := newPublisher(hub, fakeAudience{})
	p.EntityChanged(ctx, actor, domain.InOwnerBook(domain.EntityContacts))

	frame := readFrame(t, actorFrames)
	payload := envelopeOf(t, frame.Data)
	assert.Nil(t, payload.PropertyID, "the owner-book frame carries a null propertyId")
	assert.Equal(t, "contacts", payload.Entity)
	readNoFrame(t, otherFrames)
}

// TestEntityChangedDedupsPairsPerCall pins the per-transaction dedup
// (ADR 0062 §3): repeated pairs collapse into one frame — a bulk operation
// emits one frame per pair, not N.
func TestEntityChangedDedupsPairsPerCall(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	owner := uuid.Must(uuid.NewV7())
	prop := uuid.Must(uuid.NewV7())
	otherProp := uuid.Must(uuid.NewV7())

	hub := sse.NewHub(nil)
	frames := subscribe(t, hub, owner)

	p := newPublisher(hub, fakeAudience{readers: map[uuid.UUID][]uuid.UUID{
		prop:      {owner},
		otherProp: {owner},
	}})
	p.EntityChanged(ctx, owner,
		domain.On(domain.EntityTasks, prop),
		domain.On(domain.EntityTasks, prop),
		domain.HistoryOn(prop),
		domain.On(domain.EntityTasks, prop),
		domain.On(domain.EntityTasks, otherProp),
		domain.On(domain.EntityTasks, otherProp),
	)

	count := map[string]int{}
	for range 3 {
		frame := readFrame(t, frames)
		count[propertyIDOf(t, frame.Data)+" "+entityOf(t, frame.Data)]++
	}
	assert.Equal(t, map[string]int{
		prop.String() + " tasks":      1,
		prop.String() + " history":    1,
		otherProp.String() + " tasks": 1,
	}, count)
	readNoFrame(t, frames)
}

// TestEntityChangedIsBestEffort pins the best-effort canon: an audience
// failure skips that pair's frame only — the other pairs still ride out, and
// the failure never panics or propagates (the mutation committed long ago).
func TestEntityChangedIsBestEffort(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	owner := uuid.Must(uuid.NewV7())
	prop := uuid.Must(uuid.NewV7())
	broken := uuid.Must(uuid.NewV7())

	hub := sse.NewHub(nil)
	frames := subscribe(t, hub, owner)

	p := newPublisher(hub, fakeAudience{
		readers: map[uuid.UUID][]uuid.UUID{prop: {owner}},
		err:     errors.New("audience store down"),
	})
	p.EntityChanged(ctx, owner,
		domain.On(domain.EntityOperations, broken),
		domain.On(domain.EntityPayments, prop),
	)

	frame := readFrame(t, frames)
	assert.Equal(t, "payments", entityOf(t, frame.Data), "the healthy pair rides out")
	readNoFrame(t, frames) // The broken pair's frame is skipped, not propagated.
}

// TestEntityChangedSilentWhenNobodyListens: an audience of nobody — the
// deleted-object case — publishes nothing and does not reach the hub.
func TestEntityChangedSilentWhenNobodyListens(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	prop := uuid.Must(uuid.NewV7())
	hub := sse.NewHub(nil)
	p := newPublisher(hub, fakeAudience{readers: map[uuid.UUID][]uuid.UUID{}})

	p.EntityChanged(ctx, uuid.Must(uuid.NewV7()), domain.On(domain.EntityProperty, prop))
}

// TestEnvelopeShape pins the wire format (ADR 0060 §5 + ADR 0062 §2): the
// envelope v1 with the UTC RFC3339 occurredAt and the {propertyId, entity}
// payload.
func TestEnvelopeShape(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	owner := uuid.Must(uuid.NewV7())
	prop := uuid.Must(uuid.NewV7())
	hub := sse.NewHub(nil)
	frames := subscribe(t, hub, owner)

	p := newPublisher(hub, fakeAudience{readers: map[uuid.UUID][]uuid.UUID{prop: {owner}}})
	p.EntityChanged(ctx, owner, domain.On(domain.EntityRentals, prop))

	frame := readFrame(t, frames)
	var envelope struct {
		V          int    `json:"v"`
		OccurredAt string `json:"occurredAt"`
		Payload    struct {
			PropertyID string `json:"propertyId"`
			Entity     string `json:"entity"`
		} `json:"payload"`
	}
	require.NoError(t, json.Unmarshal([]byte(frame.Data), &envelope))
	assert.Equal(t, 1, envelope.V)
	assert.Equal(t, "2026-09-25T12:00:00Z", envelope.OccurredAt)
	assert.Equal(t, prop.String(), envelope.Payload.PropertyID)
	assert.Equal(t, "rentals", envelope.Payload.Entity)
}

// envelopeOf parses the wire envelope's payload out of a frame's data line
// (ADR 0060 §5): the coarse fields live inside the payload object.
func envelopeOf(t *testing.T, data string) struct {
	PropertyID *string `json:"propertyId"`
	Entity     string  `json:"entity"`
} {
	t.Helper()
	var envelope struct {
		Payload struct {
			PropertyID *string `json:"propertyId"`
			Entity     string  `json:"entity"`
		} `json:"payload"`
	}
	require.NoError(t, json.Unmarshal([]byte(data), &envelope))
	return envelope.Payload
}

func entityOf(t *testing.T, data string) string {
	t.Helper()
	return envelopeOf(t, data).Entity
}

func propertyIDOf(t *testing.T, data string) string {
	t.Helper()
	payload := envelopeOf(t, data)
	if payload.PropertyID == nil {
		return ""
	}
	return *payload.PropertyID
}

func actorOf(id uuid.UUID) uuid.UUID { return id }
