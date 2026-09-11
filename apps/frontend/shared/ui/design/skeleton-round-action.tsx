'use client';

import type { JSX } from 'react';
import { Skeleton } from './skeleton';

/**
 * Скелетон круглой кнопки действия (#606): плейсхолдер RoundActionButton —
 * круг 56 и подпись 13/15 с зазором 8. Тон базовый: круг secondary сам
 * серый (`bg-surface-muted`), у primary заглушка честно нейтральна.
 */
export type SkeletonRoundActionProps = {
  readonly className?: string;
};

export function SkeletonRoundAction({ className }: SkeletonRoundActionProps): JSX.Element {
  return (
    <span aria-hidden className={`flex flex-col items-center gap-2 ${className ?? ''}`}>
      <Skeleton className="h-14 w-14 rounded-pill" />
      <Skeleton className="h-[15px] w-12" />
    </span>
  );
}
