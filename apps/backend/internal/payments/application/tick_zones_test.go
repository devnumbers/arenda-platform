package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/transaction"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// Zone fixtures of the sweep tests: two real IANA zones whose midnights
// split around the fixture instant, plus a data-corruption stand-in.
const (
	zoneMoscow    = "Europe/Moscow"
	zoneKamchatka = "Asia/Kamchatka"
	zoneBroken    = "Not/AZone"
)

// fakeTickZoneDirectory is the TickZoneDirectory double for the zone sweep
// tests: it serves a canned zone listing.
type fakeTickZoneDirectory struct {
	zones []TickZone
	err   error
}

func (f *fakeTickZoneDirectory) ListTickZones(context.Context) ([]TickZone, error) {
	return f.zones, f.err
}

// fakeUoW runs every unit of work immediately on the caller's context — the
// sweep's per-owner transactions collapse into direct calls, so the assertions
// see exactly one owner per work function.
type fakeUoW struct {
	runs int
}

func (f *fakeUoW) Do(_ context.Context, work func(transaction.Tx) error) error {
	f.runs++
	return work(nil)
}

// noopPaymentStore and noopPropertyStore fill the unused seats of
// txStoreFactory: the sweep binds them per unit of work but never calls them.
type noopPaymentStore struct{}

func (noopPaymentStore) Get(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Payment, error) {
	panic("unused")
}

func (noopPaymentStore) ListByProperty(context.Context, uuid.UUID, uuid.UUID, string) ([]domain.Payment, error) {
	panic("unused")
}

func (noopPaymentStore) Create(context.Context, domain.Payment) error { panic("unused") }

func (noopPaymentStore) Update(context.Context, domain.Payment) error { panic("unused") }

func (noopPaymentStore) Delete(context.Context, uuid.UUID, uuid.UUID) error { panic("unused") }

func (noopPaymentStore) InsertPause(context.Context, uuid.UUID, time.Time) error {
	panic("unused")
}

func (noopPaymentStore) CloseActivePause(context.Context, uuid.UUID, time.Time) error {
	panic("unused")
}

func (noopPaymentStore) DeletePlannedFrom(context.Context, uuid.UUID, time.Time) error {
	panic("unused")
}

func (noopPaymentStore) DeletePlannedBefore(context.Context, uuid.UUID, time.Time) error {
	panic("unused")
}

func (noopPaymentStore) DeleteFuturePlanned(context.Context, uuid.UUID, time.Time) error {
	panic("unused")
}

func (noopPaymentStore) SetFavorite(context.Context, uuid.UUID, uuid.UUID, bool) error {
	panic("unused")
}

func (noopPaymentStore) WithTx(tx transaction.Tx) (PaymentStore, error) {
	return noopPaymentStore{}, nil
}

// noopOperationStore fills the operations seat of txStoreFactory in the sweep
// fixture: the tick binds it per unit of work but never calls it.
type noopOperationStore struct{}

