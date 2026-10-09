package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The scan fixtures' property lines — the catalog copy interpolates the
// name into the body and the payload carries both as the card snapshot.
const (
	scanPropertyName    = "Квартира на Ленина"
	scanPropertyAddress = "г. Москва, ул. Ленина, 1"
	// The card snapshot's type — the payload glyph's key (#1244).
	scanPropertyType = "apartment"
	// The card snapshot's photo — the same-origin streaming path (#1275).
	scanPropertyPhoto = "/api/v1/properties/0194a3f8-0000-7000-8000-0000000000ff/photo"
	// The fixture rule title and the shared due/reminder body the due-leg
	// and reminder-leg tests assert.
	scanRentTitle        = "Аренда квартиры"
	scanRentReminderBody = "Платёж «Аренда квартиры» по объекту «Квартира на Ленина»: 2\u00a0500 ₽. Срок оплаты: 20 сентября"
)

// fakePaymentSource is the payments scan source stub: per zone it answers the
// due, reminder and overdue targets the caller asks for and records every
// (leg, zone, today) question, so the tests assert each zone was swept with
// its own calendar date on all three legs. The booking windows and the
// boundary jobs' reloads answer through the scheduled/live fields (#776).
type fakePaymentSource struct {
	due     map[string][]PaymentScanTarget
	overdue map[string][]PaymentScanTarget
	// Reminder holds the reminder leg's sweep targets per zone (карта #822).
	reminder map[string][]PaymentScanTarget
	// AutoPaid holds the auto-paid leg's sweep targets per zone (#1169).
	autoPaid map[string][]PaymentScanTarget
	asked    []string
	recips   map[uuid.UUID][]uuid.UUID
	recErr   map[uuid.UUID]error
	// SchedDue, schedOverdue and schedReminder answer the boundary-booking
	// windows;
	// schedAsked records the "leg|from|until" questions in order. Live
	// answers the boundary jobs' reloads keyed "leg|rule|date".
	schedDue      []PaymentScheduleTarget
	schedOverdue  []PaymentScheduleTarget
	schedReminder []PaymentScheduleTarget
	schedAutoPaid []PaymentScheduleTarget
	schedAsked    []string
	live          map[string]PaymentScanTarget
}

func (f *fakePaymentSource) ListDueTargets(ctx context.Context, zone string, today, now time.Time) ([]PaymentScanTarget, error) {
	f.asked = append(f.asked, "due|"+zone+"|"+today.Format("2006-01-02"))
	return f.due[zone], nil
}

func (f *fakePaymentSource) ListOverdueTargets(ctx context.Context, zone string, today, now time.Time) ([]PaymentScanTarget, error) {
	f.asked = append(f.asked, "overdue|"+zone+"|"+today.Format("2006-01-02"))
	return f.overdue[zone], nil
}

func (f *fakePaymentSource) ListReminderTargets(ctx context.Context, zone string, today, now time.Time) ([]PaymentScanTarget, error) {
	f.asked = append(f.asked, "reminder|"+zone+"|"+today.Format("2006-01-02"))
	return f.reminder[zone], nil
}

func (f *fakePaymentSource) ListAutoPaidTargets(ctx context.Context, zone string, today, now time.Time) ([]PaymentScanTarget, error) {
	f.asked = append(f.asked, "auto paid|"+zone+"|"+today.Format("2006-01-02"))
	return f.autoPaid[zone], nil
}

func (f *fakePaymentSource) ListScheduledAutoPaidTargets(ctx context.Context, from, until time.Time) ([]PaymentScheduleTarget, error) {
	f.schedAsked = append(f.schedAsked, "auto paid|"+from.Format(time.RFC3339)+"|"+until.Format(time.RFC3339))
	return f.schedAutoPaid, nil
}

func (f *fakePaymentSource) GetScheduledAutoPaidPayment(
	ctx context.Context, paymentID uuid.UUID, date, now time.Time,
) (PaymentScanTarget, bool, error) {
	target, ok := f.live["auto paid|"+paymentID.String()+"|"+date.Format("2006-01-02")]
	return target, ok, nil
}

func (f *fakePaymentSource) ListScheduledDueTargets(ctx context.Context, from, until time.Time) ([]PaymentScheduleTarget, error) {
	f.schedAsked = append(f.schedAsked, "due|"+from.Format(time.RFC3339)+"|"+until.Format(time.RFC3339))
	return f.schedDue, nil
}

func (f *fakePaymentSource) ListScheduledOverdueTargets(ctx context.Context, from, until time.Time) ([]PaymentScheduleTarget, error) {
	f.schedAsked = append(f.schedAsked, "overdue|"+from.Format(time.RFC3339)+"|"+until.Format(time.RFC3339))
	return f.schedOverdue, nil
}

func (f *fakePaymentSource) ListScheduledReminderTargets(ctx context.Context, from, until time.Time) ([]PaymentScheduleTarget, error) {
	f.schedAsked = append(f.schedAsked, "reminder|"+from.Format(time.RFC3339)+"|"+until.Format(time.RFC3339))
	return f.schedReminder, nil
}

