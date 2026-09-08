import type { IsoDate, PaymentOperation } from '@/entities/payment';
import { formatDayMonthWithYear, formatOverdueDays } from '@/entities/payment';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { daysOverdue } from './overdue-days';

/**
 * Чистая модель страницы операции (Figma 1386:67731 / 1419:25859 /
 * 1419:25645 / 1444:66228): подпись под суммой, строка задержки, знаковая
 * сумма hero-блока. Правило «которую операцию можно оплатить» живёт
 * в features/payments (isOperationPayable, рядом с oldestUnpaidOperation).
 * Даты — date-строки, «сегодня» приходит параметром (ADR 0048).
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

/** Строка «Подробнее» про отклонение от срока (1419:25859 / 1419:25645):
 * просроченная и оплаченная позже срока — «Задержана на», оплаченная раньше —
 * «Заранее на», оплаченная в срок и плановая — строки нет. */
export function operationDelayRow(
  operation: PaymentOperation,
  today: IsoDate,
): { readonly label: string; readonly text: string } | null {
  if (operation.status === 'overdue') {
    return { label: 'Задержана на', text: formatOverdueDays(daysOverdue(operation.date, today)) };
  }
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

/** Строка секции «Подробнее» страницы операции; просрочка — красным. */
export type OperationDetailRow = {
  readonly label: string;
  readonly text: string;
  readonly danger?: boolean;
};

/** Строки секции «Подробнее». У операции правила — сравнение срока с
 * фактом (1419:25859 / 1419:25645): фактическая и плановая оплата,
 * отклонение, статус. У операции без правила — manual-факт (#569) или
 * хвост удалённого правила (1858-105181, #571): правила больше нет,
 * сравнивать не с чем — «Дата операции» и статус без строк задержки. */
export function operationDetailRows(
  operation: PaymentOperation,
  today: IsoDate,
): readonly OperationDetailRow[] {
  const status: OperationDetailRow =
    operation.status === 'overdue'
      ? { label: 'Статус', text: 'Просрочена', danger: true }
      : {
          label: 'Статус',
          text: operation.status === 'paid' ? 'Выполнена' : 'Запланирована',
        };

  if (operation.paymentId === null) {
    return [
      { label: 'Дата операции', text: formatDayMonthWithYear(operation.date, today) },
      status,
    ];
  }

  const rows: OperationDetailRow[] = [];
  if (operation.paidDate !== undefined) {
    rows.push({
      label: 'Фактическая оплата',
      text: formatDayMonthWithYear(operation.paidDate, today),
    });
  }
  rows.push({ label: 'Плановая оплата', text: formatDayMonthWithYear(operation.date, today) });
  const delay = operationDelayRow(operation, today);
  if (delay !== null) {
    rows.push({ label: delay.label, text: delay.text });
  }
  return [...rows, status];
}

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
