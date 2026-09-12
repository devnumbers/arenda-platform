'use client';

import type { JSX } from 'react';
import { Skeleton } from '@/shared/ui/design';

/**
 * Скелетон списка-хаба «Объектов» (канон Skeleton, DESIGN.md §7): серые
 * карточки-плейсхолдеры новой анатомии — круг 64 и пара строк.
 */
function PropertyCardSkeleton(): JSX.Element {
  return (
    <div className="flex flex-col gap-6 rounded-card bg-surface-muted p-6" aria-hidden>
      <Skeleton className="h-16 w-16 rounded-full" />
      <div className="flex flex-col gap-2.5">
        <Skeleton className="h-5 w-3/5" />
        <Skeleton className="h-4 w-2/5" />
      </div>
    </div>
  );
}

export function PropertiesLoading(): JSX.Element {
  return (
    <div className="flex flex-col gap-4" data-testid="properties-loading">
      <PropertyCardSkeleton />
      <PropertyCardSkeleton />
      <PropertyCardSkeleton />
    </div>
  );
}
