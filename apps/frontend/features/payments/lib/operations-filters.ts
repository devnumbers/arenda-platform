import type { IsoDate, OperationsCategorySummary, PaymentType } from '@/entities/payment';
import { formatIsoRangeChipLabel } from '@/shared/lib/date-format';
import { readCsvParam } from '@/shared/lib/parse-csv-uuid-param';
import { readIsoRangeParam, type UrlParamsSource } from '@/shared/lib/parse-iso-range-param';
import { buildUrlWithParams } from '@/shared/lib/url-params';
import { pluralize } from '@/shared/lib/pluralize';
import { safeInternalPath } from '@/shared/lib/safe-internal-path';

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
 * к списку; дефолт (без параметров) на всех экранах операций объекта —
 * весь период (null, карта #669: главный #674, направления #675,
 * категории #676), «Все категории».
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
 * Минимальный источник параметров — структурный тип UrlParamsSource из
 * канона parse-iso-range-param; отдельное имя сохранено для публичных
 * сигнатур фичи.
 */
export type OperationsParamsSource = UrlParamsSource;

/**
 * Чтение фильтров из URL: период — канон readIsoRangeParam (битые даты,
 * перевёрнутый или целиком будущий период отбрасываются — экраны операций
 * только paid, будущего в скоупе не бывает, резолюция #474; будущий хвост
 * обрезается «сегодня»). Категории — канон readCsvParam без предиката:
 * непустые слаги без дублей, порядок первого появления сохранён.
 */
export function readOperationsFilters(
  params: OperationsParamsSource,
  today: IsoDate,
): OperationsFilters {
  return {
    period: readIsoRangeParam(params, 'from', 'to', today),
    categories: readCsvParam(params.get('category')),
  };
}

/**
 * Направление страниц выбора категории, открытых с направленческой ленты
 * (решение владельца 01.10): ?type=income|expense сужает разбивку категорий
 * сводки до направления (контракт #540 — type сужает только массив
 * категорий). Направление ленты живёт в пути, а не в фильтрах, поэтому
 * сюда оно приходит явным параметром; кривое значение и прямое открытие с
 * главной ленты дают undefined — разбивка без сужения. В apply параметр не
 * возвращается: сериализаторы фильтров его не пишут.
 */
export function readDirectionParam(params: OperationsParamsSource): PaymentType | undefined {
  const value = params.get('type');
  return value === 'income' || value === 'expense' ? value : undefined;
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
 * Ссылка на список операций с текущими фильтрами (#472): период и
 * категории переживают переход между списками (главный ↔ направления) —
 * решению владельца о несбрасываемых фильтрах. Дефолтные фильтры дают
 * чистую базу без query — правило канона buildUrlWithParams (#792,
 * «пустой query — голый адрес»), которому функция делегирует.
 */
export function operationsFiltersHref(
  base: string,
  filters: OperationsFilters,
): string {
  return buildUrlWithParams(base, new URLSearchParams(operationsFiltersParams(filters)));
}

/**
 * Лейбл чипа выбранного периода — формат канона дат formatIsoRangeChipLabel
 * (#711 поднял формат в shared/lib/date-format): даже полный месяц — «1 —
 * 30 ноя» (Figma 1506-72116), один день — «5 ноя», через годы — точечные
 * границы.
 */
export function operationsPeriodRangeChipLabel(period: OperationsPeriod): string {
  return formatIsoRangeChipLabel(period);
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
