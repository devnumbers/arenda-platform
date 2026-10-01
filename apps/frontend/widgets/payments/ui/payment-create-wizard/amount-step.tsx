'use client';

import type { JSX } from 'react';
import type { PaymentType } from '@/entities/payment';
import { effectivePaymentType } from '@/features/payments';
import { WizardAmountField, WizardDirectionSegment } from './wizard-chrome';

/**
 * Шаг 5 визарда — сумма и тип (реворк карты #1005, #1008 по макетам шага
 * суммы операции 1858:104557/105397 мобилка, 2913:69551 широкий; один в
 * один с операционным — дополнение карты 01.10): без своего заголовка,
 * денежное поле двумя ярусами (канон WizardAmountField — <1024 дисплей
 * «0 ₽», ≥1024 бокс «Сумма») и общий сегмент «Расход/Доход». До явного
 * выбора сегмент подсвечивает эффективный «Доход», клик по пилюле —
 * явный выбор (в черновик пишется только явный). Деньги считаются только
 * через форматтеры копеек.
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
  return (
    // Ярусы шага суммы операции: дисплейный — py-64/gap-32, ПК — pt-24/
    // gap-24 (1858:105402, 2913:69553).
    <div className="flex flex-col items-center gap-8 px-6 pt-16 pb-16 desktop:gap-6 desktop:pt-6">
      <WizardAmountField
        label="Сумма"
        kopecks={amountKopecks}
        onKopecksChange={onAmountChange}
      />
      <WizardDirectionSegment
        type={effectivePaymentType(type)}
        onTypeChange={onTypeChange}
        ariaLabel="Направление платежа"
      />
    </div>
  );
}
