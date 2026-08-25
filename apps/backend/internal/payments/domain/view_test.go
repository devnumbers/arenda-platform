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
