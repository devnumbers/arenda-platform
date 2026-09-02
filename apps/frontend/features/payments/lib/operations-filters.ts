import type { IsoDate, OperationsCategorySummary } from '@/entities/payment';
import { formatDayMonth } from '@/entities/payment';
import { daysInMonth } from '@/shared/ui/design/month-grid';
import {
  operationsMonthIndex,
  operationsMonthIso,
  operationsMonthOf,
  operationsMonthRange,
  shiftOperationsMonth,
  type OperationsMonth,
} from './operations-month';

/**
 * Фильтры период/категории экранов «Операции объекта» (#477, Figma
 * 1495-64015, 1502-66060, 1502-66758, 1506-72116, 1510-74149): период —
 * включительные границы диапазона, категории — слаги из сводки (контракт
 * `category` списка операций #473). Состояние живёт в адресе
 * (?from=&to=&category=) — шарабельные ссылки, назад по истории возвращает
 * к списку; дефолт (без параметров) — текущий месяц, «Все категории».
 */

export type OperationsPeriod = {
  readonly from: IsoDate;
  readonly to: IsoDate;
};

/** Разобранные фильтры экрана: без периода в URL — дефолт (null). */
export type OperationsFilters = {
  readonly period: OperationsPeriod | null;
  readonly categories: ReadonlyArray<string>;
};

/** Дефолтный период — текущий календарный месяц «сегодня» (клиентское,
 * та же оговорка про TZ, что в operations-month). */
export function defaultOperationsPeriod(today: IsoDate): OperationsPeriod {
  const { from, to } = operationsMonthRange(operationsMonthOf(today));
  return { from, to };
}

/** Минимальный источник параметров — ReadonlyURLSearchParams Next ему
 * удовлетворяет; структурный тип держит модуль чистым для vitest. */
export type OperationsParamsSource = {
  readonly get: (name: string) => string | null;
};

const ISO_DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

const isoParts = (iso: IsoDate): { year: number; month: number; day: number } => ({
  year: Number(iso.slice(0, 4)),
  month: Number(iso.slice(5, 7)),
  day: Number(iso.slice(8, 10)),
});

const isoOfDay = (year: number, month: number, day: number): IsoDate =>
  operationsMonthIso({ year, month: month - 1 }, day);

/** Календарно корректная ISO-дата (не «2026-13-40»). */
function isRealIsoDate(iso: string): boolean {
  if (!ISO_DATE_RE.test(iso)) {
    return false;
  }
  const { year, month, day } = isoParts(iso);
  return month >= 1 && month <= 12 && day >= 1 && day <= daysInMonth(year, month - 1);
}

/**
 * Чтение фильтров из URL: битые даты, перевёрнутый или целиком будущий
 * период отбрасываются (экраны операций — только paid, будущего в скоупе
 * не бывает, резолюция #474), будущий хвост обрезается «сегодня».
 * Категории — непустые слаги без дублей.
 */
export function readOperationsFilters(
  params: OperationsParamsSource,
  today: IsoDate,
): OperationsFilters {
  const rawFrom = params.get('from');
  const rawTo = params.get('to');
  let period: OperationsPeriod | null = null;
  if (rawFrom !== null && rawTo !== null && isRealIsoDate(rawFrom) && isRealIsoDate(rawTo)) {
    const to = rawTo < today ? rawTo : today;
    if (rawFrom <= to) {
      period = { from: rawFrom, to };
    }
  }
  const categories: string[] = [];
  const seen = new Set<string>();
  for (const slug of (params.get('category') ?? '').split(',')) {
    const trimmed = slug.trim();
    if (trimmed.length > 0 && !seen.has(trimmed)) {
      seen.add(trimmed);
      categories.push(trimmed);
    }
  }
  return { period, categories };
}

/**
 * Параметры URL фильтров: явный выбор пишет from/to всегда (даже когда
 * совпадает с текущим месяцем — «выбрано вручную» видно в адресе), пустой
 * выбор каноничен дефолту. Категории — comma-list, как в контракте #473.
 */
export function operationsFiltersParams(filters: OperationsFilters): Record<string, string> {
  const result: Record<string, string> = {};
  if (filters.period !== null) {
    result.from = filters.period.from;
    result.to = filters.period.to;
  }
  if (filters.categories.length > 0) {
    result.category = filters.categories.join(',');
  }
  return result;
}

