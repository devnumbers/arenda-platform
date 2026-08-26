/**
 * Чистая математика месяца для календарных компонентов дизайн-слоя
 * (тикет #459): показывается только выбранный месяц — бесконечной сетки
 * нет (решение владельца 2026-08-26). Локали захардкожены (не Intl), чтобы
 * рендер был одинаков на сервере, клиенте и в тестах.
 */

/** Названия месяцев в именительном падеже — заголовки «Август, 2026» и
 * колёса пикера (Figma 835:20007, 848:8720). */
export const MONTH_LABELS: ReadonlyArray<string> = [
  'Январь',
  'Февраль',
  'Март',
  'Апрель',
  'Май',
  'Июнь',
  'Июль',
  'Август',
  'Сентябрь',
  'Октябрь',
  'Ноябрь',
  'Декабрь',
];

/** Сокращения дней недели, неделя с понедельника (Figma 835:20149). */
export const WEEKDAY_LABELS: ReadonlyArray<string> = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'];

/** Заголовок блока месяца: «Август, 2026». */
export function monthTitle(year: number, month: number): string {
  return `${MONTH_LABELS[month]}, ${year}`;
}

/** Число дней в месяце; month — 0..11, как у Date. */
export function daysInMonth(year: number, month: number): number {
  return new Date(year, month + 1, 0).getDate();
}

/** Индекс дня недели первого числа месяца, Пн = 0 … Вс = 6. */
export function firstWeekdayOfMonth(year: number, month: number): number {
  return (new Date(year, month, 1).getDay() + 6) % 7;
}

/** Совпадает ли дата с год/месяц/день (без времени). */
export function isCalendarDay(date: Date, year: number, month: number, day: number): boolean {
  return date.getFullYear() === year && date.getMonth() === month && date.getDate() === day;
}
