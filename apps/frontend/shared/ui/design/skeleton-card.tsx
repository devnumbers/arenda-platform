'use client';

import type { JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { Skeleton } from './skeleton';
import { SNAP_TILE_CLASSES } from '@/shared/ui/snap-rail';
import { skeletonBlockClass } from './skeleton-parts';

/**
 * Скелетон карточки-плитки (#604) — плейсхолдер серой плитки PaymentCardButton
 * (Figma 705:10625): круг иконки 44, заголовок в две строки (line-clamp-2,
 * 2×16) и подпись. Ширина плитки fluid — как у карточки ленты главного
 * экрана «Платежи» (2 на мобайл-ярусе, 3 на планшете/ПК, решение владельца
 * 18.10); другим размером управляет потребитель через className.
 * Блоки приглушённые (`bg-surface-muted-hover`) — плитка сама серая. Имена
 * API финализирован на хабах карты #603 (#605).
 */
export type SkeletonCardProps = {
  readonly className?: string;
};

export function SkeletonCard({ className }: SkeletonCardProps): JSX.Element {
  const block = skeletonBlockClass('muted');
  return (
    <div
      aria-hidden
      className={cn('flex shrink-0 flex-col rounded-card bg-surface-muted p-4', SNAP_TILE_CLASSES, className)}
    >
      <Skeleton className={cn('mb-3 h-11 w-11', block)} />
      <span className="flex flex-col gap-1">
        <Skeleton className={cn('h-8 w-full', block)} />
        <Skeleton className={cn('h-4 w-3/5', block)} />
      </span>
    </div>
  );
}
