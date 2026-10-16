/**
 * Разбор строкового query-параметра адреса (pre-merge #785, находка
 * повторного гейта К5): строка возвращается как есть — пост-обработка
 * (trim, whitelist, sanitize) остаётся заботой вызова; отсутствующее и
 * массивное (битый дубликат вида ?role=a&role=b) — undefined. Та же
 * политика «массив = битое», что у parse-enum-param: ни один параметр
 * на серверных страницах не читается «первым элементом массива».
 */
export function parseStringParam(
  value: string | string[] | undefined,
): string | undefined {
  return typeof value === 'string' ? value : undefined;
}
