import type { PaymentOperationScope } from '@/shared/api/query-keys';
import type { OperationsCategorySummary } from '@/entities/payment';

/**
 * Модель экрана поиска операций (#476, Figma 1494-61633/61657/61679/63035):
 * серверная область запроса решена контрактом `search` списка операций —
 * подстрока по названию ИЛИ категории, а числовой запрос — ещё и по сумме;
 * скоуп поиска — только оплаченные операции без ограничения периода
 * (решение карты #472: экраны операций показывают только paid).
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
    search: query,
    ...(selectedSlug !== null ? { categories: [selectedSlug] } : {}),
  };
}

/** Скоуп сводки поиска: тот же запрос без сужения по чипу — чипы всегда
 * показывают все совпавшие категории запроса. */
export function searchSummaryScope(query: string): PaymentOperationScope {
  return { status: 'paid', order: 'desc', search: query };
}
