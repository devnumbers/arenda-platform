/** Чистое ядро пикера (#505): поиск по списку опций и выбор опции по
 * значению. DOM-половина (триггер-поле и шит) — рядом в picker-field.tsx. */

/** Опция, доступная для поиска: совпадение ищется в названии и подсказке. */
export type PickerFilterable = {
  readonly label: string;
  readonly hint?: string;
};

/** Регистронезависимое вхождение подстроки запроса в название или
 * подсказку; запрос тримится, пустой (или из одних пробелов) запрос
 * пропускает всё. Возвращает новый массив, исходный не меняет. */
export function filterPickerOptions<T extends PickerFilterable>(
  options: ReadonlyArray<T>,
  query: string,
): Array<T> {
  const needle = query.trim().toLowerCase();
  if (needle.length === 0) {
    return [...options];
  }
  return options.filter(
    (option) =>
      option.label.toLowerCase().includes(needle) ||
      (option.hint ?? '').toLowerCase().includes(needle),
  );
}

/** Опция по значению; null/undefined (пикер очищен) и промах — undefined. */
export function pickerOptionByValue<T extends { readonly value: string }>(
  options: ReadonlyArray<T>,
  value: string | null | undefined,
): T | undefined {
  if (value === null || value === undefined) {
    return undefined;
  }
  return options.find((option) => option.value === value);
}
