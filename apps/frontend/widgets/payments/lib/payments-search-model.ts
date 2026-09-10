import type { PaymentSearchCategoryView } from '@/entities/payment';

/**
 * Модель экрана поиска платежей (#581, Figma 706:12168/12649/13008,
 * 862:24128, 860:22842). Поисковая область — серверный контракт
 * GET /payments/search (#575): подстрока по названию правила и категории;
 * чипы совпавших категорий приходят в ответе (matchedCategories, по числу
 * совпадений убывание). Выбор чипа сужает список серверным фильтром по
 * категории; направление в чип не входит (решение владельца: серверная
 * разбивка (категория, направление) склеивается — иначе чип-дубль
 * подписи, как «Парковка | Парковка»).
 */

/** Чип секции «Категории» (Figma 860:22842): ключ-снапшот, подпись и флаг
 * выбора; тап сужает список «Платежей», повторный — снимает. */
export type SearchCategoryChip = {
  readonly key: string;
  readonly label: string;
  readonly selected: boolean;
};

/** Идентичность чипа: дефолтная категория — по слагу каталога,
 * пользовательская — по id. Направление не входит. */
export function searchCategoryChipKey(chip: PaymentSearchCategoryView): string {
  return categoryIdentityKey(chip.category.source, chip.category.slug, chip.category.id);
}

function categoryIdentityKey(
  source: 'default' | 'custom',
  slug?: string,
  id?: string,
): string {
  return source === 'default' ? `default:${slug ?? ''}` : `custom:${id ?? ''}`;
}

/** Чипы «Категорий» — matchedCategories ответа; сервер считает их парой
 * (категория, направление), склеиваем по идентичности категории (первая
 * строка сервера — с большим числом совпадений — побеждает), порядок
 * сервера сохраняется, выбор помечает единственный чип. */
export function searchCategoryChips(
  categories: ReadonlyArray<PaymentSearchCategoryView>,
  selectedKey: string | null,
): ReadonlyArray<SearchCategoryChip> {
  const chips: SearchCategoryChip[] = [];
  const seen = new Set<string>();
  for (const view of categories) {
    const key = searchCategoryChipKey(view);
    if (seen.has(key)) continue;
    seen.add(key);
    chips.push({ key, label: view.category.label, selected: key === selectedKey });
  }
  return chips;
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

/** Фильтр чипа для серверного запроса (#581, порции по 50 + догрузка
 * скроллом): категория — слаг дефолтного каталога или id пользовательской,
 * направление не фильтруется. Сужает только список «Платежей»; чипы
 * сервер всегда считает по всему скоупу. */
export function searchChipFilter(chip: PaymentSearchCategoryView): {
  readonly category: string;
} {
  return {
    category:
      chip.category.source === 'default' ? (chip.category.slug ?? '') : (chip.category.id ?? ''),
  };
}
