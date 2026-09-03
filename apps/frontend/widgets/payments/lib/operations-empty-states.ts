import type { OperationsSummary, PaymentType } from '@/entities/payment';
import { formatMoneyKopecks } from '@/shared/lib/format-money';

/**
 * Пустые состояния экранов операций (#478): «операций еще не было» на
 * главном списке (Figma 1518-92899) и заголовок направления при пустом
 * периоде (Figma 1510-76177, 1510-75650). Подпись пустого периода —
 * общий OperationsEmptyPeriod (#474), пустой выбор категорий —
 * иллюстрация и подпись прямо на странице категорий (Figma 1518-92530).
 */

/** Заглушка в слоте суммы, пока сводка не загружена. */
const PENDING_HEADLINE = '—';

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

/**
 * H1 экрана «Доходы/Расходы объекта» (#475): пустой период — «Нет
 * доходов»/«Нет трат» вместо «0 ₽» (Figma 1510-76177, 1510-75650),
 * непустой — сумма направления, недогруженная сводка — прочерк.
 */
export function operationsTypeHeadline(
  type: PaymentType,
  totalKopecks: number | undefined,
): string {
  if (totalKopecks === undefined) {
    return PENDING_HEADLINE;
  }
  if (totalKopecks === 0) {
    return type === 'income' ? 'Нет доходов' : 'Нет трат';
  }
  return formatMoneyKopecks(totalKopecks);
}
