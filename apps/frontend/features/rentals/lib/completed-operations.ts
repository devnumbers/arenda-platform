import type { PaymentOperationScope } from '@/shared/api/query-keys';
import type { Rental } from '@/entities/rental';

/**
 * Скоуп истории завершённой аренды (ревизия #1161): платёж удалён
 * Завершением, источником служат paid-операции объекта за период аренды
 * [начало, дата завершения] — тот же источник, что у «Итогов аренды»
 * (период фильтруется серверно по дате вхождения). Направление приходит
 * с экрана: детализация держит «сначала новые», полная история — чип.
 * Не найденная аренда даёт скоуп без периода — экран глушит запрос.
 */
export function completedRentalOperationsScope(
  rental: Rental | undefined,
  order: PaymentOperationScope['order'],
): PaymentOperationScope {
  return {
    status: 'paid',
    order,
    dateFrom: rental?.startDate,
    dateTo: rental?.completedDate ?? undefined,
  };
}