func (noopOperationStore) Get(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (domain.Operation, error) {
	panic("unused")
}

func (noopOperationStore) MarkPaid(context.Context, uuid.UUID, uuid.UUID, time.Time) error {
	panic("unused")
}

func (noopOperationStore) Cancel(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return nil
}

func (noopOperationStore) Create(context.Context, domain.Operation) error {
	panic("unused")
}

func (noopOperationStore) ListByPayment(
	context.Context, uuid.UUID, uuid.UUID, uuid.UUID, OperationsListQuery,
) ([]domain.Operation, error) {
	panic("unused")
}

func (noopOperationStore) ListByProperty(
	context.Context, uuid.UUID, uuid.UUID, OperationsListQuery,
) ([]domain.Operation, error) {
	panic("unused")
}

func (noopOperationStore) SummarizeByProperty(
	context.Context, uuid.UUID, uuid.UUID, OperationsSummaryQuery,
) (OperationsSummary, error) {
	panic("unused")
}

func (noopOperationStore) CountPaidOperationsByPayment(
	context.Context, uuid.UUID, uuid.UUID, uuid.UUID,
) (int64, error) {
	panic("unused")
}

func (noopOperationStore) CountOverdueOperationsByPayment(
	context.Context, uuid.UUID, uuid.UUID, uuid.UUID, time.Time,
) (int64, error) {
	panic("unused")
}

func (noopOperationStore) CountPaidAndOverdueByPaymentIDs(
	context.Context, uuid.UUID, uuid.UUID, []uuid.UUID, time.Time,
) ([]PaidOverdueCount, error) {
	panic("unused")
}

func (noopOperationStore) NearestDateInputsOfPayments(
	context.Context, uuid.UUID, uuid.UUID, time.Time, []uuid.UUID,
) (map[uuid.UUID]NearestDateInputs, error) {
	panic("unused")
}

func (noopOperationStore) ListGlobal(context.Context, uuid.UUID, GlobalOperationsListQuery) ([]GlobalOperationRow, error) {
	panic("unused")
}

func (noopOperationStore) SummarizeGlobal(context.Context, uuid.UUID, GlobalOperationsSummaryQuery) (OperationsSummary, error) {
	panic("unused")
}

func (noopOperationStore) WithTx(tx transaction.Tx) (OperationStore, error) {
	return noopOperationStore{}, nil
}

type noopPropertyStore struct{}

func (noopPropertyStore) Get(context.Context, uuid.UUID) (PropertyRef, error) {
	panic("unused")
}

func (noopPropertyStore) GetForUpdate(context.Context, uuid.UUID) (PropertyRef, error) {
	panic("unused")
}

func (noopPropertyStore) WithTx(tx transaction.Tx) (PropertyStore, error) {
	return noopPropertyStore{}, nil
}

// noopRentalManaged fills the rental gate's seat for the in-memory fixtures:
// the tick and the favorites-order paths never consult it.
type noopRentalManaged struct{}

func (noopRentalManaged) ManagedPaymentIDs(
	context.Context, uuid.UUID, []uuid.UUID,
) (map[uuid.UUID]bool, error) {
	return map[uuid.UUID]bool{}, nil
}

func (noopRentalManaged) CompletedPaymentIDs(
	context.Context, uuid.UUID, []uuid.UUID,
) (map[uuid.UUID]bool, error) {
	return map[uuid.UUID]bool{}, nil
}

func (noopRentalManaged) WithTx(tx transaction.Tx) (RentalManagedReader, error) {
	return noopRentalManaged{}, nil
}

// sweepFixture wires a TickService over the in-memory doubles: one shared
// fakeTickStore (canned snapshot), the fake zone directory and the counting
// UoW.
type sweepFixture struct {
	tick  *TickService
	store *fakeTickStore
	uow   *fakeUoW
	now   time.Time
}

func newSweepFixture(t *testing.T, zones []TickZone, snapshot OwnerSnapshot) *sweepFixture {
	t.Helper()
	store := &fakeTickStore{snapshot: snapshot}
	uow := &fakeUoW{}
	factory := NewTxStoreFactory(store, noopPaymentStore{}, noopOperationStore{},
		noopPropertyStore{}, newFakeFavoriteOrderStore(nil), noopRentalManaged{}, nil, nil, uow)
	return &sweepFixture{
		tick:  NewTickService(factory, &fakeTickZoneDirectory{zones: zones}, nil, nil),
		store: store,
		uow:   uow,
		now:   time.Date(2026, 8, 25, 20, 0, 0, 0, time.UTC),
	}
}

func TestZoneToday_ComputesDateInZoneLocation(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 25, 20, 0, 0, 0, time.UTC)

	cases := []struct {
		zone string
		want string
	}{
		// 20:00 UTC is still the 25th in Moscow (UTC+3)…
		{zoneMoscow, "2026-08-25"},
		// …but already the 26th in Kamchatka (UTC+12): the day boundary belongs
		// to the owner's zone, not to UTC (ADR 0048 p.2).
		{zoneKamchatka, "2026-08-26"},
		{"UTC", "2026-08-25"},
	}
	for _, tc := range cases {
		t.Run(tc.zone, func(t *testing.T) {
			t.Parallel()
			today, err := zoneToday(now, tc.zone)
			require.NoError(t, err)
			assert.Equal(t, tc.want, today.Format(time.DateOnly))
			// The domain's date convention: UTC midnight, so the value compares
			// correctly against DATE columns.
			assert.Equal(t, time.UTC, today.Location())
			assert.Equal(t, 0, today.Hour()+today.Minute()+today.Second())
		})
	}
}

func TestZoneToday_UnknownZoneIsError(t *testing.T) {
	t.Parallel()
	_, err := zoneToday(time.Now(), zoneBroken)
	require.Error(t, err)
	assert.Contains(t, err.Error(), zoneBroken, "the error names the broken zone")
}

