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

// zoneMSK is the sweep zone of most tests: the Moscow owner timezone.
const zoneMSK = "Europe/Moscow"

// fakeScanZones is the zone directory stub: the distinct owner timezones
// having scan-relevant rentals.
type fakeScanZones struct {
	zones []RentalScanZone
	err   error
}

func (f *fakeScanZones) ListScanZones(ctx context.Context) ([]RentalScanZone, error) {
	return f.zones, f.err
}

// fakeRentalSource is the scan source stub: per zone it answers the targets
// the caller asks for and records every (zone, today) question, so the tests
// assert each zone was swept with its own calendar date.
type fakeRentalSource struct {
	// Targets answers the per-zone sweep; Asked records the "zone|today"
	// questions in order; Recips holds the active members by property and
	// RecErr fails a property's lookup on demand.
	targets map[string][]RentalCompletedTarget
	asked   []string
	recips  map[uuid.UUID][]uuid.UUID
	recErr  map[uuid.UUID]error
}

func (f *fakeRentalSource) ListCompletedTargets(ctx context.Context, zone string, today time.Time) ([]RentalCompletedTarget, error) {
	f.asked = append(f.asked, zone+"|"+today.Format("2006-01-02"))
	return f.targets[zone], nil
}

func (f *fakeRentalSource) ListActiveRecipients(ctx context.Context, propertyID uuid.UUID) ([]uuid.UUID, error) {
	if err := f.recErr[propertyID]; err != nil {
		return nil, err
	}
	return f.recips[propertyID], nil
}

// scanHarness builds the publisher over the real pipeline backed by the
// publisher tests' fakes: the tests read the feed rows the scan created.
type scanHarness struct {
	pipeline *Publisher
	feed     *fakeFeedRepo
	queue    *fakeQueue
	zones    *fakeScanZones
	source   *fakeRentalSource
}

func newScanHarness(zones []RentalScanZone) *scanHarness {
	feed := &fakeFeedRepo{}
	queue := &fakeQueue{}
	pipeline := NewPublisher(feed, queue, nil, &fakeUoW{}, nil)
	h := &scanHarness{
		pipeline: pipeline,
		feed:     feed,
		queue:    queue,
		zones:    &fakeScanZones{zones: zones},
		source: &fakeRentalSource{
			targets: map[string][]RentalCompletedTarget{},
			recips:  map[uuid.UUID][]uuid.UUID{},
			recErr:  map[uuid.UUID]error{},
		},
	}
	return h
}

// scanTarget builds a needs_attention rental target on a fresh property.
func scanTarget(rentalID uuid.UUID, plannedEnd time.Time, name string) RentalCompletedTarget {
	return RentalCompletedTarget{
		RentalID:        rentalID,
		PlannedEndDate:  plannedEnd,
		PropertyID:      uuid.Must(uuid.NewV7()),
		PropertyName:    name,
		PropertyAddress: "г. Москва, ул. Ленина, 1",
		OwnerID:         uuid.Must(uuid.NewV7()),
	}
}

// The trigger instant: late enough that Moscow (UTC+3) has already rolled to
// 2026-09-20 while Kaliningrad (UTC+2) is still on 2026-09-19.
var scanNow = time.Date(2026, 9, 19, 21, 30, 0, 0, time.UTC)

func TestRentalCompletedPublisher_PublishesNeedsAttentionRental(t *testing.T) {
	t.Parallel()

	h := newScanHarness([]RentalScanZone{{Timezone: zoneMSK}})
	rental := uuid.Must(uuid.NewV7())
	target := scanTarget(rental, time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), "Квартира на Ленина")
	member1, member2 := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	h.source.targets[zoneMSK] = []RentalCompletedTarget{target}
	h.source.recips[target.PropertyID] = []uuid.UUID{member1, member2}

	p := NewRentalCompletedPublisher(h.pipeline, h.zones, h.source)
	err := p.RunZoneScans(context.Background(), scanNow)
	require.NoError(t, err)

	// Owner plus two active members — one row each, all three delivered to
	// both channels.
	require.Len(t, h.feed.inserted, 3)
	recipients := []uuid.UUID{h.feed.inserted[0].UserID, h.feed.inserted[1].UserID, h.feed.inserted[2].UserID}
	assert.ElementsMatch(t, []uuid.UUID{target.OwnerID, member1, member2}, recipients)

	for _, n := range h.feed.inserted {
		assert.Equal(t, domain.EventRentalCompleted, n.EventType)
		assert.Equal(t, domain.CategoryRental, n.Category)
		assert.Equal(t, "Аренда завершена", n.Title)
		assert.Equal(t, "Договор аренды по объекту «Квартира на Ленина» завершён. Продлите договор или завершите аренду", n.Body)
		assert.Equal(t, "Квартира на Ленина", n.ContextLabel)
		// The dedup key pins the rental and the planned end it fired for.
		assert.Equal(t, domain.DedupKey("rental_completed:"+rental.String()+":2026-09-18"), n.DedupKey)
		// The payload carries the property snapshot with its address line
		// (решение владельца 19.09.2026, #745) and the bare rental id.
		require.NotNil(t, n.Payload.Property)
		assert.Equal(t, target.PropertyID, n.Payload.Property.ID)
		assert.Equal(t, "Квартира на Ленина", n.Payload.Property.Name)
		assert.Equal(t, "г. Москва, ул. Ленина, 1", n.Payload.Property.Address)
		require.NotNil(t, n.Payload.RentalID)
		assert.Equal(t, rental, *n.Payload.RentalID)
		assert.Nil(t, n.Payload.Actor, "a system scan has no initiator")
	}
	assert.Len(t, h.queue.emails, 3)
	assert.Len(t, h.queue.pushes, 3)
}

