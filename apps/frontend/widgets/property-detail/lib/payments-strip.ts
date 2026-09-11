import { isDatePaused } from '@/entities/payment';
import type { IsoDate, Payment } from '@/entities/payment';
import { sortPaymentsByNextOccurrence } from '@/features/payments';

/**
 * Группы иконок секции «Регулярные платежи» на детали объекта (тикет
 * #589, правка владельца 11.09): «Автоплатежи» и «Платежи», круги
 * категорий с красной точкой накопленной просрочки (семантика точки —
 * payments/CONTEXT.md; идентификаторы платёж с просрочкой считает
 * вызывающий код из usePropertyOverdueOperations, как на «Платежах
 * объекта»). Лимиты: обе группы — по 4, одна группа — 7. Внутри группы
 * сначала платежи с просрочкой, затем по ближайшему вхождению.
 * Паузные правила не выводятся — их место на странице платежа
 * (решение #463).
 */
export type PropertyPaymentGroup = {
  readonly label: 'Автоплатежи' | 'Платежи';
  readonly items: ReadonlyArray<{
    readonly payment: Payment;
    readonly overdue: boolean;
  }>;
};

/** Обе группы — по 4; если группа одна — в ней до 7 (решение владельца). */
const BOTH_GROUPS_LIMIT = 4;
const SINGLE_GROUP_LIMIT = 7;

function sortOverdueFirst(
  payments: ReadonlyArray<Payment>,
  overduePaymentIds: ReadonlySet<string>,
  today: IsoDate,
): Payment[] {
  const overdue = payments.filter((payment) => overduePaymentIds.has(payment.id));
  const rest = sortPaymentsByNextOccurrence(
    payments.filter((payment) => !overduePaymentIds.has(payment.id)),
    today,
  );
  return [...overdue, ...rest];
}

export function propertyPaymentGroups(
  payments: ReadonlyArray<Payment>,
  overduePaymentIds: ReadonlySet<string>,
  today: IsoDate,
): ReadonlyArray<PropertyPaymentGroup> {
  const active = payments.filter((payment) => !isDatePaused(payment.pauses, today));
  const order = (group: Payment[]) => sortOverdueFirst(group, overduePaymentIds, today);
  const auto = order(active.filter((payment) => payment.autoPay));
  const regular = order(active.filter((payment) => !payment.autoPay));
  const limit = auto.length > 0 && regular.length > 0 ? BOTH_GROUPS_LIMIT : SINGLE_GROUP_LIMIT;
  const toItem = (payment: Payment) => ({
    payment,
    overdue: overduePaymentIds.has(payment.id),
  });
  const groups: PropertyPaymentGroup[] = [
    { label: 'Автоплатежи', items: auto.slice(0, limit).map(toItem) },
    { label: 'Платежи', items: regular.slice(0, limit).map(toItem) },
  ];
  return groups.filter((group) => group.items.length > 0);
}
