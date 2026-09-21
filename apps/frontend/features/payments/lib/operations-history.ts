import type { IsoDate, PaymentOperation } from '@/entities/payment';
import { addDays, formatDayMonth, formatDayMonthWithYear } from '@/entities/payment';
import { parseEnumParam } from '@/shared/lib/parse-enum-param';

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
  /** Дата группы ('YYYY-MM-DD') — стабильный ключ секции списка. */
  readonly date: IsoDate;
  readonly label: string;
  readonly operations: ReadonlyArray<PaymentOperation>;
};

/** Направление «Истории операций» — экрана платежа (#466) и аренды (#535);
 * дефолт «сначала новые» (макеты). */
export type HistoryOrder = 'asc' | 'desc';

export const DEFAULT_HISTORY_ORDER: HistoryOrder = 'desc';

/** Разбор ?order= страниц истории (конвенция состояния в адресе):
 * неизвестное и отсутствующее значения — дефолт desc. Направление живёт
 * в адресе — переживает перезагрузку (#785). */
export function parseHistoryOrderParams(
  order: string | string[] | undefined,
): HistoryOrder {
  return parseEnumParam(order, ['asc', 'desc'], 'desc');
}

/** Собственный параметр направления в адресе — знание этого модуля;
 * писатель (useHistoryOrder) импортирует отсюда. */
export const HISTORY_ORDER_PARAMS = ['order'] as const;

/** Патч направления для адреса: дефолтные значения параметров не создают
 * (конвенция состояния в адресе); пишется через useUrlParams с
 * own: HISTORY_ORDER_PARAMS (#785). */
export function serializeHistoryOrderToParams(order: HistoryOrder): Record<string, string> {
  return order === DEFAULT_HISTORY_ORDER ? {} : { order };
}

export function groupPaidOperations(
  operations: ReadonlyArray<PaymentOperation>,
  today: IsoDate,
): ReadonlyArray<PaymentHistoryGroup> {
  return groupOperationsByLabel(operations, today, historyGroupLabel);
}

/**
 * Группировка списков «Операций объекта» (#474, Figma 1492-41825): тот же
 * обход, лейблы дня — с датой через запятую: «Сегодня, 10 ноября»,
 * «Вчера, 9 ноября», дальше «1 ноября» («10 декабря, 2025» вне текущего
 * года). Порядок групп и оговорка о «клиентском сегодня» — как в истории.
 */
export function groupOperationsByDate(
  operations: ReadonlyArray<PaymentOperation>,
  today: IsoDate,
): ReadonlyArray<PaymentHistoryGroup> {
  return groupOperationsByLabel(operations, today, operationsGroupLabel);
}

/** Общий обход: подряд идущие операции одной даты складываются в группу,
 * лейбл решает стиль подписи. Внутри группы мутабельны — наружу тип отдаёт
 * их только на чтение. */
function groupOperationsByLabel(
  operations: ReadonlyArray<PaymentOperation>,
  today: IsoDate,
  label: (date: IsoDate, today: IsoDate, yesterday: IsoDate) => string,
): ReadonlyArray<PaymentHistoryGroup> {
  const yesterday = addDays(today, -1);
  const groups: { label: string; date: IsoDate; operations: PaymentOperation[] }[] = [];
  for (const operation of operations) {
    const current = groups.at(-1);
    if (current !== undefined && current.date === operation.date) {
      current.operations.push(operation);
      continue;
    }
    groups.push({
      label: label(operation.date, today, yesterday),
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

function operationsGroupLabel(date: IsoDate, today: IsoDate, yesterday: IsoDate): string {
  if (date === today) {
    return `Сегодня, ${formatDayMonth(date)}`;
  }
  if (date === yesterday) {
    return `Вчера, ${formatDayMonth(date)}`;
  }
  return formatDayMonthWithYear(date, today);
}
