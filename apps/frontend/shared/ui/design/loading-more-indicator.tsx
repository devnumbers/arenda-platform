'use client';

import type { JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { Skeleton } from './skeleton';
import { skeletonBlockClass, type SkeletonTone } from './skeleton-parts';

/**
 * Индикатор догрузки порции («Загружаем еще»): компактный скелетон-спиннер
 * по центру с ролью `status` — единственная реализация хвоста бесконечных
 * лент (консолидация #633; до этого — шесть копий по экранам платежей,
 * операций, аренды, контактов и поиска объектов). Тон скелетона: `base` —
 * на белой поверхности, `muted` — внутри серой карточки (канон
 * `Skeleton`).
 */
export function LoadingMoreIndicator({
  tone = 'base',
  className,
}: {
  readonly tone?: SkeletonTone;
  readonly className?: string;
}): JSX.Element {
  return (
    <div
      className={cn('flex justify-center py-4', className)}
      role="status"
      aria-label="Загружаем еще"
    >
      <Skeleton className={cn('h-8 w-8', skeletonBlockClass(tone))} />
    </div>
  );
}
