package domain

import (
	"maps"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// applyPlan simulates the plan execution over a date→status map the way the
// application layer applies it to the database: inserts (dedup by date), the
// auto-pay day payment (strictly today), and the future planned rebuild.
func applyPlan(today time.Time, plan PaymentTickPlan, byDate map[time.Time]OperationStatus) {
	for _, date := range plan.Materialize {
		if _, ok := byDate[date]; !ok {
			byDate[date] = StatusPlanned
		}
	}
	if plan.InsertFuture != nil {
		if _, ok := byDate[*plan.InsertFuture]; !ok {
			byDate[*plan.InsertFuture] = StatusPlanned
		}
	}
	if plan.AutoPayToday && byDate[today] == StatusPlanned {
		byDate[today] = StatusPaid
	}
	for date, status := range byDate {
		if status != StatusPlanned || !date.After(today) {
			continue
		}
		if plan.KeepFuture == nil || !date.Equal(*plan.KeepFuture) {
			delete(byDate, date)
		}
	}
}

func tickOnce(t *testing.T, p Payment, today string, byDate map[time.Time]OperationStatus) {
	t.Helper()
	plan := PlanPaymentTick(p, d(today), byDate)
	applyPlan(d(today), plan, byDate)
}

func dailyRule(t *testing.T, since string, autoPay bool, pauses ...PauseInterval) Payment {
	t.Helper()
	p := rule(t, NewDailyRecurrence(), since, nil, pauses...)
	p.AutoPay = autoPay
	return p
}

func TestPlanPaymentTick_MaterializesDueAndSingleFuture(t *testing.T) {
	t.Parallel()
	// Prototype smoke "догон по старому since": daily, created T-3, manual —
	// 4 due occurrences (3 overdue + today) + exactly one future planned.
	p := dailyRule(t, dayT3, false)
	plan := PlanPaymentTick(p, d(dayT0), map[time.Time]OperationStatus{})
	assert.Equal(t, []string{dayT3, dayT2, dayT1, dayT0}, dates(plan.Materialize))
	assert.False(t, plan.AutoPayToday)
	require.NotNil(t, plan.KeepFuture)
	assert.Equal(t, dayNext, plan.KeepFuture.Format(time.DateOnly))
	require.NotNil(t, plan.InsertFuture)
	assert.Equal(t, dayNext, plan.InsertFuture.Format(time.DateOnly))
}

func TestPlanPaymentTick_IdempotentRerunIsNoop(t *testing.T) {
	t.Parallel()
	p := dailyRule(t, dayT3, false)
	byDate := map[time.Time]OperationStatus{}
	tickOnce(t, p, dayT0, byDate)

	first := make(map[time.Time]OperationStatus, len(byDate))
	maps.Copy(first, byDate)
	tickOnce(t, p, dayT0, byDate)
	assert.Equal(t, first, byDate, "second run at the same today must change nothing")
}

func TestPlanPaymentTick_AutoPayClosesOnlyToday(t *testing.T) {
	t.Parallel()
	// ADR 0049 §2 revision: no backdated catch-up. Daily auto-pay, created
	// T-2: T-2 and T-1 stay planned (overdue debt), today closes as paid,
	// tomorrow stands as the single future planned.
	p := dailyRule(t, dayT2, true)

	byDate := map[time.Time]OperationStatus{}
	plan := PlanPaymentTick(p, d(dayT0), byDate)
	assert.True(t, plan.AutoPayToday)
	assert.Equal(t, []string{dayT2, dayT1, dayT0}, dates(plan.Materialize))

	applyPlan(d(dayT0), plan, byDate)
	assert.Equal(t, StatusPlanned, byDate[d(dayT2)], "the creation day stays planned — debt, closed manually only")
	assert.Equal(t, StatusPlanned, byDate[d(dayT1)], "yesterday stays planned — debt")
	assert.Equal(t, StatusPaid, byDate[d(dayT0)], "today is auto-paid")
	assert.Equal(t, StatusPlanned, byDate[d(dayNext)], "exactly one future planned")
}

func TestPlanPaymentTick_WorkerMissedTheDayLeavesDebt(t *testing.T) {
	t.Parallel()
	// The tick did not run on the occurrence's day: the next run must not pay
	// it retroactively even for an auto-pay (ADR 0049 §2 consequence).
	p := dailyRule(t, "2026-08-20", true)
	today := d(dayT0)
	byDate := map[time.Time]OperationStatus{}
	plan := PlanPaymentTick(p, today, byDate)
	require.True(t, plan.AutoPayToday)
	applyPlan(today, plan, byDate)

	assert.Equal(t, StatusPlanned, byDate[d(dayT1)], "the missed day remains debt")
	assert.Equal(t, StatusPaid, byDate[today], "only the current day closes")
}

func TestPlanPaymentTick_ActivePauseStopsEverything(t *testing.T) {
	t.Parallel()
	// Prototype smoke "пауза: ручной платёж": daily, created T-3, paused on
	// T: the future planned is removed, due occurrences before the pause stay.
	p := dailyRule(t, dayT3, false, PauseInterval{From: d(dayT0), To: nil})
	byDate := map[time.Time]OperationStatus{}
	tickOnce(t, p, dayT0, byDate)
	assert.Equal(t, map[string]OperationStatus{
		dayT3: StatusPlanned,
		dayT2: StatusPlanned,
		dayT1: StatusPlanned,
	}, stringStatuses(byDate), "nothing on or after the pause day; no future planned")

	// Days advance inside the pause: still nothing new, no debt accrues
	// (prototype smoke pm2).
	tickOnce(t, p, "2026-08-30", byDate)
	assert.Len(t, byDate, 3)

	// After the resume (interval closed with the resume day), occurrences
	// resume from that day without the holes growing back (smoke pm3).
	resumed := dailyRule(t, dayT3, false, PauseInterval{From: d(dayT0), To: dp("2026-08-31")})
	tickOnce(t, resumed, "2026-08-31", byDate)
	assert.Equal(t, StatusPlanned, byDate[d("2026-08-31")], "the resume day is an occurrence again")
	_, hasHole := byDate[d(dayNext)]
	assert.False(t, hasHole, "pause holes never grow back")
	assert.Equal(t, StatusPlanned, byDate[d("2026-09-01")], "the next future planned after resume")
}

func TestPlanPaymentTick_ActivePauseStopsAutoPayDayPayment(t *testing.T) {
	t.Parallel()
	// Prototype smoke "пауза: автоплатёж": in a pause the auto-pay neither
	// creates nor pays (ADR 0002).
	p := dailyRule(t, dayT2, true, PauseInterval{From: d(dayT0), To: nil})

	plan := PlanPaymentTick(p, d(dayT0), map[time.Time]OperationStatus{})
	assert.False(t, plan.AutoPayToday, "in a pause the auto-pay neither creates nor pays")
	assert.Nil(t, plan.KeepFuture)
	assert.Equal(t, []string{dayT2, dayT1}, dates(plan.Materialize),
		"due occurrences before the pause still materialize")
}

func TestPlanPaymentTick_PauseStartedTodayAfterFuturePlannedStanding(t *testing.T) {
	t.Parallel()
	// Yesterday's tick already stood the future planned on today; today the
	// rule is paused. The standing occurrence is due (not future) — it stays
	// as debt; the pause only stops auto-pay and new generation.
	p := dailyRule(t, dayT2, true, PauseInterval{From: d(dayT0), To: nil})
	byDate := map[time.Time]OperationStatus{d(dayT0): StatusPlanned}

	plan := PlanPaymentTick(p, d(dayT0), byDate)
	assert.False(t, plan.AutoPayToday, "active pause stops the day payment")
	assert.Nil(t, plan.KeepFuture)
	assert.Equal(t, []string{dayT2, dayT1}, dates(plan.Materialize),
		"due occurrences before the pause still materialize")
	assert.Equal(t, StatusPlanned, byDate[d(dayT0)], "the standing occurrence stays as debt")
}

func TestPlanPaymentTick_RebuildsSingleFuturePlanned(t *testing.T) {
	t.Parallel()
	// A stale future planned row (left by a rule edit) is removed; the next
	// occurrence keeps exactly one future planned.
	rec := mustMonthly(t, 10)
	p := rule(t, rec, "2026-01-10", nil)
	byDate := map[time.Time]OperationStatus{
		d("2026-09-10"): StatusPlanned, // The correct next.
		d("2026-10-10"): StatusPlanned, // Stale second future row.
	}
	tickOnce(t, p, dayT0, byDate)
	assert.Equal(t, StatusPlanned, byDate[d("2026-09-10")])
	_, stale := byDate[d("2026-10-10")]
	assert.False(t, stale, "only one future planned may remain")
}

func TestPlanPaymentTick_PaidAheadDoesNotShiftSchedule(t *testing.T) {
	t.Parallel()
	// "Оплатить сейчас" closed the next occurrence early: the following one
	// becomes the new single future planned; the paid fact is untouched.
	rec := mustMonthly(t, 10)
	p := rule(t, rec, "2026-01-10", nil)
	byDate := map[time.Time]OperationStatus{d("2026-09-10"): StatusPaid}
	tickOnce(t, p, dayT0, byDate)
	assert.Equal(t, StatusPaid, byDate[d("2026-09-10")], "paid facts are untouchable")
	assert.Equal(t, StatusPlanned, byDate[d("2026-10-10")], "the occurrence after the paid one is next")
	_, extra := byDate[d("2026-11-10")]
	assert.False(t, extra)
}

func TestPlanPaymentTick_EndedRuleHasNoFuturePlanned(t *testing.T) {
	t.Parallel()
	// Prototype smoke "окончание в прошлом": occurrences materialized, no
	// future planned beyond end_date.
	end := dayT1
	p := dailyRule(t, "2026-08-20", false)
	p.EndDate = dp(end)

	byDate := map[time.Time]OperationStatus{}
	plan := PlanPaymentTick(p, d(dayT0), byDate)
	assert.Equal(t, []string{"2026-08-20", "2026-08-21", dayT3, dayT2, dayT1},
		dates(plan.Materialize))
	assert.Nil(t, plan.KeepFuture)
	assert.Nil(t, plan.InsertFuture)
}

func TestPlanPaymentTick_PaidOperationsNeverTouched(t *testing.T) {
	t.Parallel()
	p := dailyRule(t, dayT2, false)
	byDate := map[time.Time]OperationStatus{
		d(dayT2): StatusPaid,
		d(dayT1): StatusPaid,
	}
	plan := PlanPaymentTick(p, d(dayT0), byDate)
	assert.Equal(t, []string{dayT0}, dates(plan.Materialize), "only today is missing")
	applyPlan(d(dayT0), plan, byDate)
	assert.Equal(t, StatusPaid, byDate[d(dayT2)])
	assert.Equal(t, StatusPaid, byDate[d(dayT1)])
	assert.Equal(t, StatusPlanned, byDate[d(dayT0)])
}

func TestNewMaterializedOperation_SnapshotsRule(t *testing.T) {
	t.Parallel()
	rec := mustMonthly(t, 15)
	p := rule(t, rec, "2026-08-15", nil)
	p.Category = CategoryRef{Slug: new("mortgage")}
	p.AutoPay = true

	op := NewMaterializedOperation(p, d("2026-09-15"))
	assert.Equal(t, p.ID, *op.PaymentID)
	assert.Equal(t, OriginPayment, op.Origin)
	assert.Equal(t, StatusPlanned, op.Status, "materialization never pre-pays (ADR 0049 §2)")
	assert.Equal(t, p.Title, op.Title)
	assert.Equal(t, p.AmountKopecks, op.AmountKopecks)
	assert.Equal(t, p.Type, op.Type)
	require.NotNil(t, op.PaymentForm)
	assert.Equal(t, FormTransfer, *op.PaymentForm)
	assert.Equal(t, "Ипотека", op.CategoryLabel, "label snapshotted from the default catalog")
	require.NotNil(t, op.CategorySlug)
	assert.Equal(t, "mortgage", *op.CategorySlug)
	assert.Nil(t, op.PaidDate)
}

func TestCategoryRefSnapshotLabel_Fallbacks(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Прочее", CategoryRef{Slug: new("removed-from-catalog")}.SnapshotLabel(),
		"a slug removed from the catalog falls back")
	name := "Моя категория"
	assert.Equal(t, name, CategoryRef{UserCategoryName: &name}.SnapshotLabel())
	assert.Equal(t, "Прочее", CategoryRef{}.SnapshotLabel(), "dangling reference falls back")
}

