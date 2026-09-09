import type {
  GlobalPayment,
  PaymentSearchCategoryView,
} from '@/entities/payment';

/**
 * Модель экрана поиска платежей (#581, Figma 706:12168/12649/13008,
 * 862:24128, 860:22842). Поисковая область — серверный контракт
 * GET /payments/search (#575): подстрока по названию правила и категории;
 * чипы совпавших категорий приходят в ответе (matchedCategories, по числу
 * совпадений убывание). Выбор чипа сужает список на клиенте — серверного
 * фильтра по категории у поиска нет; сужение по паре (категория,
 * направление) — именно она и есть идентичность чипа.
 */

/** Чип секции «Категории» (Figma 860:22842): ключ-снапшот, подпись и флаг
 * выбора; тап сужает список «Платежей», повторный — снимает. */
export type SearchCategoryChip = {
  readonly key: string;
  readonly label: string;
  readonly selected: boolean;
};

/** Строки выдачи в свернутом списке (706:12649): три строки и «Показать
 * все»; раскрытое (706:13008) показывает всё с «Свернуть». */
export const SEARCH_ROWS_COLLAPSED_LIMIT = 3;

/** Идентичность чипа: дефолтная категория — по слагу каталога,
 * пользовательская — по id, плюс направление (доход/расход). */
export function searchCategoryChipKey(chip: PaymentSearchCategoryView): string {
  return `${categoryIdentityKey(chip.category.source, chip.category.slug, chip.category.id)}:${chip.type}`;
}

/** Ключ категории строки выдачи — та же идентичность, что у чипа. */
function paymentCategoryKey(payment: GlobalPayment): string {
  return `${categoryIdentityKey(payment.category.source, payment.category.slug, payment.category.id)}:${payment.type}`;
}

function categoryIdentityKey(
  source: 'default' | 'custom',
  slug?: string,
  id?: string,
): string {
  return source === 'default' ? `default:${slug ?? ''}` : `custom:${id ?? ''}`;
}

/** Чипы «Категорий» — matchedCategories ответа; порядок сервера
 * сохраняется, выбор помечает единственный чип. */
export function searchCategoryChips(
  categories: ReadonlyArray<PaymentSearchCategoryView>,
  selectedKey: string | null,
): ReadonlyArray<SearchCategoryChip> {
  return categories.map((chip) => {
    const key = searchCategoryChipKey(chip);
    return { key, label: chip.category.label, selected: key === selectedKey };
  });
}

/** Выбор валиден, только пока чип есть в разбивке текущего запроса: после
 * смены запроса исчезнувший чип молча перестаёт сужать список. */
export function effectiveChipKey(
  categories: ReadonlyArray<PaymentSearchCategoryView>,
  selectedKey: string | null,
): string | null {
  const known = selectedKey !== null
    && categories.some((chip) => searchCategoryChipKey(chip) === selectedKey);
  return known ? selectedKey : null;
}

/** Сужение выдачи выбранным чипом: пара (категория, направление). */
export function filterPaymentsByChip(
  items: ReadonlyArray<GlobalPayment>,
  chip: PaymentSearchCategoryView,
): ReadonlyArray<GlobalPayment> {
  const key = searchCategoryChipKey(chip);
  return items.filter((payment) => paymentCategoryKey(payment) === key);
}

/** Свернутый/раскрытый список «Платежей»: свернуто — первые три строки;
 * hasMore — показывать кнопку «Показать все»/«Свернуть» (только когда
 * выдача длиннее лимита). */
export function searchResultRows(
  items: ReadonlyArray<GlobalPayment>,
  expanded: boolean,
): { readonly rows: ReadonlyArray<GlobalPayment>; readonly hasMore: boolean } {
  const hasMore = items.length > SEARCH_ROWS_COLLAPSED_LIMIT;
  return {
    rows: expanded || !hasMore ? items : items.slice(0, SEARCH_ROWS_COLLAPSED_LIMIT),
    hasMore,
  };
}
