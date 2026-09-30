/**
 * Повторяемость платежа в тексте — порт `recurrenceLabel` прототипа с
 * формулировками резолюции #452: «Ежедневно», «Каждый понедельник»,
 * «Каждую неделю в пн, чт, вс», «Каждый месяц 1 числа», «Последний день
 * каждого месяца», «Каждое 13 мая». Зависит только от регулярности —
 * в визарде подпись доступна до даты заведения.
 */

import type { Recurrence } from '../model/types';
import { dateInMonth } from '@/shared/lib/calendar';
import { formatDayMonth } from '@/shared/lib/date-format';

/** Короткие имена дней недели, 0=вс..6=сб — как в каталоге дат прототипа. */
const WEEKDAYS_SHORT = ['вс', 'пн', 'вт', 'ср', 'чт', 'пт', 'сб'] as const;

/**
 * Готовые фразы для одного дня недели: род и падеж согласуются с правилом
 * («каждый понедельник», но «каждую среду», «каждое воскресенье»).
 */
const SINGLE_WEEKDAY_PHRASES = [
  'Каждое воскресенье',
  'Каждый понедельник',
  'Каждый вторник',
  'Каждую среду',
  'Каждый четверг',
  'Каждую пятницу',
  'Каждую субботу',
] as const;

/** Порядок перечисления — от понедельника: пн..сб, вс последним (пример «пн, чт, вс»). */
function mondayFirst(weekdays: ReadonlyArray<number>): number[] {
  return [...weekdays].sort((a, b) => (a + 6) % 7 - ((b + 6) % 7));
}

/** Короткие имена выбранных дней через запятую, от понедельника:
 * «пн, чт, вс» — словарь фраз повторяемости. */
function weekdayShortLabel(weekdays: ReadonlyArray<number>): string {
  return mondayFirst(weekdays)
    .map((weekday) => WEEKDAYS_SHORT[weekday])
    .filter((name) => name !== undefined)
    .join(', ');
}

/** День и месяц годового правила без года: «13 мая»; якорь-високосный
 * 2024 сохраняет 29 февраля. */
function formatYearlyDayMonth(month: number, day: number): string {
  return formatDayMonth(dateInMonth(2024, month - 1, day));
}

export function recurrenceLabel(recurrence: Recurrence): string {
  switch (recurrence.kind) {
    case 'daily':
      return 'Ежедневно';
    case 'weekly': {
      if (recurrence.weekdays.length === 1) {
        // Длина union'а контракта гарантирует элемент, noUncheckedIndexedAccess
        // требует явного fallback — недостижим при длине 1.
        const weekday = recurrence.weekdays[0] ?? 0;
        return SINGLE_WEEKDAY_PHRASES[weekday] ?? SINGLE_WEEKDAY_PHRASES[0];
      }
      return `Каждую неделю в ${weekdayShortLabel(recurrence.weekdays)}`;
    }
    case 'monthly': {
      // Дни перечисляются в родительном падеже («1 и 18 числа»); маркер
      // последнего дня добавляется отдельной фразой (резолюция #452).
      const days = recurrence.daysOfMonth;
      const daysPart =
        days.length === 1
          ? `${days[0] ?? 1} числа`
          : `${days.slice(0, -1).join(', ')} и ${days[days.length - 1] ?? 1} числа`;
      if (days.length === 0) {
        return 'Последний день каждого месяца';
      }
      if (recurrence.lastDay) {
        return `Каждый месяц ${daysPart} и в последний день месяца`;
      }
      return `Каждый месяц ${daysPart}`;
    }
    case 'yearly':
      return `Каждое ${formatYearlyDayMonth(recurrence.month, recurrence.day)}`;
  }
}
