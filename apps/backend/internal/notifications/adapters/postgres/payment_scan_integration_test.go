package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nambers/arenda-planform/apps/backend/internal/notifications/application"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// insertScanPayment adds a payment rule row with an explicit auto-pay mode —
// the scan's due leg sweeps the manual-mode rules only, the overdue leg both.
// An auto-pay rule travels with notify_auto_paid = true — the post-migration
// world (#1186: существующим автоплатежам флаг проставлен); the flag's own
// gate inserts its silent rules explicitly.
func insertScanPayment(t *testing.T, pool *pgxpool.Pool, ownerID, propertyID uuid.UUID, autoPay bool) uuid.UUID {
	t.Helper()
	return insertScanPaymentWithNotify(t, pool, ownerID, propertyID, autoPay, autoPay)
}

// insertScanSilentAutoPaidPayment adds an auto-pay rule with the «Не
// уведомлять» flag (#1189) — the auto-paid leg's SQL gate keeps it silent
// even with the tick's own-day stamp in place.
func insertScanSilentAutoPaidPayment(t *testing.T, pool *pgxpool.Pool, ownerID, propertyID uuid.UUID) uuid.UUID {
	t.Helper()
	return insertScanPaymentWithNotify(t, pool, ownerID, propertyID, true, false)
}

// insertScanPaymentWithNotify is the shared payment-rule fixture with both
// mode flags spelled out (auto_pay, notify_auto_paid).
func insertScanPaymentWithNotify(
	t *testing.T, pool *pgxpool.Pool, ownerID, propertyID uuid.UUID, autoPay, notifyAutoPaid bool,
) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(context.Background(), `
		INSERT INTO payments (id, owner_id, property_id, type, title, amount_kopecks, recurrence,
			since, auto_pay, notify_auto_paid, category_slug)
		VALUES ($1, $2, $3, 'expense', 'Обслуживание', 250000, '{"kind": "monthly"}',
			'2026-08-01', $4, $5, 'maintenance')`,
		id, ownerID, propertyID, autoPay, notifyAutoPaid)
	require.NoError(t, err)
	return id
}

// insertScanReminderPayment adds a payment rule row with the reminder lead
// time set (карта #822) — the reminder leg sweeps reminder-carrying rules
// only, auto-pay mode included (решение #823).
func insertScanReminderPayment(
	t *testing.T, pool *pgxpool.Pool, ownerID, propertyID uuid.UUID, autoPay bool, reminderOffsetDays int,
) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(context.Background(), `
		INSERT INTO payments (id, owner_id, property_id, type, title, amount_kopecks, recurrence,
			since, auto_pay, reminder_offset_days, category_slug)
		VALUES ($1, $2, $3, 'expense', 'Обслуживание', 250000, '{"kind": "monthly"}',
			'2026-08-01', $4, $5, 'maintenance')`,
		id, ownerID, propertyID, autoPay, reminderOffsetDays)
	require.NoError(t, err)
	return id
}

// insertScanOperation adds one operation of the rule (or a manual fact when
// the rule id is nil) at the given date with the given status.
func insertScanOperation(
	t *testing.T, pool *pgxpool.Pool, ownerID, propertyID, ruleID uuid.UUID,
	date, status string,
) {
	t.Helper()
	origin, paymentID := "payment", any(ruleID)
	if ruleID == uuid.Nil {
		origin, paymentID = "manual", nil
	}
	paidDate := any(nil)
	if status == "paid" {
		paidDate = date
	}
	_, err := pool.Exec(context.Background(), `
		INSERT INTO operations (id, owner_id, property_id, payment_id, origin,
			date, paid_date, status, type, title, amount_kopecks, category_label)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'expense', 'Обслуживание', 250000, 'Обслуживание')`,
		uuid.Must(uuid.NewV7()), ownerID, propertyID, paymentID, origin, date, paidDate, status)
	require.NoError(t, err)
}

// The scan's sweep targets: owners whose planned payment-rule operations sit
// on non-archived properties (ADR 0048 p.3). A manual fact, a settled feed
// and an archived property are no sweep targets.
func TestPaymentScanStore_ListScanZones(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	msk := createUserInZone(t, pool, "Europe/Moscow")
	mskProp := createLiveProperty(t, pool, msk)
	rule := insertScanPayment(t, pool, msk, mskProp, false)
	insertScanOperation(t, pool, msk, mskProp, rule, "2026-09-19", "planned")

	kgd := createUserInZone(t, pool, "Europe/Kaliningrad")
	kgdProp := createLiveProperty(t, pool, kgd)
	kgdRule := insertScanPayment(t, pool, kgd, kgdProp, true)
	insertScanOperation(t, pool, kgd, kgdProp, kgdRule, "2026-08-01", "planned")

	// An owner whose only payment-operation facts are settled — no zone.
	quiet := createUserInZone(t, pool, "Europe/Moscow")
	quietProp := createLiveProperty(t, pool, quiet)
	quietRule := insertScanPayment(t, pool, quiet, quietProp, false)
	insertScanOperation(t, pool, quiet, quietProp, quietRule, "2026-09-19", "paid")

	// An archived property's planned operation stays out of the sweep (the
	// ticks' canon).
	archived := createUserInZone(t, pool, "Europe/Moscow")
	archivedProp := createLiveProperty(t, pool, archived)
	archivedRule := insertScanPayment(t, pool, archived, archivedProp, false)
	insertScanOperation(t, pool, archived, archivedProp, archivedRule, "2026-09-19", "planned")
	archiveProperty(t, pool, archived, archivedProp)

	store := NewPaymentScanStore(pool)
	zones, err := store.ListScanZones(ctx)
	require.NoError(t, err)

	got := make([]string, 0, len(zones))
	for _, z := range zones {
		got = append(got, z.Timezone)
	}
	// The shared test database's parallel fixtures may add zones — assert
	// this test's zones are swept, not that the world holds exactly them.
	assert.Contains(t, got, "Europe/Moscow")
	assert.Contains(t, got, "Europe/Kaliningrad")
}

