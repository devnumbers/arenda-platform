const DAY_MS = 24 * 60 * 60 * 1000;

export type LeasePaymentInput = {
  start_date: string;
  payment_day: number;
};

/** Количество дней в месяце (monthIndex0: 0..11). */
export function lastDayOfMonth(year: number, monthIndex0: number): number {
  return new Date(year, monthIndex0 + 1, 0).getDate();
}

/** Ограничивает день последним днём месяца. */
export function clampDay(year: number, monthIndex0: number, day: number): number {
  return Math.min(day, lastDayOfMonth(year, monthIndex0));
}

/** Парсит 'YYYY-MM-DD' как локальную полночь. */
export function parseLocalDate(value: string): Date {
  return new Date(`${value}T00:00:00`);
}

/** Возвращает локальную полночь для переданной даты. */
export function startOfDay(d: Date): Date {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate());
}

/** Целое число календарных дней между двумя датами (обе на полночь). */
export function diffDays(from: Date, to: Date): number {
  const fromDay = startOfDay(from);
  const toDay = startOfDay(to);
  return Math.round((toDay.getTime() - fromDay.getTime()) / DAY_MS);
}

function makeDate(year: number, monthIndex0: number, day: number): Date {
  return new Date(year, monthIndex0, clampDay(year, monthIndex0, day));
}

/** Ближайшая плановая дата оплаты (включая сегодня); до старта — дата начала. */
export function nextPaymentDate(
  input: LeasePaymentInput,
  now: Date = new Date(),
): Date {
  const start = parseLocalDate(input.start_date);
  const today = startOfDay(now);

  if (start.getTime() > today.getTime()) {
    return start;
  }

  const year = today.getFullYear();
  const month = today.getMonth();
  const cand = makeDate(year, month, input.payment_day);

  if (cand.getTime() >= today.getTime()) {
    return cand;
  }

  const nextYear = month === 11 ? year + 1 : year;
  const nextMonth = month === 11 ? 0 : month + 1;
  return makeDate(nextYear, nextMonth, input.payment_day);
}

/** Последняя плановая оплата <= today; null, если аренда ещё не началась. */
export function currentDueDate(
  input: LeasePaymentInput,
  now: Date = new Date(),
): Date | null {
  const start = parseLocalDate(input.start_date);
  const today = startOfDay(now);

  if (start.getTime() > today.getTime()) {
    return null;
  }

  const year = today.getFullYear();
  const month = today.getMonth();
  const cand = makeDate(year, month, input.payment_day);

  if (cand.getTime() <= today.getTime()) {
    return cand;
  }

  const prevYear = month === 0 ? year - 1 : year;
  const prevMonth = month === 0 ? 11 : month - 1;
  return makeDate(prevYear, prevMonth, input.payment_day);
}

/** Порядковый номер текущего месяца аренды (>= 1). */
export function currentMonthIndex(
  input: LeasePaymentInput,
  now: Date = new Date(),
): number {
  const start = parseLocalDate(input.start_date);
  const today = startOfDay(now);

  if (start.getTime() > today.getTime()) {
    return 1;
  }

  const months =
    (today.getFullYear() - start.getFullYear()) * 12 +
    (today.getMonth() - start.getMonth()) +
    1;
  return Math.max(1, months);
}

/** Доля истечения текущего платёжного периода (0..1). */
export function monthPeriodProgress(
  input: LeasePaymentInput,
  now: Date = new Date(),
): number {
  const start = parseLocalDate(input.start_date);
  const today = startOfDay(now);

  if (start.getTime() > today.getTime()) {
    return 0;
  }

  const periodEnd = nextPaymentDate(input, today);
  const due = currentDueDate(input, today);

  if (due === null) {
    return 0;
  }

  const periodStart = start.getTime() > due.getTime() ? start : due;

  if (periodEnd.getTime() <= periodStart.getTime()) {
    return 0;
  }

  const ratio =
    (today.getTime() - periodStart.getTime()) /
    (periodEnd.getTime() - periodStart.getTime());
  return Math.min(1, Math.max(0, ratio));
}

/** Доля истечения периода перед реальной датой платежа (0..1).
 *  Начало периода — плановый день оплаты (payment_day) месяцем раньше target
 *  (или start_date, если он позже); конец — target. */
export function progressToPaymentDate(
  input: LeasePaymentInput,
  target: Date,
  now: Date = new Date(),
): number {
  const start = parseLocalDate(input.start_date);
  const today = startOfDay(now);
  const t = startOfDay(target);

  const prevMonth = t.getMonth() === 0 ? 11 : t.getMonth() - 1;
  const prevYear = t.getMonth() === 0 ? t.getFullYear() - 1 : t.getFullYear();
  let periodStart = new Date(prevYear, prevMonth, clampDay(prevYear, prevMonth, input.payment_day));
  if (start.getTime() > periodStart.getTime()) {
    periodStart = start;
  }
  if (t.getTime() <= periodStart.getTime()) {
    return 0;
  }
  const ratio = (today.getTime() - periodStart.getTime()) / (t.getTime() - periodStart.getTime());
  return Math.min(1, Math.max(0, ratio));
}

/** 'просрочено N дн.'. */
export function formatOverdue(days: number): string {
  return `просрочено ${days} дн.`;
}

/** 'N-й месяц'. */
export function formatLeaseMonthOrdinal(n: number): string {
  return `${n}-й месяц`;
}
