import {
  formatDayMonth,
  recurrenceLabel,
  type IsoDate,
  type Recurrence,
} from '@/entities/payment';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import type { PaymentDraftType } from './use-payment-wizard-draft';

/**
 * Тексты экрана успеха визарда (#464, Figma 835:19893). Заголовок — «Вы
 * создали платеж/автоплатеж «Название»» (копирайт-правки #449 применены);
 * описание показывает дату первого вхождения (превью клиентским портом)
 * и продолжение расписания канонической меткой периодичности.
 */

export type SuccessScreenCopyInput = {
  readonly draftType: PaymentDraftType;
  readonly title: string;
  readonly amountKopecks: number;
  readonly recurrence: Recurrence;
  /** null — вхождений не будет (окончание раньше даты заведения). */
  readonly firstOccurrence: IsoDate | null;
};

export type SuccessScreenCopy = {
  readonly heading: string;
  readonly description: string;
};

/** «Каждый…» после «далее» читается со строчной. */
function lowerFirst(text: string): string {
  return text.charAt(0).toLowerCase() + text.slice(1);
}

export function successScreenCopy(input: SuccessScreenCopyInput): SuccessScreenCopy {
  const entity = input.draftType === 'autopayment' ? 'автоплатеж' : 'платеж';
  const lead = input.draftType === 'autopayment' ? 'Платеж пополнится сам' : 'Первый платеж';
  const tail = lowerFirst(recurrenceLabel(input.recurrence));

  return {
    heading: `Вы создали ${entity}\n«${input.title}»`,
    description:
      input.firstOccurrence === null
        ? `Платеж сохранен; первое вхождение появится после начала его действия`
        : `${lead} ${formatDayMonth(input.firstOccurrence)} на ${formatMoneyKopecks(
            input.amountKopecks,
          )}, далее ${tail}`,
  };
}