// The due leg (решение #737, тип №2): planned operations dated exactly the
// zone's today, auto-pay rules excluded — the tick extinguishes their due
// occurrence the same day.
func TestPaymentScanStore_ListDueTargets(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	msk := createUserInZone(t, pool, "Europe/Moscow")
	prop := createLivePropertyFor(t, pool, msk, "Квартира на Ленина", "Москва, Тверская 1")
	rule := insertScanPayment(t, pool, msk, prop, false)
	insertScanOperation(t, pool, msk, prop, rule, "2026-09-19", "planned")

	// A manual fact dated today and a settled or cancelled rule occurrence —
	// none of them asks to be paid.
	insertScanOperation(t, pool, msk, prop, uuid.Nil, "2026-09-19", "planned")
	insertScanOperation(t, pool, msk, prop, rule, "2026-09-18", "paid")
	insertScanOperation(t, pool, msk, prop, rule, "2026-09-17", "cancelled")

	// The auto-pay rule's due occurrence stays out: the tick pays it today.
	autoProp := createLiveProperty(t, pool, msk)
	autoRule := insertScanPayment(t, pool, msk, autoProp, true)
	insertScanOperation(t, pool, msk, autoProp, autoRule, "2026-09-19", "planned")

	// Another zone's due operation stays out of Moscow's pass.
	kgd := createUserInZone(t, pool, "Europe/Kaliningrad")
	kgdProp := createLiveProperty(t, pool, kgd)
	kgdRule := insertScanPayment(t, pool, kgd, kgdProp, false)
	insertScanOperation(t, pool, kgd, kgdProp, kgdRule, "2026-09-19", "planned")

	// Tomorrow's occurrence is not due yet.
	insertScanOperation(t, pool, msk, prop, rule, "2026-10-19", "planned")

	// An archived property's due operation stays out (the ticks' canon).
	archivedProp := createLiveProperty(t, pool, msk)
	archivedRule := insertScanPayment(t, pool, msk, archivedProp, false)
	insertScanOperation(t, pool, msk, archivedProp, archivedRule, "2026-09-19", "planned")
	archiveProperty(t, pool, msk, archivedProp)

	store := NewPaymentScanStore(pool)
	// The instant gate (#1168): before the boundary's wall clock 10:00 —
	// 08:00 Moscow — the due leg holds the operation back.
	early := time.Date(2026, 9, 19, 5, 0, 0, 0, time.UTC)
	targets, err := store.ListDueTargets(ctx, "Europe/Moscow", time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC), early)
	require.NoError(t, err)
	got := paymentTargetsOfProps(targets, prop, autoProp, archivedProp)
	require.Empty(t, got, "the sweep must not publish ahead of the 10:00 boundary")

	// At the boundary — Moscow's 10:00 — the leg speaks.
	now := time.Date(2026, 9, 19, 7, 0, 0, 0, time.UTC)
	targets, err = store.ListDueTargets(ctx, "Europe/Moscow", time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC), now)
	require.NoError(t, err)
	// The shared test database's parallel fixtures' rows may appear in the
	// same zone's sweep — filter to the properties this test created.
	got = paymentTargetsOfProps(targets, prop, autoProp, archivedProp)
	require.Len(t, got, 1)

	target := got[0]
	assert.Equal(t, rule, target.PaymentID)
	assert.Equal(t, time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC), target.DueDate)
	assert.Equal(t, "Обслуживание", target.Title)
	assert.Equal(t, int64(250000), target.AmountKopecks)
	assert.Equal(t, prop, target.PropertyID)
	assert.Equal(t, "Квартира на Ленина", target.PropertyName)
	assert.Equal(t, "Москва, Тверская 1", target.PropertyAddress)
	assert.Equal(t, msk, target.OwnerID)
}

