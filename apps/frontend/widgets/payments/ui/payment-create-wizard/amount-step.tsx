'use client';

import type { JSX } from 'react';
import type { PaymentType } from '@/entities/payment';
import { effectivePaymentType } from '@/features/payments';
import { WizardAmountField, WizardDirectionSegment } from './wizard-chrome';

/**
 * Шаг 5 визарда — сумма и тип (один в один с операционным; макет
 * 2913:69551, решение владельца 2026-10-22 — единый ярус вместо
 * двухъярусной карты #1005): без своего заголовка, денежное поле (канон
 * WizardAmountField — бокс «Сумма» на всех ширинах) и общий сегмент
 * «Расход/Доход». До явного выбора сегмент подсвечивает эффективный
 * «Доход», клик по пилюле — явный выбор (в черновик пишется только
 * явный). Деньги считаются только через форматтеры копеек.
 *
 * Шаг появляется кликом «Далее» шага 4 — маунт в задаче жеста, поэтому
 * флаг focusOnMount (решение владельца 2026-10-06: автоподъём клавиатуры
 * обязателен, #1151; механика — research #1148 §A, аменд канона autoFocus
 * в CODING_STANDARDS.md) поднимает клавиатуру сама. Маунты вне жеста
 * (возобновление черновика сразу на шаге 5, прямой роут) флаг не получают:
 * iOS клавиатуру вне жеста не поднимает; на ПК поле активно при любом
 * маунте (решение владельца 2026-10-22).
 */

export type AmountStepProps = {
  /** Копейки; undefined — ещё не задана. */
  readonly amountKopecks: number | undefined;
  readonly onAmountChange: (amountKopecks: number | undefined) => void;
  readonly type: PaymentType | undefined;
  readonly onTypeChange: (type: PaymentType) => void;
  /** Фокус суммы при маунте шага — только для маунтов внутри жеста (#1151). */
  readonly focusOnMount?: boolean;
};

export function AmountStep({
  amountKopecks,
  onAmountChange,
  type,
  onTypeChange,
  focusOnMount,
}: AmountStepProps): JSX.Element {
  return (
    // Отступы шага едины на всех ширинах (макет 2913:69553): pt-24/px-24/
    // gap-24; pb-16 — воздух над StickyBottomBar.
    <div className="flex flex-col items-center gap-6 px-6 pt-6 pb-16">
      <WizardAmountField
        label="Сумма"
        kopecks={amountKopecks}
        onKopecksChange={onAmountChange}
        focusOnMount={focusOnMount}
      />
      <WizardDirectionSegment
        type={effectivePaymentType(type)}
        onTypeChange={onTypeChange}
        ariaLabel="Направление платежа"
      />
    </div>
  );
}