func TestRunZoneTicks_TicksEachOwnerOnItsZoneToday(t *testing.T) {
	t.Parallel()
	mscOwner := uuid.Must(uuid.NewV7())
	kgtOwner := uuid.Must(uuid.NewV7())
	payment := tickOwnerPayment(t, uuid.Must(uuid.NewV7()), "2026-08-22")

	f := newSweepFixture(t,
		[]TickZone{
			{Timezone: zoneKamchatka, Owners: []uuid.UUID{kgtOwner}},
			{Timezone: zoneMoscow, Owners: []uuid.UUID{mscOwner}},
		},
		OwnerSnapshot{Payments: []domain.Payment{payment}},
	)

	require.NoError(t, f.tick.RunZoneTicks(t.Context(), f.now))

	// Every owner got its own unit of work: two transactions, one owner each.
	require.Len(t, f.store.locked, 2)
	assert.Equal(t, 2, f.uow.runs)
	// The same snapshot serves both, and each apply carries its zone's today:
	// the 26th for Kamchatka, the 25th for Moscow.
	require.Len(t, f.store.applied, 2)
	assert.Equal(t, "2026-08-26", f.store.applied[0].today.Format(time.DateOnly))
	assert.Equal(t, "2026-08-25", f.store.applied[1].today.Format(time.DateOnly))
}

func TestRunZoneTicks_OwnerFailureIsIsolated(t *testing.T) {
	t.Parallel()
	first := uuid.Must(uuid.NewV7())
	second := uuid.Must(uuid.NewV7())
	lockFail := assert.AnError

	f := newSweepFixture(t,
		[]TickZone{{Timezone: zoneMoscow, Owners: []uuid.UUID{first, second}}},
		OwnerSnapshot{},
	)
	f.store.lockErr = lockFail

	err := f.tick.RunZoneTicks(t.Context(), f.now)
	require.ErrorIs(t, err, lockFail)
	// The first owner's failure did not stop the sweep: every owner still got
	// its own unit of work (the fake store records locked owners only on
	// success, so the counting UoW is the witness).
	assert.Equal(t, 2, f.uow.runs)
}

func TestRunZoneTicks_BrokenZoneIsSkipped(t *testing.T) {
	t.Parallel()
	healthy := uuid.Must(uuid.NewV7())

	f := newSweepFixture(t,
		[]TickZone{
			{Timezone: zoneBroken, Owners: []uuid.UUID{uuid.Must(uuid.NewV7())}},
			{Timezone: zoneMoscow, Owners: []uuid.UUID{healthy}},
		},
		OwnerSnapshot{},
	)

	err := f.tick.RunZoneTicks(t.Context(), f.now)
	require.Error(t, err)
	assert.Contains(t, err.Error(), zoneBroken)
	// The broken zone's failure did not stop the healthy zone's owners.
	assert.Equal(t, []uuid.UUID{healthy}, f.store.locked)
}

func TestRunZoneTicks_NoZonesIsNoopSuccess(t *testing.T) {
	t.Parallel()
	f := newSweepFixture(t, nil, OwnerSnapshot{})

	require.NoError(t, f.tick.RunZoneTicks(t.Context(), f.now))
	assert.Empty(t, f.store.locked)
	assert.Equal(t, 0, f.uow.runs)
}

// TestRunZoneTicks_Heartbeat follows the "tick has not run in N hours"
// alert's own signal end to end (ADR 0048 p.3): a fully successful sweep
// records the heartbeat gauge at the sweep instant, a failed sweep records
// nothing. Not parallel (it swaps the global OTel MeterProvider for a
// manual reader and restores it on exit) — the paralleltest exemption the
// global-state swap earns.
func TestRunZoneTicks_Heartbeat(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	prev := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	t.Cleanup(func() {
		otel.SetMeterProvider(prev)
		if err := provider.Shutdown(t.Context()); err != nil {
			t.Fatalf("shutdown meter provider: %v", err)
		}
	})

	metrics, err := NewMetrics()
	require.NoError(t, err)

	success := newSweepFixture(t, nil, OwnerSnapshot{})
	success.tick.metrics = metrics
	require.NoError(t, success.tick.RunZoneTicks(t.Context(), success.now))

	failure := newSweepFixture(t,
		[]TickZone{{Timezone: zoneBroken, Owners: []uuid.UUID{uuid.Must(uuid.NewV7())}}},
		OwnerSnapshot{},
	)
	failure.tick.metrics = metrics
	require.Error(t, failure.tick.RunZoneTicks(t.Context(), failure.now))

	var data metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(t.Context(), &data))
	// Exactly one heartbeat point, from the successful sweep only, at its
	// own instant — the failed sweep stays silent so the alert can see it.
	var points []metricdata.DataPoint[int64]
	for _, m := range data.ScopeMetrics {
		for _, mm := range m.Metrics {
			if mm.Name != "payments.tick.last_success" {
				continue
			}
			gauge, ok := mm.Data.(metricdata.Gauge[int64])
			if !ok {
				t.Fatalf("payments.tick.last_success is %T, want a gauge", mm.Data)
			}
			points = append(points, gauge.DataPoints...)
		}
	}
	require.Len(t, points, 1, "only the successful sweep records the heartbeat")
	assert.Equal(t, success.now.Unix(), points[0].Value)
}