// The overdue leg (решение #737, тип №3): planned operations dated strictly
// before the zone's today — auto-pay rules included, the tick never
// backdates an auto charge (ADR 0049); a due-day occurrence is not overdue
// yet.
func TestPaymentScanStore_ListOverdueTargets(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	msk := createUserInZone(t, pool, "Europe/Moscow")
	prop := createLiveProperty(t, pool, msk)
	rule := insertScanPayment(t, pool, msk, prop, false)
	insertScanOperation(t, pool, msk, prop, rule, "2026-09-10", "planned")

	// The auto-pay mode does not excuse an overdue occurrence.
	autoProp := createLiveProperty(t, pool, msk)
	autoRule := insertScanPayment(t, pool, msk, autoProp, true)
	insertScanOperation(t, pool, msk, autoProp, autoRule, "2026-09-15", "planned")

	// Paid, cancelled and today's occurrences are not overdue.
	insertScanOperation(t, pool, msk, prop, rule, "2026-09-05", "paid")
	insertScanOperation(t, pool, msk, prop, rule, "2026-09-07", "cancelled")
	insertScanOperation(t, pool, msk, prop, rule, "2026-09-19", "planned")

	// Another zone's overdue operation stays out of Moscow's pass.
	kgd := createUserInZone(t, pool, "Europe/Kaliningrad")
	kgdProp := createLiveProperty(t, pool, kgd)
	kgdRule := insertScanPayment(t, pool, kgd, kgdProp, false)
	insertScanOperation(t, pool, kgd, kgdProp, kgdRule, "2026-09-01", "planned")

	// An archived property's overdue operation stays out (the ticks' canon).
	archivedProp := createLiveProperty(t, pool, msk)
	archivedRule := insertScanPayment(t, pool, msk, archivedProp, false)
	insertScanOperation(t, pool, msk, archivedProp, archivedRule, "2026-09-02", "planned")
	archiveProperty(t, pool, msk, archivedProp)

	// Yesterday's occurrence: its 22:00 boundary (#1168) is Moscow's today
	// evening — the leg holds it until then even though the date says
	// overdue since the zone's midnight.
	freshProp := createLiveProperty(t, pool, msk)
	freshRule := insertScanPayment(t, pool, msk, freshProp, false)
	insertScanOperation(t, pool, msk, freshProp, freshRule, "2026-09-18", "planned")

	store := NewPaymentScanStore(pool)
	// The instant gate (#1168): 08:00 Moscow — yesterday's occurrence is
	// overdue by date but its 22:00 has not come; the long-past ones speak.
	early := time.Date(2026, 9, 19, 5, 0, 0, 0, time.UTC)
	targets, err := store.ListOverdueTargets(ctx, "Europe/Moscow", time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC), early)
	require.NoError(t, err)
	// Filter to the properties this test created (shared test database).
	got := paymentTargetsOfProps(targets, prop, autoProp, archivedProp, freshProp)
	require.Len(t, got, 2, "yesterday's occurrence waits for its 22:00")
	dates := map[uuid.UUID]time.Time{}
	for _, target := range got {
		dates[target.PaymentID] = target.DueDate
	}
	assert.Equal(t, time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC), dates[rule])
	assert.Equal(t, time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC), dates[autoRule])

	// At the boundary — Moscow's 22:00 — yesterday's occurrence joins.
	now := time.Date(2026, 9, 19, 19, 0, 0, 0, time.UTC)
	targets, err = store.ListOverdueTargets(ctx, "Europe/Moscow", time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC), now)
	require.NoError(t, err)
	got = paymentTargetsOfProps(targets, prop, autoProp, archivedProp, freshProp)
	require.Len(t, got, 3)
	dates = map[uuid.UUID]time.Time{}
	for _, target := range got {
		dates[target.PaymentID] = target.DueDate
	}
	assert.Equal(t, time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), dates[freshRule])
}

// paymentTargetsOfProps filters the sweep's targets to the given properties —
// the shared test database's parallel fixtures must not break the count
// assertions.
func paymentTargetsOfProps(targets []application.PaymentScanTarget, props ...uuid.UUID) []application.PaymentScanTarget {
	want := make(map[uuid.UUID]bool, len(props))
	for _, p := range props {
		want[p] = true
	}
	out := make([]application.PaymentScanTarget, 0, len(targets))
	for _, target := range targets {
		if want[target.PropertyID] {
			out = append(out, target)
		}
	}
	return out
}

