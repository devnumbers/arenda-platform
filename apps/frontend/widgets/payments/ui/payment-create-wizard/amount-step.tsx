'use client';

import type { JSX } from 'react';
import type { PaymentForm, PaymentType } from '@/entities/payment';
import { AmountField, ChipButton } from '@/shared/ui/design';
import {
  kopecksToRublesString,
  parseRublesToKopecks,
} from '@/shared/lib/format-money';
import { WizardHeading } from './wizard-chrome';

/**
 * Шаг 5 визарда — сумма и признаки (Figma 834:19662/835:19795): крупное
 * поле суммы (numpad заменён вводом с цифровой клавиатуры — правка
 * владельца в тикете дизайн-слоя) и чипы «Доход/Расход» + «Перевод/
 * Наличные». Деньги считаются только через форматтеры копеек.
 */

export type AmountStepProps = {
  /** Копейки; undefined — ещё не задана. */
  readonly amountKopecks: number | undefined;
  readonly onAmountChange: (amountKopecks: number | undefined) => void;
  readonly type: PaymentType | undefined;
  readonly onTypeChange: (type: PaymentType) => void;
  readonly paymentForm: PaymentForm | undefined;
  readonly onPaymentFormChange: (paymentForm: PaymentForm) => void;
};

const TYPE_CHIPS = [
  { value: 'income' as const, label: 'Доход' },
  { value: 'expense' as const, label: 'Расход' },
];

const FORM_CHIPS = [
  { value: 'transfer' as const, label: 'Перевод' },
  { value: 'cash' as const, label: 'Наличные' },
];

export function AmountStep({
  amountKopecks,
  onAmountChange,
  type,
  onTypeChange,
  paymentForm,
  onPaymentFormChange,
}: AmountStepProps): JSX.Element {
  const rawValue =
    amountKopecks === undefined ? '' : kopecksToRublesString(amountKopecks).replace('.', ',');

  return (
    <>
      <WizardHeading title="Сумма платежа" />
      <div className="flex flex-col items-center gap-12 px-6 pt-10">
        <AmountField
          value={rawValue}
          onChange={(next) => onAmountChange(parseRublesToKopecks(next, { positive: true }))}
          label="Сумма"
        />
        <div className="flex w-full flex-col items-center gap-2">
          <div className="flex justify-center gap-2">
            {TYPE_CHIPS.map((chip) => (
              <ChipButton
                key={chip.value}
                selected={type === chip.value}
                onClick={() => onTypeChange(chip.value)}
              >
                {chip.label}
              </ChipButton>
            ))}
          </div>
          <div className="flex justify-center gap-2">
            {FORM_CHIPS.map((chip) => (
              <ChipButton
                key={chip.value}
                selected={paymentForm === chip.value}
                onClick={() => onPaymentFormChange(chip.value)}
              >
                {chip.label}
              </ChipButton>
            ))}
          </div>
        </div>
      </div>
    </>
  );
}
