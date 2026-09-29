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

/** Порция keyset-ленты: строки плюс keyset-продолжение — opaque-курсор
 * следующей порции, null = порций больше нет. */
export type KeysetPage<TItem> = {
  readonly items: TItem[];
  readonly nextCursor: string | null;
};

/**
 * Конфиг keyset-обхода ленты — общая форма экспортируемых конфигов api-слоя
 * фич (экспорты features/ несут явные возвращаемые типы,
 * apps/frontend/AGENTS.md): ключ среза, fetch порции по keyset-курсору,
 * стартовое отсутствие курсора и правило продолжения keysetNextPageParam.
 */
export type KeysetPagedQueryConfig<
  TKey extends ReadonlyArray<unknown>,
  TPage,
> = {
  readonly queryKey: TKey;
  readonly queryFn: (context: {
    readonly pageParam?: string;
  }) => Promise<TPage>;
  readonly initialPageParam: string | undefined;
  readonly getNextPageParam: typeof keysetNextPageParam;
};