func (f *fakePaymentSource) GetScheduledDuePayment(
	ctx context.Context, paymentID uuid.UUID, date, now time.Time,
) (PaymentScanTarget, bool, error) {
	target, ok := f.live["due|"+paymentID.String()+"|"+date.Format("2006-01-02")]
	return target, ok, nil
}

func (f *fakePaymentSource) GetScheduledOverduePayment(
	ctx context.Context, paymentID uuid.UUID, date, now time.Time,
) (PaymentScanTarget, bool, error) {
	target, ok := f.live["overdue|"+paymentID.String()+"|"+date.Format("2006-01-02")]
	return target, ok, nil
}

func (f *fakePaymentSource) GetScheduledReminderPayment(
	ctx context.Context, paymentID uuid.UUID, date, now time.Time,
) (PaymentScanTarget, bool, error) {
	target, ok := f.live["reminder|"+paymentID.String()+"|"+date.Format("2006-01-02")]
	return target, ok, nil
}

// fakePaymentScheduler is the boundary-jobs stub: it books every (leg, rule,
// date, fire instant) and can fail a rule's booking on demand.
type fakePaymentScheduler struct {
	booked map[string]time.Time
	errFor map[uuid.UUID]error
	order  []string
}

func paymentBookingKey(leg string, paymentID uuid.UUID, date time.Time) string {
	return leg + "|" + paymentID.String() + "|" + date.Format("2006-01-02")
}

func (f *fakePaymentScheduler) SchedulePaymentDue(ctx context.Context, paymentID uuid.UUID, date, fireAt time.Time) error {
	return f.book("due", paymentID, date, fireAt)
}

func (f *fakePaymentScheduler) SchedulePaymentOverdue(ctx context.Context, paymentID uuid.UUID, date, fireAt time.Time) error {
	return f.book("overdue", paymentID, date, fireAt)
}

func (f *fakePaymentScheduler) SchedulePaymentReminder(ctx context.Context, paymentID uuid.UUID, date, fireAt time.Time) error {
	return f.book("reminder", paymentID, date, fireAt)
}

func (f *fakePaymentScheduler) SchedulePaymentAutoPaid(ctx context.Context, paymentID uuid.UUID, date, fireAt time.Time) error {
	return f.book("auto paid", paymentID, date, fireAt)
}

func (f *fakePaymentScheduler) book(leg string, paymentID uuid.UUID, date, fireAt time.Time) error {
	if err := f.errFor[paymentID]; err != nil {
		return err
	}
	if f.booked == nil {
		f.booked = map[string]time.Time{}
	}
	f.booked[paymentBookingKey(leg, paymentID, date)] = fireAt
	f.order = append(f.order, paymentBookingKey(leg, paymentID, date))
	return nil
}

func (f *fakePaymentSource) ListActiveRecipients(ctx context.Context, propertyID uuid.UUID) ([]uuid.UUID, error) {
	if err := f.recErr[propertyID]; err != nil {
		return nil, err
	}
	return f.recips[propertyID], nil
}

// paymentScanHarness builds the payments publisher over the real pipeline
// backed by the publisher tests' fakes: the tests read the feed rows the
// scan created.
type paymentScanHarness struct {
	pipeline  *Publisher
	feed      *fakeFeedRepo
	queue     *fakeQueue
	zones     *fakeScanZones
	source    *fakePaymentSource
	scheduler *fakePaymentScheduler
}

func newPaymentScanHarness(zones []ScanZone) *paymentScanHarness {
	feed := &fakeFeedRepo{}
	queue := &fakeQueue{}
	pipeline := NewPublisher(feed, queue, nil, &fakeUoW{}, nil)
	return &paymentScanHarness{
		pipeline: pipeline,
		feed:     feed,
		queue:    queue,
		zones:    &fakeScanZones{zones: zones},
		source: &fakePaymentSource{
			due:      map[string][]PaymentScanTarget{},
			overdue:  map[string][]PaymentScanTarget{},
			reminder: map[string][]PaymentScanTarget{},
			autoPaid: map[string][]PaymentScanTarget{},
			live:     map[string]PaymentScanTarget{},
			recips:   map[uuid.UUID][]uuid.UUID{},
			recErr:   map[uuid.UUID]error{},
		},
		scheduler: &fakePaymentScheduler{
			booked: map[string]time.Time{},
			errFor: map[uuid.UUID]error{},
		},
	}
}

// dueTarget builds a scan target on a fresh property: the rule id is the
// payload link, the date is both the dedup key's half and the text's
// «Срок оплаты».
func dueTarget(paymentID uuid.UUID, date time.Time, title string) PaymentScanTarget {
	return PaymentScanTarget{
		PaymentID:       paymentID,
		DueDate:         date,
		Title:           title,
		AmountKopecks:   250000,
		PropertyID:      uuid.Must(uuid.NewV7()),
		PropertyName:    scanPropertyName,
		PropertyAddress: scanPropertyAddress,
		PropertyType:    scanPropertyType,
		PropertyPhoto:   scanPropertyPhoto,
		OwnerID:         uuid.Must(uuid.NewV7()),
	}
}

