package sse

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// receive reads one frame or fails the test after a short wait, so a missing
// delivery fails instead of hanging.
func receive(t *testing.T, ch <-chan Frame) Frame {
	t.Helper()
	select {
	case f := <-ch:
		return f
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a frame")
		return Frame{}
	}
}

// expectClosed asserts the connection channel eventually reports closed. A
// dismissed connection's buffer still yields its queued frames first — the
// close is the LAST thing a reader observes — so drain until open=false.
func expectClosed(t *testing.T, ch <-chan Frame) {
	t.Helper()
	for {
		select {
		case _, open := <-ch:
			if !open {
				return
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for the channel to close")
			return
		}
	}
}

func TestHubDeliversFrameToRecipient(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	user := uuid.Must(uuid.NewV7())
	frames, unsub := hub.Subscribe(context.Background(), user)
	defer unsub()

	hub.Publish(context.Background(), Event{
		UserID: user,
		Frame:  Frame{Event: "notification.created", Data: `{"id":"n1"}`},
	})

	f := receive(t, frames)
	assert.Equal(t, "1", f.ID, "the hub stamps the monotonic id")
	assert.Equal(t, "notification.created", f.Event)
	assert.JSONEq(t, `{"id":"n1"}`, f.Data)
}

func TestHubFanOutToEveryUserConnection(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	user := uuid.Must(uuid.NewV7())
	a, unsubA := hub.Subscribe(context.Background(), user)
	defer unsubA()
	b, unsubB := hub.Subscribe(context.Background(), user)
	defer unsubB()

	hub.Publish(context.Background(), Event{UserID: user, Frame: Frame{Data: "x"}})

	assert.Equal(t, "x", receive(t, a).Data)
	assert.Equal(t, "x", receive(t, b).Data)
}

func TestHubKeepsUsersIsolated(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	userA := uuid.Must(uuid.NewV7())
	a, unsubA := hub.Subscribe(context.Background(), userA)
	defer unsubA()
	b, unsubB := hub.Subscribe(context.Background(), uuid.Must(uuid.NewV7()))
	defer unsubB()

	hub.Publish(context.Background(), Event{UserID: uuid.Must(uuid.NewV7()), Frame: Frame{Data: "stranger"}})
	hub.Publish(context.Background(), Event{UserID: userA, Frame: Frame{Data: "for-a"}})

	assert.Equal(t, "for-a", receive(t, a).Data)
	select {
	case f := <-b:
		t.Fatalf("another user's connection received %v", f)
	default:
	}
}

func TestHubFrameIDsAreMonotonic(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	user := uuid.Must(uuid.NewV7())
	frames, unsub := hub.Subscribe(context.Background(), user)
	defer unsub()

	for want := 1; want <= 3; want++ {
		hub.Publish(context.Background(), Event{UserID: user, Frame: Frame{Data: "x"}})
		assert.Equal(t, strconv.Itoa(want), receive(t, frames).ID)
	}
}

func TestHubUnsubscribeStopsDelivery(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	user := uuid.Must(uuid.NewV7())
	frames, unsub := hub.Subscribe(context.Background(), user)

	hub.Publish(context.Background(), Event{UserID: user, Frame: Frame{Data: "before"}})
	assert.Equal(t, "before", receive(t, frames).Data)

	unsub()
	// The unsubscribe closes the channel — the stream handler's signal to
	// return — and the hub stops addressing the connection entirely.
	expectClosed(t, frames)
	hub.Publish(context.Background(), Event{UserID: user, Frame: Frame{Data: "after"}})
}

func TestHubUnsubscribeIsIdempotent(t *testing.T) {
	t.Parallel()

	// Dismissal closes the channel exactly once and stays safe when the
	// handler's defer races the hub's own eviction or shutdown path.
	hub := NewHub(nil)
	user := uuid.Must(uuid.NewV7())
	frames, unsub := hub.Subscribe(context.Background(), user)
	unsub()
	unsub()
	expectClosed(t, frames)
}

func TestHubEvictsOldestConnectionAtPerUserLimit(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	user := uuid.Must(uuid.NewV7())
	chans := make([]<-chan Frame, 0, maxConnsPerUser+1)
	unsubs := make([]func(), 0, maxConnsPerUser+1)
	for range maxConnsPerUser + 1 {
		frames, unsub := hub.Subscribe(context.Background(), user)
		chans = append(chans, frames)
		unsubs = append(unsubs, unsub)
	}
	t.Cleanup(func() {
		for _, unsub := range unsubs {
			unsub()
		}
	})

	// The oldest connection is displaced: its channel is closed so the
	// handler returns and the client reconnects.
	expectClosed(t, chans[0])
	for _, ch := range chans[1:] {
		select {
		case f, open := <-ch:
			assert.True(t, open, "younger connections stay open (got close with frame %v)", f)
		default:
		}
	}

	// The registry keeps the limit: maxConnsPerUser live connections remain.
	hub.mu.RLock()
	live := len(hub.byUser[user])
	hub.mu.RUnlock()
	assert.Equal(t, maxConnsPerUser, live)
}

func TestHubSlowConsumerIsDroppedAndClosed(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	user := uuid.Must(uuid.NewV7())
	slow, unsubSlow := hub.Subscribe(context.Background(), user)
	defer unsubSlow()

	// Fill the slow connection's buffer without reading from it. The healthy
	// connection subscribes AFTER the fill, so only the slow side is at the
	// overflow when the next publish lands.
	for range subscriberBuffer {
		hub.Publish(context.Background(), Event{UserID: user, Frame: Frame{Data: "fill"}})
	}
	healthy, unsubHealthy := hub.Subscribe(context.Background(), user)
	defer unsubHealthy()

	// One more publish finds the buffer full: the connection is dropped and
	// closed instead of blocking the publisher.
	hub.Publish(context.Background(), Event{UserID: user, Frame: Frame{Data: "overflow"}})
	expectClosed(t, slow)
	assert.Equal(t, "overflow", receive(t, healthy).Data)
}

func TestHubConcurrentPublishDoesNotDoubleClose(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	user := uuid.Must(uuid.NewV7())
	frames, unsub := hub.Subscribe(context.Background(), user)
	defer unsub()

	// Fill the buffer so every concurrent publish hits the drop path. The
	// once-guard must keep exactly one closer.
	for range subscriberBuffer {
		hub.Publish(context.Background(), Event{UserID: user, Frame: Frame{Data: "fill"}})
	}
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			hub.Publish(context.Background(), Event{UserID: user, Frame: Frame{Data: "race"}})
		})
	}
	wg.Wait()
	expectClosed(t, frames)
}

