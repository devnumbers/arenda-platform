'use client';

import type { JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { Skeleton } from './skeleton';

/**
 * Скелетон поля формы (#607, паритет — §7 DESIGN.md): каркас поля ввода
 * канона (TextField, MoneyField, PickerTriggerBox, FieldButton) — метка
 * 16/18 и серый бокс h-14 rounded-button с зазором 8. Тон блоков базовый:
 * бокс сам `bg-surface-muted`, как реальное поле, — при подмене контент
 * занимает место скелетона без сдвига. Ширина метки — `labelWidth`.
 */
export type SkeletonFormFieldProps = {
  /** Ширина плейсхолдера метки (Tailwind-класс ширины). */
  readonly labelWidth?: string;
  readonly className?: string;
};

export function SkeletonFormField({
  labelWidth = 'w-24',
  className,
}: SkeletonFormFieldProps): JSX.Element {
  return (
    <span className={cn('flex flex-col gap-2', className)}>
      <Skeleton className={cn('h-[18px]', labelWidth)} />
      <Skeleton className="h-14 w-full rounded-button" />
    </span>
  );
}