// asScheduleTarget converts a scan target into the booking-window shape —
// the fixture's fire instant is the test's business.
func asScheduleTarget(target PaymentScanTarget) PaymentScheduleTarget {
	return PaymentScheduleTarget{PaymentID: target.PaymentID, DueDate: target.DueDate}
}

func TestPaymentsPublisher_PublishesDuePayment(t *testing.T) {
	t.Parallel()

	h := newPaymentScanHarness([]ScanZone{{Timezone: zoneMSK}})
	rule := uuid.Must(uuid.NewV7())
	target := PaymentScanTarget{
		PaymentID:       rule,
		DueDate:         time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC),
		Title:           scanRentTitle,
		AmountKopecks:   250000,
		PropertyID:      uuid.Must(uuid.NewV7()),
		PropertyName:    scanPropertyName,
		PropertyAddress: scanPropertyAddress,
		PropertyType:    scanPropertyType,
		PropertyPhoto:   scanPropertyPhoto,
		OwnerID:         uuid.Must(uuid.NewV7()),
	}
	h.source.due[zoneMSK] = []PaymentScanTarget{target}
	member1, member2 := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	h.source.recips[target.PropertyID] = []uuid.UUID{member1, member2}

	p := NewPaymentsPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	err := p.RunZoneScans(context.Background(), scanNow)
	require.NoError(t, err)

	// Owner plus two active members — one row each, all three delivered to
	// both channels.
	require.Len(t, h.feed.inserted, 3)
	recipients := []uuid.UUID{h.feed.inserted[0].UserID, h.feed.inserted[1].UserID, h.feed.inserted[2].UserID}
	assert.ElementsMatch(t, []uuid.UUID{target.OwnerID, member1, member2}, recipients)

	for _, n := range h.feed.inserted {
		assert.Equal(t, domain.EventPaymentDue, n.EventType)
		assert.Equal(t, domain.CategoryPaymentsOperations, n.Category)
		assert.Equal(t, "Оплатите платёж", n.Title)
		assert.Equal(t,
			scanRentReminderBody,
			n.Body)
		assert.Equal(t, "Квартира на Ленина", n.ContextLabel)
		// The dedup key pins the rule and the operation date it fired for.
		assert.Equal(t, domain.DedupKey("payment_due:"+rule.String()+":2026-09-20"), n.DedupKey)
		// The payload carries the property snapshot with its address line
		// (решение владельца 19.09.2026, #745), the rule id — the link target
		// is the payment's page — and the operation date the «Оплатить»
		// button's live check pins to (решение #737: payment = правило +
		// дата операции).
		require.NotNil(t, n.Payload.Property)
		assert.Equal(t, target.PropertyID, n.Payload.Property.ID)
		assert.Equal(t, "Квартира на Ленина", n.Payload.Property.Name)
		assert.Equal(t, "г. Москва, ул. Ленина, 1", n.Payload.Property.Address)
		assert.Equal(t, scanPropertyType, n.Payload.Property.Type, "the card's glyph type snapshot (#1244)")
		assert.Equal(t, scanPropertyPhoto, n.Payload.Property.Photo, "the card's photo snapshot (#1275)")
		require.NotNil(t, n.Payload.PaymentID)
		assert.Equal(t, rule, *n.Payload.PaymentID)
		require.NotNil(t, n.Payload.PaymentDate)
		assert.Equal(t, time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), *n.Payload.PaymentDate)
		assert.Nil(t, n.Payload.Actor, "a system scan has no initiator")
	}
	assert.Len(t, h.queue.emails, 3)
	assert.Len(t, h.queue.pushes, 3)
}

func TestPaymentsPublisher_PublishesOverduePayment(t *testing.T) {
	t.Parallel()

	h := newPaymentScanHarness([]ScanZone{{Timezone: zoneMSK}})
	rule := uuid.Must(uuid.NewV7())
	target := PaymentScanTarget{
		PaymentID:       rule,
		DueDate:         time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC),
		Title:           "Интернет",
		AmountKopecks:   100050,
		PropertyID:      uuid.Must(uuid.NewV7()),
		PropertyName:    scanPropertyName,
		PropertyAddress: scanPropertyAddress,
		PropertyType:    scanPropertyType,
		PropertyPhoto:   scanPropertyPhoto,
		OwnerID:         uuid.Must(uuid.NewV7()),
	}
	h.source.overdue[zoneMSK] = []PaymentScanTarget{target}

	p := NewPaymentsPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.RunZoneScans(context.Background(), scanNow))
	require.Len(t, h.feed.inserted, 1)

	n := h.feed.inserted[0]
	assert.Equal(t, domain.EventPaymentOverdue, n.EventType)
	assert.Equal(t, "Платёж просрочен", n.Title)
	assert.Equal(t,
		"Платёж «Интернет» по объекту «Квартира на Ленина» просрочен: 1\u00a0000,50 ₽. Срок оплаты был 18 сентября",
		n.Body)
	assert.Equal(t, domain.DedupKey("payment_overdue:"+rule.String()+":2026-09-18"), n.DedupKey)
}

