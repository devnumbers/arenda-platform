import type { IsoDate, PaymentOperation, PaymentSchedule } from '@/entities/payment';
import { addDays, nextOccurrencesAfter } from '@/entities/payment';

/**
 * Ленивая проекция «Графика платежей» (резолюция #452, решение владельца —
 * без потолка): материализованные сервером плановые вхождения — источник
 * истины по существующим операциям («Ближайший»), дальше — клиентская
 * проекция портом occurrences (#449, решение №7) страницами по 50 при
 * скролле. Проекция продолжается строго после последней материализованной
 * даты, поэтому ближайшее вхождение списка всегда совпадает с operations
 * и дубликатов не возникает. Без материализованных операций проекция идёт
 * от «сегодня» включительно — честный вид до прогона тика (воркер
 * ежечасный, мутации тикают в транзакции, расхождение — краевой случай).
 *
 * Бессрочное правило догружается бесконечно: страницы генерируются на лету
 * от курсора, массив держит только проскролленное; состояние живёт в
 * компоненте экрана и умирает с ним, серверные порции в react-query
 * освобождаются штатным GC после ухода со страницы.
 */
export type ScheduleEntry =
  | { readonly kind: 'operation'; readonly operation: PaymentOperation }
  | { readonly kind: 'projected'; readonly date: IsoDate };

export function materializedEntries(
  plannedOperations: ReadonlyArray<PaymentOperation>,
): ScheduleEntry[] {
  return plannedOperations.map((operation) => ({ kind: 'operation', operation }));
}

/** Курсор первой страницы проекции: строго после последней
 * материализованной даты, а без них — после дня перед «сегодня». */
export function projectionCursor(
  plannedOperations: ReadonlyArray<PaymentOperation>,
  today: IsoDate,
): IsoDate {
  const lastMaterialized = plannedOperations.at(-1);
  return lastMaterialized !== undefined ? lastMaterialized.date : addDays(today, -1);
}

/** Страница проекции: до `count` дат строго после курсора и новый курсор;
 * дат меньше `count` — правило исчерпано (endDate позади). */
export function extendProjection(
  schedule: PaymentSchedule,
  cursor: IsoDate,
  count: number,
): { readonly dates: IsoDate[]; readonly nextCursor: IsoDate; readonly exhausted: boolean } {
  const dates = nextOccurrencesAfter(schedule, cursor, count);
  const last = dates.at(-1);
  return {
    dates,
    nextCursor: last ?? cursor,
    exhausted: dates.length < count,
  };
}
