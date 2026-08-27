/** Поиск секций экрана «Платежи объекта» по названиям (спека #453, история 45):
 * регистронезависимое вхождение подстроки; пустой (или из одних пробелов)
 * запрос ничего не отфильтровывает. */
export function matchesTitleSearch(query: string, title: string): boolean {
  const needle = query.trim().toLowerCase();
  if (needle.length === 0) return true;
  return title.toLowerCase().includes(needle);
}
