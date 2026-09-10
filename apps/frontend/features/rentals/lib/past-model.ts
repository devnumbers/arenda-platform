import { fullMonthsBetween, type IsoDate } from '@/shared/lib/calendar';
import { formatDayMonthWithYear } from '@/shared/lib/date-format';
import { pluralize } from '@/shared/lib/pluralize';
import type { PaymentOperationOrder } from '@/shared/api/query-keys';
import type { Rental } from '@/entities/rental';
import { rentAmountPerMonth, type RentalTermsRow } from './rental-view';

/**
 * Чистый рендер-слой «Прошлых аренд» (#535, Figma 1302:52462, 1550:94517):
 * карточки списка завершённых, заголовок детализации («24 месяца» либо
 * «Не было платежей»), строки карточки и порядковые номера платежей
 * истории. Только правила текстов — экраны и данные в слоях выше.
 */

function monthsWord(count: number): string {
  return pluralize(count, 'месяц', 'месяца', 'месяцев');
}

/** Завершённые аренды списка «Прошлые аренды»: сервер кладёт их после
 * незавершённой по дате завершения, свежие сверху (ADR 0053 §4) —
 * фильтр порядок сохраняет. */
export function completedRentalsOf(items: ReadonlyArray<Rental>): ReadonlyArray<Rental> {
  return items.filter((rental) => rental.status === 'completed');
}

/** Прожитые полные месяцы: от начала до фактической даты завершения;
 * та не задана (не бывает у завершённой, но тип nullable) — плановое
 * окончание, затем начало (ноль). */
export function completedRentalMonths(rental: Rental): number {
  const end: IsoDate = rental.completedDate ?? rental.plannedEndDate ?? rental.startDate;
  return fullMonthsBetween(rental.startDate, end);
}

/** Заголовок карточки списка: срок в месяцах — «24 месяца»; неполный
 * месяц — «Меньше месяца» (как срок итогов #534). */
export function pastRentalCardTitle(rental: Rental): string {
  const months = completedRentalMonths(rental);
  if (months <= 0) {
    return 'Меньше месяца';
  }
  return `${months} ${monthsWord(months)}`;
}

/** Заголовок детализации (H1): без оплаченных операций — «Не было
 * платежей» (1550:94517), иначе срок карточки. */
export function pastRentalTitle(rental: Rental, paidOperationsCount: number): string {
  if (paidOperationsCount === 0) {
    return 'Не было платежей';
  }
  return pastRentalCardTitle(rental);
}

/** Строки карточки списка (1302:52462): плата, начало, окончание —
 * сроки с годом; у бессрочной окончание «Не указано» (1550:94804). */
export function pastRentalRows(rental: Rental): ReadonlyArray<RentalTermsRow> {
  return [
    { label: 'Арендная плата', value: rentAmountPerMonth(rental.rentPayment.amountKopecks) },
    {
      label: 'Начало аренды',
      value: formatDayMonthWithYear(rental.startDate, rental.today),
    },
    {
      label: 'Окончание аренды',
      value:
        rental.plannedEndDate === null
          ? 'Не указано'
          : formatDayMonthWithYear(rental.plannedEndDate, rental.today),
    },
  ];
}

/** Порядковый номер платежа в истории (#535, «24-й платеж»): нумерация
 * по дате вхождения, сначала старые; итог оплаченных — progress.paidMonths
 * аренды, при рассинхроне номер не опускается ниже единицы. */
export function paidPaymentNumber(
  index: number,
  paidTotal: number,
  order: PaymentOperationOrder,
): number {
  const number = order === 'asc' ? index + 1 : paidTotal - index;
  return Math.max(1, number);
}

/** Подпись строки истории: «24-й платеж» — цифровое порядковое, «-й» для всех. */
export function paymentOrdinalLabel(number: number): string {
  return `${number}-й платеж`;
}
