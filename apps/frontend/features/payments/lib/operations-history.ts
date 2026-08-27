import type { IsoDate, PaymentOperation } from '@/entities/payment';
import { addDays, formatDayMonthWithYear } from '@/entities/payment';

/**
 * Группировка «Истории платежей» по датам (резолюция #452): «Сегодня»,
 * «Вчера», дальше «11 августа» (год добавляется вне текущего). Порядок групп
 * повторяет серверную сортировку входа (order asc/desc закреплён за API),
 * даты в странице монотонны — одна дата даёт ровно одну группу подряд.
 * «Сегодня»/«Вчера» — от клиентского «сегодня»
 * (entities/payment/lib/client-today): зоны смотрящего сервер не сообщает,
 * расхождение с TZ собственника ограничено краевыми часами суток.
 */
export type PaymentHistoryGroup = {
  readonly label: string;
  readonly operations: ReadonlyArray<PaymentOperation>;
};

export function groupPaidOperations(
  operations: ReadonlyArray<PaymentOperation>,
  today: IsoDate,
): ReadonlyArray<PaymentHistoryGroup> {
  const yesterday = addDays(today, -1);
  // Внутренне страницы мутабельны — группа дополняется подряд идущими
  // операциями той же даты; наружу тип отдаёт их только на чтение.
  const groups: { label: string; date: IsoDate; operations: PaymentOperation[] }[] = [];
  for (const operation of operations) {
    const current = groups.at(-1);
    if (current !== undefined && current.date === operation.date) {
      current.operations.push(operation);
      continue;
    }
    groups.push({
      label: historyGroupLabel(operation.date, today, yesterday),
      date: operation.date,
      operations: [operation],
    });
  }
  return groups;
}

function historyGroupLabel(date: IsoDate, today: IsoDate, yesterday: IsoDate): string {
  if (date === today) {
    return 'Сегодня';
  }
  if (date === yesterday) {
    return 'Вчера';
  }
  return formatDayMonthWithYear(date, today);
}
