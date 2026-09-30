import { OPERATIONS_FEED_SORT, type GlobalOperationScope, type PaymentOperationScope } from '@/shared/api/query-keys';
import type { OperationsCategorySummary } from '@/entities/payment';
import type { OperationsPeriod } from '@/features/payments';

/**
 * Модель экрана поиска операций (#476, Figma 1494-61633/61657/61679/63035):
 * серверная область запроса решена контрактом `search` списка операций —
 * подстрока по названию ИЛИ категории, а числовой запрос — ещё и по сумме;
 * скоуп поиска — только оплаченные операции без ограничения периода
 * (решение карты #472). Глобальный поиск (#543) ищет тем же контрактом
 * `search`, но по всем видимым объектам и с фильтрами ленты из адреса
 * (#541): объекты (#542) и период (дефолт — весь период #673).
 * Поверхности операций читаются по фактической дате оплаты (решение
 * #933/#994): скоупы просят sort=paid_date (#992) — порядок, период и
 * сводка поиска совпадают с лентой.
 */

/** Чип секции «Категории» (Figma 1494-61657): подпись-снапшот и флаг
 * выбора; тап сужает список операций, повторный — снимает. */
export type SearchCategoryChip = {
  readonly slug: string;
  readonly label: string;
  readonly selected: boolean;
};

/** Чипы «Категорий» — разбивка сводки по поисковому запросу (категории с
 * совпавшими операциями, по сумме убывание). Выбор — одиночный: selectedSlug,
 * которого в разбивке нет (запрос изменился), молча ничего не выбирает. */
export function searchCategoryChips(
  categories: ReadonlyArray<OperationsCategorySummary>,
  selectedSlug: string | null,
): ReadonlyArray<SearchCategoryChip> {
  return categories.map((category) => ({
    slug: category.slug,
    label: category.label,
    selected: category.slug === selectedSlug,
  }));
}

/** Скоуп списка операций поиска: запрос серверу и выбранный чип-категория
 * (сужение — тем же контрактом `category`, что у экранов операций). */
export function searchListScope(
  query: string,
  selectedSlug: string | null,
): PaymentOperationScope {
  return {
    status: 'paid',
    order: 'desc',
    sort: OPERATIONS_FEED_SORT,
    search: query,
    ...(selectedSlug !== null ? { categories: [selectedSlug] } : {}),
  };
}

/** Скоуп сводки поиска: тот же запрос без сужения по чипу — чипы всегда
 * показывают все совпавшие категории запроса. */
export function searchSummaryScope(query: string): PaymentOperationScope {
  return { status: 'paid', order: 'desc', sort: OPERATIONS_FEED_SORT, search: query };
}

/** Скоуп списка глобального поиска (#543): фильтры ленты из адреса —
 * объекты (#542) и период (null — весь период #673: дат в скоупе нет,
 * поиск в дефолте ищет за всё время — прецедент объектного поиска) —
 * плюс поисковый запрос серверу и выбранный чип-категория (сужение тем же
 * контрактом `category`, что у объектного поиска). Пустой выбор объектов
 * проходит как есть — хук сам опускает пустой параметр (#541). */
export function globalSearchListScope(
  period: OperationsPeriod | null,
  propertyIds: ReadonlyArray<string>,
  query: string,
  selectedSlug: string | null,
): GlobalOperationScope {
  return {
    order: 'desc',
    sort: OPERATIONS_FEED_SORT,
    propertyIds,
    ...(period !== null ? { dateFrom: period.from, dateTo: period.to } : {}),
    search: query,
    ...(selectedSlug !== null ? { categories: [selectedSlug] } : {}),
  };
}

/** Скоуп сводки глобального поиска: тот же запрос ленты без сужения по
 * чипу — чипы всегда показывают все совпавшие категории запроса (сводка
 * категорийный фильтр не принимает — контракт #540). */
export function globalSearchSummaryScope(
  period: OperationsPeriod | null,
  propertyIds: ReadonlyArray<string>,
  query: string,
): GlobalOperationScope {
  return {
    order: 'desc',
    sort: OPERATIONS_FEED_SORT,
    propertyIds,
    ...(period !== null ? { dateFrom: period.from, dateTo: period.to } : {}),
    search: query,
  };
}
