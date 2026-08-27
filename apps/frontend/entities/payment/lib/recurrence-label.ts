/**
 * Повторяемость платежа в тексте — порт `recurrenceLabel` прототипа с
 * формулировками резолюции #452: «Ежедневно», «Каждый понедельник»,
 * «Каждую неделю в пн, чт, вс», «Каждый месяц 1 числа», «Последний день
 * каждого месяца», «Каждое 13 мая». Зависит только от регулярности —
 * в визарде подпись доступна до даты заведения.
 */

import type { Recurrence } from '../model/types';
import { dateInMonth } from './dates';
import { formatDayMonth } from './date-format';

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
      const days = mondayFirst(recurrence.weekdays)
        .map((weekday) => WEEKDAYS_SHORT[weekday])
        .filter((name) => name !== undefined);
      return `Каждую неделю в ${days.join(', ')}`;
    }
    case 'monthly':
      // День 31 прижатием даёт последний день каждого короткого месяца —
      // расписание тождественно опции «последний день», текст из резолюции #452.
      if (recurrence.dayOfMonth === 31) {
        return 'Последний день каждого месяца';
      }
      return `Каждый месяц ${recurrence.dayOfMonth} числа`;
    case 'yearly':
      // Якорь-високосный 2024 сохраняет 29 февраля для формулировки.
      return `Каждое ${formatDayMonth(dateInMonth(2024, recurrence.month - 1, recurrence.day))}`;
  }
}