func TestRentalCompletedPublisher_ExtensionRepublishesWithNewKey(t *testing.T) {
	t.Parallel()

	h := newScanHarness([]RentalScanZone{{Timezone: zoneMSK}})
	rental := uuid.Must(uuid.NewV7())
	first := scanTarget(rental, time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), "Студия на Пушкина")
	// The owner extended: same rental, the planned end moved a month out.
	extended := first
	extended.PlannedEndDate = time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	h.source.targets[zoneMSK] = []RentalCompletedTarget{first}

	p := NewRentalCompletedPublisher(h.pipeline, h.zones, h.source)
	require.NoError(t, p.RunZoneScans(context.Background(), scanNow))
	require.Len(t, h.feed.inserted, 1)

	// The next pass after the extended date has passed too: the new planned
	// end is a new key, the rental notifies again.
	h.source.targets[zoneMSK] = []RentalCompletedTarget{extended}
	require.NoError(t, p.RunZoneScans(context.Background(), scanNow))
	require.Len(t, h.feed.inserted, 2)
	assert.Equal(t, domain.DedupKey("rental_completed:"+rental.String()+":2026-10-18"), h.feed.inserted[1].DedupKey)
}

func TestRentalCompletedPublisher_RepeatScanIsNoOp(t *testing.T) {
	t.Parallel()

	h := newScanHarness([]RentalScanZone{{Timezone: zoneMSK}})
	target := scanTarget(uuid.Must(uuid.NewV7()), time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), "Квартира на Ленина")
	h.source.targets[zoneMSK] = []RentalCompletedTarget{target}

	p := NewRentalCompletedPublisher(h.pipeline, h.zones, h.source)
	require.NoError(t, p.RunZoneScans(context.Background(), scanNow))
	require.NoError(t, p.RunZoneScans(context.Background(), scanNow))
	assert.Len(t, h.feed.inserted, 1, "the second hourly pass dedups to nothing")
}

func TestRentalCompletedPublisher_EachZoneSweptWithItsOwnToday(t *testing.T) {
	t.Parallel()

	h := newScanHarness([]RentalScanZone{{Timezone: zoneMSK}, {Timezone: "Europe/Kaliningrad"}})
	p := NewRentalCompletedPublisher(h.pipeline, h.zones, h.source)
	require.NoError(t, p.RunZoneScans(context.Background(), scanNow))

	// Moscow's local calendar has already rolled to the 20th, Kaliningrad's
	// has not — the sweep must not share one today across zones.
	assert.ElementsMatch(t, []string{
		"Europe/Moscow|2026-09-20",
		"Europe/Kaliningrad|2026-09-19",
	}, h.source.asked)
}

func TestRentalCompletedPublisher_NoZonesNoTargets(t *testing.T) {
	t.Parallel()

	h := newScanHarness(nil)
	p := NewRentalCompletedPublisher(h.pipeline, h.zones, h.source)
	require.NoError(t, p.RunZoneScans(context.Background(), scanNow))
	assert.Empty(t, h.feed.inserted)

	h = newScanHarness([]RentalScanZone{{Timezone: zoneMSK}})
	p = NewRentalCompletedPublisher(h.pipeline, h.zones, h.source)
	require.NoError(t, p.RunZoneScans(context.Background(), scanNow))
	assert.Empty(t, h.feed.inserted)
}

func TestRentalCompletedPublisher_FailuresAreIsolated(t *testing.T) {
	t.Parallel()

	h := newScanHarness([]RentalScanZone{{Timezone: zoneMSK}})
	ok := scanTarget(uuid.Must(uuid.NewV7()), time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), "Квартира на Ленина")
	broken := scanTarget(uuid.Must(uuid.NewV7()), time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), "Дом на Гагарина")
	h.source.targets[zoneMSK] = []RentalCompletedTarget{ok, broken}
	h.source.recErr[broken.PropertyID] = errors.New("members lookup down")

	p := NewRentalCompletedPublisher(h.pipeline, h.zones, h.source)
	err := p.RunZoneScans(context.Background(), scanNow)
	require.Error(t, err)
	require.ErrorContains(t, err, "members lookup down")
	// The healthy rental of the same pass still published.
	require.Len(t, h.feed.inserted, 1)
	assert.Equal(t, ok.OwnerID, h.feed.inserted[0].UserID)
}

func TestRentalCompletedPublisher_BrokenZoneNameIsolated(t *testing.T) {
	t.Parallel()

	h := newScanHarness([]RentalScanZone{{Timezone: "Mars/Olympus"}, {Timezone: zoneMSK}})
	target := scanTarget(uuid.Must(uuid.NewV7()), time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), "Квартира на Ленина")
	h.source.targets[zoneMSK] = []RentalCompletedTarget{target}

	p := NewRentalCompletedPublisher(h.pipeline, h.zones, h.source)
	err := p.RunZoneScans(context.Background(), scanNow)
	require.Error(t, err)
	require.ErrorContains(t, err, "Mars/Olympus")
	assert.Len(t, h.feed.inserted, 1, "the healthy zone still swept")
}

func TestRentalCompletedPublisher_NilZoneDirectory(t *testing.T) {
	t.Parallel()

	h := newScanHarness(nil)
	p := NewRentalCompletedPublisher(h.pipeline, nil, h.source)
	err := p.RunZoneScans(context.Background(), scanNow)
	require.Error(t, err)
	require.ErrorContains(t, err, "zone directory")
}
