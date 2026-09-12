/**
 * Склейка порций бесконечных лент со страховочным дедупом по id (#597):
 * ключсет-пагинация сама не порождает повторов, но гонка смены ключа с
 * догрузкой или параллельная инвалидация могут принести одну и ту же
 * строку дважды — повторная порция поглощается, порядок склейки
 * сохраняется. Общее горло ленты операций и поиска платежей.
 */
export function flattenUniqueById<T extends { readonly id: string }>(
  pages: ReadonlyArray<ReadonlyArray<T>>,
): ReadonlyArray<T> {
  const seen = new Set<string>();
  const out: T[] = [];
  for (const page of pages) {
    for (const item of page) {
      if (!seen.has(item.id)) {
        seen.add(item.id);
        out.push(item);
      }
    }
  }
  return out;
}
