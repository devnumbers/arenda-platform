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
