package application

// Диффы правила для единой строки истории (решение владельца 12.10, карта
// #1324): «Платёж изменён: [название]» + по пояснению на каждое изменившееся
// поле, значения готовыми строками — суммы через historydomain.FormatRubles,
// даты точечные, расписание словесностью домена (RecurrenceLabel). Поля и
// порядок — тот же словарь диффа change-log (ADR 0065): тип, название,
// сумма, расписание, категория, дата окончания, автоплатёж, напоминание.

import (
	"fmt"
	"time"

	historydomain "github.com/nambers/arenda-planform/apps/backend/internal/history/domain"
	"github.com/nambers/arenda-planform/apps/backend/internal/payments/domain"
)

// paymentHistoryChanges diffs the loaded rule against the merged one for
// the action journal's unified change row (ADR 0061 §5, аменда 12.10:
// пояснения несут все изменения, суммы включительно).
func paymentHistoryChanges(before, after domain.Payment) []historydomain.AttributeChange {
	var changes []historydomain.AttributeChange
	if before.Type != after.Type {
		changes = append(changes, historydomain.AttributeChange{
			Key: "type", Label: "Тип",
			Old: domain.PaymentTypeLabel(before.Type), New: domain.PaymentTypeLabel(after.Type),
		})
	}
	if before.Title != after.Title {
		changes = append(changes, historydomain.AttributeChange{
			Key: "title", Label: "Название",
			Old: before.Title, New: after.Title,
		})
	}
	if before.AmountKopecks != after.AmountKopecks {
		changes = append(changes, historydomain.AttributeChange{
			Key: "amount_kopecks", Label: "Сумма",
			Old: historydomain.FormatRubles(before.AmountKopecks), New: historydomain.FormatRubles(after.AmountKopecks),
		})
	}
	if _, _, moved := domain.ChangeRecurrenceMoved(before.Recurrence, after.Recurrence); moved {
		changes = append(changes, historydomain.AttributeChange{
			Key: "recurrence", Label: "Расписание",
			Old: domain.RecurrenceLabel(before.Recurrence), New: domain.RecurrenceLabel(after.Recurrence),
		})
	}
	if !domain.ChangeCategoryEqual(before.Category, after.Category) {
		changes = append(changes, historydomain.AttributeChange{
			Key: "category_slug", Label: "Категория",
			Old: before.Category.SnapshotLabel(), New: after.Category.SnapshotLabel(),
		})
	}
	if !domain.ChangeDatesEqual(before.EndDate, after.EndDate) {
		changes = append(changes, historydomain.AttributeChange{
			Key: "end_date", Label: "Дата окончания",
			Old: paymentEndDateLabel(before.EndDate), New: paymentEndDateLabel(after.EndDate),
		})
	}
	if before.AutoPay != after.AutoPay {
		changes = append(changes, historydomain.AttributeChange{
			Key: "auto_pay", Label: "Автоплатёж",
			Old: before.AutoPay, New: after.AutoPay,
		})
	}
	if !paymentDayOffsetEqual(before.ReminderOffsetDays, after.ReminderOffsetDays) {
		changes = append(changes, historydomain.AttributeChange{
			Key: "reminder_offset_days", Label: "Напоминание",
			Old: paymentReminderLabel(before.ReminderOffsetDays), New: paymentReminderLabel(after.ReminderOffsetDays),
			// Выключенное напоминание — не «удалено»: напоминание выключают.
			Removed: "Напоминание: выключено",
		})
	}
	return changes
}

// paymentEndDateLabel renders an optional end date for the diff: nil (the
// open-ended rule) stays nil so the detail line reads as an addition or a
// removal; a date reads dotted («19.02.2026»).
func paymentEndDateLabel(t *time.Time) any {
	if t == nil {
		return nil
	}
	return historydomain.FormatDateDotted(*t)
}

// paymentReminderLabel renders the reminder offset for the diff: nil (off)
// stays nil; a value reads «за 1 день / за 3 дня».
func paymentReminderLabel(days *int) any {
	if days == nil {
		return nil
	}
	return fmt.Sprintf("за %d %s", *days, historydomain.PluralRU(*days, "день", "дня", "дней"))
}

func paymentDayOffsetEqual(a, b *int) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return *a == *b
	}
}