// The booking window of the due leg (issue #776): planned operations whose
// due boundary — the wall clock 10:00 of the operation date in the owner's
// timezone (#1168) — falls into (from, until], auto-pay rules excluded.
// Moscow's 10:00 of the 20th is 2026-09-20T07:00Z; the boundary carries the
// zone's shift.
func TestPaymentScanStore_ListScheduledDueTargets(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	msk := createUserInZone(t, pool, "Europe/Moscow")
	prop := createLiveProperty(t, pool, msk)
	rule := insertScanPayment(t, pool, msk, prop, false)
	// The in-window occurrence: due boundary 2026-09-20T07:00Z.
	insertScanOperation(t, pool, msk, prop, rule, "2026-09-20", "planned")

	// A settled, a cancelled, an auto-pay and an out-of-window occurrence —
	// none of them books a due job.
	insertScanOperation(t, pool, msk, prop, rule, "2026-09-21", "paid")
	insertScanOperation(t, pool, msk, prop, rule, "2026-09-22", "cancelled")
	autoProp := createLiveProperty(t, pool, msk)
	autoRule := insertScanPayment(t, pool, msk, autoProp, true)
	insertScanOperation(t, pool, msk, autoProp, autoRule, "2026-09-20", "planned")
	insertScanOperation(t, pool, msk, prop, rule, "2026-10-20", "planned")

	// An archived property's occurrence stays out (the ticks' canon).
	archivedProp := createLiveProperty(t, pool, msk)
	archivedRule := insertScanPayment(t, pool, msk, archivedProp, false)
	insertScanOperation(t, pool, msk, archivedProp, archivedRule, "2026-09-20", "planned")
	archiveProperty(t, pool, msk, archivedProp)

	store := NewPaymentScanStore(pool)
	targets, err := store.ListScheduledDueTargets(ctx,
		time.Date(2026, 9, 19, 6, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 20, 8, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	got := scheduleTargetsOfProps(targets, rule, autoRule, archivedRule)
	require.Len(t, got, 1)
	assert.Equal(t, rule, got[0].PaymentID)
	assert.Equal(t, time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), got[0].DueDate)
	assert.True(t, got[0].FireAt.Equal(time.Date(2026, 9, 20, 7, 0, 0, 0, time.UTC)),
		"the job wakes at the wall clock 10:00 of the operation date in the owner's zone")
}

// The booking window of the overdue leg (issue #776): planned operations
// whose overdue boundary — the wall clock 22:00 of the day after the
// operation date in the owner's timezone (#1168) — falls into
// (from, until], auto-pay rules included (the
// tick never backdates an auto charge, ADR 0049).
func TestPaymentScanStore_ListScheduledOverdueTargets(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	msk := createUserInZone(t, pool, "Europe/Moscow")
	prop := createLiveProperty(t, pool, msk)
	rule := insertScanPayment(t, pool, msk, prop, false)
	// The in-window occurrence: overdue boundary 2026-09-20T19:00Z —
	// Moscow's 22:00 of the 20th, the day after the operation date.
	insertScanOperation(t, pool, msk, prop, rule, "2026-09-19", "planned")

	// The auto-pay mode does not excuse the overdue leg's booking.
	autoProp := createLiveProperty(t, pool, msk)
	autoRule := insertScanPayment(t, pool, msk, autoProp, true)
	insertScanOperation(t, pool, msk, autoProp, autoRule, "2026-09-19", "planned")

	// A settled, a cancelled and an out-of-window occurrence stay out.
	insertScanOperation(t, pool, msk, prop, rule, "2026-09-20", "paid")
	insertScanOperation(t, pool, msk, prop, rule, "2026-09-21", "cancelled")
	insertScanOperation(t, pool, msk, prop, rule, "2026-09-10", "planned")

	// An archived property's occurrence stays out (the ticks' canon).
	archivedProp := createLiveProperty(t, pool, msk)
	archivedRule := insertScanPayment(t, pool, msk, archivedProp, false)
	insertScanOperation(t, pool, msk, archivedProp, archivedRule, "2026-09-19", "planned")
	archiveProperty(t, pool, msk, archivedProp)

	store := NewPaymentScanStore(pool)
	targets, err := store.ListScheduledOverdueTargets(ctx,
		time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 20, 20, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	got := scheduleTargetsOfProps(targets, rule, autoRule, archivedRule)
	require.Len(t, got, 2)
	fire := map[uuid.UUID]time.Time{}
	dates := map[uuid.UUID]time.Time{}
	for _, target := range got {
		fire[target.PaymentID] = target.FireAt
		dates[target.PaymentID] = target.DueDate
	}
	assert.True(t, fire[rule].Equal(time.Date(2026, 9, 20, 19, 0, 0, 0, time.UTC)),
		"the job wakes at the wall clock 22:00 of the day after the operation date in the owner's zone")
	assert.Equal(t, time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC), dates[rule])
	assert.True(t, fire[autoRule].Equal(time.Date(2026, 9, 20, 19, 0, 0, 0, time.UTC)))
}

// The due boundary job's delivery-time resolution (issue #776): a planned
// operation of a live manual rule on a live property whose date is still
// the zone's today answers live; paid, cancelled, auto-pay, orphaned
// (the rule row deleted), archived and rolled-over states answer no.
func TestPaymentScanStore_GetScheduledDuePayment(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	msk := createUserInZone(t, pool, "Europe/Moscow")
	prop := createLivePropertyFor(t, pool, msk, "Квартира на Ленина", "Москва, Тверская 1")
	rule := insertScanPayment(t, pool, msk, prop, false)
	insertScanOperation(t, pool, msk, prop, rule, "2026-09-20", "planned")

	// The dead siblings of the same operation date: paid, cancelled (their
	// own rules keep the (rule, date) uniqueness), an auto-pay rule's, an
	// archived property's and an orphaned one.
	paidProp := createLiveProperty(t, pool, msk)
	paidRule := insertScanPayment(t, pool, msk, paidProp, false)
	insertScanOperation(t, pool, msk, paidProp, paidRule, "2026-09-20", "paid")
	cancelProp := createLiveProperty(t, pool, msk)
	cancelRule := insertScanPayment(t, pool, msk, cancelProp, false)
	insertScanOperation(t, pool, msk, cancelProp, cancelRule, "2026-09-20", "cancelled")
	autoProp := createLiveProperty(t, pool, msk)
	autoRule := insertScanPayment(t, pool, msk, autoProp, true)
	insertScanOperation(t, pool, msk, autoProp, autoRule, "2026-09-20", "planned")
	archivedProp := createLiveProperty(t, pool, msk)
	archivedRule := insertScanPayment(t, pool, msk, archivedProp, false)
	insertScanOperation(t, pool, msk, archivedProp, archivedRule, "2026-09-23", "planned")
	archiveProperty(t, pool, msk, archivedProp)
	orphanRule := insertScanPayment(t, pool, msk, prop, false)
	insertScanOperation(t, pool, msk, prop, orphanRule, "2026-09-24", "planned")
	_, err := pool.Exec(ctx, `DELETE FROM payments WHERE id = $1`, orphanRule)
	require.NoError(t, err)

	store := NewPaymentScanStore(pool)
	// Moscow's midnight of the 20th has just passed.
	now := time.Date(2026, 9, 19, 21, 0, 30, 0, time.UTC)

	target, live, err := store.GetScheduledDuePayment(ctx, rule, time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), now)
	require.NoError(t, err)
	require.True(t, live)
	assert.Equal(t, rule, target.PaymentID)
	assert.Equal(t, time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), target.DueDate)
	assert.Equal(t, "Обслуживание", target.Title)
	assert.Equal(t, int64(250000), target.AmountKopecks)
	assert.Equal(t, prop, target.PropertyID)
	assert.Equal(t, "Квартира на Ленина", target.PropertyName)
	assert.Equal(t, "Москва, Тверская 1", target.PropertyAddress)
	assert.Equal(t, msk, target.OwnerID)

	// The dead states answer live=false — the job finishes silently.
	_, live, err = store.GetScheduledDuePayment(ctx, paidRule, time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), now)
	require.NoError(t, err)
	assert.False(t, live, "paid")

	_, live, err = store.GetScheduledDuePayment(ctx, cancelRule, time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), now)
	require.NoError(t, err)
	assert.False(t, live, "cancelled")

	_, live, err = store.GetScheduledDuePayment(ctx, autoRule, time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), now)
	require.NoError(t, err)
	assert.False(t, live, "auto-pay")

	_, live, err = store.GetScheduledDuePayment(ctx, archivedRule, time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC), now)
	require.NoError(t, err)
	assert.False(t, live, "archived property")

	_, live, err = store.GetScheduledDuePayment(ctx, orphanRule, time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC), now)
	require.NoError(t, err)
	assert.False(t, live, "orphaned operation")

	// The rollover check: once the zone's today has moved past the operation
	// date, the due leg is silent — the scan semantics say so.
	_, live, err = store.GetScheduledDuePayment(ctx, rule, time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 20, 21, 0, 30, 0, time.UTC))
	require.NoError(t, err)
	assert.False(t, live, "the job woke after the day rolled over")
}

