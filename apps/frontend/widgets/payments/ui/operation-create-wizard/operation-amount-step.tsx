'use client';

import type { JSX } from 'react';
import type { PaymentType } from '@/entities/payment';
import { WizardAmountField, WizardDirectionSegment } from '../payment-create-wizard/wizard-chrome';

/**
 * Шаг 1 визарда операции — сумма и направление (Figma 1858:104557/105397
 * мобилка, 2913:69551 широкий): денежное поле двумя ярусами (канон
 * WizardAmountField — <1024 дисплей «0 ₽», ≥1024 бокс «Сумма», карта
 * #1005) и сегмент «Расход/Доход» (общий WizardDirectionSegment) — 232px
 * на дисплейном ярусе, во всю колонку на ПК. Направление выбирается явно
 * подсветкой сегмента; стартовое значение задаёт пресет точки входа (слой
 * экрана), до явного выбора он и подсвечен. Деньги считаются только через
 * форматтеры копеек.
 */

export type OperationAmountStepProps = {
  /** Копейки; undefined — ещё не задана. */
  readonly amountKopecks: number | undefined;
  readonly onAmountChange: (amountKopecks: number | undefined) => void;
  /** Направление, видимое на сегменте (явный выбор или пресет входа). */
  readonly type: PaymentType;
  readonly onTypeChange: (type: PaymentType) => void;
};

export function OperationAmountStep({
  amountKopecks,
  onAmountChange,
  type,
  onTypeChange,
}: OperationAmountStepProps): JSX.Element {
  return (
    // Дисплейный ярус — py-64/gap-32 (1858:105402), ПК — pt-24/gap-24
    // (2913:69553).
    <div className="flex flex-col items-center gap-8 px-6 pt-16 pb-16 desktop:gap-6 desktop:pt-6">
      <WizardAmountField label="Сумма" kopecks={amountKopecks} onKopecksChange={onAmountChange} />
      <WizardDirectionSegment
        type={type}
        onTypeChange={onTypeChange}
        ariaLabel="Направление операции"
      />
    </div>
  );
}
