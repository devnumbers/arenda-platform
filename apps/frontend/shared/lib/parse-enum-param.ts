/**
 * Разбор enum-значения query-параметра адреса (#785): строковое значение из
 * `allowed` возвращается как есть; отсутствующее, пустое, неизвестное и
 * массивное (битый дубликат вида ?sort=a&sort=b) — fallback. Одна политика
 * «битого» значения на платформу — с #786 на каноне все парсеры
 * sort/order (историческое исключение страницы «Объекты» — читала первый
 * элемент массива — сошлось).
 */
export function parseEnumParam<T extends string>(
  value: string | string[] | undefined,
  allowed: ReadonlyArray<T>,
  fallback: T,
): T {
  if (typeof value !== 'string') {
    return fallback;
  }
  return (allowed as ReadonlyArray<string>).includes(value) ? (value as T) : fallback;
}
