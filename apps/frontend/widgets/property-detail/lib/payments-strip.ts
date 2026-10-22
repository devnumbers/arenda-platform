import type { IsoDate, Payment } from '@/entities/payment';
import { sortPaymentsByNextOccurrence } from '@/features/payments';

/**
 * Группы иконок секции «Регулярные платежи» на детали объекта (тикет
 * #589, правка владельца 11.09): «Автоплатежи» и «Платежи», круги
 * категорий с красной точкой накопленной просрочки (семантика точки —
 * payments/CONTEXT.md; идентификаторы платёж с просрочкой считает
 * вызывающий код из usePropertyOverdueOperations, как на «Платежах
 * объекта»). Потолки данных: обе группы — по 4, одна группа — 7;
 * сколько реально видно — решает ширина колонки (shared/lib/icon-row-fit,
 * решение владельца 22.10: невлезающие в один ряд не показываются,
 * поэтому на телефоне из четырёх видно три, на широких окнах — все
 * четыре). Порядок группы:
 * сначала платежи с накопленной просрочкой, затем по ближайшему вхождению.
 * Паузные правила — обычные круги без отличий (решение владельца 12.10,
 * #1353 — отмена правила #463: платёж на паузе не исчезает с экранов):
 * без долга дат у них нет, и они оседают в конце группы; с накопленным
 * долгом — остаются в просроченном кармане впереди, как все должники.
 */
export type PropertyPaymentGroup = {
  readonly label: 'Автоплатежи' | 'Платежи';
  /** Потолок данных группы (4 при обеих, 7 при одной) — для обрезки
   * по ширине колонки в компоненте. */
  readonly limit: number;
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
  const order = (group: Payment[]) => sortOverdueFirst(group, overduePaymentIds, today);
  const auto = order(payments.filter((payment) => payment.autoPay));
  const regular = order(payments.filter((payment) => !payment.autoPay));
  const limit = auto.length > 0 && regular.length > 0 ? BOTH_GROUPS_LIMIT : SINGLE_GROUP_LIMIT;
  const toItem = (payment: Payment) => ({
    payment,
    overdue: overduePaymentIds.has(payment.id),
  });
  const groups: PropertyPaymentGroup[] = [
    { label: 'Автоплатежи', limit, items: auto.slice(0, limit).map(toItem) },
    { label: 'Платежи', limit, items: regular.slice(0, limit).map(toItem) },
  ];
  return groups.filter((group) => group.items.length > 0);
}