func TestPaymentsPublisher_DueAndOverdueInOneSweep(t *testing.T) {
	t.Parallel()

	h := newPaymentScanHarness([]ScanZone{{Timezone: zoneMSK}})
	due := dueTarget(uuid.Must(uuid.NewV7()), time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), "Аренда")
	overdue := dueTarget(uuid.Must(uuid.NewV7()), time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC), "Обслуживание")
	h.source.due[zoneMSK] = []PaymentScanTarget{due}
	h.source.overdue[zoneMSK] = []PaymentScanTarget{overdue}

	p := NewPaymentsPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.RunZoneScans(context.Background(), scanNow))
	require.Len(t, h.feed.inserted, 2, "one row per target — no members configured, the owner alone")
}

func TestPaymentsPublisher_RepeatScanIsNoOp(t *testing.T) {
	t.Parallel()

	h := newPaymentScanHarness([]ScanZone{{Timezone: zoneMSK}})
	target := dueTarget(uuid.Must(uuid.NewV7()), time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), "Аренда")
	h.source.due[zoneMSK] = []PaymentScanTarget{target}

	p := NewPaymentsPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.RunZoneScans(context.Background(), scanNow))
	require.NoError(t, p.RunZoneScans(context.Background(), scanNow))
	assert.Len(t, h.feed.inserted, 1, "the second hourly pass dedups to nothing")
}

func TestPaymentsPublisher_OverdueStaysSingleAcrossDays(t *testing.T) {
	t.Parallel()

	h := newPaymentScanHarness([]ScanZone{{Timezone: zoneMSK}})
	target := dueTarget(uuid.Must(uuid.NewV7()), time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC), "Обслуживание")
	h.source.overdue[zoneMSK] = []PaymentScanTarget{target}

	p := NewPaymentsPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.RunZoneScans(context.Background(), scanNow))
	// The operation stayed unpaid for days: the sweep's overdue leg keeps
	// listing it, the key (rule, operation date) keeps the row single —
	// «однократно» (решение #737, тип №3).
	require.NoError(t, p.RunZoneScans(context.Background(), scanNow))
	assert.Len(t, h.feed.inserted, 1)
}

func TestPaymentsPublisher_EachZoneSweptWithItsOwnToday(t *testing.T) {
	t.Parallel()

	h := newPaymentScanHarness([]ScanZone{{Timezone: zoneMSK}, {Timezone: "Europe/Kaliningrad"}})
	p := NewPaymentsPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.RunZoneScans(context.Background(), scanNow))

	// All four legs ask per zone; Moscow's local calendar has already rolled
	// to the 20th, Kaliningrad's has not — the sweep must not share one today
	// across zones.
	assert.ElementsMatch(t, []string{
		"due|Europe/Moscow|2026-09-20",
		"reminder|Europe/Moscow|2026-09-20",
		"auto paid|Europe/Moscow|2026-09-20",
		"overdue|Europe/Moscow|2026-09-20",
		"due|Europe/Kaliningrad|2026-09-19",
		"reminder|Europe/Kaliningrad|2026-09-19",
		"auto paid|Europe/Kaliningrad|2026-09-19",
		"overdue|Europe/Kaliningrad|2026-09-19",
	}, h.source.asked)
}

func TestPaymentsPublisher_FailuresAreIsolated(t *testing.T) {
	t.Parallel()

	h := newPaymentScanHarness([]ScanZone{{Timezone: zoneMSK}})
	due := dueTarget(uuid.Must(uuid.NewV7()), time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), "Аренда")
	broken := dueTarget(uuid.Must(uuid.NewV7()), time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC), "Дом на Гагарина")
	h.source.due[zoneMSK] = []PaymentScanTarget{due}
	h.source.overdue[zoneMSK] = []PaymentScanTarget{broken}
	h.source.recErr[broken.PropertyID] = errors.New("members lookup down")

	p := NewPaymentsPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	err := p.RunZoneScans(context.Background(), scanNow)
	require.Error(t, err)
	require.ErrorContains(t, err, "members lookup down")
	// The broken overdue target did not take the due leg down: the pass
	// still published the due row (owner only), cross-leg isolation.
	require.Len(t, h.feed.inserted, 1)
	assert.Equal(t, domain.EventPaymentDue, h.feed.inserted[0].EventType)
}

func TestPaymentsPublisher_BrokenZoneNameIsolated(t *testing.T) {
	t.Parallel()

	h := newPaymentScanHarness([]ScanZone{{Timezone: "Mars/Olympus"}, {Timezone: zoneMSK}})
	due := dueTarget(uuid.Must(uuid.NewV7()), time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), "Аренда")
	overdue := dueTarget(uuid.Must(uuid.NewV7()), time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC), "Обслуживание")
	h.source.due[zoneMSK] = []PaymentScanTarget{due}
	h.source.overdue[zoneMSK] = []PaymentScanTarget{overdue}

	p := NewPaymentsPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	err := p.RunZoneScans(context.Background(), scanNow)
	require.Error(t, err)
	require.ErrorContains(t, err, "Mars/Olympus")
	// Both legs of the healthy zone still swept — one row each, owner alone.
	require.Len(t, h.feed.inserted, 2)
	assert.Equal(t, domain.EventPaymentDue, h.feed.inserted[0].EventType)
	assert.Equal(t, domain.EventPaymentOverdue, h.feed.inserted[1].EventType)
}

