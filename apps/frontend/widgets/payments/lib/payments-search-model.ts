import type { PaymentCategoryView } from '@/entities/payment';

/**
 * Модель экрана поиска платежей (#581, Figma 706:12168/12649/13008,
 * 862:24128, 860:22842). Поисковая область — серверный контракт
 * GET /payments/search (#575): подстрока по названию правила и категории;
 * чипы совпавших категорий приходят в ответе (matchedCategories, по числу
 * совпадений убывание). Выбор чипа сужает список серверным фильтром по
 * категории; направление в чип не входит (решение владельца: серверная
 * разбивка (категория, направление) склеивалась — иначе чип-дубль
 * подписи, как «Парковка | Парковка»; с #602 сервер отдаёт и вовсе одну
 * строку на категорию — без направления и счётчика).
 */

/** Чип секции «Категории» (Figma 860:22842): ключ-снапшот, подпись и флаг
 * выбора; тап сужает список «Платежей», повторный — снимает. */
export type SearchCategoryChip = {
  readonly key: string;
  readonly label: string;
  readonly selected: boolean;
};

/** Идентичность чипа: дефолтная категория — по слагу каталога,
 * пользовательская — по id. */
export function searchCategoryChipKey(category: PaymentCategoryView): string {
  return categoryIdentityKey(category.source, category.slug, category.id);
}

function categoryIdentityKey(
  source: 'default' | 'custom',
  slug?: string,
  id?: string,
): string {
  return source === 'default' ? `default:${slug ?? ''}` : `custom:${id ?? ''}`;
}

/** Чипы «Категорий» — matchedCategories ответа: по одному на категорию
 * (#602), порядок сервера (по числу совпадений) сохраняется, выбор
 * помечает единственный чип. */
export function searchCategoryChips(
  categories: ReadonlyArray<PaymentCategoryView>,
  selectedKey: string | null,
): ReadonlyArray<SearchCategoryChip> {
  return categories.map((category) => {
    const key = searchCategoryChipKey(category);
    return { key, label: category.label, selected: key === selectedKey };
  });
}

/** Выбор валиден, только пока чип есть в выдаче текущего запроса: после
 * смены запроса исчезнувший чип молча перестаёт сужать список. */
export function effectiveChipKey(
  categories: ReadonlyArray<PaymentCategoryView>,
  selectedKey: string | null,
): string | null {
  const known = selectedKey !== null
    && categories.some((category) => searchCategoryChipKey(category) === selectedKey);
  return known ? selectedKey : null;
}

/** Фильтр чипа для серверного запроса (#581, порции по 50 + догрузка
 * скроллом): категория — слаг дефолтного каталога или id пользовательской.
 * Сужает только список «Платежей»; чипы сервер всегда считает по всему
 * скоупу. */
export function searchChipFilter(category: PaymentCategoryView): {
  readonly category: string;
} {
  return {
    category: category.source === 'default' ? (category.slug ?? '') : (category.id ?? ''),
  };
}
