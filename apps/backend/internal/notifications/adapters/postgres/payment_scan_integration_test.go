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
func insertScanPayment(t *testing.T, pool *pgxpool.Pool, ownerID, propertyID uuid.UUID, autoPay bool) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(context.Background(), `
		INSERT INTO payments (id, owner_id, property_id, type, title, amount_kopecks, recurrence, since, auto_pay, payment_form, category_slug)
		VALUES ($1, $2, $3, 'expense', 'Обслуживание', 250000, '{"kind": "monthly"}', '2026-08-01', $4, 'transfer', 'maintenance')`,
		id, ownerID, propertyID, autoPay)
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
			since, auto_pay, reminder_offset_days, payment_form, category_slug)
		VALUES ($1, $2, $3, 'expense', 'Обслуживание', 250000, '{"kind": "monthly"}',
			'2026-08-01', $4, $5, 'transfer', 'maintenance')`,
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
	targets, err := store.ListDueTargets(ctx, "Europe/Moscow", time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	// The shared test database's parallel fixtures' rows may appear in the
	// same zone's sweep — filter to the properties this test created.
	got := paymentTargetsOfProps(targets, prop, autoProp, archivedProp)
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

	store := NewPaymentScanStore(pool)
	targets, err := store.ListOverdueTargets(ctx, "Europe/Moscow", time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	// Filter to the properties this test created (shared test database).
	got := paymentTargetsOfProps(targets, prop, autoProp, archivedProp)
	require.Len(t, got, 2)

	dates := map[uuid.UUID]time.Time{}
	for _, target := range got {
		dates[target.PaymentID] = target.DueDate
	}
	assert.Equal(t, time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC), dates[rule])
	assert.Equal(t, time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC), dates[autoRule])
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
// due boundary — 00:00 of the operation date in the owner's timezone —
// falls into (from, until], auto-pay rules excluded. Moscow's midnight of
// the 20th is 2026-09-19T21:00Z; the boundary carries the zone's shift.
func TestPaymentScanStore_ListScheduledDueTargets(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	msk := createUserInZone(t, pool, "Europe/Moscow")
	prop := createLiveProperty(t, pool, msk)
	rule := insertScanPayment(t, pool, msk, prop, false)
	// The in-window occurrence: due boundary 2026-09-19T21:00Z.
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
		time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	got := scheduleTargetsOfProps(targets, rule, autoRule, archivedRule)
	require.Len(t, got, 1)
	assert.Equal(t, rule, got[0].PaymentID)
	assert.Equal(t, time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), got[0].DueDate)
	assert.True(t, got[0].FireAt.Equal(time.Date(2026, 9, 19, 21, 0, 0, 0, time.UTC)),
		"the job wakes at 00:00 of the operation date in the owner's zone")
}

// The booking window of the overdue leg (issue #776): planned operations
// whose overdue boundary — 00:00 of the day after the operation date in the
// owner's timezone — falls into (from, until], auto-pay rules included (the
// tick never backdates an auto charge, ADR 0049).
func TestPaymentScanStore_ListScheduledOverdueTargets(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	msk := createUserInZone(t, pool, "Europe/Moscow")
	prop := createLiveProperty(t, pool, msk)
	rule := insertScanPayment(t, pool, msk, prop, false)
	// The in-window occurrence: overdue boundary 2026-09-19T21:00Z —
	// Moscow's midnight of the 20th, the day after the operation date.
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
		time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	got := scheduleTargetsOfProps(targets, rule, autoRule, archivedRule)
	require.Len(t, got, 2)
	fire := map[uuid.UUID]time.Time{}
	dates := map[uuid.UUID]time.Time{}
	for _, target := range got {
		fire[target.PaymentID] = target.FireAt
		dates[target.PaymentID] = target.DueDate
	}
	assert.True(t, fire[rule].Equal(time.Date(2026, 9, 19, 21, 0, 0, 0, time.UTC)),
		"the job wakes at 00:00 of the day after the operation date in the owner's zone")
	assert.Equal(t, time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC), dates[rule])
	assert.True(t, fire[autoRule].Equal(time.Date(2026, 9, 19, 21, 0, 0, 0, time.UTC)))
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
	targets, err := store.ListReminderTargets(ctx, "Europe/Moscow", time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	got := paymentTargetsOfProps(targets, prop, autoProp, offProp, paidProp, noneProp, archivedProp)
	require.Len(t, got, 2)

	dates := map[uuid.UUID]time.Time{}
	for _, target := range got {
		dates[target.PaymentID] = target.DueDate
	}
	assert.Equal(t, time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC), dates[rule])
	assert.Equal(t, time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC), dates[autoRule])
}

// Букинг ноги напоминания: граница — полночь «дата − оффсет» по поясу
// (Московская полночь 19-го = 2026-09-18T21:00Z), окно (from, until].
func TestPaymentScanStore_ListScheduledReminderTargets(t *testing.T) {
	t.Parallel()

	pool := setupPushDB(t)
	ctx := context.Background()

	msk := createUserInZone(t, pool, "Europe/Moscow")
	prop := createLiveProperty(t, pool, msk)
	rule := insertScanReminderPayment(t, pool, msk, prop, false, 3)
	insertScanOperation(t, pool, msk, prop, rule, "2026-09-22", "planned") // Boundary 2026-09-18T21:00Z.

	// Автоплатёжное — включено; другой оффсет — граница вне окна; без
	// напоминания — мимо.
	autoProp := createLiveProperty(t, pool, msk)
	autoRule := insertScanReminderPayment(t, pool, msk, autoProp, true, 1)
	insertScanOperation(t, pool, msk, autoProp, autoRule, "2026-09-21", "planned") // Boundary 2026-09-19T21:00Z.
	offProp := createLiveProperty(t, pool, msk)
	offRule := insertScanReminderPayment(t, pool, msk, offProp, false, 7)
	insertScanOperation(t, pool, msk, offProp, offRule, "2026-09-22", "planned") // Boundary 2026-09-15T21:00Z.
	noneProp := createLiveProperty(t, pool, msk)
	noneRule := insertScanPayment(t, pool, msk, noneProp, false)
	insertScanOperation(t, pool, msk, noneProp, noneRule, "2026-09-22", "planned")

	store := NewPaymentScanStore(pool)
	targets, err := store.ListScheduledReminderTargets(ctx,
		time.Date(2026, 9, 18, 20, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 19, 21, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	got := scheduleTargetsOfProps(targets, rule, autoRule, offRule, noneRule)
	require.Len(t, got, 2)
	fire := map[uuid.UUID]time.Time{}
	dates := map[uuid.UUID]time.Time{}
	for _, target := range got {
		fire[target.PaymentID] = target.FireAt
		dates[target.PaymentID] = target.DueDate
	}
	assert.True(t, fire[rule].Equal(time.Date(2026, 9, 18, 21, 0, 0, 0, time.UTC)),
		"the job wakes at 00:00 of (operation date − 3) in the owner's zone")
	assert.Equal(t, time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC), dates[rule])
	assert.True(t, fire[autoRule].Equal(time.Date(2026, 9, 19, 21, 0, 0, 0, time.UTC)))
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
