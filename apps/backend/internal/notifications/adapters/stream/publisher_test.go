package stream

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/sse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixedClock pins the envelope's occurredAt.
type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 9, 18, 9, 15, 3, 0, time.UTC) }

// subscribedHub returns a hub with one open connection for user.
func subscribedHub(t *testing.T, user uuid.UUID) (hub *sse.Hub, frames <-chan sse.Frame) {
	t.Helper()
	hub = sse.NewHub(nil)
	frames, unsub := hub.Subscribe(context.Background(), user)
	t.Cleanup(unsub)
	return hub, frames
}

// receiveFrame reads one frame from the connection.
func receiveFrame(t *testing.T, ch <-chan sse.Frame) sse.Frame {
	t.Helper()
	select {
	case f := <-ch:
		return f
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a frame")
		return sse.Frame{}
	}
}

func TestNotificationCreatedFrameCarriesEnvelope(t *testing.T) {
	t.Parallel()

	user := uuid.Must(uuid.NewV7())
	hub, frames := subscribedHub(t, user)
	publisher := NewPublisher(hub, fixedClock{})

	publisher.NotificationCreated(context.Background(), domain.Notification{
		ID:        uuid.Must(uuid.NewV7()),
		UserID:    user,
		Category:  domain.CategoryTariff,
		EventType: domain.EventSubscriptionGraceEntered,
		Title:     "Не удалось списание за подписку",
		Body:      "Привяжите другую карту.",
	})

	f := receiveFrame(t, frames)
	assert.Equal(t, EventNotificationCreated, f.Event)
	assert.NotEmpty(t, f.ID, "the hub stamps the frame id")

	var env struct {
		V          int             `json:"v"`
		OccurredAt string          `json:"occurredAt"`
		Payload    json.RawMessage `json:"payload"`
	}
	require.NoError(t, json.Unmarshal([]byte(f.Data), &env))
	assert.Equal(t, 1, env.V, "envelope schema version")
	assert.Equal(t, "2026-09-18T09:15:03Z", env.OccurredAt)

	var payload struct {
		ID       string `json:"id"`
		Category string `json:"category"`
		Title    string `json:"title"`
		Body     string `json:"body"`
		URL      string `json:"url"`
	}
	require.NoError(t, json.Unmarshal(env.Payload, &payload))
	assert.Equal(t, "/profile/tariff/payment-methods", payload.URL, "the grace events deep-link to payment methods")
	assert.Equal(t, string(domain.CategoryTariff), payload.Category)
	assert.Equal(t, "Не удалось списание за подписку", payload.Title)
	assert.Equal(t, "Привяжите другую карту.", payload.Body)
	_, err := uuid.Parse(payload.ID)
	assert.NoError(t, err, "payload carries the row id")
}

func TestNotificationCreatedWithoutDeepLinkOmitsURL(t *testing.T) {
	t.Parallel()

	user := uuid.Must(uuid.NewV7())
	hub, frames := subscribedHub(t, user)
	publisher := NewPublisher(hub, fixedClock{})

	publisher.NotificationCreated(context.Background(), domain.Notification{
		ID:        uuid.Must(uuid.NewV7()),
		UserID:    user,
		Category:  domain.CategoryPaymentsOperations,
		EventType: domain.EventPaymentDue,
		Title:     "Оплатите платёж",
		Body:      "Срок оплаты: 1 октября",
	})

	f := receiveFrame(t, frames)
	assert.NotContains(t, f.Data, `"url"`, "an event without a deep link delivers without a url")
}

func TestUnreadCountFrame(t *testing.T) {
	t.Parallel()

	user := uuid.Must(uuid.NewV7())
	hub, frames := subscribedHub(t, user)
	publisher := NewPublisher(hub, fixedClock{})

	publisher.UnreadCount(context.Background(), user, 7)

	f := receiveFrame(t, frames)
	assert.Equal(t, EventUnreadCount, f.Event)
	var env struct {
		Payload json.RawMessage `json:"payload"`
	}
	require.NoError(t, json.Unmarshal([]byte(f.Data), &env))
	var payload struct {
		Count int64 `json:"count"`
	}
	require.NoError(t, json.Unmarshal(env.Payload, &payload))
	assert.Equal(t, int64(7), payload.Count)
}

func TestFramesWithoutSubscribersAreNoop(t *testing.T) {
	t.Parallel()

	// No open connection for the user: the hub drops the frame silently.
	hub := sse.NewHub(nil)
	publisher := NewPublisher(hub, fixedClock{})

	assert.NotPanics(t, func() {
		publisher.NotificationCreated(context.Background(), domain.Notification{
			ID: uuid.Must(uuid.NewV7()), UserID: uuid.Must(uuid.NewV7()),
			Category: domain.CategorySystem, EventType: domain.EventSystemMaintenance,
			Title: "t", Body: "b",
		})
		publisher.UnreadCount(context.Background(), uuid.Must(uuid.NewV7()), 1)
	})
}