/**
 * Сдвиг периода на целые месяцы (стрелки листания экранов направлений
 * #475): день сохраняется и зажимается в короткий месяц, а граница —
 * последний день своего месяца — якорится к последнему дню результата,
 * поэтому полный месяц при листании остаётся полным месяцем.
 */
export function shiftOperationsPeriod(period: OperationsPeriod, delta: number): OperationsPeriod {
  const shiftBound = (iso: IsoDate): IsoDate => {
    const { year, month, day } = isoParts(iso);
    const shifted = shiftOperationsMonth({ year, month: month - 1 }, delta);
    const lastDay = daysInMonth(shifted.year, shifted.month);
    const isLastDayOfMonth = day === daysInMonth(year, month - 1);
    const shiftedDay = isLastDayOfMonth ? lastDay : Math.min(day, lastDay);
    return isoOfDay(shifted.year, shifted.month + 1, shiftedDay);
  };
  return { from: shiftBound(period.from), to: shiftBound(period.to) };
}

/** Сокращения месяцев для лейблов диапазона: «1 — 30 ноя». */
const MONTH_SHORT: ReadonlyArray<string> = [
  'янв',
  'фев',
  'мар',
  'апр',
  'май',
  'июн',
  'июл',
  'авг',
  'сен',
  'окт',
  'ноя',
  'дек',
];

const dottedDate = (iso: IsoDate): string => {
  const { year, month, day } = isoParts(iso);
  return `${String(day).padStart(2, '0')}.${String(month).padStart(2, '0')}.${year}`;
};

/** Лейбл чипа дефолтного периода — «Сентябрь 2026». */
export function operationsPeriodDefaultChipLabel(period: OperationsPeriod): string {
  return operationsMonthRange(operationsMonthOf(period.from)).label;
}

/**
 * Лейбл чипа выбранного периода — всегда формат диапазона, даже полный
 * месяц: «1 — 30 ноя» (Figma 1506-72116), через месяцы — «10 окт —
 * 19 ноя», один день — «5 ноя», через годы — «01.01.2025 — 01.01.2026»
 * (Figma 1510-74149).
 */
export function operationsPeriodRangeChipLabel(period: OperationsPeriod): string {
  const from = isoParts(period.from);
  const to = isoParts(period.to);
  if (from.year !== to.year) {
    return `${dottedDate(period.from)} — ${dottedDate(period.to)}`;
  }
  const fromLabel = `${from.day} ${MONTH_SHORT[from.month - 1] ?? ''}`.trim();
  if (period.from === period.to) {
    return fromLabel;
  }
  if (from.month === to.month) {
    return `${from.day} — ${to.day} ${MONTH_SHORT[to.month - 1] ?? ''}`.trim();
  }
  const toLabel = `${to.day} ${MONTH_SHORT[to.month - 1] ?? ''}`.trim();
  return `${fromLabel} — ${toLabel}`;
}

/**
 * Подпись границы периода в шите: текущий год — «1 ноября» (склонённый
 * месяц, как в строках списков), другой год — «01.01.2025».
 */
export function operationsPeriodBoundLabel(bound: IsoDate, today: IsoDate): string {
  return bound.slice(0, 4) === today.slice(0, 4) ? formatDayMonth(bound) : dottedDate(bound);
}

/** Черновик выбора в шите периода: граница без конца — диапазон не завершён. */
export type OperationsPeriodDraft = {
  readonly start: IsoDate;
  readonly end: IsoDate | null;
};

/** Черновик по применённому периоду — исходное состояние открытого шита. */
export function operationsPeriodDraftOf(period: OperationsPeriod): OperationsPeriodDraft {
  return { start: period.from, end: period.to };
}

/**
 * Тап по дню в календаре шита: до границы — новый старт; правее —
 * завершение диапазона; по завершённому — перезапуск с новой границы.
 */
export function pickOperationsPeriodDay(
  draft: OperationsPeriodDraft,
  day: IsoDate,
): OperationsPeriodDraft {
  if (draft.end !== null || day < draft.start) {
    return { start: day, end: null };
  }
  return { start: draft.start, end: day };
}

