package domain

// RU-словесность расписания правила — порт recurrenceLabel прототипа с
// формулировками резолюции #452 («Ежедневно», «Каждый понедельник», «Каждую
// неделю в пн, чт, вс», «Каждый месяц 1 числа», «Последний день каждого
// месяца», «Каждое 13 мая»). Живёт в домене платежей, потому что значения
// пояснений единой строки истории приходят готовыми (AttributeChange:
// вызывающий модуль форматирует, история не знает словарей — карта #1324,
// аменда ADR 0061 §5). Словесность повторяет фронтовый recurrenceLabel
// (entities/payment/lib/recurrence-label.ts) дословно; дубликат осознан по
// прецеденту FullMonthsBetween — подъём формулировок в общий пакет вне
// слайса.

import (
	"strconv"
	"strings"
	"time"
)

// Словарь genitiveMonths хранит родительный падеж названий месяцев для
// годового правила «Каждое 13 мая» и повторяет словарь history/domain —
// осознанный дубликат, подробности в шапке файла.
var genitiveMonths = [12]string{
	"января", "февраля", "марта", "апреля", "мая", "июня",
	"июля", "августа", "сентября", "октября", "ноября", "декабря",
}

// weekdaysShort — короткие имена дней недели, 0=вс..6=сб, как во
// фронтовом словаре фраз повторяемости.
var weekdaysShort = [7]string{"вс", "пн", "вт", "ср", "чт", "пт", "сб"}

// singleWeekdayPhrases — готовые фразы одного дня недели: род и падеж
// согласуются с правилом («каждый понедельник», но «каждую среду»).
var singleWeekdayPhrases = [7]string{
	"Каждое воскресенье",
	"Каждый понедельник",
	"Каждый вторник",
	"Каждую среду",
	"Каждый четверг",
	"Каждую пятницу",
	"Каждую субботу",
}

// RecurrenceLabel renders the schedule's RU phrase — the diff value of the
// action journal's «Расписание» line.
func RecurrenceLabel(r Recurrence) string {
	switch r.kind {
	case RecurrenceDaily:
		return "Ежедневно"
	case RecurrenceWeekly:
		if len(r.weekdays) == 1 {
			return singleWeekdayPhrases[r.weekdays[0]]
		}
		return "Каждую неделю в " + weekdaysShortLabel(r.weekdays)
	case RecurrenceMonthly:
		// Дни перечисляются в родительном падеже («1 и 18 числа»); маркер
		// последнего дня добавляется отдельной фразой (резолюция #452).
		daysPart := ""
		switch len(r.daysOfMonth) {
		case 0:
			if !r.lastDay {
				break
			}
			return "Последний день каждого месяца"
		case 1:
			daysPart = strconv.Itoa(r.daysOfMonth[0]) + " числа"
		default:
			daysPart = joinInts(r.daysOfMonth[:len(r.daysOfMonth)-1]) + " и " + strconv.Itoa(r.daysOfMonth[len(r.daysOfMonth)-1]) + " числа"
		}
		if r.lastDay {
			return "Каждый месяц " + daysPart + " и в последний день месяца"
		}
		return "Каждый месяц " + daysPart
	case RecurrenceYearly:
		return "Каждое " + strconv.Itoa(r.day) + " " + genitiveMonths[r.month-1]
	default:
		return string(r.kind)
	}
}

// weekdaysShortLabel перечисляет выбранные дни от понедельника — «пн, чт,
// вс» (порядок примера резолюции #452).
func weekdaysShortLabel(weekdays []time.Weekday) string {
	mondayFirst := make([]time.Weekday, 0, len(weekdays))
	for d := time.Monday; ; d++ {
		if d > time.Saturday {
			d = time.Sunday
		}
		for _, w := range weekdays {
			if w == d {
				mondayFirst = append(mondayFirst, w)
			}
		}
		if d == time.Sunday {
			break
		}
	}
	parts := make([]string, 0, len(mondayFirst))
	for _, w := range mondayFirst {
		parts = append(parts, weekdaysShort[w])
	}
	return strings.Join(parts, ", ")
}

func joinInts(nums []int) string {
	parts := make([]string, 0, len(nums))
	for _, n := range nums {
		parts = append(parts, strconv.Itoa(n))
	}
	return strings.Join(parts, ", ")
}