func TestHubCloseClosesEveryConnection(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	a, unsubA := hub.Subscribe(context.Background(), uuid.Must(uuid.NewV7()))
	defer unsubA()
	b, unsubB := hub.Subscribe(context.Background(), uuid.Must(uuid.NewV7()))
	defer unsubB()

	hub.Close(context.Background())
	expectClosed(t, a)
	expectClosed(t, b)
	hub.Close(context.Background()) // Idempotent.

	// Publish after close is a no-op, not a panic.
	hub.Publish(context.Background(), Event{UserID: uuid.Must(uuid.NewV7()), Frame: Frame{Data: "x"}})
}

func TestHubSubscribeAfterCloseReturnsClosedChannel(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	hub.Close(context.Background())

	frames, unsub := hub.Subscribe(context.Background(), uuid.Must(uuid.NewV7()))
	defer unsub()
	expectClosed(t, frames)
}

func TestHubOpenCount(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	user := uuid.Must(uuid.NewV7())
	_, unsubA := hub.Subscribe(context.Background(), user)
	_, unsubB := hub.Subscribe(context.Background(), user)
	unsubA()

	hub.mu.RLock()
	open := 0
	for _, conns := range hub.byUser {
		open += len(conns)
	}
	hub.mu.RUnlock()
	require.Equal(t, 1, open)
	unsubB()

	hub.mu.RLock()
	open = len(hub.byUser)
	hub.mu.RUnlock()
	assert.Equal(t, 0, open, "an empty user's entry is removed from the registry")
}
