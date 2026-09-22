import type { JSX } from 'react';
import { Skeleton } from '@/shared/ui/design';

/**
 * Состояния ленты «Истории действий» (#709): скелет с паритетом финального
 * лейаута — чип дня, шапка объекта, строки (§7 канона скелетонов). Пустое
 * состояние — на самом экране (макет 2050-158499: только серая строка
 * «Действий не было», без иллюстрации).
 */

const ROW_WIDTHS = ['w-11/12', 'w-2/3', 'w-3/4'] as const;

function HistoryFeedSkeletonGroup({ rows }: { readonly rows: number }): JSX.Element {
  return (
    <div aria-hidden>
      <div className="flex justify-center py-2.5">
        <Skeleton className="h-6 w-24 rounded-pill" />
      </div>
      <div className="flex items-center gap-3 px-6 pb-1 pt-4">
        <Skeleton className="h-7 w-7 rounded-pill" />
        <Skeleton className="h-[18px] w-36" />
      </div>
      <div className="pl-6 pr-6">
        {ROW_WIDTHS.slice(0, rows).map((width, index) => (
          <div key={index} className="flex items-start gap-3 py-[5px]">
            <Skeleton className="mt-0.5 h-6 w-6 shrink-0 rounded-pill" />
            <Skeleton className={`h-5 ${width}`} />
            <Skeleton className="ml-auto h-4 w-10 shrink-0" />
          </div>
        ))}
      </div>
    </div>
  );
}

/** Скелет ленты: две группы (шапка объекта + 3 строки; + 2 строки). */
export function HistoryFeedSkeleton(): JSX.Element {
  return (
    <div aria-hidden>
      <HistoryFeedSkeletonGroup rows={3} />
      <HistoryFeedSkeletonGroup rows={2} />
    </div>
  );
}