func TestPaymentsPublisher_NilZoneDirectory(t *testing.T) {
	t.Parallel()

	h := newPaymentScanHarness(nil)
	p := NewPaymentsPublisher(h.pipeline, nil, h.source, h.scheduler)
	err := p.RunZoneScans(context.Background(), scanNow)
	require.Error(t, err)
	require.ErrorContains(t, err, "zone directory")
}

// The boundary booking (issue #776, решение владельца 21.09.2026: все
// уведомления — чётко по времени): the hourly pass books the upcoming
// boundary jobs before the zone sweep — the due leg's job wakes at the wall
// clock 10:00 of the operation date in the owner's zone (#1168), the
// overdue leg's — at 22:00 of the day after. All windows are asked for the
// same two-day horizon; the
// sweep itself goes on as the backstop.
func TestPaymentsPublisher_SchedulesUpcomingBoundaries(t *testing.T) {
	t.Parallel()

	h := newPaymentScanHarness([]ScanZone{{Timezone: zoneMSK}})
	dueRule := uuid.Must(uuid.NewV7())
	overdueRule := uuid.Must(uuid.NewV7())
	autoPaidRule := uuid.Must(uuid.NewV7())
	h.source.schedDue = []PaymentScheduleTarget{
		{
			PaymentID: dueRule, DueDate: time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC),
			FireAt: time.Date(2026, 9, 20, 7, 0, 0, 0, time.UTC),
		},
	}
	h.source.schedOverdue = []PaymentScheduleTarget{
		{
			PaymentID: overdueRule, DueDate: time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC),
			FireAt: time.Date(2026, 9, 20, 19, 0, 0, 0, time.UTC),
		},
	}
	h.source.schedAutoPaid = []PaymentScheduleTarget{
		{
			PaymentID: autoPaidRule, DueDate: time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC),
			FireAt: time.Date(2026, 9, 21, 7, 0, 0, 0, time.UTC),
		},
	}

	p := NewPaymentsPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.RunZoneScans(context.Background(), scanNow))

	// All four legs asked for the same window: (now, now+48h].
	assert.ElementsMatch(t, []string{
		"due|" + scanNow.Format(time.RFC3339) + "|" + scanNow.Add(scheduledHorizon).Format(time.RFC3339),
		"reminder|" + scanNow.Format(time.RFC3339) + "|" + scanNow.Add(scheduledHorizon).Format(time.RFC3339),
		"auto paid|" + scanNow.Format(time.RFC3339) + "|" + scanNow.Add(scheduledHorizon).Format(time.RFC3339),
		"overdue|" + scanNow.Format(time.RFC3339) + "|" + scanNow.Add(scheduledHorizon).Format(time.RFC3339),
	}, h.source.schedAsked)
	// Each leg books its own kind at the boundary instant it was given.
	assert.Equal(t, time.Date(2026, 9, 20, 7, 0, 0, 0, time.UTC),
		h.scheduler.booked[paymentBookingKey("due", dueRule, time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC))])
	assert.Equal(t, time.Date(2026, 9, 20, 19, 0, 0, 0, time.UTC),
		h.scheduler.booked[paymentBookingKey("overdue", overdueRule, time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC))])
	assert.Equal(t, time.Date(2026, 9, 21, 7, 0, 0, 0, time.UTC),
		h.scheduler.booked[paymentBookingKey("auto paid", autoPaidRule, time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC))])
	assert.Len(t, h.scheduler.booked, 3)
}

// A broken booking does not take the pass down: the other leg books on and
// the zone sweep still publishes (the isolation canon of the scan group).
func TestPaymentsPublisher_SchedulingFailuresAreIsolated(t *testing.T) {
	t.Parallel()

	h := newPaymentScanHarness([]ScanZone{{Timezone: zoneMSK}})
	broken := dueTarget(uuid.Must(uuid.NewV7()), time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), "Дом на Гагарина")
	healthy := dueTarget(uuid.Must(uuid.NewV7()), time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC), "Аренда")
	h.source.schedOverdue = []PaymentScheduleTarget{asScheduleTarget(broken)}
	h.source.schedDue = []PaymentScheduleTarget{asScheduleTarget(healthy)}
	h.scheduler.errFor[broken.PaymentID] = errors.New("queue down")

	p := NewPaymentsPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	err := p.RunZoneScans(context.Background(), scanNow)
	require.Error(t, err)
	require.ErrorContains(t, err, "queue down")
	assert.Len(t, h.scheduler.booked, 1, "the healthy leg's booking went through")
}

