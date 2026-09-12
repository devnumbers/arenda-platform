/**
 * Правило продолжения keyset-ленты: следующая порция есть, пока сервер
 * вернул opaque-курсор продолжения; null/undefined — лента исчерпана.
 * Общее горло всех бесконечных списков на keyset-курсорах (лента операций
 * #597, поиск платежей #575, книга контактов #600, поиск объектов #601) —
 * одна форма вместо копии в каждом хуке (консолидация #633).
 */
export function keysetNextPageParam(lastPage: {
  readonly nextCursor: string | null | undefined;
}): string | undefined {
  return lastPage.nextCursor ?? undefined;
}
