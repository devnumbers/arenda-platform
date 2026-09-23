import type { JSX } from 'react';
import { Skeleton } from '@/shared/ui/design';

/**
 * Состояния ленты «Истории действий» (#709): скелет с паритетом финального
 * лейаута (макет 2157-56876) — плашка дня, шапка объекта, серые карточки
 * актёров со строками (§7 канона скелетонов). Пустое состояние — на самом
 * экране (макет 2050-158499: только серая строка «Действий не было», без
 * иллюстрации).
 */

const ROW_WIDTHS = ['w-11/12', 'w-2/3', 'w-3/4'] as const;

function HistoryFeedSkeletonActorCard({ rows }: { readonly rows: number }): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col gap-2 rounded-m bg-surface-muted p-3">
      <div className="flex items-center gap-2">
        <Skeleton className="h-6 w-6 shrink-0 rounded-pill bg-white" />
        <Skeleton className="h-[15px] w-28" />
      </div>
      {ROW_WIDTHS.slice(0, rows).map((width, index) => (
        <div key={index} className="flex items-end gap-2">
          <Skeleton className="h-4 w-[3px] shrink-0 self-stretch rounded-pill" />
          <Skeleton className="h-4 w-4 shrink-0 rounded-pill" />
          <Skeleton className={`h-[15px] ${width}`} />
          <Skeleton className="ml-auto h-[15px] w-10 shrink-0" />
        </div>
      ))}
    </div>
  );
}

function HistoryFeedSkeletonGroup(): JSX.Element {
  return (
    <div aria-hidden>
      <div className="mb-3 flex justify-center">
        <Skeleton className="h-[31px] w-28 rounded-pill" />
      </div>
      <div className="flex flex-col gap-3">
        <div className="flex items-center gap-2">
          <Skeleton className="h-6 w-6 shrink-0 rounded-pill" />
          <div className="min-w-0">
            <Skeleton className="h-[15px] w-36" />
            <Skeleton className="mt-1 h-[15px] w-24" />
          </div>
        </div>
        <div className="flex flex-col gap-1.5">
          <HistoryFeedSkeletonActorCard rows={3} />
          <HistoryFeedSkeletonActorCard rows={1} />
        </div>
      </div>
    </div>
  );
}

/** Скелет ленты: две группы (плашка + шапка объекта + карточки актёров). */
export function HistoryFeedSkeleton(): JSX.Element {
  return (
    <div aria-hidden className="pt-2">
      <HistoryFeedSkeletonGroup />
      <HistoryFeedSkeletonGroup />
    </div>
  );
}

/**
 * Скелет шита «Настройки» (#711): паритет финального лейаута (макет
 * 2177-60527 — чип периода, свёрнутые карточки групп, «Сбросить
 * фильтры»); рендерится, пока опции /history/filters в пути. Групп — 4 в
 * общей ленте и 3 на «Действиях участника» (#712: группа «Участники»
 * там не рендерится).
 */
export function HistoryFiltersSheetSkeleton({ groups = 4 }: { readonly groups?: number } = {}): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col gap-4">
      <Skeleton className="h-11 w-40 rounded-pill" />
      {Array.from({ length: groups }, (_, index) => (
        <div key={index} className="rounded-3xl bg-surface-muted py-1">
          <div className="flex min-h-14 items-center gap-2 py-4 pl-5 pr-5">
            <Skeleton className="h-6 w-6 shrink-0 rounded-lg" />
            <Skeleton className="h-[18px] w-36" />
            <Skeleton className="ml-auto h-[15px] w-10" />
          </div>
        </div>
      ))}
      <Skeleton className="h-14 w-full rounded-pill" />
    </div>
  );
}
