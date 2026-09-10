'use client';

import type { JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { Skeleton } from './skeleton';
import { skeletonBlockClass, skeletonRowWidths } from './skeleton-parts';
import { SkeletonListRow } from './skeleton-list-row';

/**
 * Скелетон серой секции с заголовком (#604) — плейсхолдер серой
 * карточки-секции (PaymentsGroup/PaymentsSection, CollapsibleSection):
 * бар заголовка 24 (20/24 SemiBold реальной секции) и строки
 * `SkeletonListRow` в приглушённом тоне. Каркас повторяет секцию: карточка
 * `rounded-card bg-surface-muted`, заголовок с вставкой 24, снизу баланс
 * 24; горизонтальные поля экрана (mx-6) приносит потребитель через
 * className — как у CollapsibleSection. Имена и API окончательно решаются
 * на первом реальном экране карты #603 (тикет #605).
 */
export type SkeletonSectionProps = {
  /** Число строк-заглушек внутри секции. */
  readonly rows?: number;
  readonly className?: string;
};

export function SkeletonSection({ rows = 2, className }: SkeletonSectionProps): JSX.Element {
  const widths = skeletonRowWidths(rows);
  return (
    <section aria-hidden className={cn('rounded-card bg-surface-muted pb-6', className)}>
      <div className="px-6 pb-3 pt-6">
        <Skeleton className={`h-6 w-40 ${skeletonBlockClass('muted')}`} />
      </div>
      <div>
        {widths.map((rowWidths, index) => (
          <SkeletonListRow key={index} tone="muted" widths={rowWidths} />
        ))}
      </div>
    </section>
  );
}
