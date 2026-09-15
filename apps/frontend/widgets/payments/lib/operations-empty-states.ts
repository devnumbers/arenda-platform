import type { OperationsSummary } from '@/entities/payment';

/**
 * Пустые состояния экранов операций (#478): «операций еще не было» на
 * главном списке (Figma 1518-92899), направлениях (#571) и «Доходах/
 * Расходах объекта» (#679). Подпись пустого периода — общий
 * OperationsEmptyPeriod (#474), пустой выбор категорий — иллюстрация и
 * подпись прямо на странице категорий (Figma 1518-92530).
 */

/**
 * Признак «оплаченных операций не было никогда» (Figma 1518-92899):
 * all-time сводка (тот же контракт #473 без периода) загружена и пуста —
 * оба итога по нулям, разбивка без категорий. Недогруженная сводка —
 * не «пусто»: экран ещё показывает скелет, а не «Операций еще не было».
 */
export function hasNoPaidOperationsEver(
  summary: OperationsSummary | undefined,
): boolean {
  if (summary === undefined) {
    return false;
  }
  return (
    summary.incomeTotalKopecks === 0
    && summary.expenseTotalKopecks === 0
    && summary.categories.length === 0
  );
}
