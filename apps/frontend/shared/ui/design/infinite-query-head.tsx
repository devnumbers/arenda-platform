'use client';

import type { JSX } from 'react';
import { useInfiniteScroll } from '@/shared/lib/hooks/useInfiniteScroll';
import { LoadingMoreIndicator } from './loading-more-indicator';
import type { SkeletonTone } from './skeleton-parts';

/**
 * Голова бесконечной ленты — зеркало InfiniteQueryTail для мессенджерских
 * списков «новые снизу» (лента «Истории действий», #709): sentinel
 * дозагрузки стоит НАД первой строкой, прокрутка вверх запрашивает
 * предыдущую порцию (у двустороннего keyset #708 — сторону before_cursor),
 * индикатор показывается во время езды. Правило дозагрузки и sentinel
 * при тёплом кэше — те же, что у хвоста (резолюция #452, #631).
 */
export type InfiniteQueryHeadQuery = {
  readonly hasNextPage: boolean | undefined;
  readonly isFetchingNextPage: boolean;
  readonly fetchNextPage: () => Promise<unknown>;
};

export function InfiniteQueryHead({
  query,
  tone = 'base',
  className,
}: {
  readonly query: InfiniteQueryHeadQuery;
  readonly tone?: SkeletonTone;
  readonly className?: string;
}): JSX.Element | null {
  const hasNext = query.hasNextPage === true;
  const sentinelRef = useInfiniteScroll(
    () => {
      if (hasNext && !query.isFetchingNextPage) {
        void query.fetchNextPage();
      }
    },
    hasNext,
    query.isFetchingNextPage,
  );

  if (!hasNext) {
    return null;
  }
  return (
    <>
      <div ref={sentinelRef} aria-hidden />
      {query.isFetchingNextPage && <LoadingMoreIndicator tone={tone} className={className} />}
    </>
  );
}
