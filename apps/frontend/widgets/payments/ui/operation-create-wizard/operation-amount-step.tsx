'use client';

import type { JSX } from 'react';
import type { PaymentType } from '@/entities/payment';
import { WizardAmountField, WizardDirectionSegment } from '../payment-create-wizard/wizard-chrome';

/**
 * Шаг 1 визарда операции — сумма и направление (макет 2913:69551,
 * решение владельца 2026-10-22 — единый ярус вместо двухъярусной карты
 * #1005): денежное поле (канон WizardAmountField — бокс «Сумма» на всех
 * ширинах; на ПК ≥1024 активно при маунте) и сегмент «Расход/Доход»
 * (общий WizardDirectionSegment, во всю ширину). Направление выбирается
 * явно подсветкой сегмента; стартовое значение задаёт пресет точки входа
 * (слой экрана), до явного выбора он и подсвечен. Деньги считаются только
 * через форматтеры копеек.
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
    // Отступы шага едины на всех ширинах (макет 2913:69553): pt-24/px-24/
    // gap-24; pb-16 — воздух над StickyBottomBar.
    <div className="flex flex-col items-center gap-6 px-6 pt-6 pb-16">
      <WizardAmountField label="Сумма" kopecks={amountKopecks} onKopecksChange={onAmountChange} />
      <WizardDirectionSegment
        type={type}
        onTypeChange={onTypeChange}
        ariaLabel="Направление операции"
      />
    </div>
  );
}
