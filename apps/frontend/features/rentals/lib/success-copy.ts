import { fullMonthsBetween, type IsoDate } from '@/shared/lib/calendar';
import { formatDottedDate } from '@/shared/lib/date-format';
import { pluralize } from '@/shared/lib/pluralize';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import type { RentalPaymentDay } from '@/entities/rental';
import { paymentDayPhrase } from './wizard-model';

/**
 * Тексты экрана успеха визарда аренды (Figma 1371:63753): заголовок
 * «Вы создали аренду», описание — день оплаты, сумма и способ отметки:
 * автоплатёж фиксирует оплату сам, иначе владелец отмечает вручную.
 * Здесь же текст попапа успеха продления (#533, Figma 1550:93723).
 */

export type RentalSuccessCopyInput = {
  readonly paymentDay: RentalPaymentDay;
  readonly amountKopecks: number;
  readonly autoPay: boolean;
};

export type RentalSuccessCopy = {
  readonly heading: string;
  readonly description: string;
};

export function rentalSuccessCopy(input: RentalSuccessCopyInput): RentalSuccessCopy {
  return {
    heading: 'Вы создали аренду',
    description: `${paymentDayPhrase(input.paymentDay)} ${formatMoneyKopecks(
      input.amountKopecks,
    )}, ${input.autoPay ? 'фиксируется автоматически' : 'отмечается вручную'}`,
  };
}

/** Вход попапа успеха продления: старое и новое окончания. */
export type RentalExtendSuccessCopyInput = {
  readonly previousEnd: IsoDate;
  readonly newEnd: IsoDate;
};

/** Текст попапа успеха продления (#533, Figma 1550:93723): «Аренда продлена
 * еще на N месяцев до ДД.ММ.ГГГГ» — N полных календарных месяцев между
 * старым и новым окончанием. Продление короче полного месяца счётчика
 * не получает: «Аренда продлена до ДД.ММ.ГГГГ». */
export function rentalExtendSuccessCopy(input: RentalExtendSuccessCopyInput): string {
  const months = fullMonthsBetween(input.previousEnd, input.newEnd);
  if (months === 0) {
    return `Аренда продлена до ${formatDottedDate(input.newEnd)}`;
  }
  return `Аренда продлена еще на ${months} ${pluralize(
    months,
    'месяц',
    'месяца',
    'месяцев',
  )} до ${formatDottedDate(input.newEnd)}`;
}
