import type { JSX } from 'react';
import { cn } from '@/shared/lib/cn';

/** Чип шага визарда (Figma 833:17498 и вариант в TopNav 934:19657):
 * пилюля на светло-голубом #EFF6FF с синим текстом «шаг N из M».
 * size="m" — начертание TopNav (14/16), базовое — 13/15.
 * Ширина фиксированная (решение 2026-08-26, замер на Onest): смена цифры
 * шага не должна растягивать чип и дёргать соседний заголовок. s=90px
 * (одноцифровые шаги занимают 85–88.5px), m=96px (89.5–93.4px); tabular-nums
 * выравнивает ширину самих цифр. Рассчитано на шаги ≤ 9 — двузначных
 * визардов в продукте нет. */
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
        'inline-flex h-7 items-center justify-center whitespace-nowrap rounded-pill bg-surface-info font-sans tabular-nums text-primary',
        size === 's' ? 'w-[90px] text-xs' : 'w-24 text-sm',
        className,
      )}
    >
      {`шаг ${step} из ${total}`}
    </span>
  );
}
