/**
 * Форматирование UTC-дневных дат для строк списков (резолюция #452 и
 * унификация 2026-09-04 — до этого дословные дубликаты в entities/payment
 * и entities/task): «11 августа» — день и склонённый месяц; вне текущего
 * года год добавляется: «13 мая, 2027». «Сегодня» приходит параметром —
 * календарное «сегодня» интерфейса считает сервер по TZ собственника
 * (ADR 0048). Полноширинный формат «12.08.2026» легаси-профиля живёт в
 * format-date.ts (локальное время, другой контракт).
 */

import type { IsoDate, IsoRange } from './calendar';
import { fromIso, isoYear } from './calendar';
import { pluralize } from './pluralize';

const dayMonthFormatter = new Intl.DateTimeFormat('ru-RU', {
  day: 'numeric',
  month: 'long',
  timeZone: 'UTC',
});

const localDayMonthFormatter = new Intl.DateTimeFormat('ru-RU', {
  day: 'numeric',
  month: 'long',
});

const localTimeFormatter = new Intl.DateTimeFormat('ru-RU', {
  hour: '2-digit',
  minute: '2-digit',
});

/** Месяцы в сокращении без точки (индекс 0..11): словарь сокращений
 * живёт в каноне дат, не на местах (чипы периода фильтров). */
export const MONTH_SHORT: ReadonlyArray<string> = [
  'янв', 'фев', 'мар', 'апр', 'май', 'июн',
  'июл', 'авг', 'сен', 'окт', 'ноя', 'дек',
];

/**
 * Лейбл чипа применённого периода — формат диапазона чипов фильтров:
 * один день — «5 ноя», один месяц — «1 — 30 ноя», через месяцы — «10 окт —
 * 19 ноя», через годы — точечные границы «01.01.2025 — 01.01.2026». Канон
 * с фильтра операций (Figma 1506-72116, 1510-74149), чип периода истории —
 * тот же формат (макет 2067-162950, #711).
 */
export function formatIsoRangeChipLabel(range: IsoRange): string {
  const fromDay = Number(range.from.slice(8, 10));
  const fromMonth = Number(range.from.slice(5, 7));
  const toDay = Number(range.to.slice(8, 10));
  const toMonth = Number(range.to.slice(5, 7));
  if (isoYear(range.from) !== isoYear(range.to)) {
    return `${formatDottedDate(range.from)} — ${formatDottedDate(range.to)}`;
  }
  const fromLabel = `${fromDay} ${MONTH_SHORT[fromMonth - 1] ?? ''}`.trim();
  if (range.from === range.to) {
    return fromLabel;
  }
  if (fromMonth === toMonth) {
    return `${fromDay} — ${toDay} ${MONTH_SHORT[toMonth - 1] ?? ''}`.trim();
  }
  const toLabel = `${toDay} ${MONTH_SHORT[toMonth - 1] ?? ''}`.trim();
  return `${fromLabel} — ${toLabel}`;
}

/** День и склонённый месяц без года: «11 августа». */
export function formatDayMonth(iso: IsoDate): string {
  return dayMonthFormatter.format(fromIso(iso));
}

/** Тот же формат с годом вне текущего: «13 мая» / «13 мая, 2027». */
export function formatDayMonthWithYear(iso: IsoDate, today: IsoDate): string {
  const base = formatDayMonth(iso);
  if (isoYear(iso) === isoYear(today)) {
    return base;
  }
  return `${base}, ${isoYear(iso)}`;
}

/** День-месяц с годом всегда, даже в текущем: «11 августа, 2026» —
 * датовые группы «Истории операций» (канон 1302:52209, решение #802). */
export function formatDayMonthYear(iso: IsoDate): string {
  return `${formatDayMonth(iso)}, ${isoYear(iso)}`;
}

/** Месяцы в предложном падеже (индекс 0..11): заголовки «Операции в
 * сентябре» (#589). Словарь склонений живёт в каноне дат, не на местах. */
export const MONTH_PREPOSITIONAL: ReadonlyArray<string> = [
  'январе', 'феврале', 'марте', 'апреле', 'мае', 'июне',
  'июле', 'августе', 'сентябре', 'октябре', 'ноябре', 'декабре',
];

/** Русское склонение «день/дня/дней»: 1 день, 3 дня, 5 дней, 21 день. */
export function formatOverdueDays(days: number): string {
  return `${days} ${pluralize(Math.abs(days), 'день', 'дня', 'дней')}`;
}

/** Полноширинная дата «01.01.2025» — границы диапазона другого года. */
export function formatDottedDate(iso: IsoDate): string {
  return `${iso.slice(8, 10)}.${iso.slice(5, 7)}.${iso.slice(0, 4)}`;
}

/** Короткая точечная дата «01.01» без года — бейдж «Аренда с DD.MM» (#586). */
export function formatDayMonthDotted(iso: IsoDate): string {
  return `${iso.slice(8, 10)}.${iso.slice(5, 7)}`;
}

/** Подпись границы диапазона в пикере периода: текущий год — «1 ноября»
 * (склонённый месяц, как в строках списков), другой год — «01.01.2025». */
export function formatRangeBound(bound: IsoDate, today: IsoDate): string {
  return isoYear(bound) === isoYear(today) ? formatDayMonth(bound) : formatDottedDate(bound);
}

/** Заголовок-дата в шапке детали подписочного платежа (#624, Figma
 * 1883-71611): «10 августа 2026, 10:56» — год всегда, локальное время
 * смотрящего (метка момента, не календарный день; иначе крайние часы
 * суток показывали бы чужую дату). Невалидная строка даёт «—», как
 * formatDate легаси-профиля. */
export function formatDateTimeHeading(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '—';
  return `${localDayMonthFormatter.format(date)} ${date.getFullYear()}, ${localTimeFormatter.format(date)}`;
}

/** Время момента «14:40» по локальным часам смотрящего — метка момента в
 * строках ленты уведомлений (#744, Figma 2329-149013; тот же довод
 * локального времени, что у formatDateTimeHeading). Невалидная строка
 * даёт «—». */
export function formatTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '—';
  return localTimeFormatter.format(date);
}

/** Момент «15 сентября, 14:40» — секция «Категория + дата-время» страницы
 * уведомления (#745, Figma 2333:184048; время на странице остаётся всегда
 * — решение владельца 17.09.2026, чарт карты #734): локальные день-месяц и
 * время смотрящего, без года. Невалидная строка даёт «—». */
export function formatDayMonthTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '—';
  return `${localDayMonthFormatter.format(date)}, ${localTimeFormatter.format(date)}`;
}

/** Момент последней активности сессии в списке устройств (#730, мок
 * 1804-105061): «14 августа, 14:41»; год — только вне текущего
 * («14 августа 2025, 09:08»), как у formatDayMonthWithYear. Локальное
 * время смотрящего — та же семантика метки момента, что у
 * formatDateTimeHeading; «сейчас» — параметром для чистоты (дефолт —
 * факт). Невалидная строка даёт «—». */
export function formatSessionLastSeen(value: string, now: Date = new Date()): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '—';
  const dayMonth = localDayMonthFormatter.format(date);
  const time = localTimeFormatter.format(date);
  return date.getFullYear() === now.getFullYear()
    ? `${dayMonth}, ${time}`
    : `${dayMonth} ${date.getFullYear()}, ${time}`;
}
