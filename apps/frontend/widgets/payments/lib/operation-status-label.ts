import { formatDayMonth, formatOverdueDays } from '@/entities/payment';
import type { IsoDate, PaymentOperation } from '@/entities/payment';

/**
 * Подзаголовок операции на плашке (Figma 1332:61665 «Transactions Button
 * States», правила владельца): оплачена раньше срока — «Заранее на N дней»,
 * позже срока — «Задержан на N дней» (оба серым), ровно в срок — дата оплаты
 * серым (решение владельца); просрочена и не оплачена — «на N дней» красным
 * (красноту и бейдж вешает вызывающий, lib остаётся чистым текстом).
 * Плановые операции подписи не получают — их дату рисует размещение.
 */
export type OperationStatusLabel = {
  readonly kind: 'early' | 'late' | 'ontime';
  readonly text: string;
};

export function operationStatusLabel(operation: PaymentOperation): OperationStatusLabel | null {
  if (operation.status === 'paid') {
    const paidDate = operation.paidDate;
    if (paidDate === undefined) return null;
    const planned = operation.date;
    if (paidDate < planned) {
      return {
        kind: 'early',
        text: `Заранее на ${formatOverdueDays(daysBetween(paidDate, planned))}`,
      };
    }
    if (paidDate > planned) {
      return {
        kind: 'late',
        text: `Задержан на ${formatOverdueDays(daysBetween(planned, paidDate))}`,
      };
    }
    return { kind: 'ontime', text: formatDayMonth(paidDate) };
  }
  return null;
}

/** Полных дней между date-строками (a < b — положительный результат). */
function daysBetween(a: IsoDate, b: IsoDate): number {
  const aMs = Date.UTC(
    Number(a.slice(0, 4)),
    Number(a.slice(5, 7)) - 1,
    Number(a.slice(8, 10)),
  );
  const bMs = Date.UTC(
    Number(b.slice(0, 4)),
    Number(b.slice(5, 7)) - 1,
    Number(b.slice(8, 10)),
  );
  return Math.round((bMs - aMs) / 86_400_000);
}