// The overdue boundary job's delivery-time resolution (issue #776): a
// planned operation whose date is strictly before the zone's today answers
// live — auto-pay rules included; a job awake before the day's end (the
// date not yet overdue) answers no.
func TestPaymentScanStore_GetScheduledOverduePayment(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	msk := createUserInZone(t, pool, "Europe/Moscow")
	prop := createLiveProperty(t, pool, msk)
	rule := insertScanPayment(t, pool, msk, prop, false)
	insertScanOperation(t, pool, msk, prop, rule, "2026-09-19", "planned")

	autoProp := createLiveProperty(t, pool, msk)
	autoRule := insertScanPayment(t, pool, msk, autoProp, true)
	insertScanOperation(t, pool, msk, autoProp, autoRule, "2026-09-19", "planned")

	paidProp := createLiveProperty(t, pool, msk)
	paidRule := insertScanPayment(t, pool, msk, paidProp, false)
	insertScanOperation(t, pool, msk, paidProp, paidRule, "2026-09-19", "paid")

	store := NewPaymentScanStore(pool)
	// Moscow's midnight of the 20th has just passed: the 19th is overdue.
	now := time.Date(2026, 9, 19, 21, 0, 30, 0, time.UTC)

	target, live, err := store.GetScheduledOverduePayment(ctx, rule, time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC), now)
	require.NoError(t, err)
	require.True(t, live)
	assert.Equal(t, rule, target.PaymentID)
	assert.Equal(t, time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC), target.DueDate)
	assert.Equal(t, "Обслуживание", target.Title)

	_, live, err = store.GetScheduledOverduePayment(ctx, autoRule, time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC), now)
	require.NoError(t, err)
	assert.True(t, live, "auto-pay rules are the overdue leg's targets too")

	// Before the boundary — Moscow's midnight of the 20th has not arrived,
	// the 19th is still the zone's today — the leg is silent; the job could
	// not wake this early, the check keeps the leg's semantics honest.
	_, live, err = store.GetScheduledOverduePayment(ctx, rule, time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 19, 20, 59, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.False(t, live, "the operation is not overdue yet")

	_, live, err = store.GetScheduledOverduePayment(ctx, paidRule, time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC), now)
	require.NoError(t, err)
	assert.False(t, live, "paid")
}

// scheduleTargetsOfProps filters the booking window's targets to the given
// rules — the schedule target carries no property, the rule id is its
// identity; the shared test database's parallel fixtures must not break the
// count assertions.
func scheduleTargetsOfProps(targets []application.PaymentScheduleTarget, rules ...uuid.UUID) []application.PaymentScheduleTarget {
	want := make(map[uuid.UUID]bool, len(rules))
	for _, r := range rules {
		want[r] = true
	}
	out := make([]application.PaymentScheduleTarget, 0, len(targets))
	for _, target := range targets {
		if want[target.PaymentID] {
			out = append(out, target)
		}
	}
	return out
}

// Нога напоминания (карта #822, #824): planned-вхождения правил с заданным
// напоминанием, чей «день напоминания» (дата операции − оффсет) — ровно
// «сегодня» пояса. Автоплатёжные включены (решение #823); без напоминания,
// гашеные, чужой пояс и архив — мимо.
func TestPaymentScanStore_ListReminderTargets(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	msk := createUserInZone(t, pool, "Europe/Moscow")
	prop := createLiveProperty(t, pool, msk)
	rule := insertScanReminderPayment(t, pool, msk, prop, false, 3)
	insertScanOperation(t, pool, msk, prop, rule, "2026-09-22", "planned") // Reminder day = 2026-09-19.

	// Автоплатёжное правило с напоминанием входит (решение #823).
	autoProp := createLiveProperty(t, pool, msk)
	autoRule := insertScanReminderPayment(t, pool, msk, autoProp, true, 7)
	insertScanOperation(t, pool, msk, autoProp, autoRule, "2026-09-26", "planned") // Reminder day = 2026-09-19.

	// Мимо: другой оффсет (день напоминания не сегодня), гашеное вхождение,
	// правило без напоминания.
	offProp := createLiveProperty(t, pool, msk)
	offRule := insertScanReminderPayment(t, pool, msk, offProp, false, 7)
	insertScanOperation(t, pool, msk, offProp, offRule, "2026-09-22", "planned") // Reminder day = 2026-09-15.
	paidProp := createLiveProperty(t, pool, msk)
	paidRule := insertScanReminderPayment(t, pool, msk, paidProp, false, 3)
	insertScanOperation(t, pool, msk, paidProp, paidRule, "2026-09-22", "paid")
	noneProp := createLiveProperty(t, pool, msk)
	noneRule := insertScanPayment(t, pool, msk, noneProp, false)
	insertScanOperation(t, pool, msk, noneProp, noneRule, "2026-09-22", "planned")

	// Чужой пояс и архив — мимо (канон тиков).
	kgd := createUserInZone(t, pool, "Europe/Kaliningrad")
	kgdProp := createLiveProperty(t, pool, kgd)
	kgdRule := insertScanReminderPayment(t, pool, kgd, kgdProp, false, 3)
	insertScanOperation(t, pool, kgd, kgdProp, kgdRule, "2026-09-22", "planned")
	archivedProp := createLiveProperty(t, pool, msk)
	archivedRule := insertScanReminderPayment(t, pool, msk, archivedProp, false, 3)
	insertScanOperation(t, pool, msk, archivedProp, archivedRule, "2026-09-22", "planned")
	archiveProperty(t, pool, msk, archivedProp)

	store := NewPaymentScanStore(pool)
	// The instant gate (#1168): до 10:00 дня напоминания нога молчит.
	early := time.Date(2026, 9, 19, 5, 0, 0, 0, time.UTC)
	targets, err := store.ListReminderTargets(ctx, "Europe/Moscow", time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC), early)
	require.NoError(t, err)
	got := paymentTargetsOfProps(targets, prop, autoProp, offProp, paidProp, noneProp, archivedProp)
	require.Empty(t, got, "the sweep must not publish ahead of the 10:00 boundary")

	// На границе — 10:00 Москвы — нога говорит.
	now := time.Date(2026, 9, 19, 7, 0, 0, 0, time.UTC)
	targets, err = store.ListReminderTargets(ctx, "Europe/Moscow", time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC), now)
	require.NoError(t, err)
	got = paymentTargetsOfProps(targets, prop, autoProp, offProp, paidProp, noneProp, archivedProp)
	require.Len(t, got, 2)

	dates := map[uuid.UUID]time.Time{}
	for _, target := range got {
		dates[target.PaymentID] = target.DueDate
	}
	assert.Equal(t, time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC), dates[rule])
	assert.Equal(t, time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC), dates[autoRule])
}

