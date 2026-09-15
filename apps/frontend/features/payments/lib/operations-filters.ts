import type { IsoDate, OperationsCategorySummary } from '@/entities/payment';
import { addDays, inclusiveDays } from '@/entities/payment';
import { lastDayOfMonth } from '@/shared/lib/calendar';
import { formatDottedDate } from '@/shared/lib/date-format';
import { pluralize } from '@/shared/lib/pluralize';
import { safeInternalPath } from '@/shared/lib/safe-internal-path';
import {
  operationsMonthOf,
  operationsMonthRange,
  shiftOperationsMonth,
} from './operations-month';

/**
 * Куда возвращаться со страниц выбора фильтров (#477): только маршруты
 * операций того же объекта (страница открывается разными списками —
 * главным и направлениями), иначе — главный список.
 */
export function resolveFilterReturnPath(raw: string | null, propertyId: string): string {
  const prefix = `/properties/${propertyId}/operations`;
  const candidate = safeInternalPath(raw);
  if (candidate !== null && candidate.startsWith(prefix)) {
    return candidate;
  }
  return prefix;
}

/**
 * Фильтры период/категории экранов «Операции объекта» (#477, Figma
 * 1495-64015, 1502-66060, 1502-66758, 1506-72116, 1510-74149): период —
 * включительные границы диапазона, категории — слаги из сводки (контракт
 * `category` списка операций #473). Состояние живёт в адресе
 * (?from=&to=&category=) — шарабельные ссылки, назад по истории возвращает
 * к списку; дефолт (без параметров) на главном экране операций объекта —
 * весь период (#674, карта #669), «Все категории». До своих тикетов
 * (#675/#676) экраны направлений/категорий объекта держат месячный дефолт
 * в данных.
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

/**
 * Текущий календарный месяц «сегодня» (клиентское, та же оговорка про TZ,
 * что в operations-month) — переходный дефолт экранов направлений/категорий
 * объекта до своих тикетов (#675/#676); главный экран операций объекта
 * дефолт-месяц больше не применяет (#674), сводка «за месяц» на странице
 * объекта — отдельный потребитель (#473).
 */
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

/** Календарно корректная ISO-дата (не «2026-13-40»). */
function isRealIsoDate(iso: string): boolean {
  if (!ISO_DATE_RE.test(iso)) {
    return false;
  }
  const { year, month, day } = isoParts(iso);
  return month >= 1 && month <= 12 && day >= 1 && day <= lastDayOfMonth(year, month - 1);
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
 * Листание периода стрелками экранов направлений (#475) — период сдвигается
 * на свою же длину (решение владельца, #472): 3–14 августа при листании
 * назад становится 22 июля — 2 августа, окна стыкуются без нахлёста и дыр;
 * один день листается по одному дню. Целый календарный месяц листается
 * соседним месяцем (1–30 сентября → 1–31 августа), чтобы дефолтные экраны
 * не «плыли» по дням.
 */
export function shiftOperationsPeriod(period: OperationsPeriod, delta: number): OperationsPeriod {
  const from = isoParts(period.from);
  const to = isoParts(period.to);
  const wholeMonth =
    from.day === 1
    && from.year === to.year
    && from.month === to.month
    && to.day === lastDayOfMonth(to.year, to.month - 1);
  if (wholeMonth) {
    const shifted = operationsMonthRange(
      shiftOperationsMonth(operationsMonthOf(period.from), delta),
    );
    return { from: shifted.from, to: shifted.to };
  }
  const length = inclusiveDays(period.from, period.to);
  return { from: addDays(period.from, length * delta), to: addDays(period.to, length * delta) };
}

/**
 * Ссылка на список операций с текущими фильтрами (#472): период и
 * категории переживают переход между списками (главный ↔ направления) —
 * решению владельца о несбрасываемых фильтрах. Дефолтные фильтры дают
 * чистую базу без query, как и в operationsFiltersParams.
 */
export function operationsFiltersHref(
  base: string,
  filters: OperationsFilters,
): string {
  const query = new URLSearchParams(operationsFiltersParams(filters)).toString();
  return query.length > 0 ? `${base}?${query}` : base;
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
    return `${formatDottedDate(period.from)} — ${formatDottedDate(period.to)}`;
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
 * Лейбл чипа периода глобальной ленты (#670): без применённого периода —
 * нейтральный «Период» (дефолт «весь период»), с применённым — формат
 * диапазона, как operationsPeriodRangeChipLabel.
 */
export function operationsPeriodChipLabel(period: OperationsPeriod | null): string {
  return period !== null ? operationsPeriodRangeChipLabel(period) : 'Период';
}

/**
 * Черновик выбора и логика тапов диапазона переехали в общий канон пикера
 * (shared/lib/calendar: pickIsoRange/settleIsoRange/booleanRunSegments,
 * подписи границ — formatRangeBound в shared/lib/date-format) вместе с
 * переводом фильтра периода на CalendarRangePicker (решение владельца
 * 2026-09-04).
 */

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

/** Русское склонение: 1 категория, 2/3/4 категории, 5+ и 11–14 категорий —
 * через общий pluralize (унификация 2026-09-04). */
function categoriesPlural(count: number): string {
  return pluralize(Math.abs(count), 'категория', 'категории', 'категорий');
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
