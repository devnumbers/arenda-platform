'use client';

import type { JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { Skeleton } from './skeleton';

/**
 * Скелетон строки CTA-кнопки (#604): плейсхолдер главной кнопки экрана
 * (Button large — h-14, rounded-button) во всю ширину; в StickyBottomBar и
 * формах потребитель совпадает с реальной кнопкой по высоте и положению.
 * Имена и API окончательно решаются на первом реальном экране карты #603
 * (тикет #605).
 */
export type SkeletonButtonProps = {
  readonly className?: string;
};

export function SkeletonButton({ className }: SkeletonButtonProps): JSX.Element {
  return <Skeleton className={cn('h-14 w-full rounded-button', className)} />;
}
