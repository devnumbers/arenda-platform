'use client';

import type { JSX } from 'react';
import { ChangeVertical } from '@/shared/assets/icons';
import type { PaymentForm, PaymentType } from '@/entities/payment';
import { AmountField, ChipButton } from '@/shared/ui/design';
import {
  kopecksToAmountInputString,
  parseRublesToKopecks,
} from '@/shared/lib/format-money';
import {
  effectivePaymentForm,
  effectivePaymentType,
  FORM_OF_PAYMENT_LABELS,
  togglePaymentForm,
  togglePaymentType,
  TYPE_LABELS,
} from '@/features/payments';
import { WizardHeading } from './wizard-chrome';

/**
 * Шаг 5 визарда — сумма и признаки (Figma 834:19662/835:19795): крупное
 * поле суммы (numpad заменён вводом с цифровой клавиатуры — правка
 * владельца в тикете дизайн-слоя) и два чипа-переключателя в один ряд —
 * форма оплаты и тип. Чип показывает текущее значение с иконкой смены;
 * клик меняет его на альтернативное, видимого выбранного состояния нет.
 * До явного выбора показываются «Перевод» и «Доход». Деньги считаются
 * только через форматтеры копеек.
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

export function AmountStep({
  amountKopecks,
  onAmountChange,
  type,
  onTypeChange,
  paymentForm,
  onPaymentFormChange,
}: AmountStepProps): JSX.Element {
  const rawValue =
    amountKopecks === undefined ? '' : kopecksToAmountInputString(amountKopecks);
  const currentType = effectivePaymentType(type);
  const currentForm = effectivePaymentForm(paymentForm);

  return (
    <>
      <WizardHeading title="Сумма платежа" />
      <div className="flex flex-col items-center gap-12 px-6 pt-10">
        <AmountField
          value={rawValue}
          onChange={(next) => onAmountChange(parseRublesToKopecks(next, { positive: true }))}
          label="Сумма"
        />
        {/* Чипы в порядке макета: форма оплаты слева, тип справа; серый фон
            без выбранного состояния — значение написано на самом чипе. */}
        <div className="flex justify-center gap-2">
          <ChipButton
            aria-label={`Форма оплаты: ${FORM_OF_PAYMENT_LABELS[currentForm]}, нажмите, чтобы сменить`}
            onClick={() => onPaymentFormChange(togglePaymentForm(currentForm))}
            trailingIcon={<ChangeVertical />}
          >
            {FORM_OF_PAYMENT_LABELS[currentForm]}
          </ChipButton>
          <ChipButton
            aria-label={`Тип платежа: ${TYPE_LABELS[currentType]}, нажмите, чтобы сменить`}
            onClick={() => onTypeChange(togglePaymentType(currentType))}
            trailingIcon={<ChangeVertical />}
          >
            {TYPE_LABELS[currentType]}
          </ChipButton>
        </div>
      </div>
    </>
  );
}
