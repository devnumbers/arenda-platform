import {
  formatDayMonth,
  recurrenceLabel,
  type IsoDate,
  type Recurrence,
} from '@/entities/payment';
import type { PaymentDraftType } from './use-payment-wizard-draft';

/**
 * Тексты экрана успеха визарда (Figma 835:19893; структура — канон успеха
 * операций 1858:105544, решение владельца 01.10). Заголовок — «Вы создали
 * платеж/автоплатеж «Название»»; название показывается, только если
 * пользователь его ввёл (правка владельца 2026-08-31: подставленный лейбл
 * категории на экране не пишется — платёж при этом сохраняется с ним же).
 * Описание показывает дату первого вхождения (превью клиентским портом)
 * и продолжение расписания канонической меткой периодичности; при не
 * определённом вхождении абзаца нет вовсе. Сумма в текстах не живёт — на
 * экране она большой блок со знаком (рисует компонент).
 */

export type SuccessScreenCopyInput = {
  readonly draftType: PaymentDraftType;
  /** Название, введённое пользователем; undefined — поле оставили пустым. */
  readonly typedTitle?: string;
  readonly recurrence: Recurrence;
  /** null — вхождений не будет (окончание раньше даты заведения). */
  readonly firstOccurrence: IsoDate | null;
};

export type SuccessScreenCopy = {
  readonly heading: string;
  /** undefined — абзац описания не рисуется. */
  readonly description?: string;
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
    heading:
      input.typedTitle === undefined
        ? `Вы создали ${entity}`
        : `Вы создали ${entity}\n«${input.typedTitle}»`,
    description:
      input.firstOccurrence === null
        ? undefined
        : `${lead} ${formatDayMonth(input.firstOccurrence)}, далее ${tail}`,
  };
}
