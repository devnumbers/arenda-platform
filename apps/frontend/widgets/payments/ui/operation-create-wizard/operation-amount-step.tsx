'use client';

import type { JSX } from 'react';
import type { PaymentType } from '@/entities/payment';
import { AmountField } from '@/shared/ui/design';
import { kopecksToAmountInputString, parseRublesToKopecks } from '@/shared/lib/format-money';
import { TYPE_LABELS } from '@/features/payments';

/**
 * Шаг 1 визарда операции — сумма и направление (Figma 1858:104557/105397):
 * крупная сумма (канон AmountField, клавиатурный ввод вместо numpad —
 * правка владельца в тикете дизайн-слоя) и сегмент «Расход/Доход».
 * Направление выбирается явно подсветкой сегмента; стартовое значение
 * задаёт пресет точки входа (слой экрана), до явного выбора он и
 * подсвечен. Деньги считаются только через форматтеры копеек.
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
  const rawValue =
    amountKopecks === undefined ? '' : kopecksToAmountInputString(amountKopecks);

  return (
    <div className="flex flex-col items-center gap-8 px-6 pt-16 pb-16">
      <AmountField
        value={rawValue}
        onChange={(next) => onAmountChange(parseRublesToKopecks(next, { positive: true }))}
        label="Сумма"
      />
      {/* Сегмент «Расход/Доход» (Figma 1858:104562): серый контейнер
          232px, выбранный сегмент — белая пилюля с тенью. */}
      <div
        role="radiogroup"
        aria-label="Направление операции"
        className="flex w-full max-w-[232px] rounded-2xl bg-surface-muted p-[2px]"
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
