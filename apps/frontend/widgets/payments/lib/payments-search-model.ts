import type {
  PaymentSearchCategoryView,
  PaymentType,
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

/** Идентичность чипа: дефолтная категория — по слагу каталога,
 * пользовательская — по id, плюс направление (доход/расход). */
export function searchCategoryChipKey(chip: PaymentSearchCategoryView): string {
  return `${categoryIdentityKey(chip.category.source, chip.category.slug, chip.category.id)}:${chip.type}`;
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

/** Фильтр чипа для серверного запроса (#581, доработка: порции по 50 +
 * догрузка скроллом): категория — слаг дефолтного каталога или id
 * пользовательской, направление — часть идентичности чипа. Сужает только
 * список «Платежей»; чипы сервер всегда считает по всему скоупу. */
export function searchChipFilter(chip: PaymentSearchCategoryView): {
  readonly category: string;
  readonly type: PaymentType;
} {
  return {
    category:
      chip.category.source === 'default' ? (chip.category.slug ?? '') : (chip.category.id ?? ''),
    type: chip.type,
  };
}
