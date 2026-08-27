/**
 * Перечисление вхождений платежа — порт occurrences.ts/pauses.ts прототипа
 * (решения №1/№8/№10/№17; якорь в регулярности):
 * - «каждый день» — все дни с даты заведения;
 * - «каждую неделю» — выбранные дни недели (0=воскресенье..6=суббота);
 * - «каждый месяц» — день месяца, 31-е прижимается к последнему дню месяца;
 * - «каждый год» — месяц и день, 29 февраля прижимается к последнему дню февраля;
 * - генерация ограничена снизу `since` — задним числом вхождений нет;
 * - `endDate` останавливает генерацию, сам день окончания включён;
 * - даты внутри интервалов пауз [from, to) вырезаются: дыры остаются навсегда;
 * - разового платежа нет — разовый факт вносится операцией вручную.
 *
 * Клиентский порт нужен для превью первого вхождения в визарде и проекции
 * «Графика платежей»; сервер остаётся источником истины по существующим
 * операциям (спека #453). Никаких часов: «сегодня» приходит параметром.
 */

import type { PauseInterval, PaymentSchedule } from '../model/types';
import type { IsoDate } from '../model/types';
import { addDays, cmp, dateInMonth, fromIso } from './dates';

/** Страховочный потолок перечисления — как в прототипе. */
const MAX_OCCURRENCES = 1000;

/** Дневной горизонт пакетной проекции бессрочного правила для «Графика
 * платежей» — те же 5 лет, что у календарного горизонта поиска порта
 * (horizonAfter ниже); живёт здесь, чтобы у проекции и порта было одно
 * число. */
export const PROJECTION_HORIZON_DAYS = 366 * 5;

function horizonAfter(date: IsoDate): IsoDate {
  const d = fromIso(date);
  return dateInMonth(d.getUTCFullYear() + 5, d.getUTCMonth(), d.getUTCDate());
}

/**
 * Попадает ли дата в один из интервалов пауз [from, to): from включительно,
 * день возобновления to уже не в паузе; пауза без to — активная бессрочная.
 */
export function isDatePaused(
  pauses: ReadonlyArray<PauseInterval>,
  date: IsoDate,
): boolean {
  return pauses.some(
    (pause) =>
      cmp(date, pause.from) >= 0 && (pause.to === undefined || cmp(date, pause.to) < 0),
  );
}

export function occurrencesBetween(
  schedule: PaymentSchedule,
  start: IsoDate,
  end: IsoDate,
): IsoDate[] {
  const { recurrence, since, pauses } = schedule;
  const hardEnd =
    schedule.endDate !== undefined && cmp(schedule.endDate, end) < 0
      ? schedule.endDate
      : end;
  if (cmp(start, hardEnd) > 0) {
    return [];
  }

  const out: IsoDate[] = [];
  switch (recurrence.kind) {
    case 'daily': {
      let date = cmp(since, start) > 0 ? since : start;
      while (cmp(date, hardEnd) <= 0 && out.length < MAX_OCCURRENCES) {
        if (!isDatePaused(pauses, date)) {
          out.push(date);
        }
        date = addDays(date, 1);
      }
      break;
    }
    case 'weekly': {
      const days = new Set(recurrence.weekdays);
      let date = cmp(since, start) > 0 ? since : start;
      while (cmp(date, hardEnd) <= 0 && out.length < MAX_OCCURRENCES) {
        if (days.has(fromIso(date).getUTCDay()) && !isDatePaused(pauses, date)) {
          out.push(date);
        }
        date = addDays(date, 1);
      }
      break;
    }
    case 'monthly': {
      // Каждый месяц строится от якоря независимо, не итеративно: иначе 31-е
      // «сползает» на 28-е навсегда (смоук прототипа «янв31→фев28→мар31»).
      const base = fromIso(since);
      for (let month = 0; out.length < MAX_OCCURRENCES; month++) {
        const date = dateInMonth(base.getUTCFullYear(), base.getUTCMonth() + month, recurrence.dayOfMonth);
        if (cmp(date, since) < 0) continue;
        if (cmp(date, start) < 0) continue;
        if (cmp(date, hardEnd) > 0) break;
        if (!isDatePaused(pauses, date)) {
          out.push(date);
        }
      }
      break;
    }
    case 'yearly': {
      const base = fromIso(since);
      for (let year = 0; out.length < MAX_OCCURRENCES; year++) {
        const date = dateInMonth(
          base.getUTCFullYear() + year,
          recurrence.month - 1,
          recurrence.day,
        );
        if (cmp(date, since) < 0) continue;
        if (cmp(date, start) < 0) continue;
        if (cmp(date, hardEnd) > 0) break;
        if (!isDatePaused(pauses, date)) {
          out.push(date);
        }
      }
      break;
    }
  }
  return out;
}

/**
 * Следующее вхождение строго после date (в пределах endDate, если задан).
 * null — вхождений больше нет (правило завершилось).
 */
export function nextOccurrenceAfter(
  schedule: PaymentSchedule,
  date: IsoDate,
): IsoDate | null {
  const candidates = occurrencesBetween(schedule, date, addDays(horizonAfter(date), 1));
  for (const candidate of candidates) {
    if (cmp(candidate, date) > 0) {
      return candidate;
    }
  }
  return null;
}

/**
 * Самое раннее вхождение правила — превью первого вхождения в визарде
 * (история 9 спеки #453): расписание всегда строится от `since`, сервер
 * ставит его при создании. null — вхождений нет (например, окончание
 * раньше даты заведения).
 */
export function firstOccurrence(schedule: PaymentSchedule): IsoDate | null {
  const first = occurrencesBetween(
    schedule,
    schedule.since,
    addDays(horizonAfter(schedule.since), 1),
  );
  return first[0] ?? null;
}