// Букинг ноги напоминания: граница — 10:00 «дата − оффсет» по поясу
// (Московские 10:00 19-го = 2026-09-19T07:00Z, #1168), окно (from, until].
func TestPaymentScanStore_ListScheduledReminderTargets(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	msk := createUserInZone(t, pool, "Europe/Moscow")
	prop := createLiveProperty(t, pool, msk)
	rule := insertScanReminderPayment(t, pool, msk, prop, false, 3)
	insertScanOperation(t, pool, msk, prop, rule, "2026-09-22", "planned") // Boundary 2026-09-19T07:00Z.

	// Автоплатёжное — включено; другой оффсет — граница вне окна; без
	// напоминания — мимо.
	autoProp := createLiveProperty(t, pool, msk)
	autoRule := insertScanReminderPayment(t, pool, msk, autoProp, true, 1)
	insertScanOperation(t, pool, msk, autoProp, autoRule, "2026-09-21", "planned") // Boundary 2026-09-20T07:00Z.
	offProp := createLiveProperty(t, pool, msk)
	offRule := insertScanReminderPayment(t, pool, msk, offProp, false, 7)
	insertScanOperation(t, pool, msk, offProp, offRule, "2026-09-22", "planned") // Boundary 2026-09-15T07:00Z.
	noneProp := createLiveProperty(t, pool, msk)
	noneRule := insertScanPayment(t, pool, msk, noneProp, false)
	insertScanOperation(t, pool, msk, noneProp, noneRule, "2026-09-22", "planned")

	store := NewPaymentScanStore(pool)
	targets, err := store.ListScheduledReminderTargets(ctx,
		time.Date(2026, 9, 18, 20, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 20, 8, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	got := scheduleTargetsOfProps(targets, rule, autoRule, offRule, noneRule)
	require.Len(t, got, 2)
	fire := map[uuid.UUID]time.Time{}
	dates := map[uuid.UUID]time.Time{}
	for _, target := range got {
		fire[target.PaymentID] = target.FireAt
		dates[target.PaymentID] = target.DueDate
	}
	assert.True(t, fire[rule].Equal(time.Date(2026, 9, 19, 7, 0, 0, 0, time.UTC)),
		"the job wakes at the wall clock 10:00 of (operation date − 3) in the owner's zone")
	assert.Equal(t, time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC), dates[rule])
	assert.True(t, fire[autoRule].Equal(time.Date(2026, 9, 20, 7, 0, 0, 0, time.UTC)))
}

// Разрешение в момент доставки (карта #822): planned-вхождение правила с
// напоминанием, чей ТЕКУЩИЙ день напоминания — «сегодня» пояса, отвечает
// live; смена оффсета после букинга, гашеное, архив и ролловер — нет.
func TestPaymentScanStore_GetScheduledReminderPayment(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	msk := createUserInZone(t, pool, "Europe/Moscow")
	prop := createLiveProperty(t, pool, msk)
	rule := insertScanReminderPayment(t, pool, msk, prop, false, 3)
	insertScanOperation(t, pool, msk, prop, rule, "2026-09-22", "planned")

	// Мёртвые состояния: гашеное вхождение; правило, чей оффсет сменён после
	// букинга (3 → 7: день напоминания уехал на 15-е); архив.
	paidProp := createLiveProperty(t, pool, msk)
	paidRule := insertScanReminderPayment(t, pool, msk, paidProp, false, 3)
	insertScanOperation(t, pool, msk, paidProp, paidRule, "2026-09-22", "paid")
	movedProp := createLiveProperty(t, pool, msk)
	movedRule := insertScanReminderPayment(t, pool, msk, movedProp, false, 3)
	insertScanOperation(t, pool, msk, movedProp, movedRule, "2026-09-22", "planned")
	_, err := pool.Exec(ctx, `UPDATE payments SET reminder_offset_days = 7 WHERE id = $1`, movedRule)
	require.NoError(t, err)
	archivedProp := createLiveProperty(t, pool, msk)
	archivedRule := insertScanReminderPayment(t, pool, msk, archivedProp, false, 3)
	insertScanOperation(t, pool, msk, archivedProp, archivedRule, "2026-09-22", "planned")
	archiveProperty(t, pool, msk, archivedProp)

	store := NewPaymentScanStore(pool)
	// Московская полночь 19-го только что пробила.
	now := time.Date(2026, 9, 18, 21, 0, 30, 0, time.UTC)
	opDate := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)

	target, live, err := store.GetScheduledReminderPayment(ctx, rule, opDate, now)
	require.NoError(t, err)
	require.True(t, live)
	assert.Equal(t, rule, target.PaymentID)
	assert.Equal(t, opDate, target.DueDate)
	assert.Equal(t, "Обслуживание", target.Title)
	assert.Equal(t, int64(250000), target.AmountKopecks)
	assert.Equal(t, prop, target.PropertyID)
	assert.Equal(t, msk, target.OwnerID)

	_, live, err = store.GetScheduledReminderPayment(ctx, paidRule, opDate, now)
	require.NoError(t, err)
	assert.False(t, live, "paid")

	_, live, err = store.GetScheduledReminderPayment(ctx, movedRule, opDate, now)
	require.NoError(t, err)
	assert.False(t, live, "the lead time changed after the booking")

	_, live, err = store.GetScheduledReminderPayment(ctx, archivedRule, opDate, now)
	require.NoError(t, err)
	assert.False(t, live, "archived property")

	// Ролловер: сегодня зоны уже не день напоминания.
	_, live, err = store.GetScheduledReminderPayment(ctx, rule, opDate,
		time.Date(2026, 9, 19, 21, 0, 30, 0, time.UTC))
	require.NoError(t, err)
	assert.False(t, live, "the job woke after the reminder day rolled over")
}

