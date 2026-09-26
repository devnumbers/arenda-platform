import { fullMonthsBetween, type IsoDate } from '@/shared/lib/calendar';
import { formatDottedDate } from '@/shared/lib/date-format';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { monthsWord } from '@/shared/lib/months-word';
import type { RentalPaymentDay } from '@/entities/rental';
import { paymentDayPhrase } from './wizard-model';

/**
 * Тексты экрана успеха визарда аренды (Figma 1371:63753): заголовок
 * «Вы создали аренду», описание — день оплаты, сумма и способ отметки:
 * автоплатёж фиксирует оплату сам, иначе владелец отмечает вручную.
 * Здесь же текст тоста успеха продления (#533, Figma 1550:93664).
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

/** Вход тоста успеха продления: старое и новое окончания; у бессрочной
 * старого окончания нет — previousEnd null. */
export type RentalExtendSuccessCopyInput = {
  readonly previousEnd: IsoDate | null;
  readonly newEnd: IsoDate;
};

/** Текст тоста успеха продления (#533, Figma 1550:93664; попап 1550:93723
 * заменён тостом на детализации — решение #802 23.09): «Аренда продлена
 * еще на N месяцев до ДД.ММ.ГГГГ» — N полных календарных месяцев между
 * старым и новым окончанием. Без счётчика живут два случая: продление
 * короче полного месяца и продление бессрочной — первой задаётся первая
 * дата окончания, дельты нет (домен «Продление», rentals/CONTEXT.md). */
export function rentalExtendSuccessCopy(input: RentalExtendSuccessCopyInput): string {
  const months =
    input.previousEnd === null ? 0 : fullMonthsBetween(input.previousEnd, input.newEnd);
  if (months === 0) {
    return `Аренда продлена до ${formatDottedDate(input.newEnd)}`;
  }
  return `Аренда продлена еще на ${months} ${monthsWord(months)} до ${formatDottedDate(input.newEnd)}`;
}

/** Заголовок финального экрана завершения аренды (#534, Figma 1433:60975):
 * «Аренда объекта «Моя квартира» завершена». */
export function rentalCompletedTitle(propertyName: string): string {
  return `Аренда объекта «${propertyName}» завершена`;
}
