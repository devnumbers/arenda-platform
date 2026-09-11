import type { Rental } from '@/entities/rental';
import { formatOverdueDays } from '@/shared/lib/date-format';
import { pluralize } from '@/shared/lib/pluralize';
import {
  rentalElapsedLine,
  rentalNextPaymentLine,
  rentalProgressPercent,
  rentalRemainingLine,
} from '@/features/rentals';

/**
 * Модель заполненного блока «Аренда» на детали объекта (тикет #589,
 * Figma 1185:40820 — активная, 1581:53905 — срок подошёл к концу):
 * заголовок «Оплачено N из M платежей», синяя строка у календаря,
 * процент прогресс-бара и серый футер. Только правила текстов — данные
 * серверные (прогресс и «до платежа» считает бэк, ADR 0053).
 */
export type PropertyRentalBlockModel = {
  /** «Оплачено 6 из 24 платежей»; у бессрочной — «Оплачено N платежей». */
  readonly paidTitle: string;
  /** Синяя строка: дни до платежа / «Последний платеж оплачен»; null —
   * будущих вхождений нет, строка не рисуется. */
  readonly paymentLine: string | null;
  /** Процент прогресс-бара; у бессрочной бара нет (null). */
  readonly percent: number | null;
  /** Серый футер: «Осталось N месяцев аренды» / «Прошло N месяцев» /
   * «Срок аренды подошел к концу». */
  readonly footerLine: string;
  /** Срок подошёл к концу (needs_attention) — блок с кнопками
   * «Продлить»/«Завершить» (1581:53905). */
  readonly endOfTerm: boolean;
};

/** Существительное после «из N» — родительный падеж: «из 21 платежа»,
 * «из 24 платежей» (та же схема, что у месяцев в rental-view). */
function paymentsFromWord(count: number): string {
  const singularGenitive = count % 10 === 1 && count % 100 !== 11;
  return singularGenitive ? 'платежа' : 'платежей';
}

/** Дни до платежа (макеты 1425:55908 / 1185:40820): у ещё не оплаченного
 * первого платежа — «N дней до платежа», дальше — «до следующего». */
function paymentDaysLine(nextPayment: NonNullable<Rental['rentPayment']['nextPayment']>, paidMonths: number): string {
  const line = `${formatOverdueDays(nextPayment.daysUntil)}`;
  return paidMonths === 0 ? `${line} до платежа` : `${line} до следующего платежа`;
}

export function buildPropertyRentalBlock(rental: Rental): PropertyRentalBlockModel {
  const { progress, status, startDate } = rental;
  const paid = progress.paidMonths;

  const paidTitle =
    progress.totalMonths === null
      ? `Оплачено ${paid} ${pluralize(paid, 'платёж', 'платежа', 'платежей')}`
      : `Оплачено ${paid} из ${progress.totalMonths} ${paymentsFromWord(progress.totalMonths)}`;

  const endOfTerm = status === 'needs_attention';

  let paymentLine: string | null;
  if (endOfTerm) {
    paymentLine = 'Последний платеж оплачен';
  } else if (rental.rentPayment.nextPayment !== null) {
    const nextPayment = rental.rentPayment.nextPayment;
    paymentLine =
      nextPayment.daysUntil <= 0
        ? rentalNextPaymentLine(nextPayment)
        : paymentDaysLine(nextPayment, paid);
  } else {
    paymentLine = null;
  }

  let footerLine: string;
  if (endOfTerm) {
    footerLine = 'Срок аренды подошел к концу';
  } else {
    footerLine =
      rentalRemainingLine(progress.monthsRemaining) ??
      rentalElapsedLine(startDate, rental.today);
  }

  return {
    paidTitle,
    paymentLine,
    percent: rentalProgressPercent(progress),
    footerLine,
    endOfTerm,
  };
}