// The due boundary job's wake (issue #776): the reload answers a live
// planned operation — «Оплатите платёж» publishes with the same copy, dedup
// key and recipients the sweep leg uses.
func TestPaymentsPublisher_DeliverPaymentDuePublishes(t *testing.T) {
	t.Parallel()

	h := newPaymentScanHarness([]ScanZone{{Timezone: zoneMSK}})
	rule := uuid.Must(uuid.NewV7())
	target := PaymentScanTarget{
		PaymentID:       rule,
		DueDate:         time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC),
		Title:           scanRentTitle,
		AmountKopecks:   250000,
		PropertyID:      uuid.Must(uuid.NewV7()),
		PropertyName:    scanPropertyName,
		PropertyAddress: scanPropertyAddress,
		PropertyType:    scanPropertyType,
		PropertyPhoto:   scanPropertyPhoto,
		OwnerID:         uuid.Must(uuid.NewV7()),
	}
	h.source.live["due|"+rule.String()+"|2026-09-20"] = target
	member := uuid.Must(uuid.NewV7())
	h.source.recips[target.PropertyID] = []uuid.UUID{member}

	p := NewPaymentsPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.DeliverPaymentDue(context.Background(), rule,
		time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), scanNow))

	// The owner plus the active member — one row each, both delivered to
	// both channels.
	require.Len(t, h.feed.inserted, 2)
	recipients := []uuid.UUID{h.feed.inserted[0].UserID, h.feed.inserted[1].UserID}
	assert.ElementsMatch(t, []uuid.UUID{target.OwnerID, member}, recipients)
	for _, n := range h.feed.inserted {
		assert.Equal(t, domain.EventPaymentDue, n.EventType)
		assert.Equal(t, "Оплатите платёж", n.Title)
		assert.Equal(t,
			scanRentReminderBody,
			n.Body)
		assert.Equal(t, domain.DedupKey("payment_due:"+rule.String()+":2026-09-20"), n.DedupKey)
	}
	assert.Len(t, h.queue.emails, 2)
	assert.Len(t, h.queue.pushes, 2)
}

// A dead operation at the wake-up — paid, cancelled, the rule edited to
// another date, the rule deleted, the property archived — finishes the job
// without publishing (the delivery-time resolution canon of #750).
func TestPaymentsPublisher_DeliverPaymentDueNoOpWhenDead(t *testing.T) {
	t.Parallel()

	h := newPaymentScanHarness([]ScanZone{{Timezone: zoneMSK}})
	p := NewPaymentsPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.DeliverPaymentDue(context.Background(), uuid.Must(uuid.NewV7()),
		time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), scanNow))
	assert.Empty(t, h.feed.inserted)
	assert.Empty(t, h.queue.emails)
}

// The overdue boundary job's wake (issue #776): «Платёж просрочен» at 00:00
// of the day after the operation date — the same row the sweep leg writes,
// whoever fires first (the dedup key keeps it one).
func TestPaymentsPublisher_DeliverPaymentOverduePublishes(t *testing.T) {
	t.Parallel()

	h := newPaymentScanHarness([]ScanZone{{Timezone: zoneMSK}})
	rule := uuid.Must(uuid.NewV7())
	target := dueTarget(rule, time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC), "Интернет")
	target.AmountKopecks = 100050
	h.source.live["overdue|"+rule.String()+"|2026-09-19"] = target

	p := NewPaymentsPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.DeliverPaymentOverdue(context.Background(), rule,
		time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC), scanNow))

	require.Len(t, h.feed.inserted, 1)
	n := h.feed.inserted[0]
	assert.Equal(t, domain.EventPaymentOverdue, n.EventType)
	assert.Equal(t, "Платёж просрочен", n.Title)
	assert.Equal(t,
		"Платёж «Интернет» по объекту «Квартира на Ленина» просрочен: 1\u00a0000,50 ₽. Срок оплаты был 19 сентября",
		n.Body)
	assert.Equal(t, domain.DedupKey("payment_overdue:"+rule.String()+":2026-09-19"), n.DedupKey)
}

