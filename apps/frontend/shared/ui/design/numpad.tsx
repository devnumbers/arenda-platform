'use client';

import type { JSX } from 'react';
import { Eraser } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import { NUMPAD_KEYS, type NumpadKey } from './numpad-input';

/** Клавиатура ввода суммы (тикет #459, Figma 834:19678): раскладка 3×4 —
 * 1..9, запятая, 0 и ластик; кегль 32/36 M/500, клавиши без заливки, как
 * в макете (Button/Text/Default), hover/active/focus-visible — конвенции
 * слойя. Сумма и её форматирование живут в numpad-input; клавиатура
 * только эмитит нажатые клавиши. */
export type NumpadProps = {
  readonly onKey: (key: NumpadKey) => void;
  readonly className?: string;
};

export function Numpad({ onKey, className }: NumpadProps): JSX.Element {
  return (
    <div role="group" aria-label="Ввод суммы" className={cn('grid grid-cols-3', className)}>
      {NUMPAD_KEYS.map((key) => (
        <button
          key={key}
          type="button"
          onClick={() => onKey(key)}
          aria-label={key === 'erase' ? 'Стереть' : key === ',' ? 'Запятая' : key}
          className={cn(
            'flex h-16 cursor-pointer items-center justify-center rounded-xl font-sans text-[2rem] font-medium leading-9 text-content outline-none transition-colors',
            'hover:bg-surface-muted active:bg-surface-muted-hover',
            'focus-visible:ring-2 focus-visible:ring-primary',
          )}
        >
          {key === 'erase' ? <Eraser className="h-8 w-8" aria-hidden /> : key}
        </button>
      ))}
    </div>
  );
}
