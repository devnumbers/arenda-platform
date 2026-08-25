import type { JSX } from 'react';
import { cn } from '@/shared/lib/cn';

/** Чип шага визарда (Figma 833:17498 и вариант в TopNav 934:19657):
 * пилюля на светло-голубом #EFF6FF с синим текстом «шаг N из M».
 * size="m" — начертание TopNav (14/16), базовое — 13/15. */
export type StepsChipSize = 's' | 'm';

export type StepsChipProps = {
  readonly step: number;
  readonly total: number;
  readonly size?: StepsChipSize;
  readonly className?: string;
};

export function StepsChip({ step, total, size = 's', className }: StepsChipProps): JSX.Element {
  return (
    <span
      className={cn(
        'inline-flex h-7 items-center justify-center rounded-pill bg-surface-info px-3 font-sans text-primary',
        size === 's' ? 'text-xs' : 'text-sm',
        className,
      )}
    >
      {`шаг ${step} из ${total}`}
    </span>
  );
}
