'use client';

import type { JSX } from 'react';
import { ChangeVertical } from '@/shared/assets/icons';
import type { PaymentType } from '@/entities/payment';
import { ChipButton } from '@/shared/ui/design';
import { effectivePaymentType, togglePaymentType, TYPE_LABELS } from '@/features/payments';
import { WizardAmountField, WizardHeading } from './wizard-chrome';

/**
 * Шаг 5 визарда — сумма и тип (реворк карты #1005, #1008 по макетам шага
 * суммы операции 1858:104557/105397 мобилка, 2913:69551 широкий): денежное
 * поле двумя ярусами (канон WizardAmountField — <768 дисплей «0 ₽», ≥768
 * бокс «Сумма») и один чип-переключатель типа. Чип показывает текущее
 * значение с иконкой смены; клик меняет его на альтернативное, видимого
 * выбранного состояния нет. До явного выбора показывается «Доход».
 * Деньги считаются только через форматтеры копеек.
 */

export type AmountStepProps = {
  /** Копейки; undefined — ещё не задана. */
  readonly amountKopecks: number | undefined;
  readonly onAmountChange: (amountKopecks: number | undefined) => void;
  readonly type: PaymentType | undefined;
  readonly onTypeChange: (type: PaymentType) => void;
};

export function AmountStep({
  amountKopecks,
  onAmountChange,
  type,
  onTypeChange,
}: AmountStepProps): JSX.Element {
  const currentType = effectivePaymentType(type);

  return (
    <>
      <WizardHeading title="Сумма платежа" />
      {/* Ярусы шага суммы операции: мобилка py-64/gap-32, широкий —
          pt-24/gap-24 (1858:105402, 2913:69553). */}
      <div className="flex flex-col items-center gap-8 px-6 pt-16 pb-16 md:gap-6 md:pt-6">
        <WizardAmountField
          label="Сумма"
          kopecks={amountKopecks}
          onKopecksChange={onAmountChange}
        />
        {/* Серый фон без выбранного состояния — значение написано на самом
            чипе. */}
        <ChipButton
          aria-label={`Тип платежа: ${TYPE_LABELS[currentType]}, нажмите, чтобы сменить`}
          onClick={() => onTypeChange(togglePaymentType(currentType))}
          trailingIcon={<ChangeVertical />}
        >
          {TYPE_LABELS[currentType]}
        </ChipButton>
      </div>
    </>
  );
}
