// Дейт-хелперы календаря: нативный Date + Intl('ru-RU'), без сторонних библиотек.
// Арифметика ведётся на строках 'YYYY-MM-DD' (локальная дата), неделя начинается с понедельника.

const WEEKDAY_SHORT_MONDAY_FIRST: readonly string[] = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'];

const WEEKDAY_FULL_BY_GETDAY: readonly string[] = [
  'воскресенье',
  'понедельник',
  'вторник',
  'среда',
  'четверг',
  'пятница',
  'суббота',
];

const MONTH_GENITIVE_SHORT: readonly string[] = [
  'янв',
  'фев',
  'мар',
  'апр',
  'мая',
  'июн',
  'июл',
  'авг',
  'сен',
  'окт',
  'ноя',
  'дек',
];

// 'YYYY-MM-DD' из даты (локальные компоненты, без сдвига пояса).
export function toISODate(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function parseISODate(iso: string): Date {
  return new Date(`${iso}T00:00:00`);
}

export function todayISO(): string {
  return toISODate(new Date());
}

// Сдвиг на N дней от строковой даты.
export function addDays(iso: string, days: number): string {
  const date = parseISODate(iso);
  date.setDate(date.getDate() + days);
  return toISODate(date);
}

// Семь дат (пн–вс) недели, содержащей isoDate.
export function weekDates(
  isoDate: string,
): readonly [string, string, string, string, string, string, string] {
  const date = parseISODate(isoDate);
  const mondayOffset = (date.getDay() + 6) % 7;
  const monday = addDays(isoDate, -mondayOffset);
  return [
    monday,
    addDays(monday, 1),
    addDays(monday, 2),
    addDays(monday, 3),
    addDays(monday, 4),
    addDays(monday, 5),
    addDays(monday, 6),
  ];
}

// Короткий день недели («Пн») для строковой даты.
export function weekdayShort(isoDate: string): string {
  const parsed = parseISODate(isoDate);
  return WEEKDAY_SHORT_MONDAY_FIRST[(parsed.getDay() + 6) % 7] ?? '';
}

// «3 августа, понедельник»
export function formatDateWithWeekday(isoDate: string): string {
  const parsed = parseISODate(isoDate);
  if (Number.isNaN(parsed.getTime())) {
    return isoDate;
  }
  const date = parsed.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' });
  const weekday = WEEKDAY_FULL_BY_GETDAY[parsed.getDay()] ?? '';
  return `${date}, ${weekday}`;
}

// «12 авг, ср»
export function formatDateShort(isoDate: string): string {
  const parsed = parseISODate(isoDate);
  if (Number.isNaN(parsed.getTime())) {
    return isoDate;
  }
  const month = MONTH_GENITIVE_SHORT[parsed.getMonth()] ?? '';
  return `${parsed.getDate()} ${month}, ${weekdayShort(isoDate)}`;
}

// Локальная дата 'YYYY-MM-DD' из date-time строки API.
// scheduled_at приходит в UTC; new Date() разбирает её, getFullYear/getMonth/getDate
// дают локальные компоненты в поясе браузера (равно поясу владельца).
export function localDateOf(scheduledAt: string): string {
  return toISODate(new Date(scheduledAt));
}
