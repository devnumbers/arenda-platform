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

// The payment-due frame's copy the created-frame tests share.
const (
	paymentDueFrameTitle = "Оплатите платёж"
	paymentDueFrameBody  = "Срок оплаты: 1 октября"
)

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
		Title:     paymentDueFrameTitle,
		Body:      paymentDueFrameBody,
	})

	f := receiveFrame(t, frames)
	assert.NotContains(t, f.Data, `"url"`, "an event without a deep link delivers without a url")
}

func TestNotificationCreatedFrameCarriesContextLabel(t *testing.T) {
	t.Parallel()

	// Тосту нужна строка над заголовком — снимок контекста строки ленты
	// (#747, макет тоста 2343:57307). Поле аддитивное к конверту v1.
	user := uuid.Must(uuid.NewV7())
	hub, frames := subscribedHub(t, user)
	publisher := NewPublisher(hub, fixedClock{})

	publisher.NotificationCreated(context.Background(), domain.Notification{
		ID:           uuid.Must(uuid.NewV7()),
		UserID:       user,
		Category:     domain.CategoryRental,
		EventType:    domain.EventRentalCompleted,
		ContextLabel: "2-комнатная на Ленина",
		Title:        "Аренда завершена",
		Body:         "Договор аренды завершён",
	})

	f := receiveFrame(t, frames)
	var env struct {
		Payload json.RawMessage `json:"payload"`
	}
	require.NoError(t, json.Unmarshal([]byte(f.Data), &env))
	var payload struct {
		ContextLabel string `json:"contextLabel"`
	}
	require.NoError(t, json.Unmarshal(env.Payload, &payload))
	assert.Equal(t, "2-комнатная на Ленина", payload.ContextLabel)
}

func TestNotificationCreatedFrameOmitsEmptyContextLabel(t *testing.T) {
	t.Parallel()

	user := uuid.Must(uuid.NewV7())
	hub, frames := subscribedHub(t, user)
	publisher := NewPublisher(hub, fixedClock{})

	publisher.NotificationCreated(context.Background(), domain.Notification{
		ID:        uuid.Must(uuid.NewV7()),
		UserID:    user,
		Category:  domain.CategorySystem,
		EventType: domain.EventSystemMaintenance,
		Title:     "t",
		Body:      "b",
	})

	f := receiveFrame(t, frames)
	assert.NotContains(t, f.Data, `"contextLabel"`, "an empty context label is omitted, not an empty string")
}

// TestNotificationCreatedFrameCarriesPaymentCategory pins the toast's
// circle-icon snapshot (#1348, решение владельца 13.10 — тост тоже с
// иконкой категории): the payment events' category ref rides in the frame
// additively to envelope v1 — old clients ignore the extra field.
func TestNotificationCreatedFrameCarriesPaymentCategory(t *testing.T) {
	t.Parallel()

	user := uuid.Must(uuid.NewV7())
	hub, frames := subscribedHub(t, user)
	publisher := NewPublisher(hub, fixedClock{})

	categoryID := uuid.Must(uuid.NewV7())
	publisher.NotificationCreated(context.Background(), domain.Notification{
		ID:        uuid.Must(uuid.NewV7()),
		UserID:    user,
		Category:  domain.CategoryPaymentsOperations,
		EventType: domain.EventPaymentDue,
		Title:     paymentDueFrameTitle,
		Body:      paymentDueFrameBody,
		Payload: domain.Payload{
			PaymentCategory: &domain.PaymentCategoryRef{UserCategoryID: &categoryID},
		},
	})

	f := receiveFrame(t, frames)
	var env struct {
		Payload json.RawMessage `json:"payload"`
	}
	require.NoError(t, json.Unmarshal([]byte(f.Data), &env))
	var payload struct {
		PaymentCategory *struct {
			Slug           string `json:"slug"`
			UserCategoryID string `json:"user_category_id"`
		} `json:"paymentCategory"`
	}
	require.NoError(t, json.Unmarshal(env.Payload, &payload))
	require.NotNil(t, payload.PaymentCategory, "the category ref travels for the toast's circle icon")
	assert.Equal(t, categoryID.String(), payload.PaymentCategory.UserCategoryID)
	assert.Empty(t, payload.PaymentCategory.Slug, "the ref's XOR shape matches the REST payload's")
}

func TestNotificationCreatedFrameOmitsEmptyPaymentCategory(t *testing.T) {
	t.Parallel()

	user := uuid.Must(uuid.NewV7())
	hub, frames := subscribedHub(t, user)
	publisher := NewPublisher(hub, fixedClock{})

	publisher.NotificationCreated(context.Background(), domain.Notification{
		ID:        uuid.Must(uuid.NewV7()),
		UserID:    user,
		Category:  domain.CategoryPaymentsOperations,
		EventType: domain.EventPaymentDue,
		Title:     paymentDueFrameTitle,
		Body:      paymentDueFrameBody,
	})

	f := receiveFrame(t, frames)
	assert.NotContains(t, f.Data, "paymentCategory", "a notification without the snapshot omits the field, not an empty object")
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
