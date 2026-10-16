package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tariffViews is the TariffEventViewSource stub: id-keyed tariff slugs the
// tests plant; a missing id errors like a real store would.
type tariffViews struct {
	tariffs map[uuid.UUID]string
}

func (v *tariffViews) TariffView(_ context.Context, tariffID uuid.UUID) (TariffView, error) {
	slug, ok := v.tariffs[tariffID]
	if !ok {
		return TariffView{}, ErrNotFound
	}
	return TariffView{Name: slug}, nil
}

// tariffPublisherHarness builds the tariff publisher over the real pipeline
// backed by the publisher tests' fakes, so the tests read the feed rows the
// events created.
type tariffPublisherHarness struct {
	feed  *fakeFeedRepo
	queue *fakeQueue
	pub   *TariffPublisher
	views *tariffViews
}

func newTariffPublisherHarness() *tariffPublisherHarness {
	feed := &fakeFeedRepo{}
	queue := &fakeQueue{}
	views := &tariffViews{tariffs: map[uuid.UUID]string{}}
	pipeline := NewPublisher(feed, queue, nil, &fakeUoW{}, nil)
	return &tariffPublisherHarness{
		feed:  feed,
		queue: queue,
		pub:   NewTariffPublisher(pipeline, views),
		views: views,
	}
}

// The shared id fixture of the tariff events' tests: the owner (the Тариф
// recipient slot), the paid tariff and the payment and transition entities.
var (
	testTariffUserID     = uuid.MustParse("00000000-0000-7000-8000-00000000b001")
	testTariffTariffID   = uuid.MustParse("00000000-0000-7000-8000-00000000b002")
	testTariffPaymentID  = uuid.MustParse("00000000-0000-7000-8000-00000000b003")
	testTariffTransition = uuid.MustParse("00000000-0000-7000-8000-00000000b004")
)

// plantTariff seeds the view source with a tariff slug.
func (h *tariffPublisherHarness) plantTariff(id uuid.UUID, slug string) {
	h.views.tariffs[id] = slug
}

func TestTariffPublisherNotifyPaymentSucceeded(t *testing.T) {
	t.Parallel()

	h := newTariffPublisherHarness()
	h.plantTariff(testTariffTariffID, "pro")
	until := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)

	require.NoError(t, h.pub.NotifyPaymentSucceeded(
		context.Background(), testTariffUserID, testTariffPaymentID, testTariffTariffID,
		499_00, "month", until,
	))

	require.Len(t, h.feed.inserted, 1)
	n := h.feed.inserted[0]
	assert.Equal(t, testTariffUserID, n.UserID, "the owner is the single recipient")
	assert.Equal(t, domain.EventSubscriptionPaymentSucceeded, n.EventType)
	assert.Equal(t, domain.CategoryTariff, n.Category, "category derived from the catalog")
	assert.Equal(t, "Оплата прошла", n.Title)
	// Решение #737 дословно: «Оплата тарифа «{тариф}» на {сумма} прошла
	// успешно. Подписка активна до {дата}»; сумма — формат #749.
	assert.Equal(t, "Оплата тарифа «Про» на 499 ₽ прошла успешно. Подписка активна до 18 октября", n.Body)
	assert.Equal(t, "Про", n.ContextLabel, "the tariff name labels the row (решение #737)")
	assert.Equal(t,
		domain.DedupKey("subscription_payment_succeeded:"+testTariffPaymentID.String()),
		n.DedupKey, "the payment is the dedup entity (решение #737 №13)")
	require.NotNil(t, n.Payload.Tariff, "the tariff snapshot rides the payload")
	assert.Equal(t, "pro", n.Payload.Tariff.Slug)
	assert.Equal(t, "month", n.Payload.Tariff.Period)
	assert.Equal(t, int64(499_00), n.Payload.Tariff.AmountKopecks)
	require.NotNil(t, n.Payload.Tariff.ActiveUntil)
	assert.True(t, until.Equal(*n.Payload.Tariff.ActiveUntil), "ActiveUntil is the applied payment's validity")

	// The feed row carries both channel deliveries, the pipeline canon.
	require.Len(t, h.queue.emails, 1)
	require.Len(t, h.queue.pushes, 1)
}

func TestTariffPublisherNotifyPaymentSucceededKopeckTail(t *testing.T) {
	t.Parallel()

	h := newTariffPublisherHarness()
	h.plantTariff(testTariffTariffID, "business")

	require.NoError(t, h.pub.NotifyPaymentSucceeded(
		context.Background(), testTariffUserID, testTariffPaymentID, testTariffTariffID,
		250_050, "year", time.Date(2027, 9, 20, 0, 0, 0, 0, time.UTC),
	))

	assert.Equal(t,
		"Оплата тарифа «Бизнес» на 2\u00a0500,50 ₽ прошла успешно. Подписка активна до 20 сентября",
		h.feed.inserted[0].Body, "the kopeck tail renders with the NBSP grouping")
}

