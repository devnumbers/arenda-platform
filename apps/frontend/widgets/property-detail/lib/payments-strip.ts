import { isDatePaused } from '@/entities/payment';
import type { IsoDate, Payment } from '@/entities/payment';
import { sortPaymentsByNextOccurrence } from '@/features/payments';

/**
 * Группы иконок секции «Регулярные платежи» на детали объекта (тикет
 * #589, Figma 1185:40820): «Автоплатежи» и «Платежи», круги категорий
 * с красной точкой накопленной просрочки (семантика точки — payments/
 * CONTEXT.md; идентификаторы платёж с просрочкой считает вызывающий
 * код из usePropertyOverdueOperations, как на «Платежах объекта»).
 * Паузные правила не выводятся — их место на странице платежа
 * (решение #463); порядок внутри группы — по ближайшему вхождению.
 */
export type PropertyPaymentGroup = {
  readonly label: 'Автоплатежи' | 'Платежи';
  readonly items: ReadonlyArray<{
    readonly payment: Payment;
    readonly overdue: boolean;
  }>;
};

export function propertyPaymentGroups(
  payments: ReadonlyArray<Payment>,
  overduePaymentIds: ReadonlySet<string>,
  today: IsoDate,
): ReadonlyArray<PropertyPaymentGroup> {
  const active = payments.filter((payment) => !isDatePaused(payment.pauses, today));
  const toItem = (payment: Payment) => ({
    payment,
    overdue: overduePaymentIds.has(payment.id),
  });
  const groups: PropertyPaymentGroup[] = [
    {
      label: 'Автоплатежи',
      items: sortPaymentsByNextOccurrence(
        active.filter((payment) => payment.autoPay),
        today,
      ).map(toItem),
    },
    {
      label: 'Платежи',
      items: sortPaymentsByNextOccurrence(
        active.filter((payment) => !payment.autoPay),
        today,
      ).map(toItem),
    },
  ];
  return groups.filter((group) => group.items.length > 0);
}
