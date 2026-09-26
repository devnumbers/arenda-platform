/** Фильтрация по названию: регистронезависимое вхождение подстроки;
 * пустой (или из одних пробелов) запрос ничего не отфильтровывает.
 * Потребитель — CategoryStep (выбор категории: шаг «Какой платёж» визарда
 * создания платежа, визард создания операции, пикер в правке платежа). */
export function matchesTitleSearch(query: string, title: string): boolean {
  const needle = query.trim().toLowerCase();
  if (needle.length === 0) return true;
  return title.toLowerCase().includes(needle);
}
