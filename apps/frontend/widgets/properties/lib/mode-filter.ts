/** Страховочный срез клиентского поиска объектов (#586): без архивных —
 * бэк /properties их не возвращает (сервис #585), а архив с #587 живёт на
 * отдельном экране со своим запросом /properties/archive. */
export function filterActiveProperties<T extends { readonly status: string }>(
  items: readonly T[],
): T[] {
  return items.filter((property) => property.status !== 'archived');
}