// stampPaid sets the rule's occurrence's payment fact with its source
// (#1169): 'auto_pay' — the state PayOperationDueToday leaves, 'manual' —
// «Оплатить сейчас».
func stampPaid(t *testing.T, pool *pgxpool.Pool, ruleID uuid.UUID, date, source string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		UPDATE operations SET status = 'paid', paid_date = $2, paid_source = $3
		WHERE payment_id = $1 AND date = $2`, ruleID, date, source)
	require.NoError(t, err)
}

// Нога «Автоплатёж исполнен» (#1169, решение владельца по гриллингу #1167):
// тиковое гашение своего дня — цель ноги; ручная оплата того же вхождения,
// неисполненное planned, не-автоплатёжное правило, чужой день (бэкфилла
// нет), до 10:00 и архив — мимо.
func TestPaymentScanStore_ListAutoPaidTargets(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	msk := createUserInZone(t, pool, "Europe/Moscow")
	prop := createLiveProperty(t, pool, msk)
	rule := insertScanPayment(t, pool, msk, prop, true)
	insertScanOperation(t, pool, msk, prop, rule, "2026-09-19", "planned")
	stampPaid(t, pool, rule, "2026-09-19", "auto_pay")

	// Ручная оплата того же дня — молчок (штампа нет).
	manualProp := createLiveProperty(t, pool, msk)
	manualRule := insertScanPayment(t, pool, msk, manualProp, true)
	insertScanOperation(t, pool, msk, manualProp, manualRule, "2026-09-19", "planned")
	stampPaid(t, pool, manualRule, "2026-09-19", "manual")

	// Неисполненное planned (тик ещё не дошёл) и не-автоплатёжное правило —
	// мимо; вчерашнее тиковое гашение — мимо (живой день, без бэкфилла).
	pendingProp := createLiveProperty(t, pool, msk)
	pendingRule := insertScanPayment(t, pool, msk, pendingProp, true)
	insertScanOperation(t, pool, msk, pendingProp, pendingRule, "2026-09-19", "planned")
	manualRuleProp := createLiveProperty(t, pool, msk)
	manualOnlyRule := insertScanPayment(t, pool, msk, manualRuleProp, false)
	insertScanOperation(t, pool, msk, manualRuleProp, manualOnlyRule, "2026-09-19", "planned")
	stampPaid(t, pool, manualOnlyRule, "2026-09-19", "auto_pay") // Невозможное состояние: non-auto rule — фильтр auto_pay гонит.
	oldProp := createLiveProperty(t, pool, msk)
	oldRule := insertScanPayment(t, pool, msk, oldProp, true)
	insertScanOperation(t, pool, msk, oldProp, oldRule, "2026-09-18", "planned")
	stampPaid(t, pool, oldRule, "2026-09-18", "auto_pay")

	// Молчащее правило (#1189): тиковый штамп своего дня есть, флаг «Не
	// уведомлять» — нога молчит.
	silentProp := createLiveProperty(t, pool, msk)
	silentRule := insertScanSilentAutoPaidPayment(t, pool, msk, silentProp)
	insertScanOperation(t, pool, msk, silentProp, silentRule, "2026-09-19", "planned")
	stampPaid(t, pool, silentRule, "2026-09-19", "auto_pay")

	// Архив — мимо (канон тиков).
	archivedProp := createLiveProperty(t, pool, msk)
	archivedRule := insertScanPayment(t, pool, msk, archivedProp, true)
	insertScanOperation(t, pool, msk, archivedProp, archivedRule, "2026-09-19", "planned")
	stampPaid(t, pool, archivedRule, "2026-09-19", "auto_pay")
	archiveProperty(t, pool, msk, archivedProp)

	store := NewPaymentScanStore(pool)
	today := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)

	// Instant-гейт (#1168): до 10:00 Москвы нога молчит.
	early := time.Date(2026, 9, 19, 5, 0, 0, 0, time.UTC)
	targets, err := store.ListAutoPaidTargets(ctx, "Europe/Moscow", today, early)
	require.NoError(t, err)
	assert.Empty(t, paymentTargetsOfProps(targets, prop, manualProp, pendingProp, oldProp, archivedProp, silentProp))

	// На границе — 10:00 Москвы — только тиковое гашение правила автоплатежа;
	// молчащее правило (#1189) и с штампом мимо.
	now := time.Date(2026, 9, 19, 7, 0, 0, 0, time.UTC)
	targets, err = store.ListAutoPaidTargets(ctx, "Europe/Moscow", today, now)
	require.NoError(t, err)
	got := paymentTargetsOfProps(targets, prop, manualProp, pendingProp, manualRuleProp, oldProp, archivedProp, silentProp)
	require.Len(t, got, 1)
	assert.Equal(t, rule, got[0].PaymentID)
	assert.Equal(t, today, got[0].DueDate)
	assert.Equal(t, "Обслуживание", got[0].Title)
}

// Букинг ноги автоплатежа (#1169): planned-вхождения auto_pay-правил,
// граница — 10:00 дня операции по поясу (#1168); не-автоплатёжные, гашеные
// и вне окна — мимо.
func TestPaymentScanStore_ListScheduledAutoPaidTargets(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	msk := createUserInZone(t, pool, "Europe/Moscow")
	prop := createLiveProperty(t, pool, msk)
	rule := insertScanPayment(t, pool, msk, prop, true)
	// The in-window occurrence: boundary 2026-09-21T07:00Z (10:00 MSK).
	insertScanOperation(t, pool, msk, prop, rule, "2026-09-21", "planned")

	// Не-автоплатёжное правило, молчащее (#1189) и вне окна — мимо.
	manualProp := createLiveProperty(t, pool, msk)
	manualRule := insertScanPayment(t, pool, msk, manualProp, false)
	insertScanOperation(t, pool, msk, manualProp, manualRule, "2026-09-21", "planned")
	silentProp := createLiveProperty(t, pool, msk)
	silentRule := insertScanSilentAutoPaidPayment(t, pool, msk, silentProp)
	insertScanOperation(t, pool, msk, silentProp, silentRule, "2026-09-21", "planned")
	insertScanOperation(t, pool, msk, prop, rule, "2026-10-21", "planned")

	store := NewPaymentScanStore(pool)
	targets, err := store.ListScheduledAutoPaidTargets(ctx,
		time.Date(2026, 9, 20, 6, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 21, 8, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	got := scheduleTargetsOfProps(targets, rule, manualRule, silentRule)
	require.Len(t, got, 1)
	assert.Equal(t, rule, got[0].PaymentID)
	assert.Equal(t, time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC), got[0].DueDate)
	assert.True(t, got[0].FireAt.Equal(time.Date(2026, 9, 21, 7, 0, 0, 0, time.UTC)),
		"the job wakes at the wall clock 10:00 of the operation date in the owner's zone")
}

// Разрешение в момент доставки (#1169, решение владельца по гриллингу
// #1167): жив ответ только тикового гашения своего дня; ручная оплата,
// неисполненное planned и ролловер — молчок.
func TestPaymentScanStore_GetScheduledAutoPaidPayment(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	msk := createUserInZone(t, pool, "Europe/Moscow")
	prop := createLiveProperty(t, pool, msk)
	rule := insertScanPayment(t, pool, msk, prop, true)
	insertScanOperation(t, pool, msk, prop, rule, "2026-09-20", "planned")
	stampPaid(t, pool, rule, "2026-09-20", "auto_pay")

	manualProp := createLiveProperty(t, pool, msk)
	manualRule := insertScanPayment(t, pool, msk, manualProp, true)
	insertScanOperation(t, pool, msk, manualProp, manualRule, "2026-09-20", "planned")
	stampPaid(t, pool, manualRule, "2026-09-20", "manual")

	pendingProp := createLiveProperty(t, pool, msk)
	pendingRule := insertScanPayment(t, pool, msk, pendingProp, true)
	insertScanOperation(t, pool, msk, pendingProp, pendingRule, "2026-09-20", "planned")

	// Молчащее правило (#1189): штамп тика своего дня есть, флаг «Не
	// уведомлять» — разрешение отвечает не-живым даже с бронью.
	silentProp := createLiveProperty(t, pool, msk)
	silentRule := insertScanSilentAutoPaidPayment(t, pool, msk, silentProp)
	insertScanOperation(t, pool, msk, silentProp, silentRule, "2026-09-20", "planned")
	stampPaid(t, pool, silentRule, "2026-09-20", "auto_pay")

	store := NewPaymentScanStore(pool)
	// Пробуждение 10:00:30 Москвы своего дня.
	now := time.Date(2026, 9, 20, 7, 0, 30, 0, time.UTC)
	opDate := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)

	target, live, err := store.GetScheduledAutoPaidPayment(ctx, rule, opDate, now)
	require.NoError(t, err)
	require.True(t, live, "the tick's own-day execution is the live case")
	assert.Equal(t, rule, target.PaymentID)
	assert.Equal(t, opDate, target.DueDate)
	assert.Equal(t, "Обслуживание", target.Title)
	assert.Equal(t, int64(250000), target.AmountKopecks)
	assert.Equal(t, prop, target.PropertyID)
	assert.Equal(t, msk, target.OwnerID)

	_, live, err = store.GetScheduledAutoPaidPayment(ctx, manualRule, opDate, now)
	require.NoError(t, err)
	assert.False(t, live, "a manual payment is silent — the owner's decision")

	_, live, err = store.GetScheduledAutoPaidPayment(ctx, pendingRule, opDate, now)
	require.NoError(t, err)
	assert.False(t, live, "the tick has not run yet")

	_, live, err = store.GetScheduledAutoPaidPayment(ctx, silentRule, opDate, now)
	require.NoError(t, err)
	assert.False(t, live, "the «Не уведомлять» flag keeps the stamped rule silent (#1189)")

	// Ролловер: джоба проснулась на следующий день — живой день прошёл,
	// бэкфилла нет (решение владельца, #1167).
	_, live, err = store.GetScheduledAutoPaidPayment(ctx, rule, opDate,
		time.Date(2026, 9, 21, 7, 0, 30, 0, time.UTC))
	require.NoError(t, err)
	assert.False(t, live, "the job woke after the live day rolled over — no backfill")
}
