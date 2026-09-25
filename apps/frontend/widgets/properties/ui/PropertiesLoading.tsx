'use client';

import type { JSX } from 'react';
import { Skeleton, skeletonBlockClass, skeletonRowWidths, type SkeletonRowWidths } from '@/shared/ui/design';

/**
 * Скелетон списка «Объектов» — хаба и архива (канон Skeleton, DESIGN.md
 * §7; паритет скелетона и контента #604): копия анатомии PropertyCard —
 * серая карточка radius 24, круг-фото 64, две строки текста; мобайл ≤560 —
 * круг над текстом (зазор 24), 561+ — круг слева вровень (зазор 16),
 * шеврон у правого верхнего края. Блоки внутри серой карточки — тон muted
 * (`bg-surface-muted-hover`), ширины строк — детерминированный цикл
 * `skeletonRowWidths`.
 */
function PropertyCardSkeleton({ widths }: { readonly widths: SkeletonRowWidths }): JSX.Element {
  const muted = skeletonBlockClass('muted');
  return (
    <div
      className="relative flex flex-col gap-6 rounded-card bg-surface-muted p-6 tablet:flex-row tablet:items-start tablet:gap-4"
      aria-hidden
    >
      <Skeleton className={`h-16 w-16 shrink-0 rounded-full ${muted}`} />
      <div className="flex min-w-0 flex-1 flex-col gap-2">
        <Skeleton className={`h-6 ${widths.title} ${muted}`} />
        <Skeleton className={`h-4 ${widths.subtitle} ${muted}`} />
      </div>
      <Skeleton className={`absolute right-6 top-6 h-6 w-6 ${muted}`} />
    </div>
  );
}

export function PropertiesLoading(): JSX.Element {
  const rows = skeletonRowWidths(3);
  return (
    <div className="flex flex-col gap-4" data-testid="properties-loading">
      {rows.map((rowWidths, index) => (
        <PropertyCardSkeleton key={index} widths={rowWidths} />
      ))}
    </div>
  );
}
