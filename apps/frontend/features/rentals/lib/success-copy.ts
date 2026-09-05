import { formatMoneyKopecks } from '@/shared/lib/format-money';
import type { RentalPaymentDay } from '@/entities/rental';
import { paymentDayPhrase } from './wizard-model';

/**
 * Тексты экрана успеха визарда аренды (Figma 1371:63753): заголовок
 * «Вы создали аренду», описание — день оплаты, сумма и способ отметки:
 * автоплатёж фиксирует оплату сам, иначе владелец отмечает вручную.
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
