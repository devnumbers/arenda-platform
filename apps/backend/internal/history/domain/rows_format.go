package domain

// The RU formatters behind the new template's rental rows (протокол D3,
// решение #1317 §7): the header reads «Добавлена аренда: 1 ноября» /
// «Добавлена аренда: 11 месяцев», the details carry the period with years.
// Живут рядом с rows.go, потому что история не может импортировать rentals
// (направление зависимости rentals → history): гражданская арифметика
// месяцев повторяет rentals/domain.FullMonthsBetween — ~8 строк дублирования
// осознаны и помечены; подъём в общий пакет — отдельный рефакторинг вне
// слайса. Даты аренды календарные (UTC-полночь) — компоненты читаются как
// записаны, без локальных сдвигов.

import (
	"fmt"
	"time"
)

// genitiveMonths — родительный падеж названий месяцев: «1 ноября»,
// «30 сентября».
var genitiveMonths = [12]string{
	"января", "февраля", "марта", "апреля", "мая", "июня",
	"июля", "августа", "сентября", "октября", "ноября", "декабря",
}

// pluralRU склоняет русское существительное по числу: один/две-четыре/прочие
// (месяц/месяца/месяцев, день/дня/дней).
func pluralRU(n int, one, few, many string) string {
	mod10 := n % 10
	mod100 := n % 100
	switch {
	case mod10 == 1 && mod100 != 11:
		return one
	case mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14):
		return few
	default:
		return many
	}
}

// formatDayMonthGenitive renders a calendar date without the year — the
// rental-created header («1 ноября»).
func formatDayMonthGenitive(t time.Time) string {
	return fmt.Sprintf("%d %s", t.Day(), genitiveMonths[int(t.Month())-1])
}

// formatPeriodLine renders the details' period line: «с 1 ноября 2026
// по 30 сентября 2027»; у бессрочной аренды конечной границы нет —
// «с 1 ноября 2026».
func formatPeriodLine(from, to time.Time) string {
	if to.IsZero() {
		return fmt.Sprintf("с %d %s %d", from.Day(), genitiveMonths[int(from.Month())-1], from.Year())
	}
	return fmt.Sprintf("с %d %s %d по %d %s %d",
		from.Day(), genitiveMonths[int(from.Month())-1], from.Year(),
		to.Day(), genitiveMonths[int(to.Month())-1], to.Year())
}

// formatTerm renders the rental term the header reads: full civil months
// («1 месяц», «11 месяцев»), a sub-month span in days («23 дня»), a mixed
// span as the exact composition («10 месяцев 21 день»). Конечная дата —
// последний занятый день (включительно), поэтому окно считается по
// to+1 день: канонический пример владельца «1 ноября → 30 сентября =
// 11 месяцев». Арифметика месяцев — та же, что у
// rentals/domain.FullMonthsBetween (rental.go), применённая к to+1: месяц не
// полный, пока день to не дошёл до дня from; остаток дней считается от
// эксклюзивной границы полных месяцев (с клампом дня в короткий месяц).
func formatTerm(from, to time.Time) string {
	end := to.AddDate(0, 0, 1)
	months := (end.Year()-from.Year())*12 + int(end.Month()) - int(from.Month())
	months = max(months, 0)
	// Оценка по номерам месяцев промахивается не больше чем на один —
	// из-за клампа дня и недошедшего дня месяца; граница — единственный
	// честный арбитр.
	if monthBoundary(from, months).After(end) {
		months--
	} else if !monthBoundary(from, months+1).After(end) {
		months++
	}
	months = max(months, 0)
	days := max(int(end.Sub(monthBoundary(from, months))/(24*time.Hour)), 0)
	switch {
	case months > 0 && days > 0:
		return fmt.Sprintf("%d %s %d %s", months, pluralRU(months, "месяц", "месяца", "месяцев"),
			days, pluralRU(days, "день", "дня", "дней"))
	case months > 0:
		return fmt.Sprintf("%d %s", months, pluralRU(months, "месяц", "месяца", "месяцев"))
	default:
		return fmt.Sprintf("%d %s", days, pluralRU(days, "день", "дня", "дней"))
	}
}

// Exclusive end of n civil months from `from`: from's day-of-month n months
// later, clamped into the target month; a clamped month runs to the target
// month's end, so its exclusive end is the first day of the following month
// (31.01 + 1 месяц = [31.01, 1.03)). Гражданская арифметика повторяет
// rentals/domain.FullMonthsBetween (rental.go), применённую к to+1 день
// (шапка файла).
func monthBoundary(from time.Time, n int) time.Time {
	y, m, d := from.Date()
	last := time.Date(y, m+time.Month(n+1), 0, 0, 0, 0, 0, from.Location()).Day()
	day := d
	if day > last {
		day = last + 1
	}
	return time.Date(y, m+time.Month(n), day, 0, 0, 0, 0, from.Location())
}