// Напоминание о платеже (карта #822, #824): третья нога свипа — за N дней
// до даты операции. Копия — форма due-ноги (факт срока), тайтл свой; дедуп —
// (правило, дата операции), автоплатёж ногу не исключает.
func TestPaymentsPublisher_PublishesReminderPayment(t *testing.T) {
	t.Parallel()

	h := newPaymentScanHarness([]ScanZone{{Timezone: zoneMSK}})
	rule := uuid.Must(uuid.NewV7())
	target := PaymentScanTarget{
		PaymentID:       rule,
		DueDate:         time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC),
		Title:           scanRentTitle,
		AmountKopecks:   250000,
		PropertyID:      uuid.Must(uuid.NewV7()),
		PropertyName:    scanPropertyName,
		PropertyAddress: scanPropertyAddress,
		PropertyType:    scanPropertyType,
		PropertyPhoto:   scanPropertyPhoto,
		OwnerID:         uuid.Must(uuid.NewV7()),
	}
	h.source.reminder[zoneMSK] = []PaymentScanTarget{target}
	member := uuid.Must(uuid.NewV7())
	h.source.recips[target.PropertyID] = []uuid.UUID{member}

	p := NewPaymentsPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.RunZoneScans(context.Background(), scanNow))

	require.Len(t, h.feed.inserted, 2)
	recipients := []uuid.UUID{h.feed.inserted[0].UserID, h.feed.inserted[1].UserID}
	assert.ElementsMatch(t, []uuid.UUID{target.OwnerID, member}, recipients)
	for _, n := range h.feed.inserted {
		assert.Equal(t, domain.EventPaymentReminder, n.EventType)
		assert.Equal(t, domain.CategoryPaymentsOperations, n.Category)
		assert.Equal(t, "Напоминание о платеже", n.Title)
		assert.Equal(t,
			scanRentReminderBody,
			n.Body)
		assert.Equal(t, "Квартира на Ленина", n.ContextLabel)
		// Дедуп — (правило, дата операции): одно напоминание на вхождение.
		assert.Equal(t, domain.DedupKey("payment_reminder:"+rule.String()+":2026-09-20"), n.DedupKey)
		require.NotNil(t, n.Payload.PaymentID)
		assert.Equal(t, rule, *n.Payload.PaymentID)
		require.NotNil(t, n.Payload.PaymentDate)
	}
	assert.Len(t, h.queue.emails, 2)
	assert.Len(t, h.queue.pushes, 2)
}

// assertBookingPass runs one hourly pass and asserts the leg asked its
// boundary window and booked its (rule, date) job at the given instant — the
// shape the per-leg booking tests share.
func assertBookingPass(
	t *testing.T, h *paymentScanHarness, leg string, rule uuid.UUID, dueDate, fireAt time.Time,
) {
	t.Helper()
	p := NewPaymentsPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.RunZoneScans(context.Background(), scanNow))
	assert.Contains(t, h.source.schedAsked,
		leg+"|"+scanNow.Format(time.RFC3339)+"|"+scanNow.Add(scheduledHorizon).Format(time.RFC3339))
	assert.Equal(t, fireAt, h.scheduler.booked[paymentBookingKey(leg, rule, dueDate)])
}

// Букинг ноги напоминания: тот же горизонт, своя граница (10:00 «дата − N»
// (#1168) — инстант считает SQL-нога), повторный запрос возвращает стоящую
// джобу.
func TestPaymentsPublisher_SchedulesReminderBoundaries(t *testing.T) {
	t.Parallel()

	h := newPaymentScanHarness([]ScanZone{{Timezone: zoneMSK}})
	rule := uuid.Must(uuid.NewV7())
	h.source.schedReminder = []PaymentScheduleTarget{
		{
			PaymentID: rule, DueDate: time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC),
			FireAt: time.Date(2026, 9, 19, 7, 0, 0, 0, time.UTC),
		},
	}

	assertBookingPass(t, h, "reminder", rule,
		time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 19, 7, 0, 0, 0, time.UTC))
}

// Пробуждение reminder-джобы: живое planned-вхождение с напоминанием
// публикует ту же строку, что и свип; мёртвое — вежливый no-op.
func TestPaymentsPublisher_DeliverPaymentReminderPublishes(t *testing.T) {
	t.Parallel()

	h := newPaymentScanHarness([]ScanZone{{Timezone: zoneMSK}})
	rule := uuid.Must(uuid.NewV7())
	target := dueTarget(rule, time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), "Интернет")
	target.AmountKopecks = 100050
	h.source.live["reminder|"+rule.String()+"|2026-09-20"] = target

	p := NewPaymentsPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.DeliverPaymentReminder(context.Background(), rule,
		time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), scanNow))

	require.Len(t, h.feed.inserted, 1)
	n := h.feed.inserted[0]
	assert.Equal(t, domain.EventPaymentReminder, n.EventType)
	assert.Equal(t, "Напоминание о платеже", n.Title)
	assert.Equal(t, domain.DedupKey("payment_reminder:"+rule.String()+":2026-09-20"), n.DedupKey)
}

func TestPaymentsPublisher_DeliverPaymentReminderNoOpWhenDead(t *testing.T) {
	t.Parallel()

	h := newPaymentScanHarness([]ScanZone{{Timezone: zoneMSK}})
	p := NewPaymentsPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.DeliverPaymentReminder(context.Background(), uuid.Must(uuid.NewV7()),
		time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), scanNow))
	assert.Empty(t, h.feed.inserted)
	assert.Empty(t, h.queue.emails)
}

