package application

// Диффы правила для пояснений единой строки (решение владельца 12.10, карта
// #1324; аменда ADR 0061 §5): все сдвинувшиеся поля, значения готовыми
// строками — суммы «5 000 ₽», расписание словесностью резолюции #452,
// напоминание «за 7 дней», выключение — «Напоминание: выключено».

import (
	"testing"
	"time"

	"github.com/google/uuid"
	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
)

func historyTestRule(t *testing.T, mutate func(p *domain.Payment)) domain.Payment {
	t.Helper()
	monthly, err := domain.NewMonthlyRecurrence([]int{5}, false)
	if err != nil {
		t.Fatalf("NewMonthlyRecurrence: %v", err)
	}
	p := domain.Payment{
		ID:            uuid.Must(uuid.NewV7()),
		Type:          domain.TypeExpense,
		Title:         "Коммуналка",
		AmountKopecks: 500_000,
		Recurrence:    monthly,
		Category:      domain.CategoryRef{UserCategoryName: new("Коммуналка")},
	}
	if mutate != nil {
		mutate(&p)
	}
	return p
}

func TestPaymentHistoryChanges_CarriesEveryMovedField(t *testing.T) {
	t.Parallel()
	newMonthly, err := domain.NewMonthlyRecurrence([]int{10}, false)
	if err != nil {
		t.Fatalf("NewMonthlyRecurrence: %v", err)
	}
	end := time.Date(2027, time.March, 20, 0, 0, 0, 0, time.UTC)
	before := historyTestRule(t, nil)
	after := historyTestRule(t, func(p *domain.Payment) {
		p.Title = "Коммуналка и интернет"
		p.AmountKopecks = 600_000
		p.Recurrence = newMonthly
		p.EndDate = &end
		p.AutoPay = true
		p.ReminderOffsetDays = new(7)
	})

	changes := paymentHistoryChanges(before, after)
	if len(changes) != 6 {
		t.Fatalf("changes = %d, want 6 (title, amount, recurrence, end date, auto pay, reminder)", len(changes))
	}

	// Значения готовыми строками: деньги, расписание, дата, напоминание.
	expect := []struct{ key, line string }{
		{"title", `Название: «Коммуналка» → «Коммуналка и интернет»`},
		{"amount_kopecks", "Сумма: «5\u00A0000 ₽» → «6\u00A0000 ₽»"},
		{"recurrence", `Расписание: «Каждый месяц 5 числа» → «Каждый месяц 10 числа»`},
		{"end_date", `Дата окончания: «20.03.2027»`},
		{"auto_pay", `Автоплатёж: «нет» → «да»`},
		{"reminder_offset_days", `Напоминание: «за 7 дней»`},
	}
	for i, e := range expect {
		if changes[i].Key != e.key {
			t.Errorf("changes[%d].key = %s, want %s", i, changes[i].Key, e.key)
		}
	}
	entry := historydomain.PaymentUpdated(after.ID, after.Title, changes)
	if len(entry.Details) != len(expect) {
		t.Fatalf("details = %q", entry.Details)
	}
	for i, e := range expect {
		if entry.Details[i] != e.line {
			t.Errorf("details[%d] = %q, want %q", i, entry.Details[i], e.line)
		}
	}
}

func TestPaymentHistoryChanges_ReminderOffReadsDisabled(t *testing.T) {
	t.Parallel()
	before := historyTestRule(t, func(p *domain.Payment) { p.ReminderOffsetDays = new(3) })
	after := historyTestRule(t, nil)

	changes := paymentHistoryChanges(before, after)
	if len(changes) != 1 {
		t.Fatalf("changes = %d, want 1", len(changes))
	}
	entry := historydomain.PaymentUpdated(after.ID, after.Title, changes)
	want := historydomain.DetailLines{"Напоминание: выключено"}
	if len(entry.Details) != 1 || entry.Details[0] != want[0] {
		t.Errorf("details = %q, want %q", entry.Details, want)
	}
}

func TestPaymentHistoryChanges_EndDateClearedReadsRemoved(t *testing.T) {
	t.Parallel()
	end := time.Date(2027, time.March, 20, 0, 0, 0, 0, time.UTC)
	before := historyTestRule(t, func(p *domain.Payment) { p.EndDate = &end })
	after := historyTestRule(t, nil)

	changes := paymentHistoryChanges(before, after)
	entry := historydomain.PaymentUpdated(after.ID, after.Title, changes)
	want := historydomain.DetailLines{"Дата окончания: удалено"}
	if len(entry.Details) != 1 || entry.Details[0] != want[0] {
		t.Errorf("details = %q, want %q", entry.Details, want)
	}
}

func TestPaymentHistoryChanges_EqualRulesDiffToNil(t *testing.T) {
	t.Parallel()
	if changes := paymentHistoryChanges(historyTestRule(t, nil), historyTestRule(t, nil)); changes != nil {
		t.Errorf("changes = %v, want nil", changes)
	}
}
