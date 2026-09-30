package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOperationView(t *testing.T) {
	t.Parallel()
	today := d(dayT0)
	base := Operation{
		ID:            uuidMustV7(),
		OwnerID:       uuidMustV7(),
		PropertyID:    uuidMustV7(),
		Origin:        OriginPayment,
		Status:        StatusPlanned,
		Type:          TypeExpense,
		Title:         "t",
		AmountKopecks: 100,
		CategoryLabel: "Прочее",
	}

	cases := []struct {
		name string
		op   func(Operation) Operation
		want OperationViewStatus
	}{
		{"planned today stays planned", func(o Operation) Operation { return o }, ViewStatusPlanned},
		{"planned future stays planned", func(o Operation) Operation {
			o.Date = d(dayNext)
			return o
		}, ViewStatusPlanned},
		{"planned yesterday is overdue", func(o Operation) Operation {
			o.Date = d(dayT1)
			return o
		}, ViewStatusOverdue},
		{"planned far past is overdue", func(o Operation) Operation {
			o.Date = d("2026-01-10")
			return o
		}, ViewStatusOverdue},
		{"paid in the past is never overdue", func(o Operation) Operation {
			o.Date = d(dayT1)
			o.Status = StatusPaid
			paid := d(dayT1)
			o.PaidDate = &paid
			return o
		}, ViewStatusPaid},
		{"paid today is paid", func(o Operation) Operation {
			o.Date = today
			o.Status = StatusPaid
			o.PaidDate = &today
			return o
		}, ViewStatusPaid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			op := tc.op(base)
			op.Date = orToday(op.Date, today)
			assert.Equal(t, tc.want, OperationView(op, today))
		})
	}
}

// orToday keeps the zero Date of the base fixture at today.
func orToday(date, today time.Time) time.Time {
	if date.IsZero() {
		return today
	}
	return date
}

// ProjectionCursor: курсор проекции — последняя материализованная дата,
// при отсутствии фактов вчера; общее правило IsCompleted и следующей даты
// оплаты (ticket #991).
func TestProjectionCursor(t *testing.T) {
	t.Parallel()
	today := d(dayT0)

	t.Run("без фактов — вчера: вхождения впереди ещё проецируются", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, d(dayT1), ProjectionCursor(today, nil))
	})

	t.Run("факты старше вчера не двигают курсор", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, d(dayT1), ProjectionCursor(today, dp(dayT3)))
	})

	t.Run("оплачено вперёд — курсор по последней материализованной", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, d(daySep5), ProjectionCursor(today, dp(daySep5)))
	})
}

// IsCompleted: правило завершено, когда не осталось неоплаченных вхождений —
// ничего planned и за последней материализованной датой вхождений нет.
func TestIsCompleted(t *testing.T) {
	t.Parallel()
	t.Run("оплачены все вхождения до endDate — завершён сразу, не на следующий день", func(t *testing.T) {
		t.Parallel()
		// Сценарий бага: since=t0, endDate=t0+2, обе операции оплачены,
		// planned пусто — завершён в тот же день (t0), хотя endDate впереди.
		p := rule(t, NewDailyRecurrence(), dayT0, new(dayNext2))
		assert.True(t, IsCompleted(p, d(dayT0), dp(dayNext2), false))
	})

	t.Run("есть planned — не завершён (идёт обычное правило или есть долг)", func(t *testing.T) {
		t.Parallel()
		p := rule(t, NewDailyRecurrence(), dayT0, new(dayNext2))
		assert.False(t, IsCompleted(p, d(dayT0), dp(dayT0), true))
		open := rule(t, NewDailyRecurrence(), dayT0, nil)
		assert.False(t, IsCompleted(open, d(dayT0), nil, true))
	})

	t.Run("без endDate правило никогда не завершено", func(t *testing.T) {
		t.Parallel()
		open := rule(t, NewDailyRecurrence(), dayT0, nil)
		// Автоплатёж погасил сегодняшнее, завтрашнее ещё есть — за cursor
		// находится вхождение.
		assert.False(t, IsCompleted(open, d(dayT0), dp(dayT0), false))
	})

	t.Run("ничего не материализовано — не завершён, вхождения ещё будут", func(t *testing.T) {
		t.Parallel()
		p := rule(t, NewDailyRecurrence(), dayT0, new(dayNext2))
		assert.False(t, IsCompleted(p, d(dayT0), nil, false))
	})

	t.Run("правило закончилось давно и всё оплачено — завершён", func(t *testing.T) {
		t.Parallel()
		p := rule(t, NewDailyRecurrence(), "2026-01-01", new("2026-01-02"))
		assert.True(t, IsCompleted(p, d(dayT0), dp("2026-01-02"), false))
	})

	t.Run("оплачено вперёд за пределами сегодня — курсор по последней дате", func(t *testing.T) {
		t.Parallel()
		open := rule(t, NewDailyRecurrence(), dayT0, nil)
		assert.False(t, IsCompleted(open, d(dayT0), dp(dayNext2), false))
		ended := rule(t, NewDailyRecurrence(), dayT0, new(dayNext))
		assert.True(t, IsCompleted(ended, d(dayT0), dp(dayNext), false))
	})
}
