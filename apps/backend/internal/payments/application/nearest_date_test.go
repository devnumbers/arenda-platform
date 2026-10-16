package application

import (
	"testing"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NearestDateOfPayment: следующая дата оплаты — хранённая плановая побеждает,
// иначе проекция от курсора; открытая пауза и завершённое правило дают null.
// Чистый шов (ticket #991) — без БД; интеграционная сторона на шве списка —
// payment_nearest_date_integration_test.go.
func TestNearestDateOfPayment(t *testing.T) {
	t.Parallel()
	today := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	date := func(s string) *time.Time {
		d, err := time.Parse(time.DateOnly, s)
		require.NoError(t, err)
		return &d
	}
	monthly30, err := domain.NewMonthlyRecurrence([]int{30}, false)
	require.NoError(t, err)
	monthly30Since := domain.Payment{Recurrence: monthly30, Since: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)}
	dailySinceAug := domain.Payment{Recurrence: domain.NewDailyRecurrence(), Since: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)}

	t.Run("хранённая плановая побеждает — проекция не зовётся", func(t *testing.T) {
		t.Parallel()
		got := NearestDateOfPayment(monthly30Since, today, NearestDateInputs{NextPlannedDate: date("2026-08-30")})
		assert.Equal(t, "2026-08-30", got.Format(time.DateOnly))
	})

	t.Run("ничего не материализовано — проекция от вчера", func(t *testing.T) {
		t.Parallel()
		got := NearestDateOfPayment(monthly30Since, today, NearestDateInputs{})
		require.NotNil(t, got)
		assert.Equal(t, "2026-08-30", got.Format(time.DateOnly))
	})

	t.Run("предоплата ближайшего — проекция от оплаченного курса", func(t *testing.T) {
		t.Parallel()
		// 30-е оплачено вперёд — «следующей» не считается, дата ищется после.
		got := NearestDateOfPayment(monthly30Since, today, NearestDateInputs{LastMaterialized: date("2026-08-30")})
		require.NotNil(t, got)
		assert.Equal(t, "2026-09-30", got.Format(time.DateOnly))
	})

	t.Run("открытая пауза — true null", func(t *testing.T) {
		t.Parallel()
		paused := dailySinceAug
		paused.Pauses = []domain.PauseInterval{{From: today}}
		got := NearestDateOfPayment(paused, today, NearestDateInputs{})
		assert.Nil(t, got)
	})

	t.Run("завершённое правило — true null", func(t *testing.T) {
		t.Parallel()
		end := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
		settled := dailySinceAug
		settled.EndDate = &end
		got := NearestDateOfPayment(settled, today, NearestDateInputs{LastMaterialized: date("2026-08-10")})
		assert.Nil(t, got)
	})

	t.Run("since впереди — первое вхождение не раньше since", func(t *testing.T) {
		t.Parallel()
		future := domain.Payment{
			Recurrence: domain.NewDailyRecurrence(),
			Since:      time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		}
		got := NearestDateOfPayment(future, today, NearestDateInputs{})
		require.NotNil(t, got)
		assert.Equal(t, "2026-09-01", got.Format(time.DateOnly))
	})
}
