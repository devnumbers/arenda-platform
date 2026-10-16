import type { PickerMenuGroup } from '@/shared/ui/design';

/** Направление чип-сортировки — общий тип трёх списков раздела
 * («Ваши участники» #697, «Участники объекта» #700, «Объекты
 * пользователей» #701). */
export type ParticipantSortOrder = 'asc' | 'desc';

const ruCollator = new Intl.Collator('ru');

/** Русская коллаторная сравнилка без регистра — единственный экземпляр
 * правила сортировки раздела (прецедент contactSortByName). */
export function compareRuText(a: string, b: string): number {
  return ruCollator.compare(a.toLowerCase(), b.toLowerCase());
}

/** Сортировка копии по строковому ключу: русская коллация, asc
 * идемпотентен, desc переворачивает; вход не мутируется. */
export function sortByRuText<T>(
  rows: ReadonlyArray<T>,
  text: (row: T) => string,
  order: ParticipantSortOrder,
): T[] {
  return [...rows].sort(
    (a, b) => compareRuText(text(a), text(b)) * (order === 'asc' ? 1 : -1),
  );
}

/** Клиентский поиск-подстрока по вычисленному тексту строки: подстрока
 * без регистра, пустой (после trim) запрос возвращает копию списка.
 * Объём мал — серверного ?search= у списков раздела нет (канон
 * серверного поиска #601 про большие ленты). */
export function filterByRuQuery<T>(
  rows: ReadonlyArray<T>,
  rowText: (row: T) => string,
  query: string,
): T[] {
  const needle = query.trim().toLowerCase();
  if (needle.length === 0) {
    return [...rows];
  }
  return rows.filter((row) => rowText(row).toLowerCase().includes(needle));
}

/** Группы пикера чип-сортировки (канон PickerMenu, как книга контактов):
 * поле — единственное, выбирается направление, применяется сразу. */
export function sortOrderPickerGroups(
  fieldLabel: string,
  order: ParticipantSortOrder,
  onChange: (order: ParticipantSortOrder) => void,
): ReadonlyArray<PickerMenuGroup> {
  return [
    {
      options: [{ label: fieldLabel, selected: true, onSelect: () => undefined }],
    },
    {
      options: [
        { label: 'Возрастание', selected: order === 'asc', onSelect: () => onChange('asc') },
        { label: 'Убывание', selected: order === 'desc', onSelect: () => onChange('desc') },
      ],
    },
  ];
}
