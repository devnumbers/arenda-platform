import type { IsoDate, PaymentOperation } from '@/entities/payment';
import { addDays, formatDayMonth, formatDayMonthWithYear } from '@/entities/payment';
import { formatDayMonthYear } from '@/shared/lib/date-format';
import { parseEnumParam } from '@/shared/lib/parse-enum-param';

/**
 * Группировка «Истории платежей» по датам (резолюция #452): «Сегодня»,
 * «Вчера», дальше датовые группы с годом всегда — «11 августа, 2026»
 * (канон 1302:52209, решение #802 23.09). Порядок групп
 * повторяет серверную сортировку входа (order asc/desc закреплён за API),
 * даты в странице монотонны — одна дата даёт ровно одну группу подряд.
 * «Сегодня»/«Вчера» — от клиентского «сегодня»
 * (dateToIsoLocal, shared/lib/calendar): зоны смотрящего сервер не сообщает,
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
  return parseEnumParam(order, ['asc', 'desc'], DEFAULT_HISTORY_ORDER);
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
  // Ключ группы — плановая дата вхождения: история платежа сортируется по
  // плановой (дефолт контракта, #992) и остаётся планировочной поверхностью
  // — досрочно оплаченное будущее вхождение остаётся в дате своего периода
  // (учёт, не касса; решение #466, подтверждено картой #990).
  return groupConsecutiveByDateKey(operations, today, historyGroupLabel, (operation) => operation.date);
}

/**
 * Группировка списков «Операций объекта» (#474, Figma 1492-41825): тот же
 * обход, лейблы дня — с датой через запятую: «Сегодня, 10 ноября»,
 * «Вчера, 9 ноября», дальше «1 ноября» («10 декабря, 2025» вне текущего
 * года). Ключ группы — фактическая дата оплаты (решение #933/#994):
 * операционные ленты сортируются по `paid_date` (#992), плановые даты
 * оплаченных наперёд месяцев немонотонны — группировка по ним развалила бы
 * ленту на повторяющиеся группы; банковский порядок собирает один день
 * оплаты одной группой подряд. `paidDate` гарантирован paid-веткой
 * контракта (CHECK paid ⟺ paid_date NOT NULL); запасной ключ — плановая.
 * «Клиентское сегодня» — та же оговорка, что в истории.
 */
export function groupOperationsByDate(
  operations: ReadonlyArray<PaymentOperation>,
  today: IsoDate,
): ReadonlyArray<PaymentHistoryGroup> {
  return groupConsecutiveByDateKey(operations, today, operationsGroupLabel, paidDateKey);
}

/** Дата-ключ операционной ленты: факт оплаты; плановой строки в скоупах
 * операций нет (статусный фильтр paid), запасной ключ — плановая дата. */
function paidDateKey(operation: PaymentOperation): IsoDate {
  return operation.paidDate ?? operation.date;
}

/** Общий обход: подряд идущие операции с одним дата-ключом складываются
 * в группу, лейбл решает стиль подписи, селектор выбирает ключ группы —
 * плановая дата (истории платежей) или факт оплаты (операционные ленты).
 * Внутри группы мутабельны — наружу тип отдаёт их только на чтение. */
function groupConsecutiveByDateKey(
  operations: ReadonlyArray<PaymentOperation>,
  today: IsoDate,
  label: (date: IsoDate, today: IsoDate, yesterday: IsoDate) => string,
  key: (operation: PaymentOperation) => IsoDate,
): ReadonlyArray<PaymentHistoryGroup> {
  const yesterday = addDays(today, -1);
  const groups: { label: string; date: IsoDate; operations: PaymentOperation[] }[] = [];
  for (const operation of operations) {
    const operationDate = key(operation);
    const current = groups.at(-1);
    if (current !== undefined && current.date === operationDate) {
      current.operations.push(operation);
      continue;
    }
    groups.push({
      label: label(operationDate, today, yesterday),
      date: operationDate,
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
  // Канон групп истории — formatDayMonthYear из канона дат
  // (1302:52209, решение #802 23.09): год в датовой группе всегда.
  return formatDayMonthYear(date);
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
