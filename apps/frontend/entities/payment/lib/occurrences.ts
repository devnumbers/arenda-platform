/**
 * Перечисление вхождений платежа — порт occurrences.ts/pauses.ts прототипа
 * (решения №1/№8/№10/№17; якорь в регулярности):
 * - «каждый день» — все дни с даты заведения;
 * - «каждую неделю» — выбранные дни недели (0=воскресенье..6=суббота);
 * - «каждый месяц» — выбранные дни месяца (1..30, прижимаются к последнему
 *   дню короткого месяца) и/или фактический последний день месяца;
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
import { addDays, cmp, dateInMonth, fromIso } from '@/shared/lib/calendar';

/** Страховочный потолок перечисления внутри одного окна — как в прототипе. */
const MAX_OCCURRENCES = 1000;

/** Дни-кандидаты одного месяца месячной регулярности: каждый выбранный день
 * прижимается к длине месяца, маркер последнего дня даёт фактический
 * последний день; даты сортируются и дедуплицируются (выбранный день может
 * совпасть с последним). */
function monthlyCandidates(
  recurrence: Extract<PaymentSchedule['recurrence'], { kind: 'monthly' }>,
  year: number,
  monthIndex0: number,
): IsoDate[] {
  const candidates = recurrence.daysOfMonth.map((day) =>
    dateInMonth(year, monthIndex0, day),
  );
  if (recurrence.lastDay) {
    candidates.push(dateInMonth(year, monthIndex0, 31));
  }
  candidates.sort(cmp);
  return candidates.filter((date, index) => index === 0 || cmp(date, candidates[index - 1] ?? date) !== 0);
}

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
      // Каждый месяц строится от якоря независимо, не итеративно: иначе 30-е
      // «сползает» на 28-е навсегда (смоук прототипа «янв31→фев28→мар31»).
      const base = fromIso(since);
      const baseYear = base.getUTCFullYear();
      const baseMonth = base.getUTCMonth();
      for (let month = 0; out.length < MAX_OCCURRENCES; month++) {
        const candidates = monthlyCandidates(recurrence, baseYear, baseMonth + month);
        const first = candidates[0];
        if (first === undefined || cmp(first, hardEnd) > 0) {
          break;
        }
        for (const date of candidates) {
          if (out.length >= MAX_OCCURRENCES) {
            break;
          }
          if (cmp(date, since) < 0) continue;
          if (cmp(date, start) < 0) continue;
          if (!isDatePaused(pauses, date)) {
            out.push(date);
          }
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

/**
 * До `count` вхождений строго после курсора `start` — страница бесконечной
 * догрузки «Графика платежей»: у бессрочного правила страницы генерируются
 * на лету без потолка, генерация синхронная и дешёвая (дневник и неделю
 * считает по дням, месяц и год — по якорям от месяца/года курсора).
 * Ограничения те же, что у occurrencesBetween: снизу `since`, паузы
 * вырезаются, `endDate` останавливает (сам день окончания включается).
 * Вернулось меньше `count` — правило исчерпано (или окончание позади).
 */
export function nextOccurrencesAfter(
  schedule: PaymentSchedule,
  start: IsoDate,
  count: number,
): IsoDate[] {
  if (count <= 0) {
    return [];
  }
  const { recurrence, since, pauses } = schedule;
  const out: IsoDate[] = [];

  // Открытая пауза [from, ∞) останавливает генерацию навсегда: кандидаты от
  // `from` и дальше лежат в ней все — без этого стопа перечисление
  // бессрочного правила на активной паузе не завершается никогда. Кандидаты
  // во всех ветках монотонно растут, поэтому одна проверка на шаг корректна.
  const openPauseFrom = pauses.find((pause) => pause.to === undefined)?.from;

  const push = (date: IsoDate): boolean => {
    if (!isDatePaused(pauses, date)) {
      out.push(date);
    }
    return out.length < count;
  };

  /** Стоп-условия на кандидата: конец правила, открытая пауза. */
  const stopAt = (date: IsoDate): boolean =>
    (schedule.endDate !== undefined && cmp(date, schedule.endDate) > 0)
    || (openPauseFrom !== undefined && cmp(date, openPauseFrom) >= 0);

  switch (recurrence.kind) {
    case 'daily': {
      let date = cmp(since, start) > 0 ? since : addDays(start, 1);
      while (!stopAt(date) && push(date)) {
        date = addDays(date, 1);
      }
      break;
    }
    case 'weekly': {
      const days = new Set(recurrence.weekdays);
      let date = cmp(since, start) > 0 ? since : addDays(start, 1);
      for (;;) {
        if (stopAt(date)) {
          break;
        }
        if (days.has(fromIso(date).getUTCDay())) {
          if (!push(date)) {
            break;
          }
        }
        date = addDays(date, 1);
      }
      break;
    }
    case 'monthly': {
      // Якоря считаются от месяца курсора независимо от него самого (как и
      // в occurrencesBetween — 30-е не сползает); якорь ≤ курсора пропускаем
      // до пуша, чтобы он не съедал лимит страницы.
      const base = fromIso(since);
      const from = fromIso(cmp(since, start) > 0 ? since : start);
      let month =
        (from.getUTCFullYear() - base.getUTCFullYear()) * 12
        + (from.getUTCMonth() - base.getUTCMonth());
      for (;;) {
        const candidates = monthlyCandidates(
          recurrence,
          base.getUTCFullYear(),
          base.getUTCMonth() + month,
        );
        month += 1;
        let stopped = false;
        for (const date of candidates) {
          if (stopAt(date)) {
            stopped = true;
            break;
          }
          if (cmp(date, start) <= 0) {
            continue;
          }
          if (!push(date)) {
            stopped = true;
            break;
          }
        }
        if (stopped) {
          break;
        }
      }
      break;
    }
    case 'yearly': {
      const base = fromIso(since);
      const from = fromIso(cmp(since, start) > 0 ? since : start);
      let year = from.getUTCFullYear() - base.getUTCFullYear();
      for (;;) {
        const date = dateInMonth(base.getUTCFullYear() + year, recurrence.month - 1, recurrence.day);
        year += 1;
        if (stopAt(date)) {
          break;
        }
        if (cmp(date, start) <= 0) {
          continue;
        }
        if (!push(date)) {
          break;
        }
      }
      break;
    }
  }
  return out;
}
