'use client';

import type { JSX } from 'react';
import type { PaymentType } from '@/entities/payment';
import { TYPE_LABELS } from '@/features/payments';
import { WizardAmountField } from '../payment-create-wizard/wizard-chrome';

/**
 * Шаг 1 визарда операции — сумма и направление (Figma 1858:104557/105397
 * мобилка, 2913:69551 широкий): денежное поле двумя ярусами (канон
 * WizardAmountField — <768 дисплей «0 ₽», ≥768 бокс «Сумма», карта
 * #1005) и сегмент «Расход/Доход» — 232px на мобилке, во всю колонку на
 * ≥768. Направление выбирается явно подсветкой сегмента; стартовое
 * значение задаёт пресет точки входа (слой экрана), до явного выбора он
 * и подсвечен. Деньги считаются только через форматтеры копеек.
 */

export type OperationAmountStepProps = {
  /** Копейки; undefined — ещё не задана. */
  readonly amountKopecks: number | undefined;
  readonly onAmountChange: (amountKopecks: number | undefined) => void;
  /** Направление, видимое на сегменте (явный выбор или пресет входа). */
  readonly type: PaymentType;
  readonly onTypeChange: (type: PaymentType) => void;
};

const SEGMENT_ORDER = ['expense', 'income'] as const;

export function OperationAmountStep({
  amountKopecks,
  onAmountChange,
  type,
  onTypeChange,
}: OperationAmountStepProps): JSX.Element {
  return (
    // Мобилка — py-64/gap-32 (1858:105402), широкий ярус — pt-24/gap-24
    // (2913:69553).
    <div className="flex flex-col items-center gap-8 px-6 pt-16 pb-16 md:gap-6 md:pt-6">
      <WizardAmountField label="Сумма" kopecks={amountKopecks} onKopecksChange={onAmountChange} />
      {/* Сегмент «Расход/Доход» (Figma 1858:104562): серый контейнер —
          232px на мобилке, на ≥768 — вся ширина колонки (2913:69741);
          выбранный сегмент — белая пилюля с тенью. */}
      <div
        role="radiogroup"
        aria-label="Направление операции"
        className="flex w-full max-w-[232px] rounded-2xl bg-surface-muted p-[2px] md:max-w-none"
      >
        {SEGMENT_ORDER.map((option) => {
          const selected = type === option;
          return (
            <button
              key={option}
              type="button"
              role="radio"
              aria-checked={selected}
              onClick={() => onTypeChange(option)}
              className={
                'min-h-10 flex-1 cursor-pointer rounded-[14px] text-sm font-medium leading-4 transition-colors outline-none focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface '
                + (selected
                  ? 'bg-surface text-content shadow-[0_2px_4px_rgba(0,0,0,0.16)]'
                  : 'text-content-secondary')
              }
            >
              {TYPE_LABELS[option]}
            </button>
          );
        })}
      </div>
    </div>
  );
}
