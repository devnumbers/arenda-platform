/**
 * Разбор enum-значения query-параметра адреса (#785): строковое значение из
 * `allowed` возвращается как есть; отсутствующее, пустое, неизвестное и
 * массивное (битый дубликат вида ?sort=a&sort=b) — fallback. Одна политика
 * «битого» значения на платформу; историческое исключение — страница
 * «Объекты» (property-sort читает первый элемент массива), сойдётся при
 * миграции её парсера.
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