// Нога «Автоплатёж исполнен» (#1169): тиковое гашение своего дня публикует
// строку владелец + активные участники с дедупом (правило, дата); копия —
// решение #737-стиля с фактом автоплатежа.
func TestPaymentsPublisher_PublishesAutoPaidPayment(t *testing.T) {
	t.Parallel()

	h := newPaymentScanHarness([]ScanZone{{Timezone: zoneMSK}})
	rule := uuid.Must(uuid.NewV7())
	target := PaymentScanTarget{
		PaymentID:       rule,
		DueDate:         time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC),
		Title:           scanRentTitle,
		AmountKopecks:   250000,
		PropertyID:      uuid.Must(uuid.NewV7()),
		PropertyName:    scanPropertyName,
		PropertyAddress: scanPropertyAddress,
		PropertyType:    scanPropertyType,
		PropertyPhoto:   scanPropertyPhoto,
		OwnerID:         uuid.Must(uuid.NewV7()),
	}
	h.source.autoPaid[zoneMSK] = []PaymentScanTarget{target}
	member := uuid.Must(uuid.NewV7())
	h.source.recips[target.PropertyID] = []uuid.UUID{member}

	p := NewPaymentsPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.RunZoneScans(context.Background(), scanNow))

	require.Len(t, h.feed.inserted, 2)
	recipients := []uuid.UUID{h.feed.inserted[0].UserID, h.feed.inserted[1].UserID}
	assert.ElementsMatch(t, []uuid.UUID{target.OwnerID, member}, recipients)
	for _, n := range h.feed.inserted {
		assert.Equal(t, domain.EventPaymentAutoPaid, n.EventType)
		assert.Equal(t, domain.CategoryPaymentsOperations, n.Category)
		assert.Equal(t, "Автоплатёж исполнен", n.Title)
		assert.Contains(t, n.Body, "оплачен автоплатежём")
		assert.Equal(t, "Квартира на Ленина", n.ContextLabel)
		// Дедуп — (правило, дата операции): одно событие на вхождение.
		assert.Equal(t, domain.DedupKey("payment_auto_paid:"+rule.String()+":2026-09-20"), n.DedupKey)
		require.NotNil(t, n.Payload.PaymentID)
		assert.Equal(t, rule, *n.Payload.PaymentID)
	}
	assert.Len(t, h.queue.emails, 2)
	assert.Len(t, h.queue.pushes, 2)
}

// Букинг ноги автоплатежа: тот же горизонт, граница — 10:00 дня операции
// (инстант считает SQL-нога), повторный запрос возвращает стоящую джобу.
func TestPaymentsPublisher_SchedulesAutoPaidBoundaries(t *testing.T) {
	t.Parallel()

	h := newPaymentScanHarness([]ScanZone{{Timezone: zoneMSK}})
	rule := uuid.Must(uuid.NewV7())
	h.source.schedAutoPaid = []PaymentScheduleTarget{
		{
			PaymentID: rule, DueDate: time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC),
			FireAt: time.Date(2026, 9, 20, 7, 0, 0, 0, time.UTC),
		},
	}

	assertBookingPass(t, h, "auto paid", rule,
		time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 20, 7, 0, 0, 0, time.UTC))
}

// Пробуждение auto-paid-джобы: тиковый штамп своего дня публикует;
// ручная оплата того же вхождения и ролловер — молчок (решение владельца
// по гриллингу #1167).
func TestPaymentsPublisher_DeliverPaymentAutoPaid(t *testing.T) {
	t.Parallel()

	h := newPaymentScanHarness([]ScanZone{{Timezone: zoneMSK}})
	rule := uuid.Must(uuid.NewV7())
	target := dueTarget(rule, time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), "Интернет")
	h.source.live["auto paid|"+rule.String()+"|2026-09-20"] = target

	p := NewPaymentsPublisher(h.pipeline, h.zones, h.source, h.scheduler)
	require.NoError(t, p.DeliverPaymentAutoPaid(context.Background(), rule,
		time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), scanNow))

	require.Len(t, h.feed.inserted, 1)
	n := h.feed.inserted[0]
	assert.Equal(t, domain.EventPaymentAutoPaid, n.EventType)
	assert.Equal(t, "Автоплатёж исполнен", n.Title)
	assert.Equal(t, domain.DedupKey("payment_auto_paid:"+rule.String()+":2026-09-20"), n.DedupKey)

	// Мёртвые состояния — ручная оплата (штампа нет), не исполнено, день
	// ролловер — фейк отвечает live=false, джоба молчит.
	h2 := newPaymentScanHarness([]ScanZone{{Timezone: zoneMSK}})
	p2 := NewPaymentsPublisher(h2.pipeline, h2.zones, h2.source, h2.scheduler)
	require.NoError(t, p2.DeliverPaymentAutoPaid(context.Background(), uuid.Must(uuid.NewV7()),
		time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), scanNow))
	assert.Empty(t, h2.feed.inserted)
	assert.Empty(t, h2.queue.emails)
}

func TestFormatAmountKopecks(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "5 ₽", formatAmountKopecks(500))
	assert.Equal(t, "2\u00a0500 ₽", formatAmountKopecks(250000))
	assert.Equal(t, "10\u00a0000 ₽", formatAmountKopecks(1000000))
	assert.Equal(t, "2\u00a0500,50 ₽", formatAmountKopecks(250050))
	assert.Equal(t, "0,05 ₽", formatAmountKopecks(5))
}

func TestFormatPaymentDueDate(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "20 сентября",
		formatPaymentDueDate(time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)))
	assert.Equal(t, "1 марта",
		formatPaymentDueDate(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)))
}