func TestTariffPublisherNotifyPaymentSucceededUnknownSlugVerbatim(t *testing.T) {
	t.Parallel()

	h := newTariffPublisherHarness()
	h.plantTariff(testTariffTariffID, "custom")

	require.NoError(t, h.pub.NotifyPaymentSucceeded(
		context.Background(), testTariffUserID, testTariffPaymentID, testTariffTariffID,
		100_00, "month", time.Time{},
	))

	assert.Equal(t, "Оплата тарифа «custom» на 100 ₽ прошла успешно.", h.feed.inserted[0].Body,
		"an unknown plan renders its slug; the zero instant (no known validity) drops the tail — "+
			"the grace deadline rendering's convention")
	assert.Nil(t, h.feed.inserted[0].Payload.Tariff.ActiveUntil)
}

func TestTariffPublisherNotifyPlanUpgraded(t *testing.T) {
	t.Parallel()

	h := newTariffPublisherHarness()
	h.plantTariff(testTariffTariffID, "business")
	until := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)

	require.NoError(t, h.pub.NotifyPlanUpgraded(
		context.Background(), testTariffUserID, testTariffTransition, testTariffTariffID,
		"month", 99_000, until,
	))

	require.Len(t, h.feed.inserted, 1)
	n := h.feed.inserted[0]
	assert.Equal(t, testTariffUserID, n.UserID, "the owner is the single recipient")
	assert.Equal(t, domain.EventSubscriptionPlanChanged, n.EventType)
	assert.Equal(t, domain.CategoryTariff, n.Category, "category derived from the catalog")
	assert.Equal(t, "Тариф изменён", n.Title)
	// Решение #737 дословно: апгрейд — «Тариф „{тариф}“ активирован».
	assert.Equal(t, "Тариф „Бизнес“ активирован", n.Body)
	assert.Equal(t, "Бизнес", n.ContextLabel)
	assert.Equal(t,
		domain.DedupKey("subscription_plan_changed:"+testTariffTransition.String()),
		n.DedupKey, "the transition is the dedup entity (решение #737 №14)")
	require.NotNil(t, n.Payload.Tariff)
	assert.Equal(t, "business", n.Payload.Tariff.Slug)
	assert.Equal(t, "month", n.Payload.Tariff.Period)
	// The tariff payload's amount line (решение #737: слаг, период, сумма,
	// дата до) — for the upgrade leg the applied payment's charge.
	assert.Equal(t, int64(99_000), n.Payload.Tariff.AmountKopecks)
	require.NotNil(t, n.Payload.Tariff.ActiveUntil)
	assert.True(t, until.Equal(*n.Payload.Tariff.ActiveUntil))
}

func TestTariffPublisherNotifyPlanDowngradeScheduled(t *testing.T) {
	t.Parallel()

	h := newTariffPublisherHarness()
	h.plantTariff(testTariffTariffID, "basic")
	effective := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	require.NoError(t, h.pub.NotifyPlanDowngradeScheduled(
		context.Background(), testTariffUserID, testTariffTransition, testTariffTariffID,
		"month", effective,
	))

	require.Len(t, h.feed.inserted, 1)
	n := h.feed.inserted[0]
	assert.Equal(t, domain.EventSubscriptionPlanChanged, n.EventType)
	assert.Equal(t, "Тариф изменён", n.Title)
	// Решение #737 дословно: даунгрейд — «С {дата} тариф сменится на „{тариф}“».
	assert.Equal(t, "С 1 октября тариф сменится на „Базовый“", n.Body)
	assert.Equal(t, "Базовый", n.ContextLabel)
	assert.Equal(t,
		domain.DedupKey("subscription_plan_changed:"+testTariffTransition.String()),
		n.DedupKey)
	require.NotNil(t, n.Payload.Tariff)
	assert.Nil(t, n.Payload.Tariff.ActiveUntil,
		"the scheduled target is not active yet — no validity to snapshot")
}

// A missing tariff view cannot be fixed by a retry either — the snapshot is
// the row's content; the failure surfaces to the post-commit wrapper that
// logs and swallows it (the grace-events canon).
func TestTariffPublisherViewResolveFailureFailsPublication(t *testing.T) {
	t.Parallel()

	h := newTariffPublisherHarness() // No tariff planted.
	ctx := context.Background()

	require.ErrorIs(t, h.pub.NotifyPaymentSucceeded(
		ctx, testTariffUserID, testTariffPaymentID, testTariffTariffID, 100_00, "month", time.Time{},
	), ErrNotFound)
	require.ErrorIs(t, h.pub.NotifyPlanUpgraded(
		ctx, testTariffUserID, testTariffTransition, testTariffTariffID, "month", 100_00, time.Time{},
	), ErrNotFound)
	require.ErrorIs(t, h.pub.NotifyPlanDowngradeScheduled(
		ctx, testTariffUserID, testTariffTransition, testTariffTariffID, "month", time.Time{},
	), ErrNotFound)
	assert.Empty(t, h.feed.inserted, "no rows without the snapshot")
}
