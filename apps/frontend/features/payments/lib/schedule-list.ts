import type { IsoDate, PaymentOperation, PaymentSchedule } from '@/entities/payment';
import { addDays, occurrencesBetween, PROJECTION_HORIZON_DAYS } from '@/entities/payment';

/**
 * Единый список «Графика платежей» (резолюция #452): материализованные
 * сервером плановые вхождения — источник истины по существующим операциям
 * («Ближайший»), дальше — клиентская проекция портом occurrences
 * (#449, решение №7). Проекция продолжается строго после последней
 * материализованной даты, поэтому ближайшее вхождение списка всегда
 * совпадает с operations и дубликатов не возникает. Без материализованных
 * операций проекция идёт от «сегодня» включительно — честный вид до прогона
 * тика (воркер ежечасный, мутации тикают в транзакции, расхождение — краевой
 * случай).
 */
export type ScheduleEntry =
  | { readonly kind: 'operation'; readonly operation: PaymentOperation }
  | { readonly kind: 'projected'; readonly date: IsoDate };

export function buildScheduleList(
  schedule: PaymentSchedule,
  plannedOperations: ReadonlyArray<PaymentOperation>,
  today: IsoDate,
): ScheduleEntry[] {
  const entries: ScheduleEntry[] = plannedOperations.map((operation) => ({
    kind: 'operation',
    operation,
  }));

  const lastMaterialized = plannedOperations.at(-1);
  const projectionStart =
    lastMaterialized !== undefined ? addDays(lastMaterialized.date, 1) : today;
  const horizonEnd = schedule.endDate ?? addDays(today, PROJECTION_HORIZON_DAYS);
  // ISO-даты сравниваются лексикографически, как во всём порту.
  if (projectionStart > horizonEnd) {
    return entries;
  }
  const projected = occurrencesBetween(schedule, projectionStart, horizonEnd);
  return entries.concat(projected.map((date): ScheduleEntry => ({ kind: 'projected', date })));
}