/** Завершённый период черновика: без конца — один день старта. */
export function settledOperationsPeriod(draft: OperationsPeriodDraft): OperationsPeriod {
  return { from: draft.start, to: draft.end ?? draft.start };
}

/**
 * Стек месяцев шита периода: от старта выбора (но не глубже окна 24
 * месяца от «сегодня») до текущего месяца плюс два приглушённых будущих —
 * будущее видно и недоступно (Figma 1495-64015: Ноябрь, Декабрь, Январь).
 */
export function operationsPeriodMonths(
  draft: OperationsPeriodDraft,
  today: IsoDate,
): ReadonlyArray<OperationsMonth> {
  const current = operationsMonthOf(today);
  const anchor = Math.min(
    operationsMonthIndex(operationsMonthOf(draft.start)),
    operationsMonthIndex(current) - 23,
  );
  const last = operationsMonthIndex(current) + 2;
  const anchorMonth: OperationsMonth = { year: Math.floor(anchor / 12), month: anchor % 12 };
  return Array.from({ length: last - anchor + 1 }, (_, index) =>
    shiftOperationsMonth(anchorMonth, index),
  );
}

/** Строка шита категорий: слаг собирает оба направления в одну сумму. */
export type OperationsCategoryRow = {
  readonly slug: string;
  readonly label: string;
  readonly totalKopecks: number;
};

/**
 * Строки шита «Выбрать категорию» (#477): сводка #473 отдаёт строки на
 * слаг+направление — шит показывает одну строку на категорию (правило
 * владельца: только категории с операциями за период). Порядок сервера
 * (по сумме убывание) сохраняется, суммы направлений складываются.
 */
export function operationsCategoryRows(
  categories: ReadonlyArray<OperationsCategorySummary>,
): ReadonlyArray<OperationsCategoryRow> {
  const bySlug = new Map<string, OperationsCategoryRow>();
  for (const category of categories) {
    const existing = bySlug.get(category.slug);
    bySlug.set(category.slug, {
      slug: category.slug,
      label: category.label,
      totalKopecks: (existing?.totalKopecks ?? 0) + category.totalKopecks,
    });
  }
  return [...bySlug.values()];
}

/** Русское склонение: 1 категория, 2/3/4 категории, 5+ и 11–14 категорий. */
function categoriesPlural(count: number): string {
  const mod100 = Math.abs(count) % 100;
  const mod10 = mod100 % 10;
  if (mod100 >= 11 && mod100 <= 14) {
    return 'категорий';
  }
  if (mod10 === 1) {
    return 'категория';
  }
  if (mod10 >= 2 && mod10 <= 4) {
    return 'категории';
  }
  return 'категорий';
}

/**
 * Лейбл чипа категорий на экранах (#477): пусто — «Все категории», одна —
 * имя из разбивки периода, несколько — счётчик. Слаг вне текущей разбивки
 * (период сменился) имени не имеет — счётчик вместо имени.
 */
export function operationsCategoryChipLabel(
  selected: ReadonlyArray<string>,
  rows: ReadonlyArray<OperationsCategoryRow>,
): string {
  if (selected.length === 0) {
    return 'Все категории';
  }
  if (selected.length === 1) {
    const row = rows.find((candidate) => candidate.slug === selected[0]);
    return row?.label ?? `1 ${categoriesPlural(1)}`;
  }
  return `${selected.length} ${categoriesPlural(selected.length)}`;
}

/**
 * Непрерывные отрезки подложки диапазона внутри одной недели календаря
 * шита периода: серые «пилюли» от границы до границы (Figma 1495-64015 —
 * ряды целиком в диапазоне и частичные ряды краёв). Индексы 0-based
 * включительно по колонкам недели.
 */
export function booleanRunSegments(
  flags: ReadonlyArray<boolean>,
): ReadonlyArray<readonly [number, number]> {
  const segments: Array<readonly [number, number]> = [];
  let runStart: number | null = null;
  for (const [index, flag] of flags.entries()) {
    if (flag && runStart === null) {
      runStart = index;
    }
    if ((!flag || index === flags.length - 1) && runStart !== null) {
      const runEnd = flag && index === flags.length - 1 ? index : index - 1;
      segments.push([runStart, runEnd]);
      runStart = null;
    }
  }
  return segments;
}
