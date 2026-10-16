/** Размер порции всех списков операций (правило платформы, резолюция #452:
 * по 50 + бесконечный скролл; серверный дефолт — те же 50). */
export const OPERATIONS_PAGE_SIZE = 50;

/**
 * Правило продолжения offset-ленты операций (резолюция #452): следующая
 * порция есть, пока последняя пришла полной; offset следующей — число
 * прочитанных страниц × размер порции. Одна форма вместо копии в каждом
 * paged-хуке (консолидация #633).
 */
export function operationsOffsetNextPageParam<T>(
  lastPage: ReadonlyArray<T>,
  allPages: ReadonlyArray<ReadonlyArray<T>>,
): number | undefined {
  return lastPage.length < OPERATIONS_PAGE_SIZE
    ? undefined
    : allPages.length * OPERATIONS_PAGE_SIZE;
}
