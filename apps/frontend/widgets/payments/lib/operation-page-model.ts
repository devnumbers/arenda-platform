import type { IsoDate, PaymentOperation } from '@/entities/payment';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { formatOverdueDays } from '@/entities/payment';
import { oldestUnpaidOperation } from '@/features/payments';
import { daysOverdue } from './overdue-days';

/**
 * Чистая модель страницы операции (Figma 1386:67731 / 1419:25859 /
 * 1419:25645 / 1444:66228): подпись под суммой, строка задержки, знаковая
 * сумма hero-блока и правило «которую операцию можно оплатить» — та же
 * логика, что у цели кнопки «Оплатить» страницы платежа
 * (oldestUnpaidOperation, история 25 спеки #453). Даты — date-строки,
 * «сегодня» приходит параметром (ADR 0048).
 */

/** Тон подписи под суммой: синий планового срока, красный просрочки. */
export type OperationSubtitle = {
  readonly text: string;
  readonly tone: 'primary' | 'danger';
};

/** Подпись под суммой (1419:25859): плановая — «N дней до оплаты» синим
 * (день наступления — «сегодня»), просроченная — «просрочена на N дней»
 * красным, у оплаченной подписи нет. */
export function operationSubtitle(
  operation: PaymentOperation,
  today: IsoDate,
): OperationSubtitle | null {
  if (operation.status === 'paid') {
    return null;
  }
  if (operation.status === 'overdue') {
    const days = daysOverdue(operation.date, today);
    return { text: `просрочена на ${formatOverdueDays(days)}`, tone: 'danger' };
  }
  const until = signedDaysBetween(today, operation.date);
  if (until <= 0) {
    return { text: 'сегодня', tone: 'primary' };
  }
  return { text: `${formatOverdueDays(until)} до оплаты`, tone: 'primary' };
}

/** Знаковых дней между date-строками (to − from); daysOverdue для плановой
 * даты не годится — он зажимает будущие даты нулём. */
function signedDaysBetween(from: IsoDate, to: IsoDate): number {
  const diff =
    Date.parse(`${to}T00:00:00Z`) - Date.parse(`${from}T00:00:00Z`);
  return Math.round(diff / 86_400_000);
}

/** Строка «Подробнее» про отклонение от срока у оплаченной операции:
 * позже срока — «Задержана на», раньше — «Заранее на», в срок — строки нет. */
export function operationDelayRow(
  operation: PaymentOperation,
): { readonly label: string; readonly text: string } | null {
  if (operation.status !== 'paid' || operation.paidDate === undefined) {
    return null;
  }
  const lateDays = daysOverdue(operation.date, operation.paidDate);
  if (lateDays > 0) {
    return { label: 'Задержана на', text: formatOverdueDays(lateDays) };
  }
  const earlyDays = daysOverdue(operation.paidDate, operation.date);
  if (earlyDays > 0) {
    return { label: 'Заранее на', text: formatOverdueDays(earlyDays) };
  }
  return null;
}

export type OperationHeroAmount = {
  readonly text: string;
  readonly tone: 'default' | 'success' | 'danger';
};

/** Сумма hero-блока (1419:25645 / 1444:66228): оплаченный доход — зелёная
 * с плюсом, расход — с минусом, просроченная — красная, остальное — тёмная
 * без знака. */
export function operationHeroAmount(operation: PaymentOperation): OperationHeroAmount {
  const money = formatMoneyKopecks(operation.amountKopecks);
  if (operation.status === 'overdue') {
    const sign = operation.type === 'expense' ? '−' : '';
    return { text: `${sign}${money}`, tone: 'danger' };
  }
  if (operation.type === 'income') {
    return operation.status === 'paid'
      ? { text: `+${money}`, tone: 'success' }
      : { text: money, tone: 'default' };
  }
  return { text: `−${money}`, tone: 'default' };
}

/** «Отметить оплаченной» стоит только у той операции, которую гасит кнопка
 * «Оплатить» страницы платежа: старейшая просрочка, без просрочек —
 * ближайшая плановая (та же oldestUnpaidOperation — решение владельца
 * «как сейчас логика устроена»). Пока списки не готовы, вызывающий кнопку
 * не показывает. */
export function isOperationPayable(
  operation: PaymentOperation,
  overdue: ReadonlyArray<PaymentOperation>,
  planned: ReadonlyArray<PaymentOperation>,
): boolean {
  if (operation.status === 'paid') {
    return false;
  }
  const payable = oldestUnpaidOperation(overdue, planned);
  return payable !== null && payable.id === operation.id;
}