func TestRecurrenceJSON_RoundTrip(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		rec  Recurrence
		json string
	}{
		{"daily", NewDailyRecurrence(), `{"kind":"daily"}`},
		{"weekly", mustWeekly(t, time.Sunday, time.Saturday), `{"kind":"weekly","weekdays":[0,6]}`},
		{"monthly", mustMonthly(t, 31), `{"kind":"monthly","dayOfMonth":31}`},
		{"yearly", mustYearly(t, time.February, 29), `{"kind":"yearly","month":2,"day":29}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data, err := tc.rec.MarshalJSON()
			require.NoError(t, err)
			assert.JSONEq(t, tc.json, string(data))

			var back Recurrence
			require.NoError(t, back.UnmarshalJSON(data))
			assert.Equal(t, tc.rec, back)
		})
	}
}

func TestRecurrenceJSON_RejectsInvalid(t *testing.T) {
	t.Parallel()
	for _, in := range []string{
		`{"kind":"weekly","weekdays":[]}`,
		`{"kind":"weekly","weekdays":[9]}`,
		`{"kind":"monthly"}`,
		`{"kind":"monthly","dayOfMonth":32}`,
		`{"kind":"yearly","month":13,"day":1}`,
		`{"kind":"yearly","month":2,"day":0}`,
		`{"kind":"once"}`,
	} {
		var rec Recurrence
		require.Error(t, rec.UnmarshalJSON([]byte(in)), "input %s must be rejected", in)
	}
}

func TestRecurrenceConstructors_Validate(t *testing.T) {
	t.Parallel()
	_, err := NewWeeklyRecurrence(nil)
	require.Error(t, err)
	_, err = NewWeeklyRecurrence([]time.Weekday{time.Monday, time.Monday})
	require.Error(t, err)
	_, err = NewMonthlyRecurrence(0)
	require.Error(t, err)
	_, err = NewYearlyRecurrence(time.January, 32)
	require.Error(t, err)
}

// stringStatuses renders a date→status map as date-string keys for readable
// assertions.
func stringStatuses(byDate map[time.Time]OperationStatus) map[string]OperationStatus {
	out := make(map[string]OperationStatus, len(byDate))
	for date, status := range byDate {
		out[date.Format(time.DateOnly)] = status
	}
	return out
}
