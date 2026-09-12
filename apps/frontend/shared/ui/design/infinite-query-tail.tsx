'use client';

import type { JSX } from 'react';
import { useInfiniteScroll } from '@/shared/lib/hooks/useInfiniteScroll';
import { LoadingMoreIndicator } from './loading-more-indicator';
import type { SkeletonTone } from './skeleton-parts';

/**
 * Срез бесконечного react-query-запроса, которого хвосту достаточно —
 * структурно, чтобы витрина могла подставить заглушку без сети.
 */
export type InfiniteQueryTailQuery = {
  readonly hasNextPage: boolean | undefined;
  readonly isFetchingNextPage: boolean;
  readonly fetchNextPage: () => Promise<unknown>;
};

/**
 * Хвост бесконечной ленты (консолидация #633): sentinel дозагрузки и
 * индикатор едущей следующей порции — одна строка на экран вместо копии
 * обвязки `useInfiniteScroll` + условного sentinel + индикатора в каждом
 * списке. Правило дозагрузки — общее (резолюция #452: по 50 + бесконечный
 * скролл): порция запрашивается, пока есть продолжение и предыдущая ещё
 * едет; sentinel жив при тёплом кэше (#631) — пересоздание обсервера после
 * каждой порции (`resetKey`) продолжает дозагрузку, пока sentinel во
 * вьюпорте, независимо от того, откуда пришла первая порция. Проекционное
 * окно без react-query (график платежей) остаётся на голом
 * `useInfiniteScroll`.
 */
export function InfiniteQueryTail({
  query,
  tone = 'base',
  className,
}: {
  readonly query: InfiniteQueryTailQuery;
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
